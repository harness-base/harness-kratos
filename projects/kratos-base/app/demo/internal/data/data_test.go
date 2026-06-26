package data_test

import (
	"context"
	"testing"
	"time"

	"github.com/z-mate/kratos-base/app/demo/internal/data"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/pgxpool"
	"github.com/z-mate/kratos-base/pkg/redisx"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// stubSource is a minimal resource.Source that always returns the same snapshot.
type stubSource struct{ v any }

func (s stubSource) Current() resource.Snapshot {
	return resource.Snapshot{Version: 1, Value: s.v}
}

// unreachableSelectPool always returns a PoolConfig pointing at an unreachable
// address, so Build will fail.
func unreachableSelectPool(_ any) (pgxpool.PoolConfig, error) {
	return pgxpool.PoolConfig{
		DSN:            "postgres://u:p@127.0.0.1:1/db?sslmode=disable",
		ConnectTimeout: time.Second,
	}, nil
}

// unreachableSelectRedis returns a redisx.Config pointing at an unreachable
// address with a short dial timeout so tests don't hang.
func unreachableSelectRedis(_ any) (redisx.Config, error) {
	return redisx.Config{
		Addrs:       []string{"192.0.2.1:6379"},
		DialTimeout: time.Second,
	}, nil
}

// disabledSelectMQ returns kind="" so MQ is not enabled.
func disabledSelectMQ(_ any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
	return "", rabbitmq.Config{}, rocketmq.Config{}, "demo.events", nil
}

// selectMQReturning builds a selectMQ function that always returns the supplied
// kind/rabbit/rocket/topic with no error. It drives data.New's initMQ down a
// concrete backend-construction branch. NewPublisher for both backends only
// constructs lazy resource.Providers (no dial happens until first Get/Publish),
// so this needs no broker.
func selectMQReturning(
	kind string,
	rabbit rabbitmq.Config,
	rocket rocketmq.Config,
	topic string,
) func(any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
	return func(_ any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
		return kind, rabbit, rocket, topic, nil
	}
}

// TestNew_InitMQ_RabbitmqBackend drives data.New's initMQ down the kind=="rabbitmq"
// branch (previously never exercised — all other tests use disabledSelectMQ or
// NewForTest, which bypass initMQ entirely). It asserts the publisher is enabled,
// the topic is threaded through, and — crucially — that the wired publisher is a
// *rabbitmq.Publisher, proving the rabbitmq construction arm was taken (not just
// "some" publisher).
func TestNew_InitMQ_RabbitmqBackend(t *testing.T) {
	sel := selectMQReturning(
		"rabbitmq",
		rabbitmq.Config{URL: "amqp://guest:guest@localhost:5672/", DialTimeout: 3 * time.Second},
		rocketmq.Config{},
		"orders.events",
	)
	d := data.New(stubSource{}, unreachableSelectPool, unreachableSelectRedis, sel)
	defer func() { _ = d.Close() }()

	if !d.MQEnabled() {
		t.Fatal("MQEnabled() = false, want true after rabbitmq initMQ branch")
	}
	if d.MQTopic() != "orders.events" {
		t.Fatalf("MQTopic() = %q, want %q", d.MQTopic(), "orders.events")
	}
	pub := d.MQPublisher()
	if pub == nil {
		t.Fatal("MQPublisher() = nil, want a *rabbitmq.Publisher")
	}
	if _, ok := pub.(*rabbitmq.Publisher); !ok {
		t.Fatalf("MQPublisher() type = %T, want *rabbitmq.Publisher (wrong backend wired)", pub)
	}
	con := d.MQConsumer()
	if _, ok := con.(*rabbitmq.Consumer); !ok {
		t.Fatalf("MQConsumer() type = %T, want *rabbitmq.Consumer", con)
	}
}

// TestNew_InitMQ_RocketmqBackend drives data.New's initMQ down the kind=="rocketmq"
// branch, the symmetric counterpart to the rabbitmq test. It asserts the wired
// publisher is a *rocketmq.Publisher so the two backend arms are distinguished:
// without the type assertion a swapped switch case would still pass MQEnabled().
func TestNew_InitMQ_RocketmqBackend(t *testing.T) {
	sel := selectMQReturning(
		"rocketmq",
		rabbitmq.Config{},
		rocketmq.Config{
			Endpoint:       "localhost:8081",
			ConsumerGroup:  "demo-group",
			AwaitDuration:  5 * time.Second,
			RequestTimeout: 2 * time.Second,
		},
		"rkt.events",
	)
	d := data.New(stubSource{}, unreachableSelectPool, unreachableSelectRedis, sel)
	defer func() { _ = d.Close() }()

	if !d.MQEnabled() {
		t.Fatal("MQEnabled() = false, want true after rocketmq initMQ branch")
	}
	if d.MQTopic() != "rkt.events" {
		t.Fatalf("MQTopic() = %q, want %q", d.MQTopic(), "rkt.events")
	}
	pub := d.MQPublisher()
	if pub == nil {
		t.Fatal("MQPublisher() = nil, want a *rocketmq.Publisher")
	}
	if _, ok := pub.(*rocketmq.Publisher); !ok {
		t.Fatalf("MQPublisher() type = %T, want *rocketmq.Publisher (wrong backend wired)", pub)
	}
	con := d.MQConsumer()
	if _, ok := con.(*rocketmq.Consumer); !ok {
		t.Fatalf("MQConsumer() type = %T, want *rocketmq.Consumer", con)
	}
}

// closeSpyPublisher records Close calls and returns a configurable error.
// It implements mq.Publisher; Publish is unused by the Close tests.
type closeSpyPublisher struct {
	closes int
	err    error
}

func (p *closeSpyPublisher) Publish(context.Context, mq.Message) error { return nil }
func (p *closeSpyPublisher) Close() error {
	p.closes++
	return p.err
}

// closeSpyConsumer records Close calls. Subscribe blocks until ctx is cancelled.
type closeSpyConsumer struct{ closes int }

func (c *closeSpyConsumer) Subscribe(ctx context.Context, _ string, _ mq.Handler) error {
	<-ctx.Done()
	return ctx.Err()
}
func (c *closeSpyConsumer) Close() error {
	c.closes++
	return nil
}

// TestClose_ClosesMQ_AndIsNilSafeForPGRedis proves Data.Close releases the MQ
// publisher and consumer exactly once each, and is safe on an MQ-only Data whose
// PG/Redis providers are nil (the NewForTest shape the lifecycle may hand it).
//
// This guards the R8F5 fix end-state: when the AfterStop hook calls d.Close(),
// the MQ connections are actually torn down — not left dangling.
//
// Mutation self-justification:
//   - Drop the `d.mqPublisher.Close()` call       → pub.closes==0, fails.
//   - Drop the `d.mqConsumer.Close()` call        → con.closes==0, fails.
//   - Remove the nil guard on d.conn/d.redisConn  → Close panics (nil deref), fails.
func TestClose_ClosesMQ_AndIsNilSafeForPGRedis(t *testing.T) {
	pub := &closeSpyPublisher{}
	con := &closeSpyConsumer{}
	// NewForTest leaves conn and redisConn nil — exercises the nil-safe path.
	d := data.NewForTest(pub, con, "demo.events")

	if err := d.Close(); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}
	if pub.closes != 1 {
		t.Errorf("mq publisher Close called %d times, want exactly 1", pub.closes)
	}
	if con.closes != 1 {
		t.Errorf("mq consumer Close called %d times, want exactly 1", con.closes)
	}
}

// TestClose_PropagatesMQPublisherError proves Data.Close surfaces the MQ
// publisher Close error (so a failed teardown isn't silently swallowed) when no
// PG/Redis error preempts it.
//
// Mutation self-justification:
//   - Return nil instead of mqErr                 → err==nil, fails.
func TestClose_PropagatesMQPublisherError(t *testing.T) {
	sentinel := errorString("publisher close failed")
	pub := &closeSpyPublisher{err: sentinel}
	d := data.NewForTest(pub, &closeSpyConsumer{}, "demo.events")

	err := d.Close()
	if err != sentinel {
		t.Fatalf("Close error = %v, want %v", err, sentinel)
	}
}

// errorString is a tiny error type so the test owns a distinct sentinel value.
type errorString string

func (e errorString) Error() string { return string(e) }

// TestEnt_BuildFailure verifies that Ent returns an error when the pool cannot
// connect (no real Postgres needed).
func TestEnt_BuildFailure(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, unreachableSelectRedis, disabledSelectMQ)
	defer func() { _ = d.Close() }()

	_, err := d.Ent(context.Background())
	if err == nil {
		t.Fatal("expected an error from Ent with unreachable DSN, got nil")
	}
	t.Logf("Ent returned expected error: %v", err)
}

// TestHealthy_BuildFailure verifies that Healthy returns an error when no
// connection can be established.
func TestHealthy_BuildFailure(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, unreachableSelectRedis, disabledSelectMQ)
	defer func() { _ = d.Close() }()

	err := d.Healthy(context.Background())
	if err == nil {
		t.Fatal("expected an error from Healthy with unreachable DSN, got nil")
	}
	t.Logf("Healthy returned expected error: %v", err)
}

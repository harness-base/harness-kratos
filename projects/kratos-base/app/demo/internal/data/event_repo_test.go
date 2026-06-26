package data_test

import (
	"context"
	"errors"
	"testing"
	"time"

	kErrors "github.com/go-kratos/kratos/v2/errors"

	"github.com/z-mate/kratos-base/app/demo/internal/data"
	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/pgxpool"
	"github.com/z-mate/kratos-base/pkg/redisx"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// stubs
// ─────────────────────────────────────────────────────────────────────────────

// fakePublisher records published messages and can inject an error.
type fakePublisher struct {
	published []mq.Message
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, m mq.Message) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, m)
	return nil
}
func (f *fakePublisher) Close() error { return nil }

// fakeConsumer satisfies mq.Consumer; Subscribe blocks until ctx is cancelled.
type fakeConsumer struct{}

func (fakeConsumer) Subscribe(ctx context.Context, _ string, _ mq.Handler) error {
	<-ctx.Done()
	return ctx.Err()
}
func (fakeConsumer) Close() error { return nil }

// noopSource is a resource.Source that always returns a zero snapshot.
type noopSource struct{}

func (noopSource) Current() resource.Snapshot { return resource.Snapshot{} }

// selectMQNoop returns kind="" (MQ disabled).
func selectMQNoop(_ any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
	return "", rabbitmq.Config{}, rocketmq.Config{}, "demo.events", nil
}

// selectPoolNoop returns a zero PoolConfig (tests that don't use PG).
func selectPoolNoop(_ any) (pgxpool.PoolConfig, error) {
	return pgxpool.PoolConfig{}, nil
}

// selectRedisNoop returns a zero redisx.Config (tests that don't use Redis).
func selectRedisNoop(_ any) (redisx.Config, error) {
	return redisx.Config{}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// tests
// ─────────────────────────────────────────────────────────────────────────────

// TestEventRepo_MQDisabled verifies that Emit returns errs.DBUnavailable (503)
// when MQ is not enabled (kind == "").
func TestEventRepo_MQDisabled(t *testing.T) {
	d := data.New(noopSource{}, selectPoolNoop, selectRedisNoop, selectMQNoop)
	repo := data.NewEventRepo(d)

	_, err := repo.Emit(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for disabled MQ, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d: %v", ke.Code, err)
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected IsDBUnavailable, got %v", err)
	}
}

// TestEventRepo_Emit_Success verifies that Emit with a working publisher
// returns an id and publishes a message with the correct fields.
func TestEventRepo_Emit_Success(t *testing.T) {
	pub := &fakePublisher{}
	d := data.NewForTest(pub, fakeConsumer{}, "demo.events")
	repo := data.NewEventRepo(d)

	id, err := repo.Emit(context.Background(), "test-payload")
	if err != nil {
		t.Fatalf("Emit: unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("Emit: id must not be empty")
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(pub.published))
	}
	msg := pub.published[0]
	if msg.Topic != "demo.events" {
		t.Fatalf("msg.Topic = %q, want demo.events", msg.Topic)
	}
	if msg.Key != id {
		t.Fatalf("msg.Key = %q, want id=%q", msg.Key, id)
	}
	if string(msg.Body) != "test-payload" {
		t.Fatalf("msg.Body = %q, want test-payload", msg.Body)
	}
}

// TestEventRepo_Emit_PublisherError verifies that a publisher error is passed
// through without double-wrapping.
func TestEventRepo_Emit_PublisherError(t *testing.T) {
	brokerErr := errs.DBUnavailable(errors.New("broker down"))
	pub := &fakePublisher{err: brokerErr}
	d := data.NewForTest(pub, fakeConsumer{}, "demo.events")
	repo := data.NewEventRepo(d)

	_, err := repo.Emit(context.Background(), "payload")
	if err == nil {
		t.Fatal("expected error from publisher, got nil")
	}
	if !errors.Is(err, brokerErr) {
		t.Fatalf("expected brokerErr, got %v", err)
	}
}

// TestEventRepo_Emit_RabbitmqUnreachable verifies that an unreachable
// RabbitMQ broker surfaces as HTTP 503 within a bounded time.
func TestEventRepo_Emit_RabbitmqUnreachable(t *testing.T) {
	src := rabbitmq.StaticSource{
		URL:         "amqp://guest:guest@127.0.0.1:5673/", // unreachable port
		DialTimeout: 200 * time.Millisecond,
	}
	pub := rabbitmq.NewPublisher(src)
	d := data.NewForTest(pub, nil, "demo.events")
	repo := data.NewEventRepo(d)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := repo.Emit(ctx, "payload")
	if err == nil {
		t.Fatal("expected error from unreachable broker, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d: %v", ke.Code, err)
	}
}

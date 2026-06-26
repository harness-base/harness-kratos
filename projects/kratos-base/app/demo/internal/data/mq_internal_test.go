package data

// White-box tests for runtimeRabbitSource.Current and runtimeRocketSource.Current
// (R2F2). These exercise the snapshot→Config mapping and the err!=nil fallback
// branch that returns a zero-valued Config while still propagating the source
// Version. The types and methods under test are unexported, so the tests live in
// package data rather than data_test.

import (
	"errors"
	"testing"

	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// versionSource is a resource.Source returning a fixed Version and Value so the
// test can assert Version is threaded through unchanged.
type versionSource struct {
	version uint64
	value   any
}

func (s versionSource) Current() resource.Snapshot {
	return resource.Snapshot{Version: s.version, Value: s.value}
}

// selectMQStub builds a selectMQFn that returns the configured tuple/err and
// records the value it was called with so the test can assert the snapshot Value
// was forwarded.
func selectMQStub(
	kind string,
	rabbit rabbitmq.Config,
	rocket rocketmq.Config,
	topic string,
	err error,
	gotValue *any,
) func(any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
	return func(v any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
		*gotValue = v
		return kind, rabbit, rocket, topic, err
	}
}

func TestRuntimeRabbitSource_Current_Success(t *testing.T) {
	want := rabbitmq.Config{URL: "amqp://u:p@host:5672/", DialTimeout: 7e9}
	var got any
	src := &runtimeRabbitSource{
		src:        versionSource{version: 42, value: "runtime-snapshot"},
		selectMQFn: selectMQStub("rabbitmq", want, rocketmq.Config{}, "t", nil, &got),
	}

	snap := src.Current()

	if got != "runtime-snapshot" {
		t.Fatalf("selectMQFn received value %#v, want the source snapshot value", got)
	}
	if snap.Version != 42 {
		t.Fatalf("Version = %d, want 42 (must thread through from source)", snap.Version)
	}
	cfg, ok := snap.Value.(rabbitmq.Config)
	if !ok {
		t.Fatalf("Value type = %T, want rabbitmq.Config", snap.Value)
	}
	if cfg.URL != want.URL {
		t.Errorf("URL = %q, want %q", cfg.URL, want.URL)
	}
	if cfg.DialTimeout != want.DialTimeout {
		t.Errorf("DialTimeout = %v, want %v", cfg.DialTimeout, want.DialTimeout)
	}
}

func TestRuntimeRabbitSource_Current_ErrorFallback(t *testing.T) {
	var got any
	src := &runtimeRabbitSource{
		src: versionSource{version: 9, value: "v"},
		// Return a non-zero rabbit config alongside the error to prove the
		// error branch discards it and returns a zero Config.
		selectMQFn: selectMQStub("rabbitmq",
			rabbitmq.Config{URL: "should-be-discarded"},
			rocketmq.Config{}, "t", errors.New("boom"), &got),
	}

	snap := src.Current()

	if snap.Version != 9 {
		t.Fatalf("Version = %d, want 9 (must thread through even on error)", snap.Version)
	}
	cfg, ok := snap.Value.(rabbitmq.Config)
	if !ok {
		t.Fatalf("Value type = %T, want rabbitmq.Config", snap.Value)
	}
	if cfg != (rabbitmq.Config{}) {
		t.Fatalf("on error Value must be a zero rabbitmq.Config, got %+v", cfg)
	}
}

func TestRuntimeRocketSource_Current_Success(t *testing.T) {
	want := rocketmq.Config{
		Endpoint:       "proxy:8081",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ConsumerGroup:  "grp",
		AwaitDuration:  5e9,
		RequestTimeout: 3e9,
		EnableTLS:      true,
	}
	var got any
	src := &runtimeRocketSource{
		src:        versionSource{version: 7, value: "runtime-snapshot"},
		selectMQFn: selectMQStub("rocketmq", rabbitmq.Config{}, want, "t", nil, &got),
	}

	snap := src.Current()

	if got != "runtime-snapshot" {
		t.Fatalf("selectMQFn received value %#v, want the source snapshot value", got)
	}
	if snap.Version != 7 {
		t.Fatalf("Version = %d, want 7 (must thread through from source)", snap.Version)
	}
	cfg, ok := snap.Value.(rocketmq.Config)
	if !ok {
		t.Fatalf("Value type = %T, want rocketmq.Config", snap.Value)
	}
	if cfg != want {
		t.Fatalf("Config = %+v, want %+v", cfg, want)
	}
}

func TestRuntimeRocketSource_Current_ErrorFallback(t *testing.T) {
	var got any
	src := &runtimeRocketSource{
		src: versionSource{version: 11, value: "v"},
		selectMQFn: selectMQStub("rocketmq", rabbitmq.Config{},
			rocketmq.Config{Endpoint: "should-be-discarded"},
			"t", errors.New("boom"), &got),
	}

	snap := src.Current()

	if snap.Version != 11 {
		t.Fatalf("Version = %d, want 11 (must thread through even on error)", snap.Version)
	}
	cfg, ok := snap.Value.(rocketmq.Config)
	if !ok {
		t.Fatalf("Value type = %T, want rocketmq.Config", snap.Value)
	}
	if cfg != (rocketmq.Config{}) {
		t.Fatalf("on error Value must be a zero rocketmq.Config, got %+v", cfg)
	}
}

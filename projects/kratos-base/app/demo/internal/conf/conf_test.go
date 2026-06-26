package conf_test

import (
	"strings"
	"testing"
	"time"

	"github.com/z-mate/kratos-base/app/demo/internal/conf"
	"github.com/z-mate/kratos-base/pkg/pgxpool"
)

// TestSelectMQ_EmptyKind verifies that an empty kind returns a zero-valued
// selection and no error.
func TestSelectMQ_EmptyKind(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = ""

	sel, err := conf.SelectMQ(rt)
	if err != nil {
		t.Fatalf("SelectMQ empty kind: unexpected error: %v", err)
	}
	if sel.Kind != "" {
		t.Fatalf("Kind = %q, want empty", sel.Kind)
	}
}

// TestSelectMQ_DefaultTopic verifies that an empty topic defaults to "demo.events".
func TestSelectMQ_DefaultTopic(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = ""
	rt.Data.MQ.Topic = ""

	sel, err := conf.SelectMQ(rt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel.Topic != "demo.events" {
		t.Fatalf("Topic = %q, want %q", sel.Topic, "demo.events")
	}
}

// TestSelectMQ_Rabbitmq verifies that the rabbitmq selection parses all fields
// including the dial_timeout string into time.Duration.
func TestSelectMQ_Rabbitmq(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = "rabbitmq"
	rt.Data.MQ.Topic = "test.topic"
	rt.Data.MQ.Rabbitmq = conf.RabbitmqConfig{
		URL:         "amqp://guest:guest@localhost:5672/",
		DialTimeout: "3s",
	}

	sel, err := conf.SelectMQ(rt)
	if err != nil {
		t.Fatalf("SelectMQ rabbitmq: unexpected error: %v", err)
	}
	if sel.Kind != "rabbitmq" {
		t.Fatalf("Kind = %q, want rabbitmq", sel.Kind)
	}
	if sel.Topic != "test.topic" {
		t.Fatalf("Topic = %q, want test.topic", sel.Topic)
	}
	if sel.Rabbit.URL != "amqp://guest:guest@localhost:5672/" {
		t.Fatalf("Rabbit.URL = %q", sel.Rabbit.URL)
	}
	const wantDT = 3e9 // 3 seconds in nanoseconds
	if sel.Rabbit.DialTimeout != wantDT {
		t.Fatalf("Rabbit.DialTimeout = %v, want 3s", sel.Rabbit.DialTimeout)
	}
}

// TestSelectMQ_Rocketmq verifies that the rocketmq selection parses all fields
// including duration strings.
func TestSelectMQ_Rocketmq(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = "rocketmq"
	rt.Data.MQ.Topic = "rkt.topic"
	rt.Data.MQ.Rocketmq = conf.RocketmqConfig{
		Endpoint:       "localhost:8081",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ConsumerGroup:  "demo-group",
		AwaitDuration:  "5s",
		RequestTimeout: "2s",
	}

	sel, err := conf.SelectMQ(rt)
	if err != nil {
		t.Fatalf("SelectMQ rocketmq: unexpected error: %v", err)
	}
	if sel.Kind != "rocketmq" {
		t.Fatalf("Kind = %q, want rocketmq", sel.Kind)
	}
	if sel.Rocket.Endpoint != "localhost:8081" {
		t.Fatalf("Rocket.Endpoint = %q", sel.Rocket.Endpoint)
	}
	if sel.Rocket.ConsumerGroup != "demo-group" {
		t.Fatalf("Rocket.ConsumerGroup = %q", sel.Rocket.ConsumerGroup)
	}
	const wantAwait = 5e9
	if sel.Rocket.AwaitDuration != wantAwait {
		t.Fatalf("Rocket.AwaitDuration = %v, want 5s", sel.Rocket.AwaitDuration)
	}
	const wantReq = 2e9
	if sel.Rocket.RequestTimeout != wantReq {
		t.Fatalf("Rocket.RequestTimeout = %v, want 2s", sel.Rocket.RequestTimeout)
	}
}

// TestSelectMQ_InvalidKind verifies that Validate rejects unknown kind values.
func TestSelectMQ_InvalidKind(t *testing.T) {
	rt := baseRuntime()
	rt.Data.MQ.Kind = "kafka"

	if err := conf.Validate(rt); err == nil {
		t.Fatal("Validate: expected error for kind=kafka, got nil")
	}
}

// TestSelectMQ_BadDialTimeout verifies that a malformed duration returns an error.
func TestSelectMQ_BadDialTimeout(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = "rabbitmq"
	rt.Data.MQ.Rabbitmq.DialTimeout = "not-a-duration"

	_, err := conf.SelectMQ(rt)
	if err == nil {
		t.Fatal("SelectMQ: expected error for bad dial_timeout, got nil")
	}
}

// TestSelectMQ_Rocketmq_BadAwaitDuration verifies the rocketmq malformed-duration
// error path for await_duration (R3F4). Only the rabbitmq dial_timeout error path
// was previously tested; the rocketmq parse branches were uncovered. We assert
// the wrapped error names the offending field so a regression that swallowed or
// mislabeled the parse error would be caught.
func TestSelectMQ_Rocketmq_BadAwaitDuration(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = "rocketmq"
	rt.Data.MQ.Rocketmq.Endpoint = "localhost:8081"
	rt.Data.MQ.Rocketmq.AwaitDuration = "not-a-duration"
	rt.Data.MQ.Rocketmq.RequestTimeout = "2s"

	_, err := conf.SelectMQ(rt)
	if err == nil {
		t.Fatal("SelectMQ: expected error for bad rocketmq await_duration, got nil")
	}
	if !strings.Contains(err.Error(), "await_duration") {
		t.Fatalf("error must identify await_duration as the bad field, got: %v", err)
	}
}

// TestSelectMQ_Rocketmq_BadRequestTimeout verifies the second rocketmq parse
// branch (request_timeout), distinct from await_duration: await_duration is valid
// so the error must come from request_timeout specifically (R3F4).
func TestSelectMQ_Rocketmq_BadRequestTimeout(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = "rocketmq"
	rt.Data.MQ.Rocketmq.Endpoint = "localhost:8081"
	rt.Data.MQ.Rocketmq.AwaitDuration = "5s"
	rt.Data.MQ.Rocketmq.RequestTimeout = "1y" // years are not a valid Go duration unit

	_, err := conf.SelectMQ(rt)
	if err == nil {
		t.Fatal("SelectMQ: expected error for bad rocketmq request_timeout, got nil")
	}
	if !strings.Contains(err.Error(), "request_timeout") {
		t.Fatalf("error must identify request_timeout as the bad field, got: %v", err)
	}
}

// TestValidate_RequiredAddrs verifies the two required-field error branches in
// Validate: an empty grpc.addr and an empty http.addr each yield a "must not be
// empty" error (R3F3). Starting from a valid baseRuntime and clearing exactly one
// field isolates each branch.
func TestValidate_RequiredAddrs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(rt *conf.Runtime)
		field  string
	}{
		{
			name:   "empty grpc addr",
			mutate: func(rt *conf.Runtime) { rt.Server.GRPC.Addr = "" },
			field:  "server.grpc.addr",
		},
		{
			name:   "empty http addr",
			mutate: func(rt *conf.Runtime) { rt.Server.HTTP.Addr = "" },
			field:  "server.http.addr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := baseRuntime()
			tt.mutate(&rt)

			err := conf.Validate(rt)
			if err == nil {
				t.Fatalf("Validate: expected error for %s, got nil", tt.field)
			}
			if !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("error must name %q, got: %v", tt.field, err)
			}
			if !strings.Contains(err.Error(), "must not be empty") {
				t.Fatalf("error must say \"must not be empty\", got: %v", err)
			}
		})
	}
}

// TestValidate_ValidKinds verifies that "", "rabbitmq", and "rocketmq" all pass.
func TestValidate_ValidKinds(t *testing.T) {
	for _, kind := range []string{"", "rabbitmq", "rocketmq"} {
		rt := baseRuntime()
		rt.Data.MQ.Kind = kind
		if err := conf.Validate(rt); err != nil {
			t.Fatalf("Validate kind=%q: unexpected error: %v", kind, err)
		}
	}
}

// TestSelectMQFlat_Passthrough verifies that SelectMQFlat returns the same
// data as SelectMQ in unpacked form.
func TestSelectMQFlat_Passthrough(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.MQ.Kind = "rabbitmq"
	rt.Data.MQ.Topic = "flat.topic"
	rt.Data.MQ.Rabbitmq = conf.RabbitmqConfig{
		URL:         "amqp://localhost/",
		DialTimeout: "1s",
	}

	kind, rabbit, _, topic, err := conf.SelectMQFlat(rt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "rabbitmq" {
		t.Fatalf("kind = %q, want rabbitmq", kind)
	}
	if topic != "flat.topic" {
		t.Fatalf("topic = %q, want flat.topic", topic)
	}
	if rabbit.URL != "amqp://localhost/" {
		t.Fatalf("rabbit.URL = %q", rabbit.URL)
	}
}

// baseRuntime returns a minimal Runtime that passes Validate.
func baseRuntime() conf.Runtime {
	var rt conf.Runtime
	rt.Server.GRPC.Addr = ":9000"
	rt.Server.HTTP.Addr = ":8000"
	return rt
}

// ─────────────────────────────────────────────────────────────────────────────
// SelectPool (R2F1)
// ─────────────────────────────────────────────────────────────────────────────

// TestSelectPool_FieldMapping verifies that every database field is mapped onto
// pgxpool.PoolConfig and that "Ns"-style duration strings are parsed to the
// correct time.Duration value.
func TestSelectPool_FieldMapping(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Database.DSN = "postgres://u:p@host:5432/db?sslmode=disable"
	rt.Data.Database.MaxOpen = 25
	rt.Data.Database.MaxIdle = 7
	rt.Data.Database.ConnMaxLifetime = "5m"
	rt.Data.Database.ConnMaxIdleTime = "90s"
	rt.Data.Database.ConnectTimeout = "5s"

	pc, err := conf.SelectPool(rt)
	if err != nil {
		t.Fatalf("SelectPool: unexpected error: %v", err)
	}
	if pc.DSN != "postgres://u:p@host:5432/db?sslmode=disable" {
		t.Errorf("DSN = %q, want the configured DSN", pc.DSN)
	}
	if pc.MaxOpen != 25 {
		t.Errorf("MaxOpen = %d, want 25", pc.MaxOpen)
	}
	if pc.MaxIdle != 7 {
		t.Errorf("MaxIdle = %d, want 7", pc.MaxIdle)
	}
	if pc.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 5m", pc.ConnMaxLifetime)
	}
	if pc.ConnMaxIdleTime != 90*time.Second {
		t.Errorf("ConnMaxIdleTime = %v, want 90s", pc.ConnMaxIdleTime)
	}
	if pc.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %v, want 5s", pc.ConnectTimeout)
	}
}

// TestSelectPool_EmptyDurations verifies that "" and "0" duration strings map to
// a zero time.Duration (driver default) rather than erroring.
func TestSelectPool_EmptyDurations(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Database.ConnMaxLifetime = ""
	rt.Data.Database.ConnMaxIdleTime = "0"
	rt.Data.Database.ConnectTimeout = ""

	pc, err := conf.SelectPool(rt)
	if err != nil {
		t.Fatalf("SelectPool: unexpected error: %v", err)
	}
	if pc.ConnMaxLifetime != 0 || pc.ConnMaxIdleTime != 0 || pc.ConnectTimeout != 0 {
		t.Fatalf("empty/zero durations must map to 0, got lifetime=%v idle=%v connect=%v",
			pc.ConnMaxLifetime, pc.ConnMaxIdleTime, pc.ConnectTimeout)
	}
}

// TestSelectPool_WrongType verifies the type-assertion error path: a non-Runtime
// argument must return an error and a zero PoolConfig.
func TestSelectPool_WrongType(t *testing.T) {
	pc, err := conf.SelectPool("not a Runtime")
	if err == nil {
		t.Fatal("SelectPool: expected error for non-Runtime arg, got nil")
	}
	if pc != (pgxpool.PoolConfig{}) {
		t.Fatalf("SelectPool: expected zero PoolConfig on error, got %+v", pc)
	}
}

// TestSelectPool_BadDuration verifies that a malformed duration propagates as an
// error (e.g. ConnMaxLifetime is the first parsed field).
func TestSelectPool_BadDuration(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Database.ConnMaxLifetime = "not-a-duration"

	pc, err := conf.SelectPool(rt)
	if err == nil {
		t.Fatal("SelectPool: expected error for bad ConnMaxLifetime, got nil")
	}
	if pc != (pgxpool.PoolConfig{}) {
		t.Fatalf("SelectPool: expected zero PoolConfig on error, got %+v", pc)
	}
}

// TestSelectPool_BadConnMaxIdleTime verifies error propagation for the
// second-parsed duration field (ConnMaxIdleTime), distinct from the first/last.
func TestSelectPool_BadConnMaxIdleTime(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Database.ConnMaxLifetime = "1m"
	rt.Data.Database.ConnMaxIdleTime = "1y" // years are not a valid Go duration unit
	rt.Data.Database.ConnectTimeout = "1m"

	_, err := conf.SelectPool(rt)
	if err == nil {
		t.Fatal("SelectPool: expected error for bad ConnMaxIdleTime, got nil")
	}
}

// TestSelectPool_BadConnectTimeout verifies error propagation for a later-parsed
// duration field (ConnectTimeout), ensuring each parse error path is covered.
func TestSelectPool_BadConnectTimeout(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Database.ConnMaxLifetime = "1m"
	rt.Data.Database.ConnMaxIdleTime = "1m"
	rt.Data.Database.ConnectTimeout = "12x"

	_, err := conf.SelectPool(rt)
	if err == nil {
		t.Fatal("SelectPool: expected error for bad ConnectTimeout, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SelectRedis (R2F1)
// ─────────────────────────────────────────────────────────────────────────────

// TestSelectRedis_FieldMapping verifies that every redis field is mapped onto
// redisx.Config and that duration strings are parsed correctly.
func TestSelectRedis_FieldMapping(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Redis = conf.RedisConfig{
		Addrs:        []string{"r1:6379", "r2:6379"},
		Username:     "user",
		Password:     "secret",
		DB:           3,
		PoolSize:     64,
		MaxRetries:   4,
		DialTimeout:  "5s",
		ReadTimeout:  "200ms",
		WriteTimeout: "1s",
		EnableTLS:    true,
	}

	rc, err := conf.SelectRedis(rt)
	if err != nil {
		t.Fatalf("SelectRedis: unexpected error: %v", err)
	}
	if len(rc.Addrs) != 2 || rc.Addrs[0] != "r1:6379" || rc.Addrs[1] != "r2:6379" {
		t.Errorf("Addrs = %v, want [r1:6379 r2:6379]", rc.Addrs)
	}
	if rc.Username != "user" {
		t.Errorf("Username = %q, want user", rc.Username)
	}
	if rc.Password != "secret" {
		t.Errorf("Password = %q, want secret", rc.Password)
	}
	if rc.DB != 3 {
		t.Errorf("DB = %d, want 3", rc.DB)
	}
	if rc.PoolSize != 64 {
		t.Errorf("PoolSize = %d, want 64", rc.PoolSize)
	}
	if rc.MaxRetries != 4 {
		t.Errorf("MaxRetries = %d, want 4", rc.MaxRetries)
	}
	if rc.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout = %v, want 5s", rc.DialTimeout)
	}
	if rc.ReadTimeout != 200*time.Millisecond {
		t.Errorf("ReadTimeout = %v, want 200ms", rc.ReadTimeout)
	}
	if rc.WriteTimeout != 1*time.Second {
		t.Errorf("WriteTimeout = %v, want 1s", rc.WriteTimeout)
	}
	if !rc.EnableTLS {
		t.Errorf("EnableTLS = false, want true")
	}
}

// TestSelectRedis_WrongType verifies the type-assertion error path.
func TestSelectRedis_WrongType(t *testing.T) {
	rc, err := conf.SelectRedis(42)
	if err == nil {
		t.Fatal("SelectRedis: expected error for non-Runtime arg, got nil")
	}
	if rc.Addrs != nil || rc.DialTimeout != 0 || rc.EnableTLS {
		t.Fatalf("SelectRedis: expected zero Config on error, got %+v", rc)
	}
}

// TestSelectRedis_BadDialTimeout verifies that a malformed dial_timeout
// propagates as an error.
func TestSelectRedis_BadDialTimeout(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Redis.DialTimeout = "bogus"

	_, err := conf.SelectRedis(rt)
	if err == nil {
		t.Fatal("SelectRedis: expected error for bad dial_timeout, got nil")
	}
}

// TestSelectRedis_BadReadTimeout verifies error propagation for the
// second-parsed duration field (read_timeout), distinct from dial/write.
func TestSelectRedis_BadReadTimeout(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Redis.DialTimeout = "1s"
	rt.Data.Redis.ReadTimeout = "1y" // invalid duration unit
	rt.Data.Redis.WriteTimeout = "1s"

	_, err := conf.SelectRedis(rt)
	if err == nil {
		t.Fatal("SelectRedis: expected error for bad read_timeout, got nil")
	}
}

// TestSelectRedis_BadWriteTimeout verifies error propagation for a later-parsed
// duration field (write_timeout).
func TestSelectRedis_BadWriteTimeout(t *testing.T) {
	rt := conf.Runtime{}
	rt.Data.Redis.DialTimeout = "1s"
	rt.Data.Redis.ReadTimeout = "1s"
	rt.Data.Redis.WriteTimeout = "nope"

	_, err := conf.SelectRedis(rt)
	if err == nil {
		t.Fatal("SelectRedis: expected error for bad write_timeout, got nil")
	}
}

package rocketmq

import (
	"context"
	"time"

	golang "github.com/apache/rocketmq-clients/golang/v5"

	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// This file exposes unexported resilience-core seams to the external
// rocketmq_test package so the pump loop (F2) can be driven with a fake
// receiver and real assertions, without a live broker.

// Delivery is the exported alias of the internal delivery view a fake message
// must satisfy (GetTopic/GetBody/GetKeys/GetProperties).
type Delivery = delivery

// Receiver is the exported alias of the internal injectable pull+ack seam.
type Receiver = receiver

// RunPump drives the unexported pump loop with the supplied receiver. backoff
// may be nil (DefaultBackoff is used). It returns pump's error (nil on ctx
// cancel, non-nil on maxReceiveErrors rebuild).
func RunPump(ctx context.Context, rcv Receiver, topic string, h mq.Handler, backoff mq.Backoff) error {
	b := backoff
	if b == nil {
		b = mq.DefaultBackoff
	}
	c := &Consumer{backoff: b}
	return c.pump(ctx, rcv, topic, h)
}

// MaxReceiveErrors exposes the consecutive-error rebuild threshold for tests.
const MaxReceiveErrors = maxReceiveErrors

// SessionFn mirrors the unexported per-connection session runner signature that
// the outer reconnect loop drives.  Tests script a SessionFn to assert reconnect
// behaviour (attempt counting, backoff-on-failure, stop on backoff=false). R2F7.
type SessionFn = func(ctx context.Context, cfg Config, topic string, h mq.Handler) error

// RunReconnectLoop drives the real Consumer.Subscribe outer reconnect loop with
// an injected session runner and backoff, bypassing the live SDK.  src supplies
// the config (must have non-empty Endpoint + ConsumerGroup so Subscribe enters
// the loop).  This exercises the production Subscribe loop body — not a copy —
// so the attempt/backoff assertions bind real code. R2F7.
//
// src is a resource.Source (not a fixed StaticSource) so a test can feed a
// MUTATING source that changes the Config between sessions, exercising the R9F2
// per-attempt config re-read (the session runner records the cfg it observes).
func RunReconnectLoop(ctx context.Context, src resource.Source, topic string, h mq.Handler, backoff mq.Backoff, session SessionFn) error {
	c := &Consumer{
		src:          src,
		backoff:      backoff,
		runSessionFn: session,
	}
	return c.Subscribe(ctx, topic, h)
}

// ToMessage exposes the delivery→mq.Message mapping for direct assertions.
func ToMessage(d Delivery) mq.Message { return toMessage(d) }

// SetEnableTLS exposes the unexported set-once TLS applier so the R2F21
// set-once + reject-divergence contract can be asserted directly. Tests must
// call ResetEnableTLS first to get a deterministic "first Build" state, since
// golang.EnableSsl and the once-flag are process-wide.
func SetEnableTLS(want bool) error { return setEnableTLS(want) }

// ResetEnableTLS clears the set-once state so a test can simulate a fresh
// process. Test-only; production never resets the global.
func ResetEnableTLS() {
	enableTLSMu.Lock()
	defer enableTLSMu.Unlock()
	enableTLSOnce = false
	enableTLSMode = false
}

// EnableSslGlobal returns the SDK's current process-global golang.EnableSsl so a
// test can assert the first Build actually wrote it.
func EnableSslGlobal() bool { return golang.EnableSsl }

// AwaitDurationOf and RequestTimeoutOf expose the unexported Config accessors so
// their default-vs-explicit branches can be asserted directly. awaitDuration's
// <=0 default branch had no coverage (R2F9); RequestTimeoutOf is included so the
// twin accessors are tested the same way. (Config.AwaitDuration etc. remain the
// public knobs; these only surface the derived defaulting logic.)
func AwaitDurationOf(c Config) time.Duration  { return c.awaitDuration() }
func RequestTimeoutOf(c Config) time.Duration { return c.requestTimeout() }

// PublisherLiveRequestTimeout exposes the unexported per-Publish request-timeout
// read so the R9F3 contract — Publish re-reads RequestTimeout from the LIVE
// snapshot, not a value frozen at construction — can be asserted directly. A test
// builds a Publisher over a mutating source, changes RequestTimeout, and checks
// this returns the new value.
func PublisherLiveRequestTimeout(pub *Publisher) time.Duration { return pub.liveRequestTimeout() }

// BuildMessageProps drives the unexported buildMessage helper (the exact mapping
// the publish path uses) and returns the resulting SDK message's keys and
// properties, so the R9F6 publisher-side trace injection can be asserted without
// a live broker.
func BuildMessageProps(ctx context.Context, m mq.Message) (keys []string, props map[string]string) {
	msg := buildMessage(ctx, m)
	return msg.GetKeys(), msg.GetProperties()
}

// NoBackoff is a Backoff that never sleeps and always continues, so pump-loop
// tests run instantly without wall-clock delays.
func NoBackoff(_ context.Context, _ int) bool { return true }

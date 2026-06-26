// Package rocketmq provides a RocketMQ v5 backend for pkg/mq using the
// Apache RocketMQ Go client (github.com/apache/rocketmq-clients/golang/v5).
//
// Architecture summary:
//   - A resource.Provider[golang.Producer] owns the SDK producer and rebuilds
//     it on config change (fingerprint change).
//   - Publisher wraps the provider with an SRE circuit-breaker; each Publish
//     calls producer.Send with the caller's context.
//   - Consumer.Subscribe runs a pull+ack loop using SimpleConsumer.Receive;
//     after maxReceiveErrors consecutive Receive failures the session is torn
//     down and a new SimpleConsumer is built (outer reconnect loop).  This is
//     required because the SDK's internal gRPC channel does not always
//     self-heal after a runtime broker drop: rebuilding sc.Start() forces a
//     fresh gRPC connection + settings exchange.
//
// RocketMQ v5 SDK design notes (verified from SDK source v5.1.3):
//   - producer.Start() blocks until the gRPC telemetry stream completes its
//     settings exchange (inited=true) — with or without WithTopics. Against
//     an unreachable endpoint it blocks INDEFINITELY (the internal poll loop
//     does not observe done). We therefore run Start() in a goroutine and
//     bound it with cfg.RequestTimeout, calling GracefulStop on timeout.
//   - producer.SetRequestTimeout sets both the route-query timeout and the
//     SendMessage RPC timeout (cli.opts.timeout + pSetting.requestTimeout).
//     Default is 3 s; we override with cfg.RequestTimeout.
//   - SimpleConsumer.Subscribe(topic, expr) does a blocking route query
//     (cli.opts.timeout) so it may fail on unreachable endpoints.
//   - The SDK does NOT reliably self-heal the gRPC stream after a runtime
//     broker drop; our outer loop rebuilds the SimpleConsumer when the inner
//     Receive loop accumulates maxReceiveErrors consecutive errors.
//   - Logging: the SDK writes zap logs to ~/logs/rocketmqlogs by default.
//     On macOS /logs is read-only; init() redirects to /tmp/rocketmq-logs
//     unless CLIENT_LOG_ROOT is already set.  Tests override via TestMain.
package rocketmq

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	golang "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"

	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// golang.EnableSsl is a process-level global in the SDK (conn.go:38) that the
// SDK reads WITHOUT synchronization while dialling every gRPC connection
// (conn.go:116, dialSetupOpts).  Crucially that read happens lazily, deep inside
// the SDK's client_manager.getRpcClient path that is reached during the client's
// telemetry/route bootstrap — i.e. inside the detached Start() goroutine, NOT
// during NewProducer/NewSimpleConsumer construction.  So merely serializing the
// "set global + construct" section (R1's buildMu) did NOT make the write
// happen-before the SDK's read: the read fires later, in another goroutine, with
// no shared lock.  That left a real data race (race-detector confirmed in R1).
//
// Because the global is process-wide and read lock-free at dial time, the only
// way to be race-free without forking the SDK is to make the write happen ONCE,
// before any dial, and never write it again.  We therefore adopt set-once
// semantics: the first Build (producer or consumer) writes golang.EnableSsl from
// its config and records the chosen mode under enableTLSMu; every later Build
// compares its EnableTLS against the recorded mode and, if they differ, returns
// an error instead of writing — the SDK simply cannot host two TLS modes in one
// process.  After the single write there are no further writers, so a concurrent
// dial reading the global races with nothing.
//
// Residual (rule-0009 §C honesty): this is set-once + reject-divergence, not a
// general fix, and we deliberately do NOT claim "race eliminated/never".  The
// write+decision (compare/set of golang.EnableSsl and the recorded mode) is
// fully serialized under enableTLSMu, so our own write is well-defined and there
// is exactly one writer for the whole process.  What remains outside our control
// is the SDK reading golang.EnableSsl lock-free at dial time: if the very first
// Build writes the global concurrently with another client's dial reading it
// (e.g. two distinct configs first-Build at the same instant), the -race
// detector can still flag that SDK read against our (single) write, because the
// SDK never takes enableTLSMu.  This cannot be eliminated without forking the
// SDK.  In every realistic deployment a single static TLS mode is chosen once at
// startup, before any dial, so there is one write, no second writer, and the
// only reader (a later dial) runs strictly after it — no race in practice.
var (
	enableTLSMu   sync.Mutex
	enableTLSOnce bool // true after the first Build has set golang.EnableSsl
	enableTLSMode bool // the TLS mode chosen by the first Build
)

// setEnableTLS applies set-once semantics to the SDK's process-global
// golang.EnableSsl.  The first call records want and writes the global; every
// later call only succeeds if want matches the recorded mode, otherwise it
// returns an error (the SDK cannot mix TLS modes in one process).  See the
// enableTLSMu doc block for the data-race rationale and its honest residual.
func setEnableTLS(want bool) error {
	enableTLSMu.Lock()
	defer enableTLSMu.Unlock()
	if enableTLSOnce {
		if enableTLSMode != want {
			return fmt.Errorf("rocketmq: SDK uses a process-global TLS flag (golang.EnableSsl); "+
				"cannot mix TLS modes in one process (already=%v, requested=%v)", enableTLSMode, want)
		}
		return nil
	}
	golang.EnableSsl = want
	enableTLSMode = want
	enableTLSOnce = true
	return nil
}

// dialReachable performs a TCP reachability precheck against the proxy endpoint
// before we call the SDK's blocking Start().  This is the source-level guard for
// F22: the SDK's startUp() ends with `for !cli.inited.Load() { sleep(1s) }`
// (client.go:589), a loop that observes ONLY inited and never cli.done, so a
// Start() goroutine launched against an unreachable endpoint spins forever even
// after GracefulStop closes cli.done.  By refusing to call Start() at all when
// the endpoint cannot be dialled, we never spawn that doomed goroutine.
//
// Residual (rule-0009 honesty): this is a precheck, not a guarantee.  If the
// TCP dial succeeds but the broker then stalls the telemetry settings handshake
// (e.g. half-open connection, broker accepting TCP but not responding to the
// gRPC settings stream), the bounded select below still abandons the Start()
// goroutine and that goroutine can still spin in the inited loop.  This is
// strictly rarer than the unreachable-endpoint case (which the precheck fully
// eliminates) and is an upstream SDK defect we cannot fix without forking it.
func dialReachable(ctx context.Context, endpoint string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", endpoint)
	if err != nil {
		return fmt.Errorf("rocketmq: endpoint %s unreachable: %w", endpoint, err)
	}
	_ = conn.Close()
	return nil
}

// maxReceiveErrors is the maximum number of consecutive Receive errors before
// the current SimpleConsumer session is abandoned and a new one is built.
// After a runtime broker drop the SDK's internal gRPC channel may not
// self-heal; rebuilding the SimpleConsumer forces a fresh connection.
const maxReceiveErrors = 3

// init redirects the RocketMQ SDK log output away from /logs (read-only on
// macOS) to /tmp/rocketmq-logs.  The redirect is skipped when the caller has
// already set CLIENT_LOG_ROOT (e.g. in TestMain).
func init() {
	if os.Getenv(golang.CLIENT_LOG_ROOT) == "" {
		_ = os.Setenv(golang.CLIENT_LOG_ROOT, "/tmp/rocketmq-logs")
		golang.ResetLogger()
	}
}

// consumerLog emits a single structured JSON line to stderr so that reconnect
// events appear in the demo process log regardless of the Kratos logger.
func consumerLog(fields map[string]any) {
	b, err := json.Marshal(fields)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = os.Stderr.Write(b)
}

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

// Config holds the parameters needed to connect to a RocketMQ v5 broker.
type Config struct {
	// Endpoint is the namesrv/proxy address, e.g. "localhost:8081".
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// AccessKey and SecretKey are optional credentials (empty = no auth).
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	// ConsumerGroup is required for Consumer; unused by Publisher.
	ConsumerGroup string `json:"consumer_group" yaml:"consumer_group"`
	// AwaitDuration is the long-poll duration for SimpleConsumer.Receive.
	// Zero uses a 5-second default.
	AwaitDuration time.Duration `json:"await_duration" yaml:"await_duration"`
	// RequestTimeout caps each SDK RPC (route query, send, ack).
	// Zero uses the SDK default (3 s).
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`
	// EnableTLS controls whether the gRPC connection to the proxy uses TLS.
	// Set to false for sandbox/dev environments with a plain-text proxy (default).
	// The SDK v5.1.3 global EnableSsl is set to this value before each Build.
	EnableTLS bool `json:"enable_tls" yaml:"enable_tls"`
}

func (c Config) awaitDuration() time.Duration {
	if c.AwaitDuration <= 0 {
		return 5 * time.Second
	}
	return c.AwaitDuration
}

func (c Config) requestTimeout() time.Duration {
	if c.RequestTimeout <= 0 {
		return 3 * time.Second
	}
	return c.RequestTimeout
}

// Fingerprint returns a stable hex string covering connection-relevant fields.
//
// It must include every field that Build applies to the live SDK client, so that
// changing any of them triggers a provider rebuild (cf. pgxpool/redisx: the
// fingerprint covers all connection-relevant knobs). AwaitDuration and
// RequestTimeout are applied at Build time (WithSimpleAwaitDuration /
// SetRequestTimeout); omitting them would let a hot-reload of either value go
// unnoticed because the version-or-fingerprint gate in resource.Provider would
// see no change and keep serving the old client. So Endpoint, AccessKey,
// SecretKey, ConsumerGroup, EnableTLS, AwaitDuration, and RequestTimeout all
// participate. (Note: requestTimeout is also re-read live on the Publish path —
// R9F3 — but it stays in the fingerprint because the Build-time
// SetRequestTimeout it controls is what bounds the consumer/route RPCs.)
func (c Config) Fingerprint() string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s|%v|%d|%d",
		c.Endpoint, c.AccessKey, c.SecretKey, c.ConsumerGroup, c.EnableTLS,
		c.AwaitDuration, c.RequestTimeout)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ─────────────────────────────────────────────────────────────────────────────
// StaticSource — convenience wrapper so tests can feed a Config directly.
// ─────────────────────────────────────────────────────────────────────────────

// StaticSource wraps a single Config as a resource.Source.
// The version is always 1 and never changes.
type StaticSource Config

func (s StaticSource) Current() resource.Snapshot {
	return resource.Snapshot{Version: 1, Value: Config(s)}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func cfgFrom(v any) (Config, bool) {
	c, ok := v.(Config)
	return c, ok
}

func sdkConfig(cfg Config) *golang.Config {
	return &golang.Config{
		Endpoint:      cfg.Endpoint,
		ConsumerGroup: cfg.ConsumerGroup,
		Credentials: &credentials.SessionCredentials{
			AccessKey:    cfg.AccessKey,
			AccessSecret: cfg.SecretKey,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Producer provider
// ─────────────────────────────────────────────────────────────────────────────

func newProducerProvider(src resource.Source, topic string) *resource.Provider[golang.Producer] {
	ad := resource.Adapter[golang.Producer]{
		Build: func(ctx context.Context, v any) (golang.Producer, error) {
			cfg, ok := cfgFrom(v)
			if !ok {
				return nil, fmt.Errorf("rocketmq: invalid config type %T", v)
			}
			if cfg.Endpoint == "" {
				return nil, fmt.Errorf("rocketmq: endpoint is required")
			}

			// F22 precheck: refuse to call the SDK's blocking Start() unless the
			// endpoint is TCP-reachable.  An abandoned Start() goroutine against
			// an unreachable endpoint spins forever (see dialReachable docs), so
			// the only way to avoid leaking it is to never launch it.  This runs
			// on every (re)build, so a failing build under broker outage costs
			// one dial — not one leaked goroutine.
			if err := dialReachable(ctx, cfg.Endpoint, cfg.requestTimeout()); err != nil {
				return nil, err
			}

			// SDK v5.1.3 quirk: producer.Start() waits for an
			// onSettingsCommand from the broker via the gRPC telemetry
			// stream. Without WithTopics the telemetry session is never
			// established (getTotalTargets returns empty until route data
			// arrives), causing Start to block indefinitely.  Passing the
			// business topic ensures a route query runs at startup, which
			// populates the target table and unblocks the telemetry sync.
			opts := []golang.ProducerOption{}
			if topic != "" {
				opts = append(opts, golang.WithTopics(topic))
			}

			// golang.EnableSsl is an SDK process-global read lock-free at dial
			// time (inside the Start() goroutine).  Set it once before any dial;
			// reject a later Build that wants a different TLS mode.  See setEnableTLS.
			if err := setEnableTLS(cfg.EnableTLS); err != nil {
				return nil, err
			}
			p, err := golang.NewProducer(sdkConfig(cfg), opts...)
			if err != nil {
				return nil, fmt.Errorf("rocketmq: NewProducer: %w", err)
			}
			// Set request timeout before Start so that route queries and
			// send RPCs use the configured timeout.
			p.SetRequestTimeout(cfg.requestTimeout())

			// Run Start in a goroutine bounded by requestTimeout/ctx so we
			// don't block forever even if a reachable endpoint stalls the
			// settings handshake (see dialReachable residual note).
			startErr := make(chan error, 1)
			go func() { startErr <- p.Start() }()

			timeout := cfg.requestTimeout()
			select {
			case err := <-startErr:
				if err != nil {
					_ = p.GracefulStop()
					return nil, fmt.Errorf("rocketmq: producer Start: %w", err)
				}
				return p, nil
			case <-ctx.Done():
				_ = p.GracefulStop()
				return nil, fmt.Errorf("rocketmq: producer Start cancelled: %w", ctx.Err())
			case <-time.After(timeout):
				// Start() did not complete within requestTimeout.  The
				// producer has no live broker connection; treat as unavailable.
				_ = p.GracefulStop()
				return nil, fmt.Errorf("rocketmq: producer Start timed out after %v (endpoint unreachable)", timeout)
			}
		},
		Close: func(p golang.Producer) error {
			if p == nil {
				return nil
			}
			return p.GracefulStop()
		},
		Fingerprint: func(v any) string {
			cfg, ok := cfgFrom(v)
			if !ok {
				return ""
			}
			return cfg.Fingerprint()
		},
		// Health probes the proxy endpoint with a TCP dial so the readiness
		// gate detects a broker outage quickly.  A dial deadline of 3 s is
		// generous enough to survive transient packet loss while still
		// reporting the broker as down within the 15 s readyz probe window.
		Health: func(ctx context.Context, _ golang.Producer) error {
			snap := src.Current()
			cfg, ok := cfgFrom(snap.Value)
			if !ok || cfg.Endpoint == "" {
				return nil
			}
			dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", cfg.Endpoint)
			if err != nil {
				return fmt.Errorf("rocketmq: endpoint %s unreachable: %w", cfg.Endpoint, err)
			}
			_ = conn.Close()
			return nil
		},
	}
	return resource.New[golang.Producer](src, ad)
}

// ─────────────────────────────────────────────────────────────────────────────
// Publisher
// ─────────────────────────────────────────────────────────────────────────────

// Publisher implements mq.Publisher for RocketMQ v5.
type Publisher struct {
	provider *resource.Provider[golang.Producer]
	breaker  circuitbreaker.CircuitBreaker
	// src is the live config source. Publish re-reads it on every call so that
	// a hot-reloaded RequestTimeout takes effect immediately (R9F3); caching the
	// startup value would freeze the ctx bound at the boot-time timeout forever.
	src resource.Source
}

// NewPublisher creates a Publisher backed by src.
// topic is the business topic the publisher will send to; it is passed to
// the SDK producer via WithTopics so that the gRPC telemetry session can be
// established at startup (required by SDK v5.1.3 when no route data exists yet).
func NewPublisher(src resource.Source, topic string) *Publisher {
	return NewPublisherWithBreaker(src, topic, sre.NewBreaker())
}

// NewPublisherWithBreaker is like NewPublisher but injects the circuit breaker.
// Production code uses NewPublisher (sre.NewBreaker); tests inject a
// deterministic stub to assert the open-circuit fast-fail path (F8).
func NewPublisherWithBreaker(src resource.Source, topic string, breaker circuitbreaker.CircuitBreaker) *Publisher {
	return &Publisher{
		provider: newProducerProvider(src, topic),
		breaker:  breaker,
		src:      src,
	}
}

// liveRequestTimeout reads the current snapshot's RequestTimeout (defaulted) so
// the Publish ctx bound follows a hot-reload (R9F3). A snapshot whose value is
// not a Config (mis-wired source) falls back to the SDK default via the
// Config.requestTimeout accessor on the zero Config.
func (pub *Publisher) liveRequestTimeout() time.Duration {
	cfg, _ := cfgFrom(pub.src.Current().Value)
	return cfg.requestTimeout()
}

// buildMessage maps an mq.Message into the SDK *golang.Message: it sets the
// topic/body, the routing key (keys[0]), business properties, AND injects the
// current span's W3C trace context into the properties so the async hop carries
// the trace to the consumer (R9F6). mq.InjectTrace uses a self-contained
// propagation.TraceContext{} (not the global propagator) and returns a copy, so
// the caller's m.Headers map is never mutated. Pure (no broker), hence unit-
// testable: see TestBuildMessage_InjectsTrace.
func buildMessage(ctx context.Context, m mq.Message) *golang.Message {
	headers := mq.InjectTrace(ctx, m.Headers)
	msg := &golang.Message{
		Topic: m.Topic,
		Body:  m.Body,
	}
	if m.Key != "" {
		msg.SetKeys(m.Key)
	}
	for k, v := range headers {
		msg.AddProperty(k, v)
	}
	return msg
}

// Publish sends m to the broker.
//
// Circuit-breaker gating: if the breaker is open, returns DBUnavailable
// immediately without attempting a dial.
//
// The RocketMQ SDK may block for up to requestTimeout on an unreachable
// endpoint.  We cap both provider.Get and p.Send with a bounded context
// derived from requestTimeout so that the HTTP handler is never held longer
// than that, and circuit-breaker failures accumulate quickly.
//
// Caller-cancellation handling (R10F4): if a failure stems from the CALLER
// cancelling ctx (e.g. the publishing client disconnects), that is not a broker
// fault. We still return DBUnavailable but leave the breaker untouched (neither
// MarkFailed nor MarkSuccess), so a burst of client disconnects/cancellations
// cannot open the circuit against a healthy broker. We test the caller's ctx,
// NOT the internally-derived sendCtx, because sendCtx's DeadlineExceeded is our
// own timeout firing on a slow broker — which must still count as a failure.
func (pub *Publisher) Publish(ctx context.Context, m mq.Message) error {
	// Fail loud on an empty topic, mirroring the rabbitmq adapter (HTTP 400): a
	// blank Topic is a caller wiring bug, not a broker fault. The guard sits BEFORE
	// breaker.Allow() so the breaker is never consulted for a malformed argument —
	// counting it would let a bug pollute the broker's health verdict. Asserted by
	// TestPublisher_EmptyTopic_InvalidArgumentBeforeBreaker (allowCalls == 0).
	if m.Topic == "" {
		return errs.InvalidArgument("mq: message topic must not be empty")
	}
	if err := pub.breaker.Allow(); err != nil {
		return errs.DBUnavailable(err)
	}

	// Re-read RequestTimeout from the live snapshot so a hot-reload bounds this
	// Publish at the current value, not the boot-time one (R9F3).
	timeout := pub.liveRequestTimeout()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	p, err := pub.provider.Get(sendCtx)
	if err != nil {
		pub.markBackendFailure(ctx, err)
		return errs.DBUnavailable(err)
	}

	// Build the SDK message (key + properties + injected trace). buildMessage is a
	// pure helper so the trace-inject wiring (R9F6) is unit-testable without a
	// broker — see TestBuildMessage_InjectsTrace.
	msg := buildMessage(ctx, m)

	// p.Send may ignore sendCtx — the v5 SDK does its own SetRequestTimeout ×
	// internal retries, so a send to a dead broker can block ~4× requestTimeout
	// regardless of the ctx we pass. Bound it ourselves: run Send in a goroutine
	// and return the moment sendCtx fires, so the HTTP handler is never held
	// longer than requestTimeout. The abandoned goroutine drains on its own
	// (done is buffered, so it never blocks). NOTE: the sre breaker won't
	// fast-open at low traffic (its request-count threshold is high), so this
	// ctx bound — not the breaker — is what guarantees bounded fail latency here.
	done := make(chan error, 1)
	go func() { _, serr := p.Send(sendCtx, msg); done <- serr }()
	select {
	case <-sendCtx.Done():
		// sendCtx fired: either the caller cancelled ctx (cancellation propagates
		// to the derived sendCtx, ctx.Err()==Canceled → not a broker fault) or our
		// own timeout elapsed (DeadlineExceeded → slow broker = fault).
		// markBackendFailure tests the caller's ctx to tell them apart.
		pub.markBackendFailure(ctx, sendCtx.Err())
		return errs.DBUnavailable(sendCtx.Err())
	case serr := <-done:
		if serr != nil {
			pub.markBackendFailure(ctx, serr)
			return errs.DBUnavailable(serr)
		}
	}

	pub.breaker.MarkSuccess()
	return nil
}

// markBackendFailure trips the breaker for a backend error UNLESS the error came
// from the caller cancelling ctx (client disconnect). A caller cancellation is
// not evidence the broker is sick, so counting it as a failure would let a wave
// of client disconnects open the circuit against a healthy broker (R10F4). It is
// passed the CALLER's ctx (not the internally-derived sendCtx) so our own send
// timeout (DeadlineExceeded on a slow broker) is NOT mistaken for a caller
// cancellation — that still counts as a failure.
func (pub *Publisher) markBackendFailure(ctx context.Context, err error) {
	if errs.IsCallerCanceled(ctx, err) {
		return
	}
	pub.breaker.MarkFailed()
}

// Close tears down the underlying producer provider.
func (pub *Publisher) Close() error {
	return pub.provider.Close()
}

// Healthy drives a producer refresh (via Get) to ensure the producer is
// reachable. It satisfies resource.Check and can be registered directly into
// a resource.Registry:
//
//	reg.Register("mq", pub.Healthy)
func (pub *Publisher) Healthy(ctx context.Context) error {
	return pub.provider.Healthy(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Consumer
// ─────────────────────────────────────────────────────────────────────────────

// delivery is the minimal read view of one received message that the pump loop
// maps into an mq.Message.  *golang.MessageView satisfies it; fake deliveries in
// tests satisfy it too, which lets the keys/properties→mq.Message mapping be
// unit-tested without a live broker (F2).
type delivery interface {
	GetTopic() string
	GetBody() []byte
	GetKeys() []string
	GetProperties() map[string]string
}

// receiver is the injectable pull+ack seam the pump loop drives.  The real
// implementation wraps a *golang.SimpleConsumer (see sdkReceiver); tests inject
// a fake to exercise the resilience core — consecutive-error rebuild, ack-skip
// on handler error, and the message mapping — with real assertions (F2).
type receiver interface {
	// Receive returns up to n deliveries, long-polling for at most await.
	Receive(ctx context.Context, n int32, await time.Duration) ([]delivery, error)
	// Ack acknowledges a single delivery previously returned by Receive.
	Ack(ctx context.Context, d delivery) error
}

// sdkReceiver adapts *golang.SimpleConsumer to the receiver interface.
type sdkReceiver struct {
	sc golang.SimpleConsumer
}

func (r sdkReceiver) Receive(ctx context.Context, n int32, await time.Duration) ([]delivery, error) {
	mvs, err := r.sc.Receive(ctx, n, await)
	if err != nil {
		return nil, err
	}
	ds := make([]delivery, len(mvs))
	for i, mv := range mvs {
		ds[i] = mv
	}
	return ds, nil
}

func (r sdkReceiver) Ack(ctx context.Context, d delivery) error {
	mv, ok := d.(*golang.MessageView)
	if !ok {
		return fmt.Errorf("rocketmq: sdkReceiver.Ack got non-MessageView delivery %T", d)
	}
	return r.sc.Ack(ctx, mv)
}

// Consumer implements mq.Consumer for RocketMQ v5 using SimpleConsumer
// (pull + ack model).  The SDK does NOT reliably self-heal its gRPC channel
// after a runtime broker drop (see package header), so the inner Receive loop
// counts consecutive errors and, at maxReceiveErrors, the outer reconnect loop
// rebuilds the SimpleConsumer to force a fresh connection.
type Consumer struct {
	src     resource.Source
	backoff mq.Backoff

	// runSessionFn is the per-connection session runner driven by the outer
	// reconnect loop.  Production leaves it nil so Subscribe binds the real
	// runSession; tests inject a scripted fake to exercise the reconnect loop
	// (attempt counting, backoff-on-failure, stop when backoff returns false)
	// deterministically without a live broker (R2F7).
	runSessionFn func(ctx context.Context, cfg Config, topic string, h mq.Handler) error
}

// NewConsumer creates a Consumer backed by src.
// If backoff is nil, mq.DefaultBackoff is used.
func NewConsumer(src resource.Source, backoff mq.Backoff) *Consumer {
	b := backoff
	if b == nil {
		b = mq.DefaultBackoff
	}
	return &Consumer{src: src, backoff: b}
}

// Subscribe starts the supervised pull+ack consumer loop for topic, calling h
// for each message.  It blocks until ctx is cancelled.
//
// Outer reconnect loop: when the broker is unreachable at startup (sc.Start or
// sc.Subscribe fails), Subscribe backs off via the configured Backoff and
// retries from scratch (creating a new SimpleConsumer each attempt).  This
// mirrors mq.RunSupervised semantics and enables the AC-MR2/MR3 self-heal
// scenarios (broker recovers after startup or runtime drop without restarting
// the demo process).
//
// Inner Receive loop: after a successful connection our loop backs off on
// Receive errors; because the SDK does NOT reliably self-heal after a runtime
// broker drop (see package header), maxReceiveErrors consecutive failures end
// the session so this outer loop rebuilds the SimpleConsumer (fresh gRPC
// connection).  When ctx is cancelled Subscribe returns ctx.Err().
func (c *Consumer) Subscribe(ctx context.Context, topic string, h mq.Handler) error {
	// Validate the startup snapshot up front so a clearly-misconfigured consumer
	// (wrong config type, no endpoint, no group) fails loud immediately instead of
	// spinning the reconnect loop. The loop body below re-reads the snapshot every
	// attempt for hot-reload (R9F2), but an invalid STARTUP config is a wiring bug,
	// not a transient outage, so we surface it before entering the loop.
	if cfg, ok := cfgFrom(c.src.Current().Value); !ok {
		return fmt.Errorf("rocketmq: invalid config type in source")
	} else if cfg.Endpoint == "" {
		return fmt.Errorf("rocketmq: endpoint is required")
	} else if cfg.ConsumerGroup == "" {
		return fmt.Errorf("rocketmq: consumer_group is required")
	}

	// Bind the session runner: production uses the real runSession (live SDK);
	// tests inject runSessionFn to drive the reconnect loop deterministically.
	runSession := c.runSession
	if c.runSessionFn != nil {
		runSession = c.runSessionFn
	}

	connectAttempt := 0
	for {
		// Respect context before each connect attempt.
		if err := ctx.Err(); err != nil {
			return err
		}

		connectAttempt++

		// R9F2: re-read the config snapshot at the TOP of every reconnect attempt
		// so a hot-reloaded Endpoint/ConsumerGroup/timeout takes effect on the next
		// session — mirroring the publisher (provider.Get re-reads) and the rabbitmq
		// consumer (getLiveConn→provider.Get re-reads). Freezing cfg outside the loop
		// (the old bug) made the consumer ignore config hot-reload entirely.
		//
		// A momentarily empty/invalid snapshot (e.g. config-center returned a blank
		// during a reload) is treated as a TRANSIENT failure: back off and retry,
		// rather than terminating the supervised consumer. Terminating here would
		// kill the only consumer over a transient blip and never recover.
		cfg, ok := cfgFrom(c.src.Current().Value)
		if !ok || cfg.Endpoint == "" || cfg.ConsumerGroup == "" {
			consumerLog(map[string]any{
				"consumer": "config_invalid",
				"topic":    topic,
				"attempt":  connectAttempt,
			})
			if !c.backoff(ctx, connectAttempt) {
				return ctx.Err()
			}
			continue
		}

		consumerLog(map[string]any{
			"consumer": "reconnect_attempt",
			"topic":    topic,
			"attempt":  connectAttempt,
		})

		err := runSession(ctx, cfg, topic, h)
		if err == nil || ctx.Err() != nil {
			// Clean exit (ctx cancelled).
			return ctx.Err()
		}

		// Session ended with an error (Start/Subscribe failed or Receive loop
		// accumulated maxReceiveErrors). Back off and reconnect.
		consumerLog(map[string]any{
			"consumer": "reconnect_failed",
			"topic":    topic,
			"attempt":  connectAttempt,
			"error":    err.Error(),
		})
		if !c.backoff(ctx, connectAttempt) {
			return ctx.Err()
		}
	}
}

// runSession creates one SimpleConsumer, runs the Receive loop until ctx is
// cancelled, a non-recoverable error occurs, or maxReceiveErrors consecutive
// Receive errors accumulate.  A nil return means ctx was cancelled (clean
// exit).  A non-nil return means the session failed and the caller should
// back off and retry.
func (c *Consumer) runSession(ctx context.Context, cfg Config, topic string, h mq.Handler) error {
	// F22 precheck: same rationale as the producer Build — never launch the
	// doomed sc.Start() goroutine against an unreachable endpoint.  See
	// dialReachable docs (and its honest residual note on stalled handshakes).
	if err := dialReachable(ctx, cfg.Endpoint, cfg.requestTimeout()); err != nil {
		return err
	}

	// WithSimpleSubscriptionExpressions is required to populate cli.initTopics so
	// that startUp() triggers a route query at startup.  Without it the SDK's
	// internal telemetry session is never bootstrapped and sc.Start() blocks
	// indefinitely in its "wait for sync settings finish" loop.
	//
	// golang.EnableSsl is set once before any dial; a later Build wanting a
	// different TLS mode is rejected (see setEnableTLS).
	if err := setEnableTLS(cfg.EnableTLS); err != nil {
		return err
	}
	sc, err := golang.NewSimpleConsumer(
		sdkConfig(cfg),
		golang.WithSimpleAwaitDuration(cfg.awaitDuration()),
		golang.WithSimpleSubscriptionExpressions(map[string]*golang.FilterExpression{
			topic: golang.SUB_ALL,
		}),
	)
	if err != nil {
		return fmt.Errorf("rocketmq: NewSimpleConsumer: %w", err)
	}
	sc.SetRequestTimeout(cfg.requestTimeout())

	// Same blocking behavior as producer.Start(): we must bound the wait.
	startErr := make(chan error, 1)
	go func() { startErr <- sc.Start() }()

	timeout := cfg.requestTimeout()
	select {
	case err := <-startErr:
		if err != nil {
			_ = sc.GracefulStop()
			return fmt.Errorf("rocketmq: consumer Start: %w", err)
		}
	case <-ctx.Done():
		_ = sc.GracefulStop()
		return nil // ctx cancelled → clean exit
	case <-time.After(timeout):
		_ = sc.GracefulStop()
		return fmt.Errorf("rocketmq: consumer Start timed out after %v (endpoint unreachable)", timeout)
	}
	defer sc.GracefulStop() //nolint:errcheck

	// Subscribe sends a route-query to the broker; on unreachable endpoint
	// this will fail immediately (bounded by requestTimeout).
	if err := sc.Subscribe(topic, golang.SUB_ALL); err != nil {
		return fmt.Errorf("rocketmq: Subscribe topic %q: %w", topic, err)
	}

	consumerLog(map[string]any{
		"consumer": "reconnect_ok",
		"topic":    topic,
	})

	return c.pump(ctx, sdkReceiver{sc: sc}, topic, h)
}

// toMessage maps one SDK delivery view into an mq.Message: keys[0] becomes Key,
// properties become Headers.  Extracted so the mapping is unit-testable (F2).
func toMessage(d delivery) mq.Message {
	m := mq.Message{
		Topic: d.GetTopic(),
		Body:  d.GetBody(),
	}
	if keys := d.GetKeys(); len(keys) > 0 {
		m.Key = keys[0]
	}
	if props := d.GetProperties(); len(props) > 0 {
		m.Headers = make(map[string]string, len(props))
		for k, v := range props {
			m.Headers[k] = v
		}
	}
	return m
}

// pump runs the resilience core of the consumer: it drives rcv.Receive in a
// loop, maps each delivery into an mq.Message, invokes h, and Acks only on
// handler success.  This is the injectable seam exercised by unit tests (F2).
//
// Return contract (mirrors the caller's expectations):
//   - nil  → ctx was cancelled (clean exit); the outer loop returns ctx.Err().
//   - err  → the session failed; after maxReceiveErrors consecutive Receive
//     errors pump returns non-nil so the outer loop tears down this session and
//     rebuilds the SimpleConsumer (a fresh Start forces a new gRPC connection,
//     which the SDK does not reliably self-heal after a runtime broker drop).
//
// Ack policy: on handler error we intentionally do NOT Ack, letting the broker
// redeliver after the invisibility timeout; on handler success we best-effort
// Ack (an Ack RPC failure is the SDK's concern and does not fail the session).
func (c *Consumer) pump(ctx context.Context, rcv receiver, topic string, h mq.Handler) error {
	const (
		maxMsgNum         = 16
		invisibleDuration = 20 * time.Second
	)

	receiveAttempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil // ctx cancelled → clean exit
		}

		msgs, err := rcv.Receive(ctx, maxMsgNum, invisibleDuration)
		if err != nil {
			// ctx cancelled → clean exit
			if ctx.Err() != nil {
				return nil
			}
			// Consecutive Receive failures: after maxReceiveErrors, abandon
			// this session and let the outer loop rebuild it.
			receiveAttempt++
			consumerLog(map[string]any{
				"consumer":           "receive_error",
				"topic":              topic,
				"attempt":            receiveAttempt,
				"max_before_rebuild": maxReceiveErrors,
				"error":              err.Error(),
			})
			if receiveAttempt >= maxReceiveErrors {
				return fmt.Errorf("rocketmq: %d consecutive Receive errors, rebuilding SimpleConsumer: %w", receiveAttempt, err)
			}
			if !c.backoff(ctx, receiveAttempt) {
				return nil // ctx cancelled during backoff
			}
			continue
		}

		// Successful receive: reset backoff counter
		receiveAttempt = 0

		for _, mv := range msgs {
			m := toMessage(mv)
			// Restore the producer's trace context from the message headers so the
			// handler joins the same trace across the async hop (R9F6).
			hctx := mq.ExtractTrace(ctx, m.Headers)
			if herr := h(hctx, m); herr != nil {
				// Do not Ack on handler error; let invisibility timeout expire
				// so the broker redelivers.
				continue
			}
			// Best-effort Ack; an Ack RPC failure is the SDK's concern.
			_ = rcv.Ack(ctx, mv)
		}
	}
}

// Close is a no-op for Consumer: each Subscribe call manages its own
// SimpleConsumer lifecycle and tears it down on ctx cancellation.
func (c *Consumer) Close() error {
	return nil
}

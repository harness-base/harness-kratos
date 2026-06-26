package rocketmq_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	golang "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/go-kratos/aegis/circuitbreaker"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// traceParentCtx builds a context carrying a sampled SpanContext with the given
// trace/span IDs so mq.InjectTrace has a live trace to serialize (R9F6 tests).
func traceParentCtx(t *testing.T, traceHex, spanHex string) context.Context {
	t.Helper()
	tid, err := oteltrace.TraceIDFromHex(traceHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex(%q): %v", traceHex, err)
	}
	sid, err := oteltrace.SpanIDFromHex(spanHex)
	if err != nil {
		t.Fatalf("SpanIDFromHex(%q): %v", spanHex, err)
	}
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	})
	return oteltrace.ContextWithSpanContext(context.Background(), sc)
}

// traceSpanContextFrom is a thin helper so the test body reads cleanly.
func traceSpanContextFrom(ctx context.Context) oteltrace.SpanContext {
	return oteltrace.SpanContextFromContext(ctx)
}

// sdkLogDir holds the package-level temp directory for SDK logs.
// Set once in TestMain to avoid races from calling ResetLogger mid-test
// (the SDK's sugarBaseLogger is a package-level global; calling ResetLogger
// while a goroutine still uses the previous logger is a data race).
var sdkLogDir string

func TestMain(m *testing.M) {
	// Create a stable temp dir for all tests in this package.
	// (os.MkdirTemp is used instead of t.TempDir because there is no *T here.)
	var err error
	sdkLogDir, err = os.MkdirTemp("", "rmq-test-logs-*")
	if err != nil {
		panic("failed to create SDK log temp dir: " + err.Error())
	}
	os.Setenv(golang.CLIENT_LOG_ROOT, sdkLogDir) //nolint:errcheck
	golang.ResetLogger()
	code := m.Run()
	os.RemoveAll(sdkLogDir) //nolint:errcheck
	os.Exit(code)
}

// redirectSDKLogs is a no-op placeholder; the redirect happens in TestMain.
// Retained for documentation purposes so test functions remain self-describing.
func redirectSDKLogs(t *testing.T) {
	t.Helper()
	// SDK log dir is set once globally in TestMain to avoid races from
	// concurrent goroutines calling ResetLogger.
}

// ─────────────────────────────────────────────────────────────────────────────
// Fingerprint tests
// ─────────────────────────────────────────────────────────────────────────────

func TestFingerprint_Stable(t *testing.T) {
	cfg := rocketmq.Config{
		Endpoint:      "localhost:8081",
		AccessKey:     "ak",
		SecretKey:     "sk",
		ConsumerGroup: "group",
	}
	fp1 := cfg.Fingerprint()
	fp2 := cfg.Fingerprint()
	if fp1 != fp2 {
		t.Fatalf("Fingerprint not stable: %q vs %q", fp1, fp2)
	}
	if fp1 == "" {
		t.Fatal("Fingerprint must not be empty")
	}
}

func TestFingerprint_ChangesOnEndpoint(t *testing.T) {
	base := rocketmq.Config{Endpoint: "a:1", AccessKey: "k", SecretKey: "s", ConsumerGroup: "g"}
	changed := base
	changed.Endpoint = "b:2"
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("fingerprint must change when Endpoint changes")
	}
}

func TestFingerprint_ChangesOnAccessKey(t *testing.T) {
	base := rocketmq.Config{Endpoint: "a:1", AccessKey: "k", SecretKey: "s", ConsumerGroup: "g"}
	changed := base
	changed.AccessKey = "other"
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("fingerprint must change when AccessKey changes")
	}
}

func TestFingerprint_ChangesOnSecretKey(t *testing.T) {
	base := rocketmq.Config{Endpoint: "a:1", AccessKey: "k", SecretKey: "s", ConsumerGroup: "g"}
	changed := base
	changed.SecretKey = "other"
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("fingerprint must change when SecretKey changes")
	}
}

func TestFingerprint_ChangesOnConsumerGroup(t *testing.T) {
	base := rocketmq.Config{Endpoint: "a:1", AccessKey: "k", SecretKey: "s", ConsumerGroup: "g1"}
	changed := base
	changed.ConsumerGroup = "g2"
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("fingerprint must change when ConsumerGroup changes")
	}
}

// TestFingerprint_ChangesOnAwaitDuration and TestFingerprint_ChangesOnRequestTimeout
// pin R9F1+R9F7: both fields are applied to the live SDK at Build time
// (WithSimpleAwaitDuration / SetRequestTimeout), so a hot-reload of either must
// change the fingerprint or the provider would keep serving a client built with
// the OLD value (the version/fingerprint gate sees no change → no rebuild).
//
// Mutation self-proof: drop AwaitDuration (resp. RequestTimeout) from
// Config.Fingerprint's Fprintf and this test FAILS — the two configs collide.
func TestFingerprint_ChangesOnAwaitDuration(t *testing.T) {
	base := rocketmq.Config{Endpoint: "a:1", ConsumerGroup: "g", AwaitDuration: 5 * time.Second}
	changed := base
	changed.AwaitDuration = 10 * time.Second
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("fingerprint must change when AwaitDuration changes (applied via WithSimpleAwaitDuration at Build)")
	}
}

func TestFingerprint_ChangesOnRequestTimeout(t *testing.T) {
	base := rocketmq.Config{Endpoint: "a:1", ConsumerGroup: "g", RequestTimeout: 3 * time.Second}
	changed := base
	changed.RequestTimeout = 1 * time.Second
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("fingerprint must change when RequestTimeout changes (applied via SetRequestTimeout at Build)")
	}
}

func TestFingerprint_DifferentFieldsDifferentValues(t *testing.T) {
	// Sanity: distinct configs must not accidentally collide.
	configs := []rocketmq.Config{
		{Endpoint: "e1", AccessKey: "a1", SecretKey: "s1", ConsumerGroup: "g1"},
		{Endpoint: "e2", AccessKey: "a1", SecretKey: "s1", ConsumerGroup: "g1"},
		{Endpoint: "e1", AccessKey: "a2", SecretKey: "s1", ConsumerGroup: "g1"},
		{Endpoint: "e1", AccessKey: "a1", SecretKey: "s2", ConsumerGroup: "g1"},
		{Endpoint: "e1", AccessKey: "a1", SecretKey: "s1", ConsumerGroup: "g2"},
	}
	seen := make(map[string]struct{})
	for _, c := range configs {
		fp := c.Fingerprint()
		if _, dup := seen[fp]; dup {
			t.Fatalf("fingerprint collision for config %+v", c)
		}
		seen[fp] = struct{}{}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Config validation
// ─────────────────────────────────────────────────────────────────────────────

func TestPublisher_EmptyEndpoint_ReturnsError(t *testing.T) {
	redirectSDKLogs(t)

	src := rocketmq.StaticSource(rocketmq.Config{
		// Endpoint intentionally empty
		AccessKey: "ak",
		SecretKey: "sk",
	})
	pub := rocketmq.NewPublisher(src, "")
	defer pub.Close() //nolint:errcheck

	ctx := context.Background()
	err := pub.Publish(ctx, mq.Message{Topic: "test", Body: []byte("hello")})
	if err == nil {
		t.Fatal("expected error for empty Endpoint, got nil")
	}
}

func TestConsumer_EmptyEndpoint_ReturnsError(t *testing.T) {
	src := rocketmq.StaticSource(rocketmq.Config{
		ConsumerGroup: "group",
		// Endpoint intentionally empty
	})
	c := rocketmq.NewConsumer(src, nil)
	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Subscribe(ctx, "topic", func(_ context.Context, _ mq.Message) error { return nil })
	if err == nil {
		t.Fatal("expected error for empty Endpoint, got nil")
	}
}

func TestConsumer_EmptyConsumerGroup_ReturnsError(t *testing.T) {
	src := rocketmq.StaticSource(rocketmq.Config{
		Endpoint: "localhost:8081",
		// ConsumerGroup intentionally empty
	})
	c := rocketmq.NewConsumer(src, nil)
	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Subscribe(ctx, "topic", func(_ context.Context, _ mq.Message) error { return nil })
	if err == nil {
		t.Fatal("expected error for empty ConsumerGroup, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Unreachable endpoint: Publish must return DBUnavailable within bounded time
// ─────────────────────────────────────────────────────────────────────────────

// TestPublisher_UnreachableEndpoint_ReturnsDBUnavailable verifies that
// Publish against a port with no listener returns errs.DBUnavailable within a
// reasonable deadline.
//
// SDK behavior observed (v5.1.3):
//   - NewProducer without WithTopics → Start() does NOT query the route; it
//     returns immediately.
//   - On Send(), getPublishingTopicRouteResult → queryRoute is called, which
//     uses a gRPC dial + QueryRoute RPC bounded by requestTimeout (default 3 s,
//     overridden here to 1 s so the test finishes quickly).
//   - The gRPC dial to 127.0.0.1:1 (nothing listening) typically fails within
//     the requestTimeout via "connection refused" or dial timeout.
//
// Threshold: 5 s (= 1 s requestTimeout × 3 maxAttempts + buffer).
func TestPublisher_UnreachableEndpoint_ReturnsDBUnavailable(t *testing.T) {
	redirectSDKLogs(t)

	cfg := rocketmq.Config{
		Endpoint:       "127.0.0.1:1", // port 1 is always unreachable
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 500 * time.Millisecond, // short timeout to keep test fast
	}
	src := rocketmq.StaticSource(cfg)
	pub := rocketmq.NewPublisher(src, "")
	defer pub.Close() //nolint:errcheck

	// Threshold: requestTimeout(500ms) × maxAttempts(3) + buffer → 5 s is safe.
	// In practice gRPC "connection refused" is near-instant on localhost.
	const deadline = 5 * time.Second
	start := time.Now()

	ctx := context.Background()
	err := pub.Publish(ctx, mq.Message{Topic: "test-topic", Body: []byte("x")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for unreachable endpoint, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got: %v", err)
	}
	if elapsed > deadline {
		t.Fatalf("Publish took %v, expected < %v; SDK may be retrying indefinitely", elapsed, deadline)
	}
	t.Logf("Publish returned DBUnavailable in %v: %v", elapsed, err)
}

// stubBreaker is a deterministic circuitbreaker.CircuitBreaker for tests:
// when allow is set it rejects every Allow() with ErrNotAllowed (open circuit),
// otherwise it permits. It records Allow/MarkSuccess/MarkFailed call counts so
// tests can assert the breaker was actually consulted. (Pattern mirrors the
// data layer's errAlwaysBreaker.)
type stubBreaker struct {
	mu          sync.Mutex
	open        bool
	allowCalls  int
	failedCalls int
	okCalls     int
}

func (b *stubBreaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.allowCalls++
	if b.open {
		return circuitbreaker.ErrNotAllowed
	}
	return nil
}

func (b *stubBreaker) MarkSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.okCalls++
}

func (b *stubBreaker) MarkFailed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failedCalls++
}

// TestPublisher_OpenCircuit_FastFailsWithoutDial verifies that when the breaker
// is open, Publish fails fast with DBUnavailable WITHOUT touching the network
// (no dial, no Send). We drive a deterministic open-state stub breaker rather
// than relying on the probabilistic sre breaker, so the assertions below are
// real, not best-effort logs.
func TestPublisher_OpenCircuit_FastFailsWithoutDial(t *testing.T) {
	redirectSDKLogs(t)

	cfg := rocketmq.Config{
		Endpoint:  "127.0.0.1:1", // unreachable: proves we never dial when open
		AccessKey: "ak",
		SecretKey: "sk",
		// A long RequestTimeout makes the point sharp: if Publish were to dial,
		// the call would take well over the assertion bound below. It must not.
		RequestTimeout: 5 * time.Second,
	}
	src := rocketmq.StaticSource(cfg)

	br := &stubBreaker{open: true}
	pub := rocketmq.NewPublisherWithBreaker(src, "", br)
	defer pub.Close() //nolint:errcheck

	start := time.Now()
	err := pub.Publish(context.Background(), mq.Message{Topic: "t", Body: []byte("x")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when circuit is open, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable from open circuit, got: %v", err)
	}
	// Hard bound: an open-circuit reject is a map/atomic op with no network hop.
	// Far below the 5 s RequestTimeout a dial would have cost.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("open circuit Publish took %v; expected near-instant fast-fail (it must not dial)", elapsed)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected exactly 1 Allow() call, got %d", br.allowCalls)
	}
	// When Allow() rejects we must NOT have attempted the send path, so neither
	// MarkSuccess nor MarkFailed should have fired.
	if br.failedCalls != 0 || br.okCalls != 0 {
		t.Fatalf("open circuit must short-circuit before send: failed=%d ok=%d (want 0/0)", br.failedCalls, br.okCalls)
	}
}

// TestPublisher_EmptyTopic_InvalidArgumentBeforeBreaker pins the R11F6
// consistency fix: an empty Topic is a caller wiring bug, not a broker fault, so
// Publish guards it up front and returns InvalidArgument (HTTP 400) — matching
// the rabbitmq adapter's symmetric guard.
//
// The guard must sit BEFORE breaker.Allow(): we inject a stub breaker (closed,
// so it WOULD permit a dial if reached) against an unreachable endpoint and
// hard-assert the breaker was never consulted (allowCalls == 0). A guard placed
// after the breaker would still return an error but would have called Allow()
// first — so allowCalls == 0 is the load-bearing assertion that proves ordering.
//
// Mutation self-proof: delete the `if m.Topic == ""` guard and Publish falls
// through to breaker.Allow() + provider.Get (which dials 127.0.0.1:1) — the
// returned error becomes DBUnavailable, not InvalidArgument, and allowCalls
// becomes 1, failing both the IsInvalidArgument and allowCalls==0 assertions.
func TestPublisher_EmptyTopic_InvalidArgumentBeforeBreaker(t *testing.T) {
	redirectSDKLogs(t)

	cfg := rocketmq.Config{
		Endpoint: "127.0.0.1:1", // unreachable: a dial (if the guard were missing) is obvious
		// A long timeout makes the point sharp: if Publish fell through to the
		// dial path, it would take well over the near-instant guard return.
		RequestTimeout: 5 * time.Second,
	}
	src := rocketmq.StaticSource(cfg)

	br := &stubBreaker{open: false} // closed: would permit a dial if reached
	pub := rocketmq.NewPublisherWithBreaker(src, "", br)
	defer pub.Close() //nolint:errcheck

	start := time.Now()
	err := pub.Publish(context.Background(), mq.Message{Topic: "", Body: []byte("x")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for empty topic, got nil (must not silently drop)")
	}
	if !errs.IsInvalidArgument(err) {
		t.Fatalf("expected InvalidArgument for empty topic, got %T: %v", err, err)
	}
	if code := errs.FromError(err).Code; code != 400 {
		t.Fatalf("expected HTTP 400 for empty topic, got %d", code)
	}
	// The guard return is a pure string check — no network hop. If it fell through
	// to the dial against 127.0.0.1:1 this would be far slower.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("empty-topic guard took %v; expected near-instant (it must not dial)", elapsed)
	}
	// The crux: the guard must precede breaker.Allow(). allowCalls==0 distinguishes
	// "guard first" from "guard after the breaker".
	if br.allowCalls != 0 {
		t.Fatalf("empty-topic guard must fire before breaker.Allow(): got allowCalls=%d, want 0", br.allowCalls)
	}
	if br.okCalls != 0 || br.failedCalls != 0 {
		t.Fatalf("guard must not touch the send path: ok=%d failed=%d, want 0/0", br.okCalls, br.failedCalls)
	}
}

// TestPublisher_ClosedCircuit_MarksFailedOnUnreachable verifies the
// complementary path: with a permitting (closed) breaker against an unreachable
// endpoint, Publish attempts the send, fails, and reports the failure to the
// breaker via MarkFailed (driving it toward open). This proves the breaker is
// genuinely wired into the send path, not bypassed.
func TestPublisher_ClosedCircuit_MarksFailedOnUnreachable(t *testing.T) {
	redirectSDKLogs(t)

	cfg := rocketmq.Config{
		Endpoint:       "127.0.0.1:1",
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 300 * time.Millisecond,
	}
	src := rocketmq.StaticSource(cfg)

	br := &stubBreaker{open: false}
	pub := rocketmq.NewPublisherWithBreaker(src, "", br)
	defer pub.Close() //nolint:errcheck

	err := pub.Publish(context.Background(), mq.Message{Topic: "t", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got: %v", err)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected exactly 1 Allow() call, got %d", br.allowCalls)
	}
	if br.failedCalls != 1 {
		t.Fatalf("expected exactly 1 MarkFailed() on unreachable send, got %d", br.failedCalls)
	}
	if br.okCalls != 0 {
		t.Fatalf("expected 0 MarkSuccess() on failed send, got %d", br.okCalls)
	}
}

// TestPublisher_CallerCanceled_DoesNotMarkFailed verifies the R10F4 fix: when the
// failure stems from the CALLER cancelling ctx (client disconnect), Publish still
// returns DBUnavailable but must NOT trip the breaker — neither MarkFailed nor
// MarkSuccess — so a burst of client disconnects cannot open the circuit against
// a healthy broker.
//
// The cancelled caller ctx propagates into the derived sendCtx, so the very first
// broker touch (provider.Get → dialReachable) fails with a context.Canceled
// error; markBackendFailure inspects the caller ctx (Canceled) and skips the
// breaker. This is the mutation-self-proving partner of
// TestPublisher_ClosedCircuit_MarksFailedOnUnreachable (which proves a non-cancel
// error DOES MarkFailed exactly once): if the guard were removed, failedCalls
// here would become 1 and the test fails.
func TestPublisher_CallerCanceled_DoesNotMarkFailed(t *testing.T) {
	redirectSDKLogs(t)

	cfg := rocketmq.Config{
		Endpoint:  "127.0.0.1:1", // unreachable; but we never get that far
		AccessKey: "ak",
		SecretKey: "sk",
		// Long timeout so that, were the breaker (wrongly) tripped via a real
		// dial/timeout path, this would be a slow DeadlineExceeded failure rather
		// than the fast Canceled path we are asserting.
		RequestTimeout: 5 * time.Second,
	}
	src := rocketmq.StaticSource(cfg)

	br := &stubBreaker{open: false}
	pub := rocketmq.NewPublisherWithBreaker(src, "", br)
	defer pub.Close() //nolint:errcheck

	// An already-cancelled caller ctx simulates a client that disconnected before
	// the publish completed.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := pub.Publish(ctx, mq.Message{Topic: "t", Body: []byte("x")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for cancelled caller ctx, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got: %v", err)
	}
	// The cancellation short-circuits before any real dial completes, so this must
	// be fast — well under the 5 s RequestTimeout.
	if elapsed > time.Second {
		t.Fatalf("cancelled-ctx Publish took %v; expected fast cancellation, not a dial/timeout", elapsed)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected exactly 1 Allow() call, got %d", br.allowCalls)
	}
	// The crux: a caller cancellation is NOT a broker fault.
	if br.failedCalls != 0 {
		t.Fatalf("caller cancellation must NOT MarkFailed, got %d MarkFailed calls", br.failedCalls)
	}
	if br.okCalls != 0 {
		t.Fatalf("caller cancellation must NOT MarkSuccess, got %d MarkSuccess calls", br.okCalls)
	}
}

// settableSource is a resource.Source whose Config can be swapped at runtime, so
// a test can simulate a hot-reload and assert the consumer/publisher re-reads it.
type settableSource struct {
	mu  sync.Mutex
	cfg rocketmq.Config
}

func (s *settableSource) set(c rocketmq.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = c
}

func (s *settableSource) Current() resource.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return resource.Snapshot{Version: 1, Value: s.cfg}
}

// TestPublisher_LiveRequestTimeout_FollowsHotReload is the R9F3 load-bearing
// test: Publish must derive its ctx bound from the LIVE RequestTimeout snapshot,
// not a value frozen at NewPublisher time. We build the publisher with a 5 s
// startup timeout, then hot-reload the source to 200 ms and assert the live read
// reflects the new value.
//
// Mutation self-proof: revert the Publisher to cache requestTimeout at
// construction (the original bug) and PublisherLiveRequestTimeout would keep
// returning the 5 s startup value, failing the want-200ms assertion below.
func TestPublisher_LiveRequestTimeout_FollowsHotReload(t *testing.T) {
	src := &settableSource{cfg: rocketmq.Config{
		Endpoint:       "127.0.0.1:1",
		RequestTimeout: 5 * time.Second, // startup value
	}}
	pub := rocketmq.NewPublisher(src, "")
	defer pub.Close() //nolint:errcheck

	if got := rocketmq.PublisherLiveRequestTimeout(pub); got != 5*time.Second {
		t.Fatalf("startup live request timeout = %v, want 5s", got)
	}

	// Hot-reload to a shorter timeout.
	src.set(rocketmq.Config{Endpoint: "127.0.0.1:1", RequestTimeout: 200 * time.Millisecond})

	if got := rocketmq.PublisherLiveRequestTimeout(pub); got != 200*time.Millisecond {
		t.Fatalf("after hot-reload live request timeout = %v, want 200ms (R9F3 regression: value frozen at construction)", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Consumer: ctx cancel → Subscribe returns cleanly
// ─────────────────────────────────────────────────────────────────────────────

// TestConsumer_CtxCancel_ReturnsCleanly verifies that cancelling ctx causes
// Subscribe to return ctx.Err() rather than hanging.
//
// Because 127.0.0.1:1 is unreachable, sc.Subscribe(topic) will fail with a
// route-query error, so Subscribe returns that error before entering the loop.
// This test therefore exercises early-return on Subscribe failure with a
// short context.
func TestConsumer_CtxCancel_ReturnsCleanly(t *testing.T) {
	redirectSDKLogs(t)

	cfg := rocketmq.Config{
		Endpoint:       "127.0.0.1:1",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ConsumerGroup:  "grp",
		RequestTimeout: 200 * time.Millisecond,
		AwaitDuration:  100 * time.Millisecond,
	}
	src := rocketmq.StaticSource(cfg)
	c := rocketmq.NewConsumer(src, nil)
	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Subscribe(ctx, "topic", func(_ context.Context, _ mq.Message) error { return nil })
	if err == nil {
		t.Fatal("expected Subscribe to return an error for unreachable endpoint")
	}
	// Either the context deadline or a connection error is acceptable.
	t.Logf("Subscribe returned (expected): %v", err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Consumer: injected Backoff is called on Receive errors
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Pump loop (resilience core) — driven with a fake receiver, real assertions
// ─────────────────────────────────────────────────────────────────────────────

// fakeDelivery is an in-memory rocketmq.Delivery for pump-loop tests.
type fakeDelivery struct {
	topic string
	body  []byte
	keys  []string
	props map[string]string
}

func (d fakeDelivery) GetTopic() string                 { return d.topic }
func (d fakeDelivery) GetBody() []byte                  { return d.body }
func (d fakeDelivery) GetKeys() []string                { return d.keys }
func (d fakeDelivery) GetProperties() map[string]string { return d.props }

// fakeReceiver is a scripted rocketmq.Receiver. Each Receive call returns the
// next (batch, err) from the script; once the script is exhausted it returns a
// generic error so the pump's consecutive-error path keeps firing. acked
// records every delivery passed to Ack so tests can assert ack behaviour.
type fakeReceiver struct {
	mu      sync.Mutex
	batches [][]rocketmq.Delivery
	errs    []error
	calls   int
	acked   []rocketmq.Delivery
	ackErr  error
}

func (r *fakeReceiver) Receive(_ context.Context, _ int32, _ time.Duration) ([]rocketmq.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.calls
	r.calls++
	if i < len(r.batches) {
		return r.batches[i], r.errs[i]
	}
	return nil, errors.New("fakeReceiver: script exhausted")
}

func (r *fakeReceiver) Ack(_ context.Context, d rocketmq.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acked = append(r.acked, d)
	return r.ackErr
}

// TestPump_ConsecutiveReceiveErrors_TriggersRebuild verifies the resilience
// core: after maxReceiveErrors consecutive Receive errors the pump returns a
// non-nil error so the outer reconnect loop tears down and rebuilds the
// session. (F2)
func TestPump_ConsecutiveReceiveErrors_TriggersRebuild(t *testing.T) {
	rcv := &fakeReceiver{} // empty script → every Receive errors
	ctx := context.Background()

	err := rocketmq.RunPump(ctx, rcv, "t", func(context.Context, mq.Message) error { return nil }, rocketmq.NoBackoff)
	if err == nil {
		t.Fatal("expected non-nil error after consecutive Receive errors (drives rebuild)")
	}
	// The pump must give up at exactly the threshold, not spin forever.
	if rcv.calls != rocketmq.MaxReceiveErrors {
		t.Fatalf("expected exactly %d Receive calls before rebuild, got %d", rocketmq.MaxReceiveErrors, rcv.calls)
	}
}

// TestPump_TransientErrorThenRecovery_NoRebuild verifies the consecutive-error
// counter is RESET on a successful Receive, not merely "stayed below threshold".
//
// R2F11: the previous script (err,err,good) only ever produced 2 errors — below
// the threshold of 3 — so it would pass even if the counter never reset. To make
// the reset semantics load-bearing we script err,err,good,err,err,good with the
// SECOND good batch cancelling. Four errors flow through, but they straddle the
// first good: 2 before it and 2 after it. Without a reset the counter would hit
// 3 on the fourth error and the pump would return non-nil (rebuild). The pump
// must instead absorb all four and exit cleanly on ctx-cancel — proving reset.
func TestPump_TransientErrorThenRecovery_NoRebuild(t *testing.T) {
	good := fakeDelivery{topic: "t", body: []byte("ok")}
	rcv := &fakeReceiver{
		// err, err, good, err, err, good — 4 errors total but split 2/2 around
		// the first good batch, so the consecutive count never reaches 3 IFF the
		// good batch resets it.
		batches: [][]rocketmq.Delivery{nil, nil, {good}, nil, nil, {good}},
		errs: []error{
			errors.New("transient"), errors.New("transient"), nil,
			errors.New("transient"), errors.New("transient"), nil,
		},
	}

	var handled atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	h := func(context.Context, mq.Message) error {
		// Cancel only on the SECOND recovery delivery so the full
		// err,err,good,err,err,good sequence is consumed.
		if handled.Add(1) == 2 {
			cancel()
		}
		return nil
	}

	err := rocketmq.RunPump(ctx, rcv, "t", h, rocketmq.NoBackoff)
	if err != nil {
		t.Fatalf("expected nil (clean ctx-cancel exit); a non-nil rebuild error means the counter did NOT reset: %v", err)
	}
	// All six scripted Receive calls must have run (4 errors + 2 goods). If the
	// counter failed to reset, the pump would have returned at call 4 (the third
	// consecutive error counting from the start) and rcv.calls would be < 6.
	if rcv.calls != 6 {
		t.Fatalf("expected exactly 6 Receive calls (err,err,good,err,err,good), got %d — counter likely not reset", rcv.calls)
	}
	if handled.Load() != 2 {
		t.Fatalf("expected both recovery messages handled, got %d", handled.Load())
	}
	if len(rcv.acked) != 2 {
		t.Fatalf("expected both recovered messages acked, got %d", len(rcv.acked))
	}
}

// TestPump_HandlerError_SkipsAck verifies that when the handler returns an
// error the message is NOT acked (so the broker redelivers after the
// invisibility timeout). (F2)
func TestPump_HandlerError_SkipsAck(t *testing.T) {
	bad := fakeDelivery{topic: "t", body: []byte("boom")}
	rcv := &fakeReceiver{
		batches: [][]rocketmq.Delivery{{bad}},
		errs:    []error{nil},
	}

	var handled atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	h := func(context.Context, mq.Message) error {
		handled.Add(1)
		cancel()
		return errors.New("handler failed")
	}

	err := rocketmq.RunPump(ctx, rcv, "t", h, rocketmq.NoBackoff)
	if err != nil {
		t.Fatalf("expected nil (ctx-cancel) exit, got: %v", err)
	}
	if handled.Load() != 1 {
		t.Fatalf("expected handler to be called once, got %d", handled.Load())
	}
	if len(rcv.acked) != 0 {
		t.Fatalf("handler error must skip Ack, but %d message(s) were acked", len(rcv.acked))
	}
}

// TestPump_SuccessfulMessage_MapsAndAcks verifies the normal path: keys and
// properties are mapped into mq.Message correctly and the message is acked. (F2)
func TestPump_SuccessfulMessage_MapsAndAcks(t *testing.T) {
	d := fakeDelivery{
		topic: "orders",
		body:  []byte("payload"),
		keys:  []string{"order-42", "ignored-second-key"},
		props: map[string]string{"trace": "abc", "content-type": "json"},
	}
	rcv := &fakeReceiver{
		batches: [][]rocketmq.Delivery{{d}},
		errs:    []error{nil},
	}

	var got mq.Message
	var handled atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	h := func(_ context.Context, m mq.Message) error {
		got = m
		handled.Add(1)
		cancel()
		return nil
	}

	err := rocketmq.RunPump(ctx, rcv, "orders", h, rocketmq.NoBackoff)
	if err != nil {
		t.Fatalf("expected nil (ctx-cancel) exit, got: %v", err)
	}
	if handled.Load() != 1 {
		t.Fatalf("expected exactly 1 handled message, got %d", handled.Load())
	}

	// Field mapping assertions.
	if got.Topic != "orders" {
		t.Errorf("Topic: want %q got %q", "orders", got.Topic)
	}
	if string(got.Body) != "payload" {
		t.Errorf("Body: want %q got %q", "payload", string(got.Body))
	}
	if got.Key != "order-42" {
		t.Errorf("Key: want first key %q got %q", "order-42", got.Key)
	}
	if got.Headers["trace"] != "abc" || got.Headers["content-type"] != "json" {
		t.Errorf("Headers not mapped from properties: got %+v", got.Headers)
	}

	// Ack assertions: exactly the delivery we handled, acked once.
	if len(rcv.acked) != 1 {
		t.Fatalf("expected exactly 1 ack, got %d", len(rcv.acked))
	}
	if rcv.acked[0].GetTopic() != "orders" || string(rcv.acked[0].GetBody()) != "payload" {
		t.Errorf("acked the wrong delivery: %+v", rcv.acked[0])
	}
}

// TestBuildMessage_InjectsTrace is the R9F6 publisher-side test: the publish
// path (buildMessage) must inject the current span's W3C traceparent into the SDK
// message properties, alongside the business key and headers, so the consumer can
// recover the trace. Pure helper, no broker.
//
// Mutation self-proof: drop the mq.InjectTrace call in buildMessage (write only
// m.Headers) and the traceparent property is absent → the IsValid/trace-id
// assertions on the extracted SpanContext below FAIL.
func TestBuildMessage_InjectsTrace(t *testing.T) {
	const traceHex = "abcdef0123456789abcdef0123456789"
	const spanHex = "abcdef0123456789"

	producerCtx := traceParentCtx(t, traceHex, spanHex)
	m := mq.Message{
		Topic:   "orders",
		Key:     "order-7",
		Body:    []byte("p"),
		Headers: map[string]string{"content-type": "json"},
	}

	keys, props := rocketmq.BuildMessageProps(producerCtx, m)

	if len(keys) != 1 || keys[0] != "order-7" {
		t.Fatalf("keys = %v, want [order-7]", keys)
	}
	if props["content-type"] != "json" {
		t.Errorf("business header dropped: %+v", props)
	}
	if _, ok := props["traceparent"]; !ok {
		t.Fatalf("buildMessage did not inject a traceparent property (R9F6): %+v", props)
	}
	// And the injected traceparent must round-trip to the producer's trace id.
	sc := traceSpanContextFrom(mq.ExtractTrace(context.Background(), props))
	if !sc.IsValid() {
		t.Fatal("injected traceparent did not produce a valid SpanContext on extract")
	}
	if sc.TraceID().String() != traceHex {
		t.Fatalf("injected trace id = %q, want %q", sc.TraceID(), traceHex)
	}
	if sc.SpanID().String() != spanHex {
		t.Fatalf("injected parent span id = %q, want %q", sc.SpanID(), spanHex)
	}
	// The caller's headers map must not have been mutated by the inject.
	if _, leaked := m.Headers["traceparent"]; leaked {
		t.Error("buildMessage mutated the caller's Message.Headers (leaked traceparent)")
	}
}

// TestPump_ExtractsTraceIntoHandlerCtx is the R9F6 consumer-side test for the
// real rocketmq pump: a delivery whose properties carry a W3C traceparent must
// produce a handler ctx that joins that trace. We assert the SpanContext seen by
// the handler carries the producer's trace id.
//
// Mutation self-proof: revert pump to call h(ctx, m) instead of
// h(extractTrace(ctx, m.Headers), m) and the handler ctx has no span context,
// failing the IsValid/trace-id assertions below.
func TestPump_ExtractsTraceIntoHandlerCtx(t *testing.T) {
	const traceHex = "11223344556677889900aabbccddeeff"
	const spanHex = "1122334455667788"

	// Build a traceparent the way a producer would (mq.InjectTrace), then ship it
	// as a delivery property so the pump's toMessage→Headers→extract path runs.
	producerCtx := traceParentCtx(t, traceHex, spanHex)
	props := mq.InjectTrace(producerCtx, nil)
	if _, ok := props["traceparent"]; !ok {
		t.Fatalf("test setup: InjectTrace produced no traceparent: %+v", props)
	}

	d := fakeDelivery{topic: "t", body: []byte("b"), props: props}
	rcv := &fakeReceiver{
		batches: [][]rocketmq.Delivery{{d}},
		errs:    []error{nil},
	}

	var gotTrace string
	var gotRemoteParent string
	var handled atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	h := func(hctx context.Context, _ mq.Message) error {
		sc := traceSpanContextFrom(hctx)
		gotTrace = sc.TraceID().String()
		gotRemoteParent = sc.SpanID().String()
		handled.Add(1)
		cancel()
		return nil
	}

	if err := rocketmq.RunPump(ctx, rcv, "t", h, rocketmq.NoBackoff); err != nil {
		t.Fatalf("expected clean ctx-cancel exit, got %v", err)
	}
	if handled.Load() != 1 {
		t.Fatalf("expected handler called once, got %d", handled.Load())
	}
	if gotTrace != traceHex {
		t.Fatalf("handler ctx trace id = %q, want %q (pump did not extract trace into handler ctx — R9F6)", gotTrace, traceHex)
	}
	if gotRemoteParent != spanHex {
		t.Fatalf("handler ctx parent span id = %q, want %q", gotRemoteParent, spanHex)
	}
}

// TestToMessage_NoKeysNoProps verifies the mapping leaves Key empty and Headers
// nil when the delivery carries neither, matching the original behaviour. (F2)
func TestToMessage_NoKeysNoProps(t *testing.T) {
	m := rocketmq.ToMessage(fakeDelivery{topic: "t", body: []byte("b")})
	if m.Key != "" {
		t.Errorf("expected empty Key, got %q", m.Key)
	}
	if m.Headers != nil {
		t.Errorf("expected nil Headers, got %+v", m.Headers)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Config defaulting accessors (awaitDuration / requestTimeout) — R2F9
// ─────────────────────────────────────────────────────────────────────────────

// TestConfig_AwaitDuration_Defaulting covers Config.awaitDuration: a non-positive
// value falls back to the 5 s default, a positive value is returned unchanged.
// The <=0 default branch was previously uncovered (R2F9); the twin
// requestTimeout accessor is asserted the same way for symmetry.
func TestConfig_AwaitDuration_Defaulting(t *testing.T) {
	const def = 5 * time.Second
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero->default", 0, def},
		{"negative->default", -1 * time.Second, def},
		{"positive->passthrough", 2 * time.Second, 2 * time.Second},
		{"one-ns->passthrough", time.Nanosecond, time.Nanosecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rocketmq.AwaitDurationOf(rocketmq.Config{AwaitDuration: tc.in})
			if got != tc.want {
				t.Fatalf("awaitDuration(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestConfig_RequestTimeout_Defaulting mirrors the awaitDuration table for the
// twin accessor: <=0 falls back to the 3 s SDK default, positive passes through.
func TestConfig_RequestTimeout_Defaulting(t *testing.T) {
	const def = 3 * time.Second
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero->default", 0, def},
		{"negative->default", -5 * time.Second, def},
		{"positive->passthrough", 750 * time.Millisecond, 750 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rocketmq.RequestTimeoutOf(rocketmq.Config{RequestTimeout: tc.in})
			if got != tc.want {
				t.Fatalf("requestTimeout(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// EnableSsl set-once + reject-divergence — R2F21
// ─────────────────────────────────────────────────────────────────────────────

// TestSetEnableTLS_SetOnceThenRejectDivergence asserts the R2F21 contract that
// replaced R1's buildMu: the SDK's process-global golang.EnableSsl is written
// exactly once (by the first Build) and a later Build that wants a DIFFERENT TLS
// mode is rejected with an error rather than silently re-writing the global mid
// process (which is the race R1 failed to fix). A later Build with the SAME mode
// is a no-op success.
//
// Note: these subtests mutate process-wide state (the once-flag + the SDK
// global), so they run sequentially and ResetEnableTLS between cases — they must
// not use t.Parallel.
func TestSetEnableTLS_SetOnceThenRejectDivergence(t *testing.T) {
	// Restore a clean default-mode (false) state for any later test that builds a
	// real client with EnableTLS defaulting to false; these subtests mutate the
	// process-wide once-flag and SDK global.
	t.Cleanup(func() {
		rocketmq.ResetEnableTLS()
		_ = rocketmq.SetEnableTLS(false)
	})

	t.Run("first-write-true-applies-to-global", func(t *testing.T) {
		rocketmq.ResetEnableTLS()
		if err := rocketmq.SetEnableTLS(true); err != nil {
			t.Fatalf("first SetEnableTLS(true) must succeed, got %v", err)
		}
		if !rocketmq.EnableSslGlobal() {
			t.Fatal("first Build must write the SDK global golang.EnableSsl=true")
		}
	})

	t.Run("first-write-false-applies-to-global", func(t *testing.T) {
		rocketmq.ResetEnableTLS()
		if err := rocketmq.SetEnableTLS(false); err != nil {
			t.Fatalf("first SetEnableTLS(false) must succeed, got %v", err)
		}
		if rocketmq.EnableSslGlobal() {
			t.Fatal("first Build must write the SDK global golang.EnableSsl=false")
		}
	})

	t.Run("same-mode-later-build-is-noop-success", func(t *testing.T) {
		rocketmq.ResetEnableTLS()
		if err := rocketmq.SetEnableTLS(true); err != nil {
			t.Fatalf("first SetEnableTLS(true): %v", err)
		}
		// A second, third Build with the same mode must all succeed and leave the
		// global untouched (true).
		for i := 0; i < 3; i++ {
			if err := rocketmq.SetEnableTLS(true); err != nil {
				t.Fatalf("same-mode SetEnableTLS(true) #%d must succeed, got %v", i, err)
			}
		}
		if !rocketmq.EnableSslGlobal() {
			t.Fatal("same-mode rebuilds must not flip the global")
		}
	})

	t.Run("divergent-mode-later-build-is-rejected-global-unchanged", func(t *testing.T) {
		rocketmq.ResetEnableTLS()
		if err := rocketmq.SetEnableTLS(false); err != nil {
			t.Fatalf("first SetEnableTLS(false): %v", err)
		}
		// A later Build wanting TLS=true must be rejected (SDK can't mix modes in
		// one process), and crucially must NOT have flipped the global — that
		// silent flip mid-process is the exact race we are preventing.
		err := rocketmq.SetEnableTLS(true)
		if err == nil {
			t.Fatal("divergent SetEnableTLS(true) after first(false) must return an error")
		}
		if rocketmq.EnableSslGlobal() {
			t.Fatal("rejected divergent Build must NOT flip the SDK global (race prevention)")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Outer reconnect loop (Subscribe) driven with a scripted session runner — R2F7
// ─────────────────────────────────────────────────────────────────────────────

// reconnectCfg is a minimal valid config so Subscribe passes its guards and
// enters the outer reconnect loop. The Endpoint is never dialled because the
// session runner is injected.
func reconnectCfg() rocketmq.StaticSource {
	return rocketmq.StaticSource(rocketmq.Config{
		Endpoint:      "injected:0", // never dialled — session runner is faked
		ConsumerGroup: "grp",
	})
}

// TestReconnectLoop_RetriesThenStopsWhenBackoffReturnsFalse drives the real
// Subscribe outer loop with a session runner that always fails. It asserts:
//   - the loop reconnects, incrementing the attempt counter monotonically;
//   - backoff is consulted once per failed session, with the attempt number;
//   - the loop stops solely because backoff returned false — NOT because ctx was
//     cancelled. ctx stays live; the only stop signal is the false return value.
//
// This is the deterministic coverage the loop lacked (R2F7). Importantly the
// stop is driven by backoff's return value (not a ctx cancel), so a regression
// that ignored that return value would run an extra session and fail the
// session-count assertion. A hard safety bound cancels ctx only if the loop
// runs far past the stop point, so a broken loop fails fast instead of hanging.
func TestReconnectLoop_RetriesThenStopsWhenBackoffReturnsFalse(t *testing.T) {
	const stopAfter = 3 // 3 sessions fail; backoff returns false after the 3rd
	const safetyBound = 20

	var sessionCalls atomic.Int32
	// Safety net: if the loop ignores backoff=false and keeps going, cancel ctx
	// well past stopAfter so the test fails fast (extra sessions) instead of
	// hanging forever.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := func(_ context.Context, _ rocketmq.Config, _ string, _ mq.Handler) error {
		if sessionCalls.Add(1) >= safetyBound {
			cancel()
		}
		return errors.New("session failed (injected)")
	}

	var backoffAttempts []int
	backoff := mq.Backoff(func(_ context.Context, attempt int) bool {
		backoffAttempts = append(backoffAttempts, attempt)
		// Stop by RETURN VALUE only — do not cancel ctx here.
		return attempt < stopAfter
	})

	err := rocketmq.RunReconnectLoop(ctx, reconnectCfg(), "topic", nil, backoff, session)
	// backoff returned false without cancelling ctx, so the loop returns
	// ctx.Err() which is nil at that point — the documented stop-on-backoff path.
	if err != nil {
		t.Fatalf("expected nil when backoff returns false with live ctx, got %v", err)
	}
	// Exactly stopAfter sessions ran. If the loop ignored backoff=false it would
	// have run more (up to the safety bound) — this is the load-bearing check.
	if got := sessionCalls.Load(); got != stopAfter {
		t.Fatalf("expected exactly %d session attempts (loop must stop on backoff=false), got %d", stopAfter, got)
	}
	// Backoff was consulted once per failed session, attempt = 1,2,3 in order.
	want := []int{1, 2, 3}
	if len(backoffAttempts) != len(want) {
		t.Fatalf("expected backoff called %d times, got %d (%v)", len(want), len(backoffAttempts), backoffAttempts)
	}
	for i := range want {
		if backoffAttempts[i] != want[i] {
			t.Fatalf("backoff attempt[%d] = %d, want %d (counter not monotonic)", i, backoffAttempts[i], want[i])
		}
	}
}

// TestReconnectLoop_RereadsConfigEachAttempt is the R9F2 load-bearing test: the
// outer reconnect loop must re-read the config snapshot at the top of EVERY
// attempt, so a hot-reloaded Endpoint reaches the next session. The previous
// (buggy) code froze cfg once before the loop, so every session saw the startup
// Endpoint forever.
//
// Setup: a settableSource starts at Endpoint "old:1". The first session records
// the endpoint it was handed, then hot-reloads the source to "new:2" and FAILS
// (forcing a reconnect). The second session records its endpoint and cancels ctx
// for a clean exit. We assert the second session saw "new:2" — only possible if
// the loop re-read the snapshot.
//
// Mutation self-proof: revert Subscribe to read `cfg := cfgFrom(...)` once before
// the for loop (the original bug) and the second session would still see "old:1",
// failing the want-"new:2" assertion below. (Verified by mutation in the change.)
func TestReconnectLoop_RereadsConfigEachAttempt(t *testing.T) {
	src := &settableSource{cfg: rocketmq.Config{Endpoint: "old:1", ConsumerGroup: "grp"}}

	var seen []string
	var mu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sessionCalls atomic.Int32
	session := func(_ context.Context, cfg rocketmq.Config, _ string, _ mq.Handler) error {
		mu.Lock()
		seen = append(seen, cfg.Endpoint)
		mu.Unlock()
		n := sessionCalls.Add(1)
		if n == 1 {
			// Hot-reload BETWEEN sessions, then fail to force a reconnect.
			src.set(rocketmq.Config{Endpoint: "new:2", ConsumerGroup: "grp"})
			return errors.New("first session fails to force reconnect")
		}
		cancel() // second session: clean exit
		return nil
	}

	backoff := mq.Backoff(func(context.Context, int) bool { return true })

	err := rocketmq.RunReconnectLoop(ctx, src, "topic", nil, backoff, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled after clean second session, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("expected exactly 2 sessions, got %d (endpoints seen: %v)", len(seen), seen)
	}
	if seen[0] != "old:1" {
		t.Fatalf("first session endpoint = %q, want %q", seen[0], "old:1")
	}
	// The load-bearing assertion: the second session must see the hot-reloaded
	// endpoint, proving the loop re-reads cfg each attempt (R9F2).
	if seen[1] != "new:2" {
		t.Fatalf("second session endpoint = %q, want %q (loop did NOT re-read config — R9F2 regression)", seen[1], "new:2")
	}
}

// TestReconnectLoop_TransientInvalidSnapshot_BacksOffNotTerminate is the second
// half of R9F2: a momentarily empty/invalid snapshot (e.g. config-center
// returned a blank during a reload) must be treated as transient — back off and
// retry — NOT terminate the supervised consumer. Terminating would kill the only
// consumer over a transient blip with no recovery.
//
// Setup: the startup snapshot is valid (so Subscribe enters the loop). The first
// backoff call flips the source to a blank-endpoint Config (invalid) so the NEXT
// loop iteration sees an invalid snapshot and must back off again; the second
// backoff flips it back to a valid Config so a session finally runs and cancels
// ctx. We assert the session ran exactly once on the recovered endpoint and that
// the loop backed off (rather than returning an error) over the invalid window.
//
// Mutation self-proof: change the in-loop invalid-snapshot branch to `return ...`
// instead of `backoff;continue` and Subscribe would terminate when the invalid
// snapshot appears — the recovered session would never run (its endpoint would
// not be "recovered:2"), failing the assertions below.
func TestReconnectLoop_TransientInvalidSnapshot_BacksOffNotTerminate(t *testing.T) {
	// Start valid so Subscribe's startup guard passes and we enter the loop.
	src := &settableSource{cfg: rocketmq.Config{Endpoint: "start:1", ConsumerGroup: "grp"}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sessionCalls atomic.Int32
	var sessionEndpoint string
	session := func(_ context.Context, cfg rocketmq.Config, _ string, _ mq.Handler) error {
		sessionCalls.Add(1)
		sessionEndpoint = cfg.Endpoint
		if cfg.Endpoint == "start:1" {
			// First (startup) session fails so the loop backs off and re-reads.
			return errors.New("startup session fails to enter reconnect path")
		}
		cancel() // recovered snapshot reached: exit cleanly
		return nil
	}

	var backoffCalls atomic.Int32
	backoff := mq.Backoff(func(context.Context, int) bool {
		switch backoffCalls.Add(1) {
		case 1:
			// After the startup session failed: simulate a blank during reload so
			// the NEXT iteration hits the invalid-snapshot branch.
			src.set(rocketmq.Config{Endpoint: "", ConsumerGroup: "grp"})
		case 2:
			// The invalid snapshot backed off (proving no terminate); recover now.
			src.set(rocketmq.Config{Endpoint: "recovered:2", ConsumerGroup: "grp"})
		}
		return true
	})

	err := rocketmq.RunReconnectLoop(ctx, src, "topic", nil, backoff, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled (consumer must NOT terminate on a transient invalid snapshot), got %v", err)
	}
	// The recovered session must have run on the post-reload endpoint. If the loop
	// had terminated on the invalid snapshot, this would never be "recovered:2".
	if sessionEndpoint != "recovered:2" {
		t.Fatalf("final session endpoint = %q, want %q (loop must back off over the invalid window and retry to the valid one)", sessionEndpoint, "recovered:2")
	}
	// Two sessions total: the failed startup one and the recovered one. The
	// invalid snapshot in between produced NO session (it was skipped via backoff).
	if got := sessionCalls.Load(); got != 2 {
		t.Fatalf("expected exactly 2 sessions (startup-fail + recovered), got %d — invalid snapshot must not start a session", got)
	}
	// Backoff fired twice: once after the failed startup session, once for the
	// invalid snapshot. Both returned true (continue), so the loop never terminated.
	if got := backoffCalls.Load(); got != 2 {
		t.Fatalf("expected exactly 2 backoffs (startup-fail + invalid-snapshot), got %d", got)
	}
}

// TestReconnectLoop_StopsOnCleanSessionExit verifies the other loop exit: when a
// session returns nil (clean ctx-cancel inside the session), Subscribe returns
// ctx.Err() immediately and does NOT back off or start another session. (R2F7)
func TestReconnectLoop_StopsOnCleanSessionExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var sessionCalls atomic.Int32
	session := func(_ context.Context, _ rocketmq.Config, _ string, _ mq.Handler) error {
		sessionCalls.Add(1)
		cancel() // session observed ctx-cancel and is exiting cleanly
		return nil
	}

	var backoffCalls atomic.Int32
	backoff := mq.Backoff(func(context.Context, int) bool {
		backoffCalls.Add(1)
		return true
	})

	err := rocketmq.RunReconnectLoop(ctx, reconnectCfg(), "topic", nil, backoff, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled on clean session exit, got %v", err)
	}
	if got := sessionCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 session on clean exit, got %d", got)
	}
	if got := backoffCalls.Load(); got != 0 {
		t.Fatalf("clean session exit must not back off, but backoff was called %d times", got)
	}
}

// TestReconnectLoop_ReconnectsAfterFailureThenSucceeds verifies the self-heal
// path: the first session fails (broker down), the loop backs off and retries,
// and the second session exits cleanly (broker recovered). Two sessions run with
// exactly one backoff between them — the AC-MR2/MR3 reconnect contract. (R2F7)
func TestReconnectLoop_ReconnectsAfterFailureThenSucceeds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var sessionCalls atomic.Int32
	session := func(_ context.Context, _ rocketmq.Config, _ string, _ mq.Handler) error {
		n := sessionCalls.Add(1)
		if n == 1 {
			return errors.New("broker down (injected)") // first attempt fails
		}
		cancel() // second attempt: broker recovered, then ctx-cancel
		return nil
	}

	var backoffCalls atomic.Int32
	backoff := mq.Backoff(func(_ context.Context, attempt int) bool {
		backoffCalls.Add(1)
		if attempt != 1 {
			t.Errorf("backoff attempt = %d, want 1 (counter must reset across sessions only on success path)", attempt)
		}
		return true // keep reconnecting
	})

	err := rocketmq.RunReconnectLoop(ctx, reconnectCfg(), "topic", nil, backoff, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled after recovery, got %v", err)
	}
	if got := sessionCalls.Load(); got != 2 {
		t.Fatalf("expected 2 sessions (fail then recover), got %d", got)
	}
	if got := backoffCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 backoff between the failed and recovered session, got %d", got)
	}
}

// TestBackoff_CountsAndRespectsCtx verifies mq.DefaultBackoff semantics:
// it returns false when ctx is already cancelled.
func TestBackoff_CountsAndRespectsCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	result := mq.DefaultBackoff(ctx, 1)
	if result {
		t.Fatal("DefaultBackoff should return false when ctx is already cancelled")
	}
}

// TestBackoff_InjectableContract verifies that an injected fake Backoff is
// type-compatible with mq.Backoff and can count calls.
func TestBackoff_InjectableContract(t *testing.T) {
	var callCount atomic.Int32
	fakeBackoff := mq.Backoff(func(ctx context.Context, attempt int) bool {
		callCount.Add(1)
		return false // always stop
	})

	ctx := context.Background()
	result := fakeBackoff(ctx, 1)
	if result {
		t.Fatal("fakeBackoff should return false")
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 backoff call, got %d", callCount.Load())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// StaticSource
// ─────────────────────────────────────────────────────────────────────────────

func TestStaticSource_AlwaysVersion1(t *testing.T) {
	src := rocketmq.StaticSource(rocketmq.Config{Endpoint: "e:1"})
	s1 := src.Current()
	s2 := src.Current()
	if s1.Version != 1 || s2.Version != 1 {
		t.Fatalf("StaticSource version must be 1, got %d/%d", s1.Version, s2.Version)
	}
	want := rocketmq.Config{Endpoint: "e:1"}
	if s1.Value != want {
		t.Fatalf("StaticSource value mismatch: want %+v got %+v", want, s1.Value)
	}
}

func TestStaticSource_ImplementsResourceSource(t *testing.T) {
	var _ resource.Source = rocketmq.StaticSource{}
}

// ─────────────────────────────────────────────────────────────────────────────
// SDK log env variable verification
// ─────────────────────────────────────────────────────────────────────────────

// TestSDKLog_EnvVarRespected verifies that CLIENT_LOG_ROOT env was applied in
// TestMain so SDK logs land in our temp dir rather than ~/logs/rocketmqlogs.
func TestSDKLog_EnvVarRespected(t *testing.T) {
	// Verify the env var is still set to the package-level temp dir.
	got := os.Getenv(golang.CLIENT_LOG_ROOT)
	if got != sdkLogDir {
		t.Fatalf("CLIENT_LOG_ROOT should be %q, got %q", sdkLogDir, got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Interface compliance
// ─────────────────────────────────────────────────────────────────────────────

func TestPublisher_ImplementsMQPublisher(t *testing.T) {
	var _ mq.Publisher = (*rocketmq.Publisher)(nil)
}

func TestConsumer_ImplementsMQConsumer(t *testing.T) {
	var _ mq.Consumer = (*rocketmq.Consumer)(nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// errs sentinel used in publish path
// ─────────────────────────────────────────────────────────────────────────────

func TestDBUnavailable_IsDetectable(t *testing.T) {
	err := errs.DBUnavailable(errors.New("down"))
	if !errs.IsDBUnavailable(err) {
		t.Fatal("errs.IsDBUnavailable must be true for DBUnavailable errors")
	}
}

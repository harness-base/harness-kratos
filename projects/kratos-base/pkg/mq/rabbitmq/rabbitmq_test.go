package rabbitmq_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/go-kratos/aegis/circuitbreaker"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// unreachableURL targets a port that is bound to nothing so the dial
// fails quickly (DialTimeout 1 s).
const unreachableURL = "amqp://guest:guest@127.0.0.1:1/"

// ─────────────────────────────────────────────────────────────────────────────
// Config.Fingerprint
// ─────────────────────────────────────────────────────────────────────────────

func TestConfig_Fingerprint_Stable(t *testing.T) {
	t.Parallel()
	cfg := rabbitmq.Config{
		URL:         "amqp://localhost:5672/",
		DialTimeout: 5 * time.Second,
	}
	fp1 := cfg.Fingerprint()
	fp2 := cfg.Fingerprint()
	if fp1 != fp2 {
		t.Fatalf("fingerprint not stable: %q vs %q", fp1, fp2)
	}
	if fp1 == "" {
		t.Fatal("fingerprint must not be empty")
	}
}

func TestConfig_Fingerprint_ChangesWithURL(t *testing.T) {
	t.Parallel()
	base := rabbitmq.Config{URL: "amqp://a:5672/", DialTimeout: 5 * time.Second}
	other := rabbitmq.Config{URL: "amqp://b:5672/", DialTimeout: 5 * time.Second}
	if base.Fingerprint() == other.Fingerprint() {
		t.Fatal("fingerprint must differ when URL changes")
	}
}

func TestConfig_Fingerprint_ChangesWithDialTimeout(t *testing.T) {
	t.Parallel()
	base := rabbitmq.Config{URL: "amqp://a:5672/", DialTimeout: 5 * time.Second}
	other := rabbitmq.Config{URL: "amqp://a:5672/", DialTimeout: 2 * time.Second}
	if base.Fingerprint() == other.Fingerprint() {
		t.Fatal("fingerprint must differ when DialTimeout changes")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Config defaults: prefetch (R10F2) and poison ceiling (R10F1/R10F5)
// ─────────────────────────────────────────────────────────────────────────────

// TestConfig_PrefetchCount_Default pins the QoS prefetch resolution: an unset
// (or non-positive) PrefetchCount falls back to the sensible default; an
// explicit positive value is honoured verbatim. The default is load-bearing —
// without a prefetch bound the broker would push the entire ready backlog at one
// manual-ack consumer (unbounded in-flight set, OOM risk, R10F2).
func TestConfig_PrefetchCount_Default(t *testing.T) {
	t.Parallel()

	if got := rabbitmq.ResolvedPrefetch(rabbitmq.Config{}); got != 64 {
		t.Fatalf("default prefetch: got %d, want 64", got)
	}
	if got := rabbitmq.ResolvedPrefetch(rabbitmq.Config{PrefetchCount: -1}); got != 64 {
		t.Fatalf("non-positive prefetch must fall back to default: got %d, want 64", got)
	}
	if got := rabbitmq.ResolvedPrefetch(rabbitmq.Config{PrefetchCount: 200}); got != 200 {
		t.Fatalf("explicit prefetch must be honoured: got %d, want 200", got)
	}
}

// TestConfig_MaxDeliveryAttempts_Default pins the poison-ceiling resolution: an
// unset value falls back to the default ceiling, an explicit value is honoured.
// The ceiling is what bounds the redelivery loop before dead-lettering.
func TestConfig_MaxDeliveryAttempts_Default(t *testing.T) {
	t.Parallel()

	if got := rabbitmq.ResolvedMaxAttempts(rabbitmq.Config{}); got != 5 {
		t.Fatalf("default max attempts: got %d, want 5", got)
	}
	if got := rabbitmq.ResolvedMaxAttempts(rabbitmq.Config{MaxDeliveryAttempts: -3}); got != 5 {
		t.Fatalf("non-positive max attempts must fall back to default: got %d, want 5", got)
	}
	if got := rabbitmq.ResolvedMaxAttempts(rabbitmq.Config{MaxDeliveryAttempts: 3}); got != 3 {
		t.Fatalf("explicit max attempts must be honoured: got %d, want 3", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Publisher with unreachable broker → DBUnavailable within ~DialTimeout
// ─────────────────────────────────────────────────────────────────────────────

func TestPublisher_UnreachableURL_DBUnavailable(t *testing.T) {
	t.Parallel()

	cfg := rabbitmq.Config{
		URL:         unreachableURL,
		DialTimeout: 1 * time.Second,
	}
	src := rabbitmq.StaticSource(cfg)
	pub := rabbitmq.NewPublisher(src)
	defer pub.Close() //nolint:errcheck

	ctx := context.Background()
	start := time.Now()
	err := pub.Publish(ctx, mq.Message{Topic: "t", Body: []byte("hello")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("Publish took %v, expected <3s", elapsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Circuit-breaker open → fast-fail without dialling (R2F3 / R2F10)
// ─────────────────────────────────────────────────────────────────────────────

// stubBreaker is a deterministic circuitbreaker.CircuitBreaker for tests:
// when open it rejects every Allow() with ErrNotAllowed (open circuit),
// otherwise it permits. It records Allow/MarkSuccess/MarkFailed call counts so
// tests can hard-assert the breaker was consulted and the send path was (not)
// reached. Mirrors the rocketmq R1 stubBreaker.
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

// TestPublisher_OpenCircuit_FastFailsWithoutDial is the R2F3/R2F10 fix: the old
// TestPublisher_CircuitBreakerOpens_FastFail could not distinguish "breaker
// opened" from "breaker never opened" (every call merely had to finish under a
// generous 2×DialTimeout bound — true even if the breaker stayed closed the
// whole time). Here we inject a deterministically OPEN stub breaker and hard
// assert the open-circuit contract:
//   - Publish returns DBUnavailable
//   - it returns near-instantly (<50ms) because no dial is attempted — proven
//     sharp by an unreachable URL whose dial would otherwise cost DialTimeout
//   - Allow() is consulted exactly once
//   - the send path is never reached: neither MarkSuccess nor MarkFailed fires
func TestPublisher_OpenCircuit_FastFailsWithoutDial(t *testing.T) {
	t.Parallel()

	cfg := rabbitmq.Config{
		URL: unreachableURL,
		// A long dial timeout makes the point sharp: if Publish were to dial,
		// the call would take ~5s. An open breaker must short-circuit instead.
		DialTimeout: 5 * time.Second,
	}
	src := rabbitmq.StaticSource(cfg)

	br := &stubBreaker{open: true}
	pub := rabbitmq.NewPublisherWithBreaker(src, br)
	defer pub.Close() //nolint:errcheck

	start := time.Now()
	err := pub.Publish(context.Background(), mq.Message{Topic: "t", Body: []byte("x")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when circuit is open, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable from open circuit, got %T: %v", err, err)
	}
	// Hard bound: an open-circuit reject is a map/atomic op with no network hop,
	// far below the 5s DialTimeout a dial would have cost. This is the assertion
	// the old test lacked — it directly proves the breaker actually opened.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("open circuit Publish took %v; expected near-instant fast-fail (it must not dial)", elapsed)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected exactly 1 Allow() call, got %d", br.allowCalls)
	}
	// When Allow() rejects we must NOT have touched the dial/publish path, so
	// neither MarkSuccess nor MarkFailed should have fired.
	if br.failedCalls != 0 || br.okCalls != 0 {
		t.Fatalf("open circuit must short-circuit before send: failed=%d ok=%d (want 0/0)", br.failedCalls, br.okCalls)
	}
}

// TestPublisher_ClosedCircuit_MarksFailedOnUnreachable is the complementary
// path: with a permitting (closed) breaker against an unreachable broker,
// Publish attempts the dial, fails, and reports the failure to the breaker via
// MarkFailed (driving it toward open). This proves the breaker is genuinely
// wired into the dial/publish path, not bypassed — the open-circuit test alone
// could pass even if MarkFailed were never called on the failure path.
func TestPublisher_ClosedCircuit_MarksFailedOnUnreachable(t *testing.T) {
	t.Parallel()

	cfg := rabbitmq.Config{
		URL:         unreachableURL,
		DialTimeout: 500 * time.Millisecond,
	}
	src := rabbitmq.StaticSource(cfg)

	br := &stubBreaker{open: false}
	pub := rabbitmq.NewPublisherWithBreaker(src, br)
	defer pub.Close() //nolint:errcheck

	err := pub.Publish(context.Background(), mq.Message{Topic: "t", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected error for unreachable broker")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected exactly 1 Allow() call, got %d", br.allowCalls)
	}
	if br.failedCalls != 1 {
		t.Fatalf("expected exactly 1 MarkFailed() on unreachable dial, got %d", br.failedCalls)
	}
	if br.okCalls != 0 {
		t.Fatalf("expected 0 MarkSuccess() on failed publish, got %d", br.okCalls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Empty-topic guard fires before the breaker (R3F2)
// ─────────────────────────────────────────────────────────────────────────────

// TestPublisher_EmptyTopic_InvalidArgumentBeforeBreaker pins the "fail loud, do
// not silently drop" contract for an empty topic. With the default exchange a
// message whose routing key (== topic) is "" would be silently discarded by the
// broker, so Publish guards it up front and returns InvalidArgument (HTTP 400).
//
// The guard must sit BEFORE breaker.Allow() and any dial: we inject a stub
// breaker and hard-assert it was never consulted (allowCalls == 0). A guard
// placed after the breaker would still return an error, but would have called
// Allow() first — so allowCalls==0 is the load-bearing assertion that proves
// ordering. The unreachable URL would make any dial obvious; here none happens.
func TestPublisher_EmptyTopic_InvalidArgumentBeforeBreaker(t *testing.T) {
	t.Parallel()

	cfg := rabbitmq.Config{
		URL:         unreachableURL,
		DialTimeout: 5 * time.Second,
	}
	src := rabbitmq.StaticSource(cfg)

	br := &stubBreaker{open: false} // closed: would permit a dial if reached
	pub := rabbitmq.NewPublisherWithBreaker(src, br)
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
	// The guard must precede breaker.Allow(): if it did not, allowCalls would be
	// 1. This is the assertion that distinguishes "guard first" from "guard after
	// the breaker".
	if br.allowCalls != 0 {
		t.Fatalf("empty-topic guard must fire before breaker.Allow(): got allowCalls=%d, want 0", br.allowCalls)
	}
	if br.okCalls != 0 || br.failedCalls != 0 {
		t.Fatalf("guard must not touch the send path: ok=%d failed=%d, want 0/0", br.okCalls, br.failedCalls)
	}
	// No dial happens, so this is near-instant despite the 5s DialTimeout.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("empty-topic guard took %v; it must return before any dial", elapsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// getLiveConn self-heal: dead cached connection → Close + re-Get once (R2F4)
// ─────────────────────────────────────────────────────────────────────────────

// fakeConn is a rabbitmq.LiveConn whose closed state is scripted per Get so the
// getLiveConn self-heal branch can be driven deterministically.
type fakeConn struct{ closed bool }

func (c *fakeConn) IsClosed() bool { return c.closed }

// scriptedConnSource is a rabbitmq.ConnSource that returns a scripted sequence
// of connections from Get and records how many times Get and Close were called.
// It models the provider's "caches a dead handle until invalidated" behaviour:
// the first Get yields a corpse, Close invalidates it, and the second Get yields
// a fresh live connection.
type scriptedConnSource struct {
	conns      []rabbitmq.LiveConn
	getErrs    []error
	getCalls   int
	closeCalls int
}

func (s *scriptedConnSource) Get(_ context.Context) (rabbitmq.LiveConn, error) {
	i := s.getCalls
	s.getCalls++
	var err error
	if i < len(s.getErrs) {
		err = s.getErrs[i]
	}
	if i < len(s.conns) {
		return s.conns[i], err
	}
	return nil, err
}

func (s *scriptedConnSource) Close() error {
	s.closeCalls++
	return nil
}

// TestGetLiveConn_DeadConn_RebuildsOnce is the R2F4 fix: getLiveConn's self-heal
// core (IsClosed → Close → retry Get ONCE) had no seam and was untestable. With
// the connSource seam we drive a source whose first Get returns a CLOSED
// connection (the cached corpse) and second Get returns a LIVE one, then hard
// assert getLiveConn detected the corpse, invalidated the cache exactly once,
// re-fetched exactly once, and returned the live connection.
func TestGetLiveConn_DeadConn_RebuildsOnce(t *testing.T) {
	t.Parallel()

	dead := &fakeConn{closed: true}
	live := &fakeConn{closed: false}
	src := &scriptedConnSource{conns: []rabbitmq.LiveConn{dead, live}}

	conn, err := rabbitmq.GetLiveConn(context.Background(), src)
	if err != nil {
		t.Fatalf("expected nil error after rebuild, got %v", err)
	}
	if conn != live {
		t.Fatalf("expected the live (rebuilt) connection, got %#v", conn)
	}
	if src.getCalls != 2 {
		t.Fatalf("expected exactly 2 Get calls (corpse + rebuild), got %d", src.getCalls)
	}
	if src.closeCalls != 1 {
		t.Fatalf("expected exactly 1 Close call to invalidate the dead handle, got %d", src.closeCalls)
	}
}

// TestGetLiveConn_HealthyConn_NoRebuild guards the happy path: a live connection
// on the first Get must be returned as-is, with no invalidation and no second
// fetch. This makes TestGetLiveConn_DeadConn_RebuildsOnce's "1 Close" assertion
// load-bearing — without this companion, a buggy getLiveConn that always closed
// + refetched would still pass the dead-conn test.
func TestGetLiveConn_HealthyConn_NoRebuild(t *testing.T) {
	t.Parallel()

	live := &fakeConn{closed: false}
	src := &scriptedConnSource{conns: []rabbitmq.LiveConn{live}}

	conn, err := rabbitmq.GetLiveConn(context.Background(), src)
	if err != nil {
		t.Fatalf("expected nil error for healthy conn, got %v", err)
	}
	if conn != live {
		t.Fatalf("expected the live connection, got %#v", conn)
	}
	if src.getCalls != 1 {
		t.Fatalf("healthy conn must not refetch: expected 1 Get call, got %d", src.getCalls)
	}
	if src.closeCalls != 0 {
		t.Fatalf("healthy conn must not be invalidated: expected 0 Close calls, got %d", src.closeCalls)
	}
}

// TestGetLiveConn_DeadConnRebuildFails_ReturnsError verifies the failure tail of
// the self-heal branch: when the first Get returns a corpse and the rebuild Get
// fails (broker still down), getLiveConn surfaces that error rather than a
// closed connection — this is what hands control to the caller's retry/backoff.
func TestGetLiveConn_DeadConnRebuildFails_ReturnsError(t *testing.T) {
	t.Parallel()

	dead := &fakeConn{closed: true}
	wantErr := errors.New("dial refused")
	src := &scriptedConnSource{
		conns:   []rabbitmq.LiveConn{dead, nil},
		getErrs: []error{nil, wantErr},
	}

	conn, err := rabbitmq.GetLiveConn(context.Background(), src)
	if conn != nil {
		t.Fatalf("expected nil conn when rebuild fails, got %#v", conn)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the rebuild dial error to surface, got %v", err)
	}
	if src.getCalls != 2 {
		t.Fatalf("expected 2 Get calls (corpse + failed rebuild), got %d", src.getCalls)
	}
	if src.closeCalls != 1 {
		t.Fatalf("expected exactly 1 Close call, got %d", src.closeCalls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// adaptDeliveries — field mapping, header filtering, close propagation
// ─────────────────────────────────────────────────────────────────────────────

// recordingAck is a stub amqp.Acknowledger that records the arguments of the
// last Ack/Nack/Reject call so the Ack/Nack closures can be hard-asserted
// without a live broker.
type recordingAck struct {
	mu sync.Mutex

	ackTag      uint64
	ackMultiple bool
	ackCalled   bool

	nackTag      uint64
	nackMultiple bool
	nackRequeue  bool
	nackCalled   bool
}

func (r *recordingAck) Ack(tag uint64, multiple bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ackTag = tag
	r.ackMultiple = multiple
	r.ackCalled = true
	return nil
}

func (r *recordingAck) Nack(tag uint64, multiple, requeue bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nackTag = tag
	r.nackMultiple = multiple
	r.nackRequeue = requeue
	r.nackCalled = true
	return nil
}

func (r *recordingAck) Reject(_ uint64, _ bool) error { return nil }

// TestAdaptDeliveries_FieldMapping feeds one amqp.Delivery with a mixed header
// table (string + non-string values) and asserts the resulting mq.Delivery:
// RoutingKey→Topic, MessageId→Key, Body verbatim, only string headers survive.
func TestAdaptDeliveries_FieldMapping(t *testing.T) {
	t.Parallel()

	in := make(chan amqp.Delivery, 1)
	in <- amqp.Delivery{
		RoutingKey: "orders",
		MessageId:  "msg-42",
		Body:       []byte("payload"),
		Headers: amqp.Table{
			"trace":   "abc123", // string → kept
			"retries": int32(3), // non-string → dropped
			"flag":    true,     // non-string → dropped
			"region":  "eu",     // string → kept
		},
	}
	close(in) // signals end of stream → out must close

	out := make(chan mq.Delivery, 1)
	go rabbitmq.AdaptDeliveries(context.Background(), in, out)

	got, ok := <-out
	if !ok {
		t.Fatal("expected one delivery, got closed channel")
	}

	if got.Msg.Topic != "orders" {
		t.Fatalf("Topic: got %q, want %q", got.Msg.Topic, "orders")
	}
	if got.Msg.Key != "msg-42" {
		t.Fatalf("Key: got %q, want %q", got.Msg.Key, "msg-42")
	}
	if string(got.Msg.Body) != "payload" {
		t.Fatalf("Body: got %q, want %q", got.Msg.Body, "payload")
	}
	if len(got.Msg.Headers) != 2 {
		t.Fatalf("Headers: got %d entries, want 2 (non-string values must be dropped): %#v",
			len(got.Msg.Headers), got.Msg.Headers)
	}
	if got.Msg.Headers["trace"] != "abc123" {
		t.Fatalf("Headers[trace]: got %q, want %q", got.Msg.Headers["trace"], "abc123")
	}
	if got.Msg.Headers["region"] != "eu" {
		t.Fatalf("Headers[region]: got %q, want %q", got.Msg.Headers["region"], "eu")
	}
	if _, present := got.Msg.Headers["retries"]; present {
		t.Fatal("Headers[retries]: non-string header must be dropped")
	}
	if _, present := got.Msg.Headers["flag"]; present {
		t.Fatal("Headers[flag]: non-string header must be dropped")
	}

	// in is closed → out must close after draining.
	if _, ok := <-out; ok {
		t.Fatal("out must be closed after in is drained and closed")
	}
}

// TestTracePropagation_WireRoundTrip is the R9F6 rabbitmq round-trip: a trace
// context the publisher would inject (mq.InjectTrace → AMQP header table) must
// survive the wire-shaped representation (amqp.Table string values →
// amqp.Delivery.Headers → AdaptDeliveries → mq.Message.Headers) and be
// recoverable by the consumer (mq.ExtractTrace) with the same trace id.
//
// This pins the rabbitmq-specific link in the chain: traceparent is a STRING
// header, so it survives adaptDeliveries' string-only filter that drops
// non-string headers (see TestAdaptDeliveries_FieldMapping). If that filter were
// to drop string headers, or adaptDeliveries dropped Headers entirely, the trace
// would not reach the consumer and this test fails.
//
// Scope: the publisher side binds the REAL publish-path header builder
// (rabbitmq.BuildPublishTable → buildPublishTable, the same helper Publish calls)
// so dropping the inject in production fails this test. The wire shape (amqp.Table
// → adaptDeliveries → message headers) and the consumer extract complete the
// round-trip. (The full Publish method still needs a live broker channel for its
// dial/declare steps, but the trace-inject decision it delegates to
// buildPublishTable is fully covered here.)
func TestTracePropagation_WireRoundTrip(t *testing.T) {
	t.Parallel()

	const traceHex = "0f0e0d0c0b0a09080706050403020100"
	const spanHex = "0f0e0d0c0b0a0908"

	tid, err := oteltrace.TraceIDFromHex(traceHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	sid, err := oteltrace.SpanIDFromHex(spanHex)
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	producerCtx := oteltrace.ContextWithSpanContext(context.Background(),
		oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
			TraceID: tid, SpanID: sid, TraceFlags: oteltrace.FlagsSampled, Remote: true,
		}))

	// Publisher side: build the outgoing header table via the SAME helper the
	// production Publish path uses, so this round-trip binds the real publisher
	// inject (R9F6) rather than a re-implementation.
	publisherHeaders := rabbitmq.BuildPublishTable(producerCtx, mq.Message{
		Headers: map[string]string{"content-type": "application/json"},
	})
	if _, ok := publisherHeaders["traceparent"]; !ok {
		t.Fatalf("publisher did not inject a traceparent: %+v", publisherHeaders)
	}
	table := amqp.Table{}
	for k, v := range publisherHeaders {
		table[k] = v // string values survive adaptDeliveries' string filter
	}

	in := make(chan amqp.Delivery, 1)
	in <- amqp.Delivery{RoutingKey: "t", Body: []byte("x"), Headers: table}
	close(in)

	out := make(chan mq.Delivery, 1)
	go rabbitmq.AdaptDeliveries(context.Background(), in, out)

	got, ok := <-out
	if !ok {
		t.Fatal("expected one delivery")
	}

	// Consumer side: extract the trace from the mapped message headers.
	hctx := mq.ExtractTrace(context.Background(), got.Msg.Headers)
	sc := oteltrace.SpanContextFromContext(hctx)
	if !sc.IsValid() {
		t.Fatal("extracted SpanContext invalid; trace did not survive the rabbitmq wire round-trip")
	}
	if sc.TraceID().String() != traceHex {
		t.Fatalf("trace id not preserved: want %s got %s", traceHex, sc.TraceID())
	}
	if sc.SpanID().String() != spanHex {
		t.Fatalf("parent span id not preserved: want %s got %s", spanHex, sc.SpanID())
	}
	// Business header survived alongside the trace header.
	if got.Msg.Headers["content-type"] != "application/json" {
		t.Errorf("business header dropped: %+v", got.Msg.Headers)
	}
}

// TestAdaptDeliveries_AckNackClosures verifies the Ack/Nack closures invoke the
// delivery's Acknowledger with the exact arguments the production code passes:
// Ack(false) and Nack(false, true) — i.e. single message, requeue on nack.
//
// R11F9: Nack ALWAYS requeues; poison-message dead-lettering is broker-side via
// the quorum queue's x-delivery-limit (asserted in TestTopicQueueArgs_*), not an
// app-side requeue=false at a ceiling. So requeue must be true unconditionally.
func TestAdaptDeliveries_AckNackClosures(t *testing.T) {
	t.Parallel()

	rec := &recordingAck{}
	in := make(chan amqp.Delivery, 1)
	in <- amqp.Delivery{
		Acknowledger: rec,
		DeliveryTag:  7,
		RoutingKey:   "t",
	}
	close(in)

	out := make(chan mq.Delivery, 1)
	go rabbitmq.AdaptDeliveries(context.Background(), in, out)

	d, ok := <-out
	if !ok {
		t.Fatal("expected a delivery")
	}

	if err := d.Ack(); err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}
	if !rec.ackCalled {
		t.Fatal("Ack closure did not call Acknowledger.Ack")
	}
	if rec.ackTag != 7 {
		t.Fatalf("Ack tag: got %d, want 7 (must use the delivery's DeliveryTag)", rec.ackTag)
	}
	if rec.ackMultiple {
		t.Fatal("Ack multiple: got true, want false (single-message ack)")
	}

	if err := d.Nack(); err != nil {
		t.Fatalf("Nack returned error: %v", err)
	}
	if !rec.nackCalled {
		t.Fatal("Nack closure did not call Acknowledger.Nack")
	}
	if rec.nackTag != 7 {
		t.Fatalf("Nack tag: got %d, want 7", rec.nackTag)
	}
	if rec.nackMultiple {
		t.Fatal("Nack multiple: got true, want false (single-message nack)")
	}
	if !rec.nackRequeue {
		t.Fatal("Nack requeue: got false, want true (failed message must be requeued; the broker dead-letters via x-delivery-limit, not the app)")
	}
}

// TestAdaptDeliveries_NackAlwaysRequeues_EvenWhenRedelivered is the R11F9
// mutation guard for the broker-native design: the Nack closure must requeue
// REGARDLESS of redelivery state, because attempt counting moved to the broker
// (quorum x-delivery-limit). A delivery already marked Redelivered (and even one
// carrying an x-death header from a prior dead-letter cycle) must STILL be
// requeued by the app — the broker, not adaptDeliveries, decides when the limit
// is exceeded.
//
// Without this, a regression that reintroduced app-side counting (Nack with
// requeue=false once Redelivered/x-death crossed a ceiling) would silently
// resurrect the R10 design. Here a Redelivered+x-death delivery that the old code
// would Nack(requeue=false) must Nack(requeue=true).
func TestAdaptDeliveries_NackAlwaysRequeues_EvenWhenRedelivered(t *testing.T) {
	t.Parallel()

	rec := &recordingAck{}
	in := make(chan amqp.Delivery, 1)
	in <- amqp.Delivery{
		Acknowledger: rec,
		DeliveryTag:  11,
		RoutingKey:   "t",
		Redelivered:  true, // would have advanced an app-side counter under R10
		Headers: amqp.Table{ // a prior dead-letter cycle's x-death — R10 counted this
			"x-death": []any{amqp.Table{"count": int64(99)}},
		},
	}
	close(in)

	out := make(chan mq.Delivery, 1)
	go rabbitmq.AdaptDeliveries(context.Background(), in, out)

	d, ok := <-out
	if !ok {
		t.Fatal("expected a delivery")
	}
	if err := d.Nack(); err != nil {
		t.Fatalf("Nack returned error: %v", err)
	}
	if !rec.nackCalled {
		t.Fatal("Nack closure did not call Acknowledger.Nack")
	}
	if !rec.nackRequeue {
		t.Fatal("Nack requeue: got false, want true — a Redelivered/x-death delivery must STILL requeue; broker-side x-delivery-limit (not the app) breaks the loop (R11F9)")
	}
}

// TestAdaptDeliveries_CtxCancelNoLeak is the regression guard for F25: when the
// consumer stops reading and ctx is cancelled while out's buffer is full, the
// adapter must exit on ctx.Done() instead of blocking forever on `out <- md`.
// The adapter goroutine signals completion by closing out (defer), so we wait
// on that close with a timeout — a leaked goroutine would never close out.
func TestAdaptDeliveries_CtxCancelNoLeak(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	// Feed two deliveries. The test drains the first (synchronization handshake,
	// below), so the adapter parks on the SECOND `out <- md` with no reader left.
	in := make(chan amqp.Delivery, 2)
	in <- amqp.Delivery{RoutingKey: "t"}
	in <- amqp.Delivery{RoutingKey: "t"}
	// Do NOT close in: even with a live (open) amqp channel, the adapter must
	// still unblock purely on ctx cancellation.

	out := make(chan mq.Delivery) // unbuffered → each send blocks until read

	done := make(chan struct{})
	go func() {
		rabbitmq.AdaptDeliveries(ctx, in, out)
		close(done)
	}()

	// Deterministic synchronization (no time.Sleep): read exactly ONE delivery.
	// out is unbuffered, so this receive only completes once the adapter has
	// executed `out <- md` for the first delivery — proof the goroutine is alive
	// and inside the loop. The adapter then loops to the SECOND delivery and
	// parks on `out <- md` again (nobody reads it). Now cancel: whether the
	// goroutine is already blocked in the select or reaches it next, the
	// ctx.Done() case must release it. A leaked goroutine never closes out.
	select {
	case <-out:
		// first delivery handed off → adapter confirmed in the loop
	case <-time.After(2 * time.Second):
		t.Fatal("adaptDeliveries never sent the first delivery on out")
	}
	cancel()

	select {
	case <-done:
		// adapter returned → no leak
	case <-time.After(2 * time.Second):
		t.Fatal("adaptDeliveries leaked: did not return after ctx cancel while parked on out <- md")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Quorum dead-letter topology (R11F9): the QueueDeclare args that actually drive
// broker-side poison-message dead-lettering — quorum type + x-delivery-limit + DLX
// ─────────────────────────────────────────────────────────────────────────────

// TestTopicQueueArgs_QuorumDeliveryLimitAndDeadLetter pins the args the main
// queue is declared with. These args are the WHOLE poison-message mechanism in
// the R11F9 design: the broker (not application code) counts redeliveries and
// dead-letters. Both the publisher and the consumer declare the main queue and
// MUST pass identical args with the SAME resolved maxAttempts (RabbitMQ rejects a
// mismatched re-declare with PRECONDITION_FAILED), so this is the single source
// of truth for that topology.
//
// Why this is NOT vacuous (the R10 single-test was: it fed synthetic x-death
// tables a real broker's requeue loop never produces). Here we assert the EXACT
// arguments that production passes to ch.QueueDeclare — i.e. the bytes the broker
// reads to decide queue type, redelivery limit, and dead-letter route. There is
// no synthetic broker state: topicQueueArgs is the literal declare payload.
//
// Mutation self-justification:
//   - Drop x-queue-type=quorum → a classic queue IGNORES x-delivery-limit, so a
//     Nack(requeue=true) hot-loops the poison message forever (the R10 bug). This
//     test fails (queue type assertion).
//   - Wrong x-delivery-limit (e.g. == maxAttempts, or 0, or missing) → the broker
//     dead-letters after the wrong number of attempts (or, if missing, never).
//     This test fails (the limit must be exactly maxAttempts-1, an int).
//   - Drop the DLX args → a dead-letter has nowhere to route and the broker drops
//     the poison message instead of parking it in the DLQ. This test fails.
//
// Honest scope: this test proves the DECLARATION is correct. The end-to-end
// behaviour (a message that fails handling really gets redelivered maxAttempts
// times and then lands in <topic>.dlq) is enforced by the broker and needs a
// live-broker e2e to observe directly; that can be added later. A unit test
// cannot exercise the broker's quorum counter, so it must not pretend to (no
// synthetic x-death) — it pins the only thing the application controls: the args.
func TestTopicQueueArgs_QuorumDeliveryLimitAndDeadLetter(t *testing.T) {
	t.Parallel()

	const topic = "orders"
	const maxAttempts = 5
	args := rabbitmq.TopicQueueArgs(topic, maxAttempts)

	if got := args["x-queue-type"]; got != "quorum" {
		t.Fatalf("x-queue-type: got %v, want \"quorum\" (classic queues ignore x-delivery-limit, so a poison message would hot-loop forever)", got)
	}

	// x-delivery-limit must be an int == maxAttempts-1: the broker dead-letters
	// once the per-message delivery count EXCEEDS this limit, and attempts are
	// 1-based, so a limit of maxAttempts-1 yields exactly maxAttempts deliveries.
	gotLimit, ok := args["x-delivery-limit"].(int)
	if !ok {
		t.Fatalf("x-delivery-limit: got %T (%v), want an int (amqp requires a numeric type the broker accepts)", args["x-delivery-limit"], args["x-delivery-limit"])
	}
	if gotLimit != maxAttempts-1 {
		t.Fatalf("x-delivery-limit: got %d, want %d (maxAttempts-1, so the broker allows exactly %d deliveries before dead-lettering)", gotLimit, maxAttempts-1, maxAttempts)
	}

	if got := args["x-dead-letter-exchange"]; got != "" {
		t.Fatalf("x-dead-letter-exchange: got %v, want \"\" (default exchange routes by queue name)", got)
	}
	want := rabbitmq.DLQName(topic)
	if got := args["x-dead-letter-routing-key"]; got != want {
		t.Fatalf("x-dead-letter-routing-key: got %v, want %q (must target the companion DLQ)", got, want)
	}
	if want != "orders.dlq" {
		t.Fatalf("dlqName(orders): got %q, want %q", want, "orders.dlq")
	}
}

// TestTopicQueueArgs_DeliveryLimitTracksMaxAttempts guards that the limit is
// derived from the resolved maxAttempts rather than hardcoded: a different
// ceiling must produce a different x-delivery-limit. Without this companion, a
// buggy topicQueueArgs that hardcoded x-delivery-limit (ignoring its argument)
// would still pass the maxAttempts=5 case above.
//
// It also pins the off-by-one direction at the boundary: maxAttempts=1 means
// "dead-letter on the first failure", i.e. x-delivery-limit must be 0 (zero
// redeliveries allowed), never negative or 1.
func TestTopicQueueArgs_DeliveryLimitTracksMaxAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		maxAttempts int
		wantLimit   int
	}{
		{1, 0},  // first failure → dead-letter immediately (no redeliveries)
		{2, 1},  // one redelivery, then dead-letter
		{3, 2},  // explicit ceiling honoured
		{10, 9}, // larger ceiling
	}
	for _, tc := range tests {
		args := rabbitmq.TopicQueueArgs("t", tc.maxAttempts)
		got, ok := args["x-delivery-limit"].(int)
		if !ok {
			t.Fatalf("maxAttempts=%d: x-delivery-limit not an int: %T", tc.maxAttempts, args["x-delivery-limit"])
		}
		if got != tc.wantLimit {
			t.Fatalf("maxAttempts=%d: x-delivery-limit got %d, want %d (maxAttempts-1)", tc.maxAttempts, got, tc.wantLimit)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// R10F4: caller-cancelled ctx must NOT trip the breaker; timeout still does
// ─────────────────────────────────────────────────────────────────────────────

// TestPublisher_CallerCanceledCtx_DoesNotMarkFailed proves the R10F4 exemption:
// when the CALLER explicitly cancels its ctx mid-publish (client disconnect),
// Publish surfaces DBUnavailable but must NOT report a failure to the breaker —
// a client hang-up is not evidence the broker is sick, and counting it would let
// a burst of disconnects open the breaker against a healthy broker.
//
// Mutation self-justification: the breaker is a permitting stub, so Allow()==1
// confirms the send path was entered. If failPublish dropped the
// IsCallerCanceled guard and always MarkFailed (the pre-fix behaviour),
// failedCalls would be 1 and this test fails. okCalls must also be 0 (the
// publish never succeeded).
func TestPublisher_CallerCanceledCtx_DoesNotMarkFailed(t *testing.T) {
	t.Parallel()

	cfg := rabbitmq.Config{URL: unreachableURL, DialTimeout: 5 * time.Second}
	src := rabbitmq.StaticSource(cfg)
	br := &stubBreaker{open: false} // permit the send path
	pub := rabbitmq.NewPublisherWithBreaker(src, br)
	defer pub.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // caller cancels BEFORE the dial — explicit client hang-up

	err := pub.Publish(ctx, mq.Message{Topic: "t", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected an error when caller ctx is cancelled")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected the send path to be entered exactly once (Allow=1), got %d", br.allowCalls)
	}
	if br.failedCalls != 0 {
		t.Fatalf("caller-cancelled ctx must NOT MarkFailed (not a backend fault), got failedCalls=%d", br.failedCalls)
	}
	if br.okCalls != 0 {
		t.Fatalf("a cancelled publish never succeeds, got okCalls=%d", br.okCalls)
	}
}

// TestPublisher_DeadlineExceededCtx_StillMarksFailed is the complementary
// asymmetry guard: a DEADLINE (context.DeadlineExceeded) is a backend-health
// signal (broker too slow/unreachable in time) and MUST still MarkFailed — only
// an explicit caller Cancel is exempt. Without this companion, a buggy
// failPublish that blanket-exempted ALL ctx errors (cancel AND deadline) would
// still pass the cancel-only test above.
//
// Determinism: we hand Publish a ctx whose deadline is already in the PAST, so
// ctx.Err() == context.DeadlineExceeded (never Canceled) for the whole call.
// errs.IsCallerCanceled returns false for that, so failPublish must MarkFailed.
// (The full deadline-vs-cancel truth table is locked at the decision-function
// level in pkg/errs; here we only prove the rabbitmq layer does not over-exempt.)
func TestPublisher_DeadlineExceededCtx_StillMarksFailed(t *testing.T) {
	t.Parallel()

	cfg := rabbitmq.Config{URL: unreachableURL, DialTimeout: 5 * time.Second}
	src := rabbitmq.StaticSource(cfg)
	br := &stubBreaker{open: false}
	pub := rabbitmq.NewPublisherWithBreaker(src, br)
	defer pub.Close() //nolint:errcheck

	// Deadline in the past → ctx.Err() == DeadlineExceeded immediately, and
	// stays that way (it is NOT context.Canceled), so the exemption must not fire.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("setup: expected DeadlineExceeded, got %v", ctx.Err())
	}

	err := pub.Publish(ctx, mq.Message{Topic: "t", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected an error on deadline")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	if br.allowCalls != 1 {
		t.Fatalf("expected Allow=1, got %d", br.allowCalls)
	}
	// A deadline is a backend-health signal and must trip the breaker, unlike an
	// explicit caller Cancel.
	if br.failedCalls != 1 {
		t.Fatalf("deadline-exceeded must MarkFailed (backend fault), got failedCalls=%d", br.failedCalls)
	}
	if br.okCalls != 0 {
		t.Fatalf("a timed-out publish never succeeds, got okCalls=%d", br.okCalls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// StaticSource helper used in tests
// ─────────────────────────────────────────────────────────────────────────────

// Verify StaticSource satisfies resource.Source (compile-time check done via
// usage in NewPublisher above; this is an extra sanity).
var _ resource.Source = rabbitmq.StaticSource(rabbitmq.Config{})

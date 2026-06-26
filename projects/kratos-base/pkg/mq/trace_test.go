package mq_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/z-mate/kratos-base/pkg/mq"
)

// spanCtx builds a context carrying a remote-flavoured SpanContext with the
// given trace/span IDs, sampled, so InjectTrace has a live W3C trace context to
// serialize. It does not need an SDK tracer provider — a manufactured
// SpanContext is enough to exercise the propagator round-trip.
func spanCtx(t *testing.T, traceHex, spanHex string) context.Context {
	t.Helper()
	tid, err := trace.TraceIDFromHex(traceHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex(%q): %v", traceHex, err)
	}
	sid, err := trace.SpanIDFromHex(spanHex)
	if err != nil {
		t.Fatalf("SpanIDFromHex(%q): %v", spanHex, err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	return trace.ContextWithSpanContext(context.Background(), sc)
}

// TestInjectExtractTrace_RoundTrip is the R9F6 load-bearing assertion: a trace
// context injected by a producer into message headers must be recoverable by a
// consumer via ExtractTrace, yielding the SAME trace id (and span id, which on
// the consumer side becomes the parent/remote span). This is what makes the
// async MQ hop a single connected trace.
//
// Mutation self-proof: if InjectTrace is reverted to a no-op (returns headers
// unchanged with no traceparent), the extracted SpanContext is invalid and the
// trace-id assertion FAILS. Likewise if ExtractTrace is reverted to `return ctx`
// the extracted SpanContext stays invalid and FAILS.
func TestInjectExtractTrace_RoundTrip(t *testing.T) {
	const traceHex = "0102030405060708090a0b0c0d0e0f10"
	const spanHex = "0102030405060708"

	producerCtx := spanCtx(t, traceHex, spanHex)

	// Producer side: inject into the (empty) message headers.
	headers := mq.InjectTrace(producerCtx, nil)
	if headers == nil {
		t.Fatal("InjectTrace returned nil headers; a live span context must produce a traceparent")
	}
	if _, ok := headers["traceparent"]; !ok {
		t.Fatalf("InjectTrace did not write a W3C traceparent header; got %+v", headers)
	}

	// Consumer side: extract from a fresh background ctx (the async hop).
	consumerCtx := mq.ExtractTrace(context.Background(), headers)
	got := trace.SpanContextFromContext(consumerCtx)
	if !got.IsValid() {
		t.Fatal("ExtractTrace produced no valid SpanContext; trace did not cross the hop")
	}
	if got.TraceID().String() != traceHex {
		t.Fatalf("trace id not preserved across hop: want %s got %s", traceHex, got.TraceID())
	}
	// The producer's span becomes the remote parent on the consumer side.
	if got.SpanID().String() != spanHex {
		t.Fatalf("parent span id not preserved across hop: want %s got %s", spanHex, got.SpanID())
	}
	if !got.IsRemote() {
		t.Error("extracted SpanContext should be marked remote (came over the wire)")
	}
}

// TestInjectTrace_PreservesExistingHeadersWithoutMutatingInput verifies two
// contracts: existing business headers survive the inject, and the caller's map
// is never mutated (InjectTrace returns a copy). The no-mutation guarantee
// matters because Publish callers may reuse the same Message.Headers map.
func TestInjectTrace_PreservesExistingHeadersWithoutMutatingInput(t *testing.T) {
	producerCtx := spanCtx(t, "0102030405060708090a0b0c0d0e0f10", "0102030405060708")

	in := map[string]string{"content-type": "application/json"}
	out := mq.InjectTrace(producerCtx, in)

	if out["content-type"] != "application/json" {
		t.Errorf("existing header dropped: got %+v", out)
	}
	if _, ok := out["traceparent"]; !ok {
		t.Errorf("traceparent not added alongside existing headers: %+v", out)
	}
	// Input map must be untouched (no traceparent leaked into the caller's map).
	if _, leaked := in["traceparent"]; leaked {
		t.Error("InjectTrace mutated the caller's headers map (leaked traceparent into input)")
	}
	if len(in) != 1 {
		t.Errorf("InjectTrace mutated the caller's headers map: len now %d, want 1", len(in))
	}
}

// TestInjectTrace_NoSpan_NoHeaders verifies that with no active span and nil
// input, InjectTrace returns nil (nothing to carry) — the publish loop ranges
// over nil safely. And with no span but existing headers, those headers survive
// unchanged with no spurious traceparent.
func TestInjectTrace_NoSpan_NoHeaders(t *testing.T) {
	if got := mq.InjectTrace(context.Background(), nil); got != nil {
		t.Fatalf("no span + nil headers must yield nil, got %+v", got)
	}

	in := map[string]string{"k": "v"}
	out := mq.InjectTrace(context.Background(), in)
	if _, ok := out["traceparent"]; ok {
		t.Error("no active span must not inject a traceparent")
	}
	if out["k"] != "v" {
		t.Errorf("existing header dropped when no span: %+v", out)
	}
}

// TestExtractTrace_NoHeaders_ReturnsCtxUnchanged verifies the empty-headers fast
// path: ExtractTrace returns the same ctx (no allocation, no invalid span
// context substituted).
func TestExtractTrace_NoHeaders_ReturnsCtxUnchanged(t *testing.T) {
	type k struct{}
	base := context.WithValue(context.Background(), k{}, "v")

	if got := mq.ExtractTrace(base, nil); got != base {
		t.Error("ExtractTrace with nil headers must return the same ctx")
	}
	if got := mq.ExtractTrace(base, map[string]string{}); got != base {
		t.Error("ExtractTrace with empty headers must return the same ctx")
	}
	// And it must not have planted a (zero/invalid) span context.
	if sc := trace.SpanContextFromContext(mq.ExtractTrace(base, nil)); sc.IsValid() {
		t.Error("ExtractTrace with no headers must not synthesize a span context")
	}
}

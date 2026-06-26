// White-box tests for the middleware-chain ordering produced by the REAL server
// assembly. They drive the exact slices that NewGRPCServer / NewHTTPServer hand
// to transgrpc.Middleware / transhttp.Middleware (via the grpcMiddlewares /
// httpMiddlewares helpers — the single source of truth), so a regression in the
// wiring is caught here rather than in a synthetic stand-in chain.
package server

import (
	"context"
	"io"
	"testing"

	kErrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// discardLogger is a real kratos logger that drops output, so the logging.Server
// stage in the real chains has a valid logger and behaves as in production
// (logging.Server(nil) would nil-panic on the first request).
func discardLogger() log.Logger { return log.NewStdLogger(io.Discard) }

// Ordering invariant under test (R9F5 fix, R12 rebuild to bind the real chain):
//
// metrics.Server records a request only when its wrapped handler is invoked: it
// calls handler(ctx, req), then reads the code/reason off the returned error and
// increments the counter. So a middleware that REJECTS a request before the
// handler runs (validate → 400, ratelimit → 429) is counted iff metrics is OUTER
// to it, and silently dropped if metrics is INNER (the rejector short-circuits
// before metrics runs).
//
// NewGRPCServer keeps metrics above ratelimit/validate; NewHTTPServer keeps it
// above ratelimit (R12F4 added ratelimit to the HTTP chain for transport
// symmetry). The tests drive the REAL slices and assert a downstream rejection /
// failure is counted with its true code.
//
// Mutation self-proof: move metrics.Server to the innermost position in
// grpcMiddlewares and TestGRPCChain_MetricsCountsValidateRejectedRequest FAILS
// (validate short-circuits before metrics → the 400 stops being counted). Do the
// same in httpMiddlewares and TestHTTPChain_RatelimitPresentBelowMetrics FAILS
// (metrics becomes the innermost stage → its detected index reaches the bottom).

// failingValidate is a request value that the REAL validate.Validator() in the
// gRPC chain rejects: kratos' validator checks req.(interface{Validate() error})
// and, on a non-nil error, returns a 400 BadRequest. This drives the genuine
// validate code path against the genuine chain; only the probe value stands in
// for a proto message, which is how middleware.Middleware (typed over `any`) is
// meant to be exercised.
type failingValidate struct{}

func (failingValidate) Validate() error { return kErrors.BadRequest("VALIDATOR", "field invalid") }

// newCounter builds a real otel Int64Counter backed by a manual reader — the
// same instrument type NewGRPCServer/NewHTTPServer build at runtime — so the
// test reads back exactly what metrics.Server recorded.
func newCounter(t *testing.T) (metric.Int64Counter, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	counter, err := metrics.DefaultRequestsCounter(mp.Meter("test"), metrics.DefaultServerRequestsCounterName)
	if err != nil {
		t.Fatalf("build counter: %v", err)
	}
	return counter, reader
}

// totalRecorded sums every requests-counter data point the reader has collected.
func totalRecorded(t *testing.T, reader *sdkmetric.ManualReader) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metrics.DefaultServerRequestsCounterName {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("counter %q has unexpected data type %T", m.Name, m.Data)
			}
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}
	return total
}

// recordedCodeFor returns the counter value recorded against a specific code
// label, or 0 if none — so a test proves the rejection's TRUE code (not OK) was
// counted, not merely that some request was counted.
func recordedCodeFor(t *testing.T, reader *sdkmetric.ManualReader, wantCode int64) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metrics.DefaultServerRequestsCounterName {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("counter %q has unexpected data type %T", m.Name, m.Data)
			}
			for _, dp := range sum.DataPoints {
				if v, ok := dp.Attributes.Value("code"); ok && v.AsInt64() == wantCode {
					return dp.Value
				}
			}
		}
	}
	return 0
}

// erroringHandler is the innermost business handler; it records whether it ran
// and returns the given error, letting a test prove the handler is (or is not)
// reached and feed a downstream failure to metrics.
func erroringHandler(called *bool, retErr error) middleware.Handler {
	return func(context.Context, any) (any, error) {
		*called = true
		return "ok", retErr
	}
}

// TestGRPCChain_MetricsCountsValidateRejectedRequest pins the gRPC ordering: the
// REAL grpcMiddlewares slice puts metrics.Server above validate.Validator(), so
// a request the validator rejects (400) is still counted with its true code, and
// the business handler never runs.
//
// Mutation self-proof: move metrics.Server to the end of grpcMiddlewares
// (innermost) and this test FAILS — validate short-circuits before metrics, so
// recordedCodeFor(...,400) drops to 0.
func TestGRPCChain_MetricsCountsValidateRejectedRequest(t *testing.T) {
	counter, reader := newCounter(t)
	var handlerCalled bool

	// The exact slice NewGRPCServer hands to transgrpc.Middleware.
	chain := middleware.Chain(grpcMiddlewares(counter, nil, discardLogger())...)(
		erroringHandler(&handlerCalled, nil),
	)

	_, err := chain(context.Background(), failingValidate{})
	if err == nil {
		t.Fatal("expected validate rejection error, got nil")
	}
	if handlerCalled {
		t.Fatal("business handler must not run when validate rejects the request")
	}
	// 400 (BadRequest from validate), not 200 — proves metrics observed the
	// rejection, i.e. metrics is OUTER to validate in the real chain.
	if got := recordedCodeFor(t, reader, 400); got != 1 {
		t.Fatalf("validate-rejected request not counted with code 400: got %d, want 1 "+
			"(metrics.Server must sit ABOVE validate in grpcMiddlewares)", got)
	}
}

// TestHTTPChain_MetricsCountsDownstreamFailure proves metrics in the REAL
// httpMiddlewares slice counts a request that fails downstream with its true
// error code, rather than silently dropping it from the error rate. The HTTP
// chain has no validate middleware (the demo protos carry no Validate() rules, so
// adding it would be a no-op), so the failure is fed by the innermost handler
// returning a 400.
func TestHTTPChain_MetricsCountsDownstreamFailure(t *testing.T) {
	counter, reader := newCounter(t)
	var handlerCalled bool

	chain := middleware.Chain(httpMiddlewares(counter, nil, discardLogger())...)(
		erroringHandler(&handlerCalled, kErrors.BadRequest("BAD", "bad input")),
	)

	_, err := chain(context.Background(), struct{}{})
	if err == nil {
		t.Fatal("expected downstream error, got nil")
	}
	if !handlerCalled {
		t.Fatal("handler should run; nothing in the HTTP chain rejects a plain request")
	}
	if got := recordedCodeFor(t, reader, 400); got != 1 {
		t.Fatalf("downstream-failed request not counted with code 400: got %d, want 1", got)
	}
}

// metricsStageIndex locates, behaviorally, the index of the metrics stage in a
// freshly built copy of the slice produced by build(): it is the smallest prefix
// length whose execution records a request. Each prefix is built with its own
// metrics instrument (via build) so the readers don't share state. This pins the
// metrics position without a magic index — if metrics moves in the helper, the
// detected index moves with it.
func metricsStageIndex(t *testing.T, length int, build func(metric.Int64Counter) []middleware.Middleware) int {
	t.Helper()
	for i := 0; i < length; i++ {
		counter, reader := newCounter(t)
		prefix := build(counter)[:i+1]
		chain := middleware.Chain(prefix...)(erroringHandler(new(bool), kErrors.BadRequest("PROBE", "p")))
		_, _ = chain(context.Background(), struct{}{})
		if totalRecorded(t, reader) > 0 {
			return i
		}
	}
	return -1
}

// TestHTTPChain_RatelimitPresentBelowMetrics proves the R12F4 fix: the HTTP chain
// now carries ratelimit.Server() (BBR overload protection) like gRPC, sitting
// BELOW metrics.
//
//   - Presence: the production ratelimit.Server() uses BBR and never rejects under
//     test load, so its slot can't be triggered behaviorally; we guard its
//     PRESENCE structurally via the slice length. A revert that drops ratelimit
//     shrinks the HTTP chain from 6 to 5 and fails here.
//   - Ordering: we locate the real metrics stage behaviorally and assert it is NOT
//     the innermost stage — at least one middleware (the added ratelimit) sits
//     below it. Moving metrics to the innermost slot in httpMiddlewares makes the
//     detected index reach len-1 and fails this assertion.
func TestHTTPChain_RatelimitPresentBelowMetrics(t *testing.T) {
	build := func(c metric.Int64Counter) []middleware.Middleware {
		return httpMiddlewares(c, nil, discardLogger())
	}
	counter, _ := newCounter(t)
	httpLen := len(build(counter))

	// R12F4 presence guard: recovery,tracing,metrics,logging,metadata,ratelimit.
	if httpLen != 6 {
		t.Fatalf("httpMiddlewares length = %d, want 6 "+
			"(ratelimit.Server must be present below metrics, mirroring gRPC)", httpLen)
	}

	idx := metricsStageIndex(t, httpLen, build)
	if idx < 0 {
		t.Fatal("metrics stage not found in httpMiddlewares (metrics.Server missing?)")
	}
	if idx >= httpLen-1 {
		t.Fatalf("metrics stage is innermost (index %d of %d) — nothing sits below it; "+
			"ratelimit must sit BELOW metrics", idx, httpLen)
	}
}

// TestGRPCChain_MetricsPositionedAboveTail double-checks the gRPC side with the
// same behavioral index detection: metrics must sit above the ratelimit+validate
// tail, i.e. it is neither innermost nor second-from-innermost (validate is last,
// ratelimit second-last).
func TestGRPCChain_MetricsPositionedAboveTail(t *testing.T) {
	build := func(c metric.Int64Counter) []middleware.Middleware {
		return grpcMiddlewares(c, nil, discardLogger())
	}
	counter, _ := newCounter(t)
	grpcLen := len(build(counter))
	if grpcLen != 7 {
		t.Fatalf("grpcMiddlewares length = %d, want 7", grpcLen)
	}
	idx := metricsStageIndex(t, grpcLen, build)
	if idx < 0 {
		t.Fatal("metrics stage not found in grpcMiddlewares")
	}
	// ratelimit (len-2) and validate (len-1) must both sit below metrics.
	if idx >= grpcLen-2 {
		t.Fatalf("metrics stage at index %d of %d sits at/below the ratelimit+validate tail; "+
			"it must sit ABOVE both", idx, grpcLen)
	}
}

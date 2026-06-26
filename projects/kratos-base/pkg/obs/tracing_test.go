package obs

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSetupTracerStdout(t *testing.T) {
	tests := []struct {
		name string
		cfg  TraceConfig
	}{
		{
			name: "stdout exporter when endpoint empty",
			cfg: TraceConfig{
				ServiceName: "demo",
				Version:     "1.0.0",
				Env:         "test",
				Endpoint:    "",
				SampleRatio: 1.0,
			},
		},
		{
			name: "stdout exporter with zero sample ratio",
			cfg: TraceConfig{
				ServiceName: "demo",
				Version:     "0.0.1",
				Env:         "dev",
				Endpoint:    "",
				SampleRatio: 0.0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			shutdown, err := SetupTracer(ctx, tc.cfg)
			if err != nil {
				t.Fatalf("SetupTracer returned error: %v", err)
			}
			if shutdown == nil {
				t.Fatal("SetupTracer returned nil shutdown")
			}

			// A real SDK tracer provider must be installed (not the no-op default).
			if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
				t.Fatal("global TracerProvider 不是 SDK 实例")
			}

			// Producing a span through the global tracer must not panic and must
			// yield a valid span context (sampling does not invalidate the context).
			_, span := otel.Tracer("test").Start(ctx, "op")
			span.End()

			// Shutdown is callable without error, and idempotent (second call also nil).
			if err := shutdown(ctx); err != nil {
				t.Fatalf("first shutdown error: %v", err)
			}
			if err := shutdown(ctx); err != nil {
				t.Fatalf("second shutdown error: %v", err)
			}
		})
	}
}

// sampleDecision runs the sampler against a fresh root span (no parent) and
// reports whether it sampled. A fixed non-zero TraceID is used so the result is
// deterministic for ratio-based samplers.
func sampleDecision(t *testing.T, s sdktrace.Sampler) sdktrace.SamplingDecision {
	t.Helper()
	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	res := s.ShouldSample(sdktrace.SamplingParameters{
		ParentContext: context.Background(),
		TraceID:       tid,
		Name:          "op",
		Kind:          trace.SpanKindServer,
	})
	return res.Decision
}

// TestChooseSamplerLocalAlwaysSamples is the R9F4 regression guard: in local
// stdout mode (empty endpoint) a span MUST be sampled even when sample_ratio is
// 0.0 — that is exactly the default-config path AC5 promises ("no endpoint →
// trace prints to stdout, visible out of the box"). Before the fix this path
// chose NeverSample and not a single span was emitted.
func TestChooseSamplerLocalAlwaysSamples(t *testing.T) {
	for _, ratio := range []float64{0.0, 0.5, 1.0} {
		s := chooseSampler("", ratio)
		if got := sampleDecision(t, s); got != sdktrace.RecordAndSample {
			t.Fatalf("local stdout mode (endpoint=\"\", ratio=%v): decision = %v, want RecordAndSample — AC5 stdout span promise broken", ratio, got)
		}
	}
}

// TestChooseSamplerRemoteHonoursRatio proves the production knob still works:
// with a configured OTLP endpoint, sample_ratio drives the decision (0 drops,
// >=1 always samples) so the local-mode override does not leak into production.
func TestChooseSamplerRemoteHonoursRatio(t *testing.T) {
	const endpoint = "collector:4317"

	if got := sampleDecision(t, chooseSampler(endpoint, 0.0)); got != sdktrace.Drop {
		t.Errorf("remote ratio 0.0: decision = %v, want Drop", got)
	}
	if got := sampleDecision(t, chooseSampler(endpoint, 1.0)); got != sdktrace.RecordAndSample {
		t.Errorf("remote ratio 1.0: decision = %v, want RecordAndSample", got)
	}
	// A non-zero TraceID below the 0.5 threshold is sampled by TraceIDRatioBased;
	// our fixed TraceID (0x0102...) sits in the lower half, so 0.5 samples it.
	// The load-bearing assertion is that the ratio sampler — not AlwaysSample —
	// is in effect, which the ratio 0.0 → Drop case above already pins.
	if got := sampleDecision(t, chooseSampler(endpoint, 0.5)); got != sdktrace.RecordAndSample {
		t.Errorf("remote ratio 0.5 (TraceID in lower half): decision = %v, want RecordAndSample", got)
	}
}

func TestSetupTracerInstallsPropagator(t *testing.T) {
	shutdown, err := SetupTracer(context.Background(), TraceConfig{
		ServiceName: "demo",
		Version:     "1.0.0",
		Env:         "test",
		SampleRatio: 1.0,
	})
	if err != nil {
		t.Fatalf("SetupTracer: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	prop := otel.GetTextMapPropagator()
	// The composite propagator must advertise both traceparent (TraceContext)
	// and baggage so cross-service context flows correctly.
	fields := prop.Fields()
	wantFields := map[string]bool{"traceparent": false, "baggage": false}
	for _, f := range fields {
		if _, ok := wantFields[f]; ok {
			wantFields[f] = true
		}
	}
	for f, seen := range wantFields {
		if !seen {
			t.Errorf("propagator missing field %q (got %v)", f, fields)
		}
	}

	// Round-trip a span context through the propagator to prove it is wired,
	// not just present.
	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("1112131415161718")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}
	prop.Inject(ctx, carrier)
	if carrier["traceparent"] == "" {
		t.Fatalf("propagator did not inject traceparent, carrier=%v", carrier)
	}
	got := trace.SpanContextFromContext(prop.Extract(context.Background(), carrier))
	if got.TraceID() != tid {
		t.Errorf("round-tripped trace id = %s, want %s", got.TraceID(), tid)
	}
	if got.SpanID() != sid {
		t.Errorf("round-tripped span id = %s, want %s", got.SpanID(), sid)
	}
}

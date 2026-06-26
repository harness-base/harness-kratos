// Package obs wires the service's observability backends: OpenTelemetry tracing
// (this file) and a Prometheus-backed metrics pipeline (metrics.go).
//
// Local-first: with no exporter endpoint configured, traces print to stdout so a
// developer sees them without any collector (AC5). Point Endpoint at a collector
// to switch to OTLP/gRPC. otel is locked to a single version across all modules
// (see ADR-0002) so the SDK and exporters agree.
package obs

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// TraceConfig parameterizes SetupTracer. Endpoint selects the exporter: empty
// means the local stdout exporter; non-empty is an OTLP/gRPC collector address.
// SampleRatio is the head-sampling fraction (0..1) applied via ParentBased so a
// sampled parent keeps its children sampled — but it is honoured only when an
// OTLP Endpoint is set. In local stdout mode (empty Endpoint) every span is
// sampled regardless of SampleRatio so traces are visible out of the box (AC5).
type TraceConfig struct {
	ServiceName string
	Version     string
	Env         string
	Endpoint    string
	SampleRatio float64
}

// SetupTracer installs a global OpenTelemetry TracerProvider and W3C propagator
// for the process and returns a shutdown func that flushes and stops the span
// processor. It is the single entry point for tracing wiring; call once at boot
// and defer the returned shutdown.
//
// The returned shutdown is safe to call; calling it more than once is a no-op
// after the first (the SDK's Shutdown is idempotent and returns nil once the
// provider is already stopped).
func SetupTracer(ctx context.Context, cfg TraceConfig) (shutdown func(context.Context) error, err error) {
	res, resErr := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			semconv.DeploymentEnvironmentName(cfg.Env),
		),
	)
	// resource.New may return a non-nil error for ErrPartialResource or
	// ErrSchemaURLConflict even when a usable resource was built.  Only treat
	// errors that are neither as fatal; partial resources are acceptable for
	// local dev (we still get service.name / service.version).
	if resErr != nil &&
		!errors.Is(resErr, resource.ErrPartialResource) &&
		!errors.Is(resErr, resource.ErrSchemaURLConflict) {
		return nil, fmt.Errorf("obs: build trace resource: %w", resErr)
	}
	if res == nil {
		res = resource.Default()
	}

	exp, err := newSpanExporter(ctx, cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("obs: build span exporter: %w", err)
	}

	// Use a SimpleSpanProcessor for the stdout (local) exporter so spans are
	// written synchronously and appear in the log immediately (AC5 observability
	// check). For remote OTLP exporters use the batcher for throughput.
	var processor sdktrace.SpanProcessor
	if cfg.Endpoint == "" {
		processor = sdktrace.NewSimpleSpanProcessor(exp)
	} else {
		processor = sdktrace.NewBatchSpanProcessor(exp)
	}

	sampler := chooseSampler(cfg.Endpoint, cfg.SampleRatio)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(processor),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// chooseSampler selects the head sampler from the endpoint + sample ratio.
//
// Local stdout mode (empty endpoint) ALWAYS samples, regardless of sampleRatio.
// This is what makes AC5's "no endpoint → traces print to stdout, visible out of
// the box" promise hold under the default config (which ships sample_ratio: 1.0
// for clarity, but would otherwise break the moment a developer set it to 0). A
// local run has no collector to throttle, so there is no reason to drop spans —
// and a silent NeverSample on the default path is exactly the trap that hid the
// "not a single span" regression.
//
// With a configured OTLP endpoint we honour sampleRatio so production keeps
// head-sampling control:
//   - >= 1.0     → AlwaysSample
//   - <= 0.0     → NeverSample
//   - otherwise  → ParentBased(TraceIDRatioBased) so a sampled parent keeps its
//     children sampled across services.
func chooseSampler(endpoint string, sampleRatio float64) sdktrace.Sampler {
	if endpoint == "" {
		// Local stdout mode: always emit so a developer sees spans immediately.
		return sdktrace.AlwaysSample()
	}
	switch {
	case sampleRatio >= 1.0:
		return sdktrace.AlwaysSample()
	case sampleRatio <= 0.0:
		return sdktrace.NeverSample()
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))
	}
}

// newSpanExporter picks the span exporter from the configured endpoint: stdout
// when empty (local default), OTLP/gRPC otherwise.
func newSpanExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	if endpoint == "" {
		return stdouttrace.New()
	}
	// WithInsecure: the endpoint is an in-cluster collector; TLS is terminated at
	// the mesh/ingress layer, not here.
	return otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
}

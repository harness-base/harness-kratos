package obs

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// metrics holds the process-wide Prometheus registry and the otel MeterProvider
// bridged onto it. Instruments created through the global otel MeterProvider
// (e.g. by Kratos's otel metrics middleware) are gathered by this registry and
// exposed at /metrics (AC5).
var (
	metricsOnce sync.Once
	metricsReg  *prometheus.Registry
	metricsMP   *sdkmetric.MeterProvider
)

// setup lazily builds the registry + otel→Prometheus bridge exactly once and
// installs the MeterProvider globally. It is invoked by the public accessors so
// callers never see an uninitialized registry.
//
// The only error otelprom.New can return here stems from invalid options; we
// pass a freshly created registerer, so a failure means a fundamentally broken
// metrics stack at startup — we panic rather than hand back a silently
// half-initialized (nil) registry that would mask the problem downstream.
func setup() {
	metricsOnce.Do(func() {
		reg := prometheus.NewRegistry()
		exp, err := otelprom.New(otelprom.WithRegisterer(reg))
		if err != nil {
			panic("obs: init prometheus exporter: " + err.Error())
		}
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exp))
		otel.SetMeterProvider(mp)
		metricsReg = reg
		metricsMP = mp
	})
}

// Registry returns the process-wide Prometheus registry that backs the otel
// metrics pipeline, initializing it on first use. Use it to register additional
// native Prometheus collectors alongside the otel-sourced metrics.
func Registry() *prometheus.Registry {
	setup()
	return metricsReg
}

// MeterProvider returns the otel MeterProvider bridged onto the Prometheus
// registry, initializing it on first use. It is also installed as the global
// provider via otel.SetMeterProvider, so otel.Meter(...) yields equivalent
// instruments.
func MeterProvider() *sdkmetric.MeterProvider {
	setup()
	return metricsMP
}

// Handler returns the HTTP handler that serves the registry in Prometheus text
// format, suitable for mounting at /metrics.
func Handler() http.Handler {
	setup()
	return promhttp.HandlerFor(Registry(), promhttp.HandlerOpts{})
}

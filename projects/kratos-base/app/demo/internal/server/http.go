// Package server wires the transport layer for the demo service.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metadata"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	transhttp "github.com/go-kratos/kratos/v2/transport/http"

	v1 "github.com/z-mate/kratos-base/api/demo/v1"
	"github.com/z-mate/kratos-base/app/demo/internal/conf"
	"github.com/z-mate/kratos-base/app/demo/internal/service"
	"github.com/z-mate/kratos-base/pkg/obs"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// httpMiddlewares is the single source of truth for the HTTP middleware chain.
// NewHTTPServer builds the metrics instruments and delegates here; tests drive
// this exact slice (white-box) to pin the ordering against the real chain.
//
// Middleware order (outermost to innermost):
//  1. recovery  – panic → HTTP 500, never crash the process
//  2. tracing   – starts/propagates an OTel span (uses global TracerProvider)
//  3. metrics   – OTel counter + histogram, bridged to Prometheus via obs.setup()
//  4. logging   – structured request/response log via KratosAdapter logger
//  5. metadata  – kratos service-metadata propagation
//  6. ratelimit – adaptive BBR CPU-based rate limiting (aegis)
//
// metrics sits ABOVE ratelimit (and below recovery so a recovered panic is
// counted with its real 500) so EVERY request — including ones rejected by
// ratelimit (429) or a recovered panic (500) — is counted with its true
// code/reason. ratelimit mirrors the gRPC chain so both transports, which serve
// the same business API, share the same BBR overload protection rather than
// leaving the HTTP entry point unguarded.
func httpMiddlewares(counter metric.Int64Counter, histogram metric.Float64Histogram, logger log.Logger) []middleware.Middleware {
	return []middleware.Middleware{
		recovery.Recovery(),
		tracing.Server(),
		metrics.Server(
			metrics.WithRequests(counter),
			metrics.WithSeconds(histogram),
		),
		logging.Server(logger),
		metadata.Server(),
		ratelimit.Server(),
	}
}

// NewHTTPServer constructs a Kratos HTTP server with the standard middleware
// chain, registers the DemoService handler, and mounts the health/metrics
// endpoints.
//
// Endpoints:
//
//	GET /v1/ping          – proto-generated handler (Ping)
//	GET /v1/greet/{id}    – proto-generated handler (GetGreet)
//	GET /healthz          – liveness: always 200 while the process is alive
//	GET /readyz           – readiness: 200 when all resource checks pass, 503 otherwise
//	GET /metrics          – Prometheus metrics (obs.Handler)
func NewHTTPServer(
	rt conf.Runtime,
	svc *service.DemoService,
	reg *resource.Registry,
	logger log.Logger,
) *transhttp.Server {
	// Build OTel metric instruments (counter + histogram) using the global
	// MeterProvider which is wired to the Prometheus bridge by obs.Registry()
	// at startup. These instruments track HTTP request counts and latencies
	// and are exposed at /metrics (AC5).
	meter := otel.Meter("demo")
	counter, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		panic("server: http: init requests counter: " + err.Error())
	}
	histogram, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		panic("server: http: init seconds histogram: " + err.Error())
	}

	srv := transhttp.NewServer(
		transhttp.Address(rt.Server.HTTP.Addr),
		transhttp.Middleware(httpMiddlewares(counter, histogram, logger)...),
	)

	// Register proto-generated routes (/v1/ping, /v1/greet/{id}).
	v1.RegisterDemoServiceHTTPServer(srv, svc)

	// Liveness: the process is alive → always 200.
	srv.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Readiness: ask every registered resource check; 200 = all OK, 503 = any failure.
	// Use a dedicated context with a 15 s deadline rather than r.Context() because
	// the Kratos HTTP server applies a default 1 s per-request timeout that would
	// cancel resource probes (e.g. RocketMQ producer Start) before they complete.
	srv.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ok, details := reg.Ready(ctx)

		type response struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks,omitempty"`
		}
		resp := response{}
		if ok {
			resp.Status = "ok"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		} else {
			resp.Status = "unavailable"
			resp.Checks = make(map[string]string, len(details))
			for name, err := range details {
				resp.Checks[name] = err.Error()
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Metrics: expose Prometheus-format metrics.
	srv.Handle("/metrics", obs.Handler())

	return srv
}

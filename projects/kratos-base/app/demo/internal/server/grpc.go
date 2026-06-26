// Package server wires the transport layer for the demo service.
package server

import (
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
	"github.com/go-kratos/kratos/v2/middleware/validate"
	transgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	v1 "github.com/z-mate/kratos-base/api/demo/v1"
	"github.com/z-mate/kratos-base/app/demo/internal/conf"
	"github.com/z-mate/kratos-base/app/demo/internal/service"
)

// NewGRPCServer constructs a Kratos gRPC server with the standard middleware
// chain and registers the DemoService handler.
//
// Middleware order (outermost to innermost):
//  1. recovery  – panic → gRPC Internal, never crash the process
//  2. tracing   – starts/propagates an OTel span (uses global TracerProvider)
//  3. metrics   – OTel counter + histogram, bridged to Prometheus via obs.setup()
//  4. logging   – structured request/response log via KratosAdapter logger
//  5. metadata  – kratos service-metadata propagation
//  6. ratelimit – adaptive BBR CPU-based rate limiting (aegis)
//  7. validate  – proto-field Validate() contract check
//
// metrics sits ABOVE ratelimit/validate (and below recovery so a panic is
// recovered into a code it can observe) so that EVERY request is counted with
// its real code/reason — including ones rejected by ratelimit (429/Resource
// Exhausted), validate (400/InvalidArgument), or a recovered panic (500/
// Internal). If metrics were innermost, those rejected requests would never
// reach it and the error rate would silently under-count.
//
// grpcMiddlewares is the single source of truth for the gRPC middleware chain.
// NewGRPCServer builds the metrics instruments and delegates here; tests drive
// this exact slice (white-box) to pin the ordering against the real chain.
func grpcMiddlewares(counter metric.Int64Counter, histogram metric.Float64Histogram, logger log.Logger) []middleware.Middleware {
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
		validate.Validator(), //nolint:staticcheck // SA1019: contrib/validate not yet a direct dep; task spec mandates this middleware
	}
}

func NewGRPCServer(rt conf.Runtime, svc *service.DemoService, logger log.Logger) *transgrpc.Server {
	// Build OTel metric instruments using the global MeterProvider, which is
	// wired to the Prometheus bridge by obs.Registry()/obs.Handler() on first
	// use (or explicitly by obs.MeterProvider() in main).  We use the global
	// otel.Meter shortcut here so grpc.go stays free of a hard dep on obs.
	meter := otel.Meter("demo")

	counter, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		// Instrument creation can only fail if the global MeterProvider is the
		// no-op provider (i.e. obs was never initialised) or if the name is
		// syntactically invalid.  We treat this as a fatal misconfiguration.
		panic("server: init requests counter: " + err.Error())
	}

	histogram, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		panic("server: init seconds histogram: " + err.Error())
	}

	srv := transgrpc.NewServer(
		transgrpc.Address(rt.Server.GRPC.Addr),
		transgrpc.Middleware(grpcMiddlewares(counter, histogram, logger)...),
	)

	v1.RegisterDemoServiceServer(srv, svc)
	return srv
}

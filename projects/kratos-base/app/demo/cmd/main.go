// Command demo is the entrypoint for the demo micro-service (kratos-base S0).
//
// Boot sequence:
//  1. Parse -conf flag → bootstrap.yaml path.
//  2. Load bootstrap.yaml → choose config source (local file / future: nacos/etcd).
//  3. Load & scan runtime config into conf.Runtime.
//  4. Initialise structured logger (logx) and OTel tracer + metrics (obs).
//  5. Build confcenter.Manager[conf.Runtime] (hot-reload capable).
//  6. Build service-registry runner (non-fatal, retries on failure).
//     6b. Wire-assemble the full dependency graph → kratos.App. The runner is
//     threaded into newApp, which hooks its Start/Stop onto the kratos
//     lifecycle (AfterStart registers; BeforeStop deregisters) so the
//     instance is advertised only while the transport ports are listening.
//  7. app.Run() — blocks until SIGTERM/SIGINT, then gracefully stops.
package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"log/slog"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/config"
	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/z-mate/kratos-base/app/demo/internal/conf"
	"github.com/z-mate/kratos-base/pkg/bootstrap"
	"github.com/z-mate/kratos-base/pkg/confcenter"
	"github.com/z-mate/kratos-base/pkg/logx"
	"github.com/z-mate/kratos-base/pkg/obs"
	"github.com/z-mate/kratos-base/pkg/registryx"
)

// Build-time injection points (set via -ldflags).
var (
	// Name is the service name, injected at build time.
	Name = "demo"
	// Version is the build version, injected at build time.
	Version = "0.0.0-dev"
)

func main() {
	confPath := flag.String("conf", "configs/bootstrap.yaml", "path to bootstrap.yaml")
	flag.Parse()

	// ── Phase 1: bootstrap ────────────────────────────────────────────────
	bs, err := bootstrap.Load(*confPath)
	if err != nil {
		stdlog.Fatalf("demo: load bootstrap: %v", err)
	}

	c, err := bootstrap.NewConfigSource(bs)
	if err != nil {
		stdlog.Fatalf("demo: new config source: %v", err)
	}
	if err = c.Load(); err != nil {
		stdlog.Fatalf("demo: config load: %v", err)
	}

	// ── Phase 2: runtime config ───────────────────────────────────────────
	var rt conf.Runtime
	if err = c.Scan(&rt); err != nil {
		stdlog.Fatalf("demo: config scan: %v", err)
	}

	// ── Phase 3: logger ───────────────────────────────────────────────────
	slogLogger := logx.New(rt.Log.Level, Name, Version, "dev")
	kratosLogger := logx.KratosAdapter(slogLogger)

	// ── Phase 4: OTel tracer + metrics ────────────────────────────────────
	ctx := context.Background()
	shutdownTracer, err := obs.SetupTracer(ctx, obs.TraceConfig{
		ServiceName: Name,
		Version:     Version,
		Endpoint:    rt.Trace.Endpoint,
		SampleRatio: rt.Trace.SampleRatio,
	})
	if err != nil {
		stdlog.Fatalf("demo: setup tracer: %v", err)
	}
	defer func() {
		if serr := shutdownTracer(ctx); serr != nil {
			slogLogger.Error("demo: tracer shutdown error", slog.String("err", serr.Error()))
		}
	}()

	// Initialise the Prometheus/OTel metrics bridge so that the global
	// otel.MeterProvider used by the gRPC metrics middleware routes to
	// Prometheus (obs.Handler()).
	obs.Registry() // side-effect: calls obs.setup() which sets the global MeterProvider

	// Emit a startup span so the stdout trace exporter (AC5) has at least one
	// span to export during process startup. With SimpleSpanProcessor this is
	// synchronous: the JSON span appears in stdout immediately after Start+End.
	{
		_, startupSpan := otel.Tracer(Name).Start(ctx, "demo.startup")
		startupSpan.End()
	}

	// Enrich the Kratos logger with trace_id / span_id valuers so that the
	// logging middleware, which calls log.WithContext(ctx, logger), produces
	// per-request log lines with the active trace_id (AC5).
	kratosLogger = kratoslog.With(kratosLogger,
		"trace_id", tracing.TraceID(),
		"span_id", tracing.SpanID(),
	)

	// ── Phase 5: config manager + hot-reload ──────────────────────────────
	mgr, err := confcenter.NewManager(rt, conf.Validate)
	if err != nil {
		stdlog.Fatalf("demo: new config manager: %v", err)
	}

	if err = confcenter.BindKratosWatch(
		ctx, c,
		[]string{"server", "data", "log", "trace"},
		func(cfg config.Config) (conf.Runtime, error) {
			var r conf.Runtime
			if scanErr := cfg.Scan(&r); scanErr != nil {
				return r, scanErr
			}
			return r, nil
		},
		mgr,
		slogLogger,
	); err != nil {
		stdlog.Fatalf("demo: bind config watch: %v", err)
	}

	// ── Phase 6: service registry runner (non-fatal) ──────────────────────
	// Built before wire assembly so it can be threaded into newApp, where its
	// Start/Stop are hooked into the kratos lifecycle (AfterStart/BeforeStop).
	// This keeps registration strictly inside the "server is listening" window:
	// we never advertise the instance before the port is open, nor leave it
	// advertised after the port closes (R10F3/F6/F7).
	reg, _, regErr := registryx.New(bs.Infra.Registry.Kind, bs)
	if regErr != nil {
		// Registry construction failed (e.g. unreachable etcd probe).
		// Log a warning — registration is non-fatal per AC-D1.
		slogLogger.Warn("demo: registry construction failed, running without registration",
			slog.String("kind", bs.Infra.Registry.Kind),
			slog.String("err", regErr.Error()),
		)
	}

	advertise := bs.Infra.Registry.Advertise
	if advertise == "" {
		// Infer from runtime server.grpc.addr.
		// If addr starts with ":" (port only), prepend 127.0.0.1.
		addr := rt.Server.GRPC.Addr
		if strings.HasPrefix(addr, ":") {
			advertise = "127.0.0.1" + addr
		} else {
			advertise = addr
		}
	}

	inst := &registry.ServiceInstance{
		ID:        fmt.Sprintf("%s-%s", Name, uuid.New().String()),
		Name:      Name,
		Version:   Version,
		Endpoints: []string{"grpc://" + advertise},
	}

	runner := registryx.NewRunner(reg, inst, slogLogger, nil)

	// ── Phase 6b: wire assembly ───────────────────────────────────────────
	// newApp hooks runner.Start onto kratos.AfterStart and runner.Stop onto
	// kratos.BeforeStop, so the registry lifecycle is driven from inside the
	// kratos lifecycle rather than around app.Run().
	app, cleanup, err := wireApp(mgr, kratosLogger, runner)
	if err != nil {
		stdlog.Fatalf("demo: wire app: %v", err)
	}
	defer cleanup()

	// ── Phase 7: run (blocks until signal) ───────────────────────────────
	slogLogger.Info("demo service starting", slog.String("name", Name), slog.String("version", Version), slog.String("grpc_addr", rt.Server.GRPC.Addr), slog.String("http_addr", rt.Server.HTTP.Addr))
	if err = app.Run(); err != nil {
		slogLogger.Error("demo service exited with error", slog.String("err", err.Error()))
		os.Exit(1)
	}
}

// Package registryx provides service registry/discovery construction and a
// non-fatal registration runner for kratos-base.
//
// Design:
//   - New builds a (Registrar, Discovery) pair from the bootstrap config.
//     kind=local returns (nil, nil) — zero-registration, direct-connect mode.
//   - Runner wraps a Registrar and executes registration in the background.
//     Registration failures are logged as warnings and retried with exponential
//     back-off (default cap 30 s).  The caller's app.Run() is never blocked or
//     killed by a registration failure (AC-D1).
//   - Connections (etcd client, nacos naming client) are shared via the
//     bootstrap.Bootstrap.Infra.{Etcd,Nacos,K8s} fields — no duplicated config.
package registryx

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	etcdreg "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	kubereg "github.com/go-kratos/kratos/contrib/registry/kubernetes/v2"
	nacosreg "github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/z-mate/kratos-base/pkg/backends"
	"github.com/z-mate/kratos-base/pkg/bootstrap"
)

// New constructs a (Registrar, Discovery) pair from the bootstrap config.
//
// Supported kinds:
//   - "local" (or "")  → (nil, nil, nil) — zero-registration mode.
//   - "etcd"           → etcd-backed registry via contrib/registry/etcd.
//   - "nacos"          → Nacos-backed registry via contrib/registry/nacos (v2 SDK).
//   - "k8s"            → Kubernetes-backed registry via contrib/registry/kubernetes.
//   - anything else    → error.
//
// Connection credentials are read from bs.Infra.{Etcd,Nacos,K8s} — the same
// sections used by the config backend, so credentials are not duplicated.
func New(kind string, bs bootstrap.Bootstrap) (registry.Registrar, registry.Discovery, error) {
	switch kind {
	case "", "local":
		return nil, nil, nil

	case "etcd":
		// For registry purposes we use lazy-connect etcd (no Status probe).
		// etcdreg.New stores the client; the actual connection happens on
		// Register/Deregister.  Connectivity failures surface via the Runner's
		// retry loop rather than at startup time — this is intentional for
		// non-fatal registration semantics (AC-D1).
		client, err := backends.NewEtcdClientLazy(bs.Infra.Etcd)
		if err != nil {
			return nil, nil, fmt.Errorf("registryx/etcd: build client: %w", err)
		}
		r := etcdreg.New(client)
		return r, r, nil

	case "nacos":
		namingClient, err := backends.NewNacosNamingClient(bs.Infra.Nacos)
		if err != nil {
			return nil, nil, fmt.Errorf("registryx/nacos: build naming client: %w", err)
		}
		r := nacosreg.New(namingClient)
		return r, r, nil

	case "k8s":
		k := bs.Infra.K8s
		cfg, err := clientcmd.BuildConfigFromFlags("", k.KubeconfigPath)
		if err != nil {
			return nil, nil, fmt.Errorf("registryx/k8s: build rest config: %w", err)
		}
		clientSet, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("registryx/k8s: build clientset: %w", err)
		}
		ns := k.Namespace
		if ns == "" {
			ns = kubereg.GetNamespace()
		}
		r := kubereg.NewRegistry(clientSet, ns)
		return r, r, nil

	default:
		return nil, nil, fmt.Errorf("registryx: unknown registry kind %q (supported: local, etcd, nacos, k8s)", kind)
	}
}

// ---------------------------------------------------------------------------
// Runner — non-fatal registration runner
// ---------------------------------------------------------------------------

// Runner manages background service registration against a Registrar.
// Registration failures produce warn-level log entries and are retried with
// exponential back-off; they never kill the host process (AC-D1).
//
// Lifecycle:
//  1. Call Start(ctx) — non-blocking; spawns a background goroutine.
//  2. The goroutine calls reg.Register, retrying on failure according to the
//     injected backoff function.  On success it parks until ctx is cancelled.
//  3. Call Stop(ctx) — triggers shutdown; the goroutine calls reg.Deregister
//     with a short timeout and exits cleanly.
type Runner struct {
	reg     registry.Registrar
	inst    *registry.ServiceInstance
	log     *slog.Logger
	backoff func(ctx context.Context, attempt int) bool // return false to stop retrying

	once   sync.Once
	stopCh chan struct{}
	doneCh chan struct{}
}

// backoffDelay computes the exponential back-off for a 0-based attempt:
// 2^attempt seconds, capped at 30s. The exponent is clamped BEFORE the shift —
// without it, 1<<uint(attempt) overflows int64 (time.Duration ns) around
// attempt≥34, producing a negative/zero delay that slips past the `> cap` upper
// guard and degenerates the registration retry loop into a no-backoff CPU/RPC
// spin (exactly AC-D1's "registry down for a long time, retry forever" path).
// 2^5s = 32s already exceeds the 30s cap, so maxExp=5 both reaches the cap and
// keeps the shift far from overflow. Pure func so the clamp is unit-testable
// without waiting out real timers. Mirrors pkg/mq/supervisor.go backoffDelay.
func backoffDelay(attempt int) time.Duration {
	const (
		base   = time.Second
		cap_   = 30 * time.Second
		maxExp = 5 // 2^5s = 32s ≥ cap_; clamp guards int64 overflow at large attempt
	)
	exp := attempt
	if exp < 0 {
		exp = 0
	}
	if exp > maxExp {
		exp = maxExp
	}
	delay := time.Duration(1<<uint(exp)) * base
	if delay > cap_ {
		delay = cap_
	}
	return delay
}

// DefaultBackoff is an exponential back-off that caps at 30 s and respects
// context cancellation.  It is the default when NewRunner receives nil.
func DefaultBackoff(ctx context.Context, attempt int) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(backoffDelay(attempt)):
		return true
	}
}

// NewRunner creates a Runner.
//
//   - reg: the Registrar to use.  nil is accepted — all operations become no-ops.
//   - inst: the ServiceInstance to register.
//   - logger: structured logger for warn/info messages.
//   - backoff: back-off function called between retries.  nil → DefaultBackoff.
func NewRunner(
	reg registry.Registrar,
	inst *registry.ServiceInstance,
	logger *slog.Logger,
	backoff func(ctx context.Context, attempt int) bool,
) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	if backoff == nil {
		backoff = DefaultBackoff
	}
	return &Runner{
		reg:     reg,
		inst:    inst,
		log:     logger,
		backoff: backoff,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Start begins the background registration loop.  It is non-blocking.
// If reg is nil, Start is a no-op.  Calling Start more than once is safe
// (subsequent calls are ignored via sync.Once).
func (r *Runner) Start(ctx context.Context) {
	if r.reg == nil {
		close(r.doneCh)
		return
	}
	r.once.Do(func() {
		go r.run(ctx)
	})
}

// Stop signals the background goroutine to exit and waits for it to finish.
// Before exiting the goroutine calls reg.Deregister with a 5-second timeout.
// Stop is idempotent.
func (r *Runner) Stop(ctx context.Context) error {
	// If reg is nil, doneCh was closed immediately in Start; just return.
	if r.reg == nil {
		return nil
	}
	select {
	case <-r.stopCh:
		// already signalled
	default:
		close(r.stopCh)
	}
	select {
	case <-r.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// run is the background goroutine body.
func (r *Runner) run(ctx context.Context) {
	defer close(r.doneCh)

	// Merge the parent ctx with our internal stopCh so either can trigger exit.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-r.stopCh:
			cancel()
		case <-runCtx.Done():
		}
	}()

	// Registration loop with back-off on failure.
	// Each Register attempt uses a per-attempt timeout so that a blocking
	// backend (e.g. etcd gRPC dial) cannot prevent the runner from retrying
	// or responding to ctx cancellation.
	const registerTimeout = 5 * time.Second

	for attempt := 0; ; attempt++ {
		if runCtx.Err() != nil {
			return
		}
		regCtx, regCancel := context.WithTimeout(runCtx, registerTimeout)
		err := r.reg.Register(regCtx, r.inst)
		regCancel()
		if err != nil {
			r.log.Warn("registryx: registration failed, will retry",
				slog.String("service", r.inst.Name),
				slog.Int("attempt", attempt+1),
				slog.String("err", err.Error()),
			)
			if !r.backoff(runCtx, attempt) {
				// context cancelled during back-off
				return
			}
			continue
		}
		r.log.Info("registryx: service registered",
			slog.String("service", r.inst.Name),
			slog.Any("endpoints", r.inst.Endpoints),
		)
		break
	}

	// Parked — wait for shutdown signal.
	<-runCtx.Done()

	// Deregister with a short independent timeout so we don't block Stop forever.
	deregCtx, deregCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer deregCancel()
	if err := r.reg.Deregister(deregCtx, r.inst); err != nil {
		r.log.Warn("registryx: deregister failed",
			slog.String("service", r.inst.Name),
			slog.String("err", err.Error()),
		)
	} else {
		r.log.Info("registryx: service deregistered", slog.String("service", r.inst.Name))
	}
}

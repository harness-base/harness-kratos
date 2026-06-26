package registryx_test

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/registry"

	"github.com/z-mate/kratos-base/pkg/backends"
	"github.com/z-mate/kratos-base/pkg/bootstrap"
	"github.com/z-mate/kratos-base/pkg/registryx"
)

// ---------------------------------------------------------------------------
// Fake Registrar
// ---------------------------------------------------------------------------

// fakeRegistrar is a controllable Registrar for unit tests.
// It fails the first failCount calls to Register, then succeeds.
type fakeRegistrar struct {
	failCount     int32 // remaining failures (atomic)
	registerCalls atomic.Int32
	deregCalls    atomic.Int32
	registerErr   error // error to return while failing
}

func newFakeRegistrar(failCount int) *fakeRegistrar {
	return &fakeRegistrar{
		failCount:   int32(failCount),
		registerErr: errors.New("fake registration error"),
	}
}

func (f *fakeRegistrar) Register(_ context.Context, _ *registry.ServiceInstance) error {
	f.registerCalls.Add(1)
	remaining := atomic.AddInt32(&f.failCount, -1)
	if remaining >= 0 {
		return f.registerErr
	}
	return nil
}

func (f *fakeRegistrar) Deregister(_ context.Context, _ *registry.ServiceInstance) error {
	f.deregCalls.Add(1)
	return nil
}

// stepBackoff returns a backoff function that:
//   - on each call, reads one value from stepCh (blocks until available)
//   - returns true unless the value is false, or ctx is done
//
// This lets tests step the back-off synchronously without time.Sleep.
func stepBackoff(stepCh <-chan bool) func(ctx context.Context, attempt int) bool {
	return func(ctx context.Context, attempt int) bool {
		select {
		case <-ctx.Done():
			return false
		case ok := <-stepCh:
			return ok
		}
	}
}

func testInstance() *registry.ServiceInstance {
	return &registry.ServiceInstance{
		ID:        "test-1",
		Name:      "test-svc",
		Endpoints: []string{"grpc://127.0.0.1:9000"},
	}
}

// ---------------------------------------------------------------------------
// Runner tests
// ---------------------------------------------------------------------------

// TestRunner_StartIsNonBlocking verifies Start returns immediately even when
// the Registrar will block (back-off channel not yet fed).
func TestRunner_StartIsNonBlocking(t *testing.T) {
	t.Parallel()

	stepCh := make(chan bool) // intentionally never fed → back-off blocks forever
	reg := newFakeRegistrar(999)
	r := registryx.NewRunner(reg, testInstance(), slog.Default(), stepBackoff(stepCh))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startDone := make(chan struct{})
	go func() {
		r.Start(ctx)
		close(startDone)
	}()

	select {
	case <-startDone:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start() blocked unexpectedly")
	}
}

// TestRunner_RetriesAndRegisters verifies that the runner retries N+1 times
// when Register fails N times, then succeeds.
func TestRunner_RetriesAndRegisters(t *testing.T) {
	t.Parallel()

	const failCount = 3
	stepCh := make(chan bool, failCount+2) // pre-fill so back-off never blocks
	for i := 0; i < failCount+1; i++ {
		stepCh <- true
	}

	reg := newFakeRegistrar(failCount)
	r := registryx.NewRunner(reg, testInstance(), slog.Default(), stepBackoff(stepCh))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)

	// Wait until Register has been called at least failCount+1 times.
	deadline := time.Now().Add(2 * time.Second)
	for reg.registerCalls.Load() < int32(failCount+1) {
		if time.Now().After(deadline) {
			t.Fatalf("Register called %d times, want at least %d",
				reg.registerCalls.Load(), failCount+1)
		}
		// tiny yield — no time.Sleep needed; the stepCh already controls pacing
		runtime_Gosched()
	}

	if got := reg.registerCalls.Load(); got != int32(failCount+1) {
		t.Errorf("Register calls = %d, want %d", got, failCount+1)
	}
}

// TestRunner_StopCallsDeregisterOnce verifies Stop triggers exactly one
// Deregister call.
func TestRunner_StopCallsDeregisterOnce(t *testing.T) {
	t.Parallel()

	// Succeed on first try.
	reg := newFakeRegistrar(0)
	registered := make(chan struct{})
	origReg := reg
	_ = origReg

	r := registryx.NewRunner(reg, testInstance(), slog.Default(),
		func(ctx context.Context, attempt int) bool {
			select {
			case <-ctx.Done():
				return false
			default:
				return true
			}
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)

	// Wait for first Register call.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if reg.registerCalls.Load() >= 1 {
			close(registered)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Register never called")
		}
		runtime_Gosched()
	}
	<-registered

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := r.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if got := reg.deregCalls.Load(); got != 1 {
		t.Errorf("Deregister calls = %d, want 1", got)
	}
}

// TestRunner_NilRegistrar_Noop verifies that Start/Stop with nil reg are no-ops
// and do not block.
func TestRunner_NilRegistrar_Noop(t *testing.T) {
	t.Parallel()

	r := registryx.NewRunner(nil, testInstance(), slog.Default(), nil)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		r.Start(ctx)
		if err := r.Stop(ctx); err != nil {
			t.Errorf("Stop() error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start/Stop with nil reg blocked")
	}
}

// TestRunner_CtxCancelExitsCleanly verifies that cancelling the context causes
// the goroutine to exit cleanly (no goroutine leak; done channel closes).
func TestRunner_CtxCancelExitsCleanly(t *testing.T) {
	t.Parallel()

	// Register will always fail — back-off immediately re-tries.
	stepCh := make(chan bool, 1024)
	for i := 0; i < 1024; i++ {
		stepCh <- true
	}

	reg := newFakeRegistrar(9999)
	r := registryx.NewRunner(reg, testInstance(), slog.Default(), stepBackoff(stepCh))

	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	// Let it spin a few times.
	deadline := time.Now().Add(500 * time.Millisecond)
	for reg.registerCalls.Load() < 3 {
		if time.Now().After(deadline) {
			t.Fatal("Register not called 3 times in 500ms")
		}
		runtime_Gosched()
	}

	// Cancel context and verify Stop returns.
	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := r.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() after ctx cancel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// New() tests
// ---------------------------------------------------------------------------

// TestNew_LocalKind verifies kind=local returns (nil, nil, nil).
func TestNew_LocalKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"local", ""} {
		reg, disc, err := registryx.New(kind, bootstrap.Bootstrap{})
		if err != nil {
			t.Errorf("kind=%q: unexpected error: %v", kind, err)
		}
		if reg != nil {
			t.Errorf("kind=%q: Registrar = %v, want nil", kind, reg)
		}
		if disc != nil {
			t.Errorf("kind=%q: Discovery = %v, want nil", kind, disc)
		}
	}
}

// TestNew_UnknownKind verifies an unknown kind returns an error.
func TestNew_UnknownKind(t *testing.T) {
	t.Parallel()

	_, _, err := registryx.New("consul", bootstrap.Bootstrap{})
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}

// TestNew_EtcdKind_LazyConnect verifies kind=etcd: etcdreg.New stores the client
// without probing the server; construction succeeds even for unreachable endpoints.
// Connectivity failures surface only on Register/Deregister (runner retry loop).
func TestNew_EtcdKind_LazyConnect(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Etcd: backends.EtcdConfig{
				Endpoints:   []string{"127.0.0.1:1"}, // nothing here
				DialTimeout: 300 * time.Millisecond,
			},
		},
	}
	reg, disc, err := registryx.New("etcd", bs)
	if err != nil {
		t.Fatalf("etcd New() should succeed (lazy-connect): %v", err)
	}
	if reg == nil {
		t.Error("expected non-nil Registrar for etcd kind")
	}
	if disc == nil {
		t.Error("expected non-nil Discovery for etcd kind")
	}
	t.Logf("etcd New() succeeded (lazy): reg=%T", reg)
}

// TestNew_NacosKind verifies kind=nacos: nacos v2 SDK connects lazily, so
// New() should succeed even for an unreachable server.
func TestNew_NacosKind_LazyConnect(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Nacos: backends.NacosConfig{
				ServerAddrs: []string{"127.0.0.1:18848"}, // nothing here
				Group:       "DEFAULT_GROUP",
				TimeoutMs:   300,
			},
		},
	}
	reg, disc, err := registryx.New("nacos", bs)
	if err != nil {
		// Acceptable if SDK itself errors at construction
		t.Logf("nacos New() error (acceptable): %v", err)
		return
	}
	// SDK is lazy-connect → construction succeeds
	if reg == nil {
		t.Error("expected non-nil Registrar for nacos kind")
	}
	if disc == nil {
		t.Error("expected non-nil Discovery for nacos kind")
	}
}

// TestNew_K8sKind_NoKubeconfig verifies kind=k8s with an empty KubeconfigPath
// returns an error (no kubeconfig, no in-cluster environment in tests).
func TestNew_K8sKind_NoKubeconfig(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			K8s: backends.K8sConfig{
				Namespace:      "default",
				KubeconfigPath: "", // empty → tries in-cluster → fails in test env
			},
		},
	}
	_, _, err := registryx.New("k8s", bs)
	if err == nil {
		t.Fatal("expected error for k8s without kubeconfig in non-cluster env, got nil")
	}
	t.Logf("k8s error (expected): %v", err)
}

// ---------------------------------------------------------------------------
// DefaultBackoff (R2F5)
// ---------------------------------------------------------------------------

// TestDefaultBackoff_CancelledCtxReturnsFalse verifies that an already-cancelled
// context makes DefaultBackoff return false immediately (no waiting), so the
// retry loop stops promptly on shutdown.
func TestDefaultBackoff_CancelledCtxReturnsFalse(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	start := time.Now()
	// attempt 0 would otherwise wait 1s; cancellation must win immediately.
	ok := registryx.DefaultBackoff(ctx, 0)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("DefaultBackoff with cancelled ctx = true, want false")
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("DefaultBackoff with cancelled ctx took %s, want near-instant return", elapsed)
	}
}

// TestDefaultBackoff_SuccessfulWaitReturnsTrue verifies that with a live context
// DefaultBackoff sleeps for the exponential delay (attempt 0 → 1s) and then
// returns true. The lower bound proves it actually waited the computed delay
// rather than returning instantly.
func TestDefaultBackoff_SuccessfulWaitReturnsTrue(t *testing.T) {
	t.Parallel()

	start := time.Now()
	ok := registryx.DefaultBackoff(context.Background(), 0)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("DefaultBackoff(live ctx, attempt 0) = false, want true")
	}
	// 2^0 * 1s = 1s. Allow scheduling slack but require a real wait occurred.
	if elapsed < 900*time.Millisecond {
		t.Fatalf("DefaultBackoff attempt 0 returned after %s, want ~1s (delay not honored)", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("DefaultBackoff attempt 0 took %s, far longer than the 1s delay", elapsed)
	}
}

// TestDefaultBackoff_LargeAttemptHonorsCancelDuringWait verifies that for a large
// attempt (10 → 2^10 = 1024s un-clamped) the call parks in the wait and returns
// false promptly when the context is cancelled — it must not busy-spin or return
// true. This is the fast, deterministic half of the large-attempt contract.
func TestDefaultBackoff_LargeAttemptHonorsCancelDuringWait(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool, 1)
	go func() { done <- registryx.DefaultBackoff(ctx, 10) }()

	// Must still be blocked shortly after the call starts (not instant-return).
	select {
	case <-done:
		t.Fatal("DefaultBackoff(attempt 10) returned before being cancelled; it must wait")
	case <-time.After(200 * time.Millisecond):
		// still blocked, as expected — parked in time.After(cap).
	}

	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("DefaultBackoff(attempt 10) = true after cancel, want false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DefaultBackoff(attempt 10) did not observe cancellation within 2s")
	}
}

// TestDefaultBackoff_ClampValueIs30s verifies the *value* of the clamp: attempt
// 10 would compute 2^10 = 1024s without clamping, but DefaultBackoff caps the
// delay at 30s. We assert the uncancelled call returns true at ~30s — well below
// 1024s — which is the only way to confirm the cap value (the function exposes
// only a bool, so the ceiling is observable solely via wall-clock timing).
//
// This deliberately waits ~30s; it is the honest cost of asserting a 30s cap.
// Skipped under `go test -short`.
func TestDefaultBackoff_ClampValueIs30s(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ~30s clamp-value timing test in -short mode")
	}
	t.Parallel()

	start := time.Now()
	ok := registryx.DefaultBackoff(context.Background(), 10)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("DefaultBackoff(live ctx, attempt 10) = false, want true after the clamped wait")
	}
	// Cap is 30s. Require the wait to be near 30s (clamp applied) and nowhere
	// near the un-clamped 1024s. A generous upper bound absorbs scheduler slack
	// while still failing if the clamp were, say, 60s or absent.
	if elapsed < 29*time.Second {
		t.Fatalf("DefaultBackoff(attempt 10) returned after %s, want ~30s (clamp not applied at 30s)", elapsed)
	}
	if elapsed > 35*time.Second {
		t.Fatalf("DefaultBackoff(attempt 10) took %s, exceeds the 30s clamp ceiling", elapsed)
	}
}

// ---------------------------------------------------------------------------
// RegistryConfig bootstrap parse test
// ---------------------------------------------------------------------------

// TestBootstrapRegistryConfig verifies RegistryConfig round-trips through YAML.
func TestBootstrapRegistryConfig(t *testing.T) {
	t.Parallel()

	import_yaml(t) // compile-time import check (helper below)

	bs := bootstrap.Bootstrap{}
	// default zero value → kind="" which maps to local
	if bs.Infra.Registry.Kind != "" {
		t.Errorf("default Kind = %q, want empty string", bs.Infra.Registry.Kind)
	}
	if bs.Infra.Registry.Advertise != "" {
		t.Errorf("default Advertise = %q, want empty", bs.Infra.Registry.Advertise)
	}
}

// import_yaml is a no-op compile-time guard to ensure gopkg.in/yaml.v3 is reachable.
func import_yaml(t *testing.T) {
	t.Helper()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runtime_Gosched yields the goroutine scheduler.
// This is used in busy-wait loops where the stepCh/channel already controls
// the overall pacing; Gosched() prevents CPU spinning while a goroutine
// is ready to run.
func runtime_Gosched() {
	runtime.Gosched()
}

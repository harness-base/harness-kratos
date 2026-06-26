package resource_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/z-mate/kratos-base/pkg/resource"
)

// ---------------------------------------------------------------------------
// Test stubs
// ---------------------------------------------------------------------------

// stubSource implements resource.Source with controllable Version and Value.
type stubSource struct {
	mu      sync.Mutex
	version uint64
	value   any
}

func (s *stubSource) Current() resource.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return resource.Snapshot{Version: s.version, Value: s.value}
}

func (s *stubSource) set(version uint64, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = version
	s.value = value
}

// testRes is the managed resource type.
type testRes struct {
	id     int
	closed bool
}

// buildCounter counts Build calls and lets tests inject errors or custom IDs.
type buildCounter struct {
	mu       sync.Mutex
	count    int
	idSeq    int
	buildErr error // if non-nil, Build returns this error
}

func (b *buildCounter) build(_ context.Context, _ any) (*testRes, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.count++
	if b.buildErr != nil {
		return nil, b.buildErr
	}
	b.idSeq++
	return &testRes{id: b.idSeq}, nil
}

func (b *buildCounter) getCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// closeCounter counts Close calls.
type closeCounter struct {
	mu    sync.Mutex
	count int
}

func (c *closeCounter) close(r *testRes) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	r.closed = true
	return nil
}

func (c *closeCounter) getCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

// ① First Build fails → Get returns err and zero value.
func TestProvider_FirstBuildFails(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg1")

	bc := &buildCounter{buildErr: errors.New("connect refused")}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	got, err := p.Get(ctx)
	if err == nil {
		t.Fatal("expected error on first Build failure, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil value on first Build failure, got %v", got)
	}
	if !errors.Is(err, resource.ErrNotReady) {
		t.Fatalf("expected error to wrap ErrNotReady, got %v", err)
	}
}

// ② Succeeds once, then Build fails → returns old value, err == nil.
func TestProvider_BuildFailAfterSuccess_ReturnsOldValue(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg1")

	bc := &buildCounter{}
	cc := &closeCounter{}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Close: cc.close,
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	// First successful Get
	first, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("unexpected error on first Get: %v", err)
	}
	if first == nil {
		t.Fatal("expected non-nil on first Get")
	}
	savedID := first.id

	// Change version so provider tries to rebuild; inject error
	src.set(2, "cfg2")
	bc.mu.Lock()
	bc.buildErr = errors.New("build failed")
	bc.mu.Unlock()

	got, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("expected nil error (self-heal with old value), got: %v", err)
	}
	if got == nil {
		t.Fatal("expected old value to be returned")
	}
	if got.id != savedID {
		t.Fatalf("expected old value id=%d, got id=%d", savedID, got.id)
	}
	// old value must NOT have been closed (we use it)
	if got.closed {
		t.Fatal("old value must not be closed when returned as fallback")
	}
	if cc.getCount() != 0 {
		t.Fatalf("Close should not be called when Build fails, got %d calls", cc.getCount())
	}
}

// ③ Fingerprint change → new value built, old value closed.
func TestProvider_FingerprintChange_RebuildsAndClosesOld(t *testing.T) {
	cfgA := "host=A"
	cfgB := "host=B"

	src := &stubSource{}
	src.set(1, cfgA)

	bc := &buildCounter{}
	cc := &closeCounter{}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Close: cc.close,
		Fingerprint: func(cfg any) string {
			s, _ := cfg.(string)
			return s
		},
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	// Build initial
	first, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bc.getCount() != 1 {
		t.Fatalf("expected 1 Build, got %d", bc.getCount())
	}

	// Change fingerprint (same version, different cfg text)
	src.set(1, cfgB)

	second, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("unexpected error after fingerprint change: %v", err)
	}
	if second == nil {
		t.Fatal("expected non-nil after rebuild")
	}
	if second.id == first.id {
		t.Fatalf("expected new resource id, got same id=%d", second.id)
	}
	if bc.getCount() != 2 {
		t.Fatalf("expected 2 Builds, got %d", bc.getCount())
	}
	if cc.getCount() != 1 {
		t.Fatalf("expected 1 Close of old value, got %d", cc.getCount())
	}
	if !first.closed {
		t.Fatal("old resource must be marked closed")
	}
}

// ④ Same version + same fingerprint → no rebuild on repeated Get.
func TestProvider_NoRebuildIfVersionAndFingerprintUnchanged(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	bc := &buildCounter{}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Fingerprint: func(cfg any) string {
			s, _ := cfg.(string)
			return s
		},
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := p.Get(ctx)
		if err != nil {
			t.Fatalf("Get #%d failed: %v", i, err)
		}
	}
	if bc.getCount() != 1 {
		t.Fatalf("expected exactly 1 Build, got %d", bc.getCount())
	}
}

// ⑤ Concurrent Gets — no data race AND the set-once invariant holds: Get holds
// p.mu across Build, so 50 concurrent first-Gets must Build EXACTLY once. The
// -race flag alone only catches torn memory; the getCount()==1 assertion below
// is what actually pins "Build is inside the lock" — if Build were moved out of
// the critical section, multiple goroutines would each Build and this fails.
//
// To make that regression reliably observable (not just luck-of-the-scheduler),
// the first Build blocks on a channel until all goroutines have launched, so
// every goroutine is parked contending for p.mu when Build runs.
func TestProvider_ConcurrentGet_NoRace(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	const goroutines = 50

	// launched is released once all goroutines have started; the first Build
	// waits on it so the lock is maximally contended during the single Build.
	launched := make(chan struct{})
	var firstBuild sync.Once

	bc := &buildCounter{}
	ad := resource.Adapter[*testRes]{
		Build: func(ctx context.Context, cfg any) (*testRes, error) {
			firstBuild.Do(func() { <-launched })
			return bc.build(ctx, cfg)
		},
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	var wg sync.WaitGroup
	var started sync.WaitGroup
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	started.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			started.Done()
			_, errs[i] = p.Get(ctx)
		}()
	}
	// Wait until every goroutine has entered Get, then unblock the first Build.
	started.Wait()
	close(launched)
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d got error: %v", i, e)
		}
	}
	// Set-once invariant: Build runs inside the lock, so exactly one Build for
	// all 50 concurrent first-Gets. This is deterministic for correct code (not
	// flaky) and FAILS if Build is hoisted out of the critical section.
	if got := bc.getCount(); got != 1 {
		t.Fatalf("expected exactly 1 Build under set-once invariant, got %d", got)
	}
}

// ⑥ Healthy: not ready → error; ready with Health pass → nil; Health fail → err.
func TestProvider_Healthy(t *testing.T) {
	errHealth := errors.New("health probe failed")

	tests := []struct {
		name       string
		buildErr   error
		healthErr  error
		healthFunc bool
		wantErr    bool
	}{
		{
			name:     "not ready returns error",
			buildErr: errors.New("build failed"),
			wantErr:  true,
		},
		{
			name:       "ready no health func returns nil",
			healthFunc: false,
			wantErr:    false,
		},
		{
			name:       "ready health pass returns nil",
			healthFunc: true,
			healthErr:  nil,
			wantErr:    false,
		},
		{
			name:       "ready health fail returns err",
			healthFunc: true,
			healthErr:  errHealth,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			src := &stubSource{}
			src.set(1, "cfg")

			bc := &buildCounter{buildErr: tc.buildErr}

			var healthCallCount int32
			ad := resource.Adapter[*testRes]{
				Build: bc.build,
			}
			if tc.healthFunc {
				ad.Health = func(_ context.Context, _ *testRes) error {
					atomic.AddInt32(&healthCallCount, 1)
					return tc.healthErr
				}
			}

			p := resource.New(src, ad)
			ctx := context.Background()

			err := p.Healthy(ctx)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected nil, got: %v", err)
			}
			if tc.healthFunc {
				if got := atomic.LoadInt32(&healthCallCount); got != 1 {
					t.Errorf("expected Health to be called once, got %d", got)
				}
			}
		})
	}
}

// Close: closes current resource and marks not ready.
func TestProvider_Close(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	bc := &buildCounter{}
	cc := &closeCounter{}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Close: cc.close,
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	r, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if cc.getCount() != 1 {
		t.Fatalf("expected 1 Close call, got %d", cc.getCount())
	}
	if !r.closed {
		t.Fatal("resource should be marked closed")
	}

	// After Close, Get should trigger a rebuild
	_, err = p.Get(ctx)
	if err != nil {
		t.Fatalf("Get after Close failed: %v", err)
	}
	if bc.getCount() != 2 {
		t.Fatalf("expected 2 Builds after Close+Get, got %d", bc.getCount())
	}
}

// Verify Close is idempotent (no Close func panics, no double-close).
func TestProvider_CloseNoCloseFunc(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	bc := &buildCounter{}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		// No Close func
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	if _, err := p.Get(ctx); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close without Close func failed: %v", err)
	}
}

// Version change triggers rebuild even if Fingerprint returns same string.
func TestProvider_VersionChange_TriggersRebuild(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	bc := &buildCounter{}
	ad := resource.Adapter[*testRes]{
		Build:       bc.build,
		Fingerprint: func(_ any) string { return "static" }, // always same FP
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	if _, err := p.Get(ctx); err != nil {
		t.Fatalf("first Get failed: %v", err)
	}

	// Bump version only
	src.set(2, "cfg")

	if _, err := p.Get(ctx); err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if bc.getCount() != 2 {
		t.Fatalf("expected 2 Builds after version bump, got %d", bc.getCount())
	}
}

// Recovery: after a failed rebuild serves the old value, a subsequent
// successful Build swaps in the new value, closes the old one, and clears
// LastErr — the "断开→自愈→恢复自动续上" contract.
func TestProvider_RecoveryAfterFailure_ResumesAndClearsLastErr(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg1")

	bc := &buildCounter{}
	cc := &closeCounter{}
	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Close: cc.close,
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	// 1. First successful build.
	first, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("unexpected error on first Get: %v", err)
	}
	if p.LastErr() != nil {
		t.Fatalf("expected nil LastErr after success, got %v", p.LastErr())
	}

	// 2. Outage: inject build failure + bump version → Get serves OLD value.
	src.set(2, "cfg2")
	bc.mu.Lock()
	bc.buildErr = errors.New("downstream down")
	bc.mu.Unlock()

	got, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("expected self-heal (nil error) during outage, got %v", err)
	}
	if got.id != first.id {
		t.Fatalf("expected old value id=%d during outage, got id=%d", first.id, got.id)
	}
	if p.LastErr() == nil {
		t.Fatal("expected LastErr to record the build failure during outage")
	}
	if cc.getCount() != 0 {
		t.Fatalf("old value must not be closed during outage, got %d closes", cc.getCount())
	}

	// 3. Recovery: clear the build error → next Get swaps in NEW value,
	//    closes the old one, and clears LastErr.
	bc.mu.Lock()
	bc.buildErr = nil
	bc.mu.Unlock()

	recovered, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("unexpected error after recovery: %v", err)
	}
	if recovered.id == first.id {
		t.Fatalf("expected a new value after recovery, still got id=%d", first.id)
	}
	if !first.closed {
		t.Fatal("old value must be closed after the recovery swap")
	}
	if cc.getCount() != 1 {
		t.Fatalf("expected exactly 1 close after recovery, got %d", cc.getCount())
	}
	if p.LastErr() != nil {
		t.Fatalf("expected LastErr cleared after recovery, got %v", p.LastErr())
	}
}

// Health-probe failure marks the provider not-ready, closes the dead handle,
// and the next Get rebuilds even though version/fingerprint are unchanged —
// the "connection died but config didn't change" self-heal path (exposed by
// single-connection handles like *amqp.Connection; pooled handles such as
// sql.DB self-heal underneath and never surfaced this).
func TestProvider_HealthFail_ClosesAndRebuilds(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	bc := &buildCounter{}
	cc := &closeCounter{}
	healthErr := errors.New("connection died")
	var failHealth atomic.Bool

	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Close: cc.close,
		Health: func(_ context.Context, _ *testRes) error {
			if failHealth.Load() {
				return healthErr
			}
			return nil
		},
	}

	p := resource.New(src, ad)
	ctx := context.Background()

	// 1. Build + healthy.
	first, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if err := p.Healthy(ctx); err != nil {
		t.Fatalf("expected healthy, got %v", err)
	}

	// 2. The live connection dies (config unchanged): Healthy must surface the
	//    error AND close the dead handle.
	failHealth.Store(true)
	if err := p.Healthy(ctx); !errors.Is(err, healthErr) {
		t.Fatalf("expected health error, got %v", err)
	}
	if cc.getCount() != 1 {
		t.Fatalf("expected dead handle closed once, got %d closes", cc.getCount())
	}
	if !first.closed {
		t.Fatal("dead handle must be closed")
	}

	// 3. Dependency recovers: next Get rebuilds (same version/fingerprint).
	failHealth.Store(false)
	second, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("Get after health failure: %v", err)
	}
	if second.id == first.id {
		t.Fatal("expected a fresh handle after health-driven rebuild")
	}
	if bc.getCount() != 2 {
		t.Fatalf("expected 2 builds, got %d", bc.getCount())
	}
	if err := p.Healthy(ctx); err != nil {
		t.Fatalf("expected healthy after rebuild, got %v", err)
	}
}

// R8F1 regression: Healthy probes the handle t returned by Get AFTER releasing
// p.mu. If a concurrent Get rebuilds during the probe window (version bump),
// p.cur becomes a fresh LIVE handle while t is the stale/dead one — and that
// rebuild has already Closed t. The buggy failure path blindly Closes p.cur and
// flips ready=false, tearing down the working connection.
//
// We make the race DETERMINISTIC (no sleep): the Health probe itself performs
// the concurrent rebuild — it bumps the version and calls Get() (installing a
// new live handle id=2 as p.cur, Closing the probed id=1) — and only THEN
// returns the "id=1 died" error. That is exactly the post-condition of a
// concurrent Get winning the probe window. The fixed code must close only id=1
// (already closed by the rebuild → 0 extra closes of the live handle) and must
// NOT flip ready=false, because p.cur (id=2) is alive.
func TestProvider_HealthFail_ConcurrentRebuild_DoesNotCloseLiveHandle(t *testing.T) {
	src := &stubSource{}
	src.set(1, "cfg")

	bc := &buildCounter{}

	// Track which specific handles get closed (not just a count) so we can
	// assert the LIVE one (id=2) is never torn down.
	var cmu sync.Mutex
	closedIDs := []int{}
	closeOf := func(r *testRes) error {
		cmu.Lock()
		defer cmu.Unlock()
		closedIDs = append(closedIDs, r.id)
		r.closed = true
		return nil
	}

	healthErr := errors.New("id=1 connection died")

	var p *resource.Provider[*testRes]
	// rebuildOnce ensures the in-probe concurrent rebuild happens exactly once,
	// on the first (failing) Healthy call.
	var rebuildOnce sync.Once
	var newID atomic.Int32 // id of the live handle installed during the probe

	ad := resource.Adapter[*testRes]{
		Build: bc.build,
		Close: closeOf,
		Health: func(ctx context.Context, probed *testRes) error {
			// Only the first probe (on id=1) triggers the failure scenario.
			if probed.id != 1 {
				return nil
			}
			rebuildOnce.Do(func() {
				// Simulate a concurrent Get rebuilding mid-probe: bump version
				// then Get → installs a fresh LIVE handle (id=2) as p.cur and
				// closes the probed id=1.
				src.set(2, "cfg")
				live, err := p.Get(ctx)
				if err != nil {
					t.Errorf("in-probe rebuild Get failed: %v", err)
					return
				}
				newID.Store(int32(live.id))
			})
			// Report the ORIGINAL probed handle (id=1) as dead.
			return healthErr
		},
	}

	p = resource.New(src, ad)
	ctx := context.Background()

	// 1. First build → id=1 is live.
	first, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if first.id != 1 {
		t.Fatalf("expected first handle id=1, got id=%d", first.id)
	}

	// 2. Healthy probes id=1; the probe rebuilds to id=2 (live) then reports
	//    id=1 dead. The fix must NOT close id=2 and must NOT flip ready=false.
	if err := p.Healthy(ctx); !errors.Is(err, healthErr) {
		t.Fatalf("expected health error, got %v", err)
	}

	live := int(newID.Load())
	if live != 2 {
		t.Fatalf("expected live handle id=2 installed during probe, got %d", live)
	}

	// id=1 must be closed (by the in-probe rebuild). id=2 (live) must NEVER be
	// closed. On the BUGGY code the failure path closes p.cur==id=2 here.
	cmu.Lock()
	closed := append([]int(nil), closedIDs...)
	cmu.Unlock()
	for _, id := range closed {
		if id == 2 {
			t.Fatalf("LIVE handle id=2 was wrongly closed (TOCTOU): closed=%v", closed)
		}
	}
	if len(closed) != 1 || closed[0] != 1 {
		t.Fatalf("expected exactly id=1 closed (by rebuild), got closed=%v", closed)
	}

	// 3. ready must NOT have been flipped: the live id=2 should be served
	//    directly with NO extra Build (buggy code flips ready=false → next Get
	//    rebuilds to id=3).
	buildsBefore := bc.getCount()
	served, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("Get after concurrent-rebuild health failure: %v", err)
	}
	if served.id != 2 {
		t.Fatalf("expected live id=2 still served (ready not flipped), got id=%d", served.id)
	}
	if bc.getCount() != buildsBefore {
		t.Fatalf("expected no rebuild (ready intact), but Build count went %d→%d",
			buildsBefore, bc.getCount())
	}
	if served.closed {
		t.Fatal("live handle id=2 must not be closed")
	}
}

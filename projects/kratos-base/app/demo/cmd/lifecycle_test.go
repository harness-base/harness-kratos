package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2"
)

// failTimeout is a generous upper bound used ONLY to fail-fast on a hung or
// deadlocked wiring mutation. The happy path is fully channel-driven, never
// gated on this timer — it is a watchdog, not a synchronization primitive.
const failTimeout = 2 * time.Second

// recordingServer is a fake transport.Server that records the order in which
// Start/Stop run against a shared, mutex-guarded sequence. Its Start blocks
// (like a real listening server) until Stop is called, and it closes
// enteredStart at Start-body entry so the spy runner can establish a
// happens-before relationship with the server actually being up.
type recordingServer struct {
	mu  *sync.Mutex
	seq *[]string

	enteredStart chan struct{} // closed when Start body begins
	stop         chan struct{} // closed by Stop to release a blocked Start
}

func newRecordingServer(mu *sync.Mutex, seq *[]string) *recordingServer {
	return &recordingServer{
		mu:           mu,
		seq:          seq,
		enteredStart: make(chan struct{}),
		stop:         make(chan struct{}),
	}
}

func (s *recordingServer) record(ev string) {
	s.mu.Lock()
	*s.seq = append(*s.seq, ev)
	s.mu.Unlock()
}

func (s *recordingServer) Start(ctx context.Context) error {
	s.record("server.start")
	close(s.enteredStart)
	// Block like a real server until Stop releases us (or ctx dies).
	select {
	case <-s.stop:
	case <-ctx.Done():
	}
	return nil
}

func (s *recordingServer) Stop(_ context.Context) error {
	s.record("server.stop")
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	return nil
}

// spyRunner satisfies registryRunner and records Start/Stop ordering into the
// same shared sequence. Its Start blocks on enteredStart before recording: this
// is what makes the AfterStart-vs-BeforeStart distinction deterministic. If the
// production wiring put runner.Start on BeforeStart, kratos would call it before
// any server goroutine launches, so enteredStart would never close and Start
// would deadlock — surfaced as a watchdog timeout failure below.
type spyRunner struct {
	mu  *sync.Mutex
	seq *[]string

	enteredStart  <-chan struct{} // server's Start-body entry signal
	startedCh     chan struct{}   // closed once runner.Start has recorded
	startCalls    int
	stopCalls     int
	lastStopBound bool // true if Stop received a bounded (non-background) ctx
}

func newSpyRunner(mu *sync.Mutex, seq *[]string, enteredStart <-chan struct{}) *spyRunner {
	return &spyRunner{
		mu:           mu,
		seq:          seq,
		enteredStart: enteredStart,
		startedCh:    make(chan struct{}),
	}
}

func (r *spyRunner) Start(_ context.Context) {
	// Establish happens-before: only proceed once the transport server has
	// actually entered its Start body. Under correct AfterStart wiring this is
	// already true (kratos runs AfterStart after server start begins); under a
	// BeforeStart mutation this never unblocks → deadlock → test fails.
	<-r.enteredStart

	r.mu.Lock()
	*r.seq = append(*r.seq, "runner.start")
	r.startCalls++
	r.mu.Unlock()
	close(r.startedCh)
}

func (r *spyRunner) Stop(ctx context.Context) error {
	r.mu.Lock()
	*r.seq = append(*r.seq, "runner.stop")
	r.stopCalls++
	// The BeforeStop hook wraps ctx in a WithTimeout, so a deadline must be set.
	_, hasDeadline := ctx.Deadline()
	r.lastStopBound = hasDeadline
	r.mu.Unlock()
	return nil
}

func indexOf(seq []string, ev string) int {
	for i, s := range seq {
		if s == ev {
			return i
		}
	}
	return -1
}

// TestRegistryLifecycle_StartAfterServerStart_StopBeforeServerStop pins the
// registry Runner into the kratos lifecycle:
//
//   - runner.Start is driven from AfterStart, so it runs only after the
//     transport server's Start has begun (no "registered but port not open"
//     window). The spy's Start blocks on the server's enteredStart signal, so a
//     BeforeStart mutation deadlocks (caught by the watchdog).
//   - runner.Stop is driven from BeforeStop, so it runs strictly before the
//     transport server's Stop (no "still registered but port closed" window).
//
// Mutation self-justification:
//   - AfterStart → BeforeStart  : runner.Start blocks on enteredStart forever
//     (servers not yet launched) → watchdog timeout FAIL.
//   - AfterStart → AfterStop    : runner.Start never runs during startup →
//     startedCh never closes → watchdog timeout FAIL.
//   - drop the AfterStart hook   : runner.Start never called → FAIL.
//   - BeforeStop → AfterStop     : runner.stop recorded AFTER server.stop →
//     order assertion FAIL.
//   - drop the BeforeStop hook    : runner.stop never called → stopCalls==0 FAIL.
//   - revert to main-defer Stop   : Stop runs after app.Run returns (after
//     server.stop) → order assertion FAIL (same as BeforeStop→AfterStop).
func TestRegistryLifecycle_StartAfterServerStart_StopBeforeServerStop(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	srv := newRecordingServer(&mu, &seq)
	runner := newSpyRunner(&mu, &seq, srv.enteredStart)

	// Build a kratos.App using the EXACT production lifecycle options under test.
	opts := []kratos.Option{kratos.Server(srv)}
	opts = append(opts, registryLifecycleOptions(runner)...)
	app := kratos.New(opts...)

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run() }()

	// Wait for the runner to have started — proves AfterStart fired after the
	// server's Start began. Purely channel-driven; the timer is only a watchdog.
	select {
	case <-runner.startedCh:
	case <-time.After(failTimeout):
		t.Fatal("runner.Start was not driven from AfterStart within watchdog window " +
			"(wiring regression: not hooked, or hooked to BeforeStart/AfterStop)")
	}

	// Trigger graceful shutdown: kratos runs BeforeStop (runner.Stop) before it
	// cancels the run ctx and stops the servers.
	if err := app.Stop(); err != nil {
		t.Fatalf("app.Stop() returned error: %v", err)
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("app.Run() returned error: %v", err)
		}
	case <-time.After(failTimeout):
		t.Fatal("app.Run() did not return after Stop within watchdog window")
	}

	mu.Lock()
	defer mu.Unlock()

	if runner.startCalls != 1 {
		t.Errorf("runner.Start called %d times, want exactly 1", runner.startCalls)
	}
	if runner.stopCalls != 1 {
		t.Errorf("runner.Stop called %d times, want exactly 1", runner.stopCalls)
	}
	if !runner.lastStopBound {
		t.Error("runner.Stop received a ctx without deadline; BeforeStop hook must bound the deregister wait (registryStopTimeout)")
	}

	// Deterministic core assertion: deregister happens before the server stops.
	iRunnerStop := indexOf(seq, "runner.stop")
	iServerStop := indexOf(seq, "server.stop")
	if iRunnerStop < 0 || iServerStop < 0 {
		t.Fatalf("missing stop events in sequence %v", seq)
	}
	if iRunnerStop >= iServerStop {
		t.Errorf("runner.stop (idx %d) must precede server.stop (idx %d); registry must be withdrawn before the port closes. seq=%v",
			iRunnerStop, iServerStop, seq)
	}

	// Sanity: the start side ran before the stop side.
	if iRunnerStart := indexOf(seq, "runner.start"); iRunnerStart < 0 || iRunnerStart >= iRunnerStop {
		t.Errorf("runner.start must precede runner.stop; seq=%v", seq)
	}
}

// TestRegistryLifecycle_StartHookIsNonFatal proves the AfterStart hook is
// non-fatal (AC-D1): it returns nil so a registry that is down (the runner
// retries in the background) can never abort kratos startup.
//
// Mutation self-justification:
//   - make the AfterStart hook return the registry error → this test FAILs
//     because app.Run would abort startup and return that error.
func TestRegistryLifecycle_StartHookIsNonFatal(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	srv := newRecordingServer(&mu, &seq)
	runner := newSpyRunner(&mu, &seq, srv.enteredStart)

	opts := []kratos.Option{kratos.Server(srv)}
	opts = append(opts, registryLifecycleOptions(runner)...)
	app := kratos.New(opts...)

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run() }()

	select {
	case <-runner.startedCh:
	case <-time.After(failTimeout):
		t.Fatal("runner.Start (AfterStart) did not run within watchdog window")
	}

	// If startup were fatal on registry failure, app.Run would already have
	// returned a non-nil error here; instead it must still be blocked serving.
	select {
	case err := <-runErr:
		t.Fatalf("app.Run() returned %v before Stop; AfterStart registration must be non-fatal", err)
	default:
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("app.Stop() returned error: %v", err)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("app.Run() returned error: %v", err)
		}
	case <-time.After(failTimeout):
		t.Fatal("app.Run() did not return after Stop")
	}
}

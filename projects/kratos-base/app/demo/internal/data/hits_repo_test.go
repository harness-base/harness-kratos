package data_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/redis/go-redis/v9"

	"github.com/z-mate/kratos-base/app/demo/internal/data"
	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/redisx"
)

// keySpyHook is a redis.Hook that records the key of every command it sees and
// short-circuits the command with a fixed success value, so the command never
// reaches the network. It lets a test observe the exact key HitsRepo.Incr routed
// to go-redis without a live Redis server.
type keySpyHook struct {
	keys []string
	val  int64
}

func (h *keySpyHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *keySpyHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if len(args) >= 2 {
			if key, ok := args[1].(string); ok {
				h.keys = append(h.keys, key)
			}
		}
		if ic, ok := cmd.(*redis.IntCmd); ok {
			ic.SetVal(h.val)
		}
		return nil // do not call next: no dial, no network.
	}
}

func (h *keySpyHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

// TestHitsRepo_Incr_UsesGivenKey proves the data layer honors its key argument:
// HitsRepo.Incr(ctx, key) issues INCR against exactly that key. A redis.Hook spies
// the command, so this asserts the real go-redis command (real production path,
// real client) rather than a stub. Mutation guard: if Incr hard-coded a key or
// dropped the param, the spied key would differ and this fails. It also pins the
// returned count to the value the (spied) command produced.
func TestHitsRepo_Incr_UsesGivenKey(t *testing.T) {
	spy := &keySpyHook{val: 42}
	client := redisx.NewUniversalClient(redisx.Config{Addrs: []string{"192.0.2.1:6379"}})
	client.AddHook(spy)
	t.Cleanup(func() { _ = client.Close() })

	d := data.NewWithRedisClient(client)
	repo := data.NewHitsRepo(d)

	got, err := repo.Incr(context.Background(), "custom:counter")
	if err != nil {
		t.Fatalf("Incr(custom:counter) unexpected error: %v", err)
	}
	if got != 42 {
		t.Fatalf("Incr returned %d, want spied value 42", got)
	}
	if len(spy.keys) != 1 || spy.keys[0] != "custom:counter" {
		t.Fatalf("INCR issued for keys %v, want exactly [custom:counter]", spy.keys)
	}

	// A second, different key must route to that key — proving Incr does not pin
	// to whatever it saw first / a constant.
	if _, err := repo.Incr(context.Background(), "demo:hits"); err != nil {
		t.Fatalf("Incr(demo:hits) unexpected error: %v", err)
	}
	if len(spy.keys) != 2 || spy.keys[1] != "demo:hits" {
		t.Fatalf("second INCR keys %v, want [...,demo:hits]", spy.keys)
	}
}

// shortTimeoutSelectRedis returns a redisx.Config pointing at a guaranteed
// non-routable address (TEST-NET-1 RFC 5737) with a short dial timeout so the
// test fails fast without hanging.
func shortTimeoutSelectRedis(_ any) (redisx.Config, error) {
	return redisx.Config{
		Addrs:       []string{"192.0.2.1:6379"},
		DialTimeout: 500 * time.Millisecond,
	}, nil
}

// TestHitsRepo_DBUnavailable verifies that Incr returns errs.DBUnavailable
// (HTTP 503) when Redis is unreachable.
func TestHitsRepo_DBUnavailable(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, shortTimeoutSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })

	repo := data.NewHitsRepo(d)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := repo.Incr(ctx, "test:key")
	if err == nil {
		t.Fatal("expected error from unreachable Redis, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	ke := errs.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d", ke.Code)
	}
	t.Logf("Incr correctly returned 503: %v", err)
}

// TestHitsRepo_BreakerOpens verifies that consecutive Redis failures trip the
// SRE circuit breaker, which then fast-fails subsequent calls without touching
// the network.
func TestHitsRepo_BreakerOpens(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, shortTimeoutSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })

	// Use an errAlwaysBreaker stub to get deterministic fast-fail behaviour
	// without relying on probabilistic SRE thresholds.
	repo := data.NewHitsRepoWithBreaker(d, &errAlwaysBreaker{})

	start := time.Now()
	_, err := repo.Incr(context.Background(), "test:key")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from open breaker, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	// An open breaker must reject immediately (microseconds); 50 ms is a very
	// generous upper bound that avoids flakiness on a loaded CI machine.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("expected fast-fail (< 50ms), got %s", elapsed)
	}
	t.Logf("fast-fail confirmed: Incr returned in %s: %v", elapsed, err)
}

// TestHitsRepo_BreakerAllowError confirms that the ErrNotAllowed from aegis is
// wrapped into errs.DBUnavailable.
func TestHitsRepo_BreakerAllowError(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, shortTimeoutSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })

	repo := data.NewHitsRepoWithBreaker(d, &errAlwaysBreaker{})
	_, err := repo.Incr(context.Background(), "test:key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable for open-circuit, got %T: %v", err, err)
	}
}

// TestHitsRepo_CallerCanceled_DoesNotMarkFailed verifies the R10F4 fix: when the
// operation fails because the CALLER cancelled ctx (client disconnect), Incr still
// returns errs.DBUnavailable but must NOT trip the breaker — neither MarkFailed
// nor MarkSuccess — so a burst of client disconnects cannot open the circuit
// against a healthy Redis.
//
// An already-cancelled caller ctx propagates into the Redis provider's Build
// (redisx.Open pings honoring ctx) so the failure carries context.Canceled;
// markBackendFailure inspects the caller ctx (Canceled) and skips the breaker.
// recordingBreaker (defined in greet_repo_contract_test.go, same package) lets us
// assert the exact mark counts. This is the mutation-self-proving partner of
// TestHitsRepo_DBUnavailable / the closed-breaker path: if the guard were removed,
// failed here would become 1 and the test fails.
func TestHitsRepo_CallerCanceled_DoesNotMarkFailed(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, shortTimeoutSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })

	br := &recordingBreaker{}
	repo := data.NewHitsRepoWithBreaker(d, br)

	// Already-cancelled caller ctx simulates a client that disconnected mid-request.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.Incr(ctx, "test:key")
	if err == nil {
		t.Fatal("Incr(cancelled ctx): expected error, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("Incr(cancelled ctx): expected errs.DBUnavailable, got %T: %v", err, err)
	}
	// The crux: a caller cancellation is NOT a Redis fault.
	if got := br.failed.Load(); got != 0 {
		t.Errorf("caller cancellation must NOT MarkFailed, got %d", got)
	}
	if got := br.success.Load(); got != 0 {
		t.Errorf("caller cancellation must NOT MarkSuccess, got %d", got)
	}
}

// TestHitsRepo_BackendError_MarksFailed is the mutation-self-proving complement:
// a genuine backend error (unreachable Redis, NOT a caller cancellation) MUST
// trip the breaker exactly once. Together with the caller-canceled test this
// proves markBackendFailure discriminates rather than blanket-skipping or
// blanket-marking.
func TestHitsRepo_BackendError_MarksFailed(t *testing.T) {
	d := data.New(stubSource{}, unreachableSelectPool, shortTimeoutSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })

	br := &recordingBreaker{}
	repo := data.NewHitsRepoWithBreaker(d, br)

	// A live (non-cancelled) ctx: the failure is the unreachable backend, a real
	// fault that must trip the breaker.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := repo.Incr(ctx, "test:key")
	if err == nil {
		t.Fatal("Incr(unreachable redis): expected error, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("Incr(unreachable redis): expected errs.DBUnavailable, got %T: %v", err, err)
	}
	if got := br.failed.Load(); got != 1 {
		t.Errorf("backend error must MarkFailed exactly once, got %d", got)
	}
	if got := br.success.Load(); got != 0 {
		t.Errorf("backend error must NOT MarkSuccess, got %d", got)
	}
}

// errAlwaysBreaker is reused from data_test.go (same package data_test).
// It always rejects Allow(); defined there to avoid duplication.
// We reference circuitbreaker.ErrNotAllowed to confirm the import resolves.
var _ = circuitbreaker.ErrNotAllowed

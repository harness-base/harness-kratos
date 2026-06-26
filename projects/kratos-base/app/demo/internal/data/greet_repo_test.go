package data_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"

	"github.com/z-mate/kratos-base/app/demo/internal/data"
	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/pgxpool"
)

// shortTimeoutSelectPool returns a PoolConfig pointing at an unreachable address
// with a short connect timeout so tests don't hang.
func shortTimeoutSelectPool(_ any) (pgxpool.PoolConfig, error) {
	return pgxpool.PoolConfig{
		// 192.0.2.1 is TEST-NET-1 (RFC 5737): guaranteed non-routable, so the
		// OS will not refuse immediately (ECONNREFUSED) but will wait for timeout.
		DSN:            "postgres://u:p@192.0.2.1:5432/db?sslmode=disable",
		ConnectTimeout: time.Second,
	}, nil
}

// TestGreetRepo_DBUnavailable verifies that a greetRepo backed by an unreachable
// DB returns errs.DBUnavailable (HTTP 503) on the first call.
func TestGreetRepo_DBUnavailable(t *testing.T) {
	d := data.New(stubSource{}, shortTimeoutSelectPool, unreachableSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })
	// Use the production public constructor; just verify the first call returns 503.
	repo := data.NewGreetRepo(d)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := repo.Get(ctx, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable, got %T: %v", err, err)
	}
	ke := errs.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d", ke.Code)
	}
}

// TestGreetRepo_BreakerOpens verifies two behaviours:
//  1. The SRE breaker can reach the open state after enough failures (real SRE).
//  2. When the breaker is open, repo.Get fails fast without touching the network.
//
// Part 2 uses errAlwaysBreaker (a deterministic stub) to avoid the probabilistic
// nature of the SRE algorithm making the timing assertion flaky.
func TestGreetRepo_BreakerOpens(t *testing.T) {
	// --- Part 1: verify that the real SRE breaker opens after failures ---
	b := sre.NewBreaker(
		sre.WithRequest(5),
		sre.WithSuccess(0.9),
	)

	// Saturate with failures well above the threshold.
	for i := 0; i < 20; i++ {
		b.MarkFailed()
	}

	// With request=5 and 20 failures and 0 successes, drop probability ≥ 99%.
	// Try up to 20 attempts; at least one must be rejected.
	var allowErr error
	for attempt := 0; attempt < 20; attempt++ {
		if err := b.Allow(); err != nil {
			allowErr = err
			break
		}
	}
	if allowErr == nil {
		t.Fatal("expected breaker to reject Allow() after 20 failures, but it allowed all 20 attempts")
	}
	t.Logf("SRE breaker correctly rejected Allow(): %v", allowErr)

	// --- Part 2: verify fast-fail path using a deterministic always-open stub ---
	// The SRE breaker is probabilistic; use errAlwaysBreaker for deterministic
	// timing assertions (no network call should occur).
	d := data.New(stubSource{}, shortTimeoutSelectPool, unreachableSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })
	repo := data.NewGreetRepoWithBreaker(d, &errAlwaysBreaker{})

	start := time.Now()
	_, err := repo.Get(context.Background(), 1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from open breaker, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable from open breaker, got %T: %v", err, err)
	}
	// A breaker rejection takes microseconds; 50ms is a very generous upper bound
	// to avoid flakiness on a loaded CI machine, while still being much less than
	// any network timeout.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("expected fast-fail from open breaker (< 50ms), got %s", elapsed)
	}
	t.Logf("fast-fail confirmed: Get returned in %s: %v", elapsed, err)
}

// TestGreetRepo_BreakerAllowError confirms that the ErrNotAllowed from the
// breaker package is wrapped into errs.DBUnavailable.
func TestGreetRepo_BreakerAllowError(t *testing.T) {
	// Build a manually-opened breaker stub.
	d := data.New(stubSource{}, shortTimeoutSelectPool, unreachableSelectRedis, disabledSelectMQ)
	t.Cleanup(func() { _ = d.Close() })

	// Use the errAlwaysBreaker to confirm Allow error path.
	repo := data.NewGreetRepoWithBreaker(d, &errAlwaysBreaker{})
	_, err := repo.Get(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("expected DBUnavailable for open-circuit, got %T: %v", err, err)
	}
}

// errAlwaysBreaker is a hand-written stub that always rejects Allow().
type errAlwaysBreaker struct{}

func (e *errAlwaysBreaker) Allow() error { return circuitbreaker.ErrNotAllowed }
func (e *errAlwaysBreaker) MarkSuccess() {}
func (e *errAlwaysBreaker) MarkFailed()  {}

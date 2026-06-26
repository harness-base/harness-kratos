package pgxpool_test

import (
	"context"
	"testing"
	"time"

	"github.com/z-mate/kratos-base/pkg/pgxpool"
)

// unreachableDSN points to a port that should refuse connections quickly.
const unreachableDSN = "postgres://u:p@127.0.0.1:1/db?sslmode=disable"

// TestFingerprint_Stability verifies that the same config always yields the
// same fingerprint.
func TestFingerprint_Stability(t *testing.T) {
	cfg := pgxpool.PoolConfig{
		DSN:             "postgres://user:pass@localhost:5432/testdb",
		MaxOpen:         10,
		MaxIdle:         5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnectTimeout:  5 * time.Second,
	}

	fp1 := cfg.Fingerprint()
	fp2 := cfg.Fingerprint()
	if fp1 != fp2 {
		t.Fatalf("same config produced different fingerprints: %q vs %q", fp1, fp2)
	}
	if fp1 == "" {
		t.Fatal("Fingerprint returned empty string")
	}
}

// TestFingerprint_ChangeDSN verifies that changing the DSN changes the fingerprint.
func TestFingerprint_ChangeDSN(t *testing.T) {
	base := pgxpool.PoolConfig{
		DSN:     "postgres://user:pass@localhost:5432/db1",
		MaxOpen: 5,
	}
	changed := base
	changed.DSN = "postgres://user:pass@localhost:5432/db2"

	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("different DSNs produced the same fingerprint")
	}
}

// TestFingerprint_ChangePoolParam verifies that changing a pool parameter
// changes the fingerprint.
func TestFingerprint_ChangePoolParam(t *testing.T) {
	base := pgxpool.PoolConfig{
		DSN:     "postgres://user:pass@localhost:5432/db",
		MaxOpen: 5,
	}
	changed := base
	changed.MaxOpen = 20

	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("different MaxOpen values produced the same fingerprint")
	}
}

// TestOpen_UnreachableDSN verifies that Open returns promptly (within ~2 s of
// the timeout) when the server is not reachable. No real Postgres needed.
func TestOpen_UnreachableDSN(t *testing.T) {
	cfg := pgxpool.PoolConfig{
		DSN:            unreachableDSN,
		ConnectTimeout: time.Second,
	}

	start := time.Now()
	db, err := pgxpool.Open(context.Background(), cfg)
	elapsed := time.Since(start)

	if err == nil {
		_ = db.Close()
		t.Fatal("expected an error for unreachable DSN, got nil")
	}

	// We gave a 1 s timeout; allow 2× for CI scheduling jitter.
	const maxElapsed = 2 * time.Second
	if elapsed > maxElapsed {
		t.Fatalf("Open took %v, expected < %v (should respect ConnectTimeout)", elapsed, maxElapsed)
	}
	t.Logf("Open returned in %v with error: %v", elapsed, err)
}

// TestOpen_HonorsCanceledContext verifies Open returns promptly when the
// caller's ctx is already canceled, even with a long ConnectTimeout — proving
// the ping derives its deadline from ctx, not just ConnectTimeout. This is the
// behavior the resource.Provider rebuild path relies on so a bad-DSN config
// change cannot block a readyz request past its own deadline.
func TestOpen_HonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	cfg := pgxpool.PoolConfig{
		DSN:            unreachableDSN,
		ConnectTimeout: 30 * time.Second, // long: without ctx-awareness Open would block this long
	}

	start := time.Now()
	db, err := pgxpool.Open(ctx, cfg)
	elapsed := time.Since(start)

	if err == nil {
		_ = db.Close()
		t.Fatal("expected an error for canceled ctx, got nil")
	}
	const maxElapsed = 2 * time.Second
	if elapsed > maxElapsed {
		t.Fatalf("Open took %v with canceled ctx, expected < %v (must honor caller ctx, not ConnectTimeout)", elapsed, maxElapsed)
	}
	t.Logf("Open honored canceled ctx, returned in %v with error: %v", elapsed, err)
}

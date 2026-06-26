package mq

import (
	"context"
	"testing"
	"time"
)

// TestBackoffDelay_ExponentialAndClamp pins the load-bearing reconnect-backoff
// values used by DefaultBackoff for BOTH the rabbitmq and rocketmq consumer
// reconnect loops: the base, the exponential growth, and the maxExp ceiling.
// A wrong base/maxExp would silently change (and could unbound) the reconnect
// interval — this is the regression guard for that. Pure-value assertions, no
// timers. (The 30s cap_ branch is a defensive absolute bound that maxExp keeps
// from binding at the current constants, so it is intentionally not asserted.)
func TestBackoffDelay_ExponentialAndClamp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},     // exp=0 → base
		{2, 200 * time.Millisecond},     // exp=1
		{5, 1600 * time.Millisecond},    // exp=4 → 16*100ms
		{9, 25600 * time.Millisecond},   // exp=8 → maxExp ceiling 2^8*100ms = 25.6s
		{100, 25600 * time.Millisecond}, // exp clamped to maxExp → still 25.6s
	}
	for _, c := range cases {
		if got := backoffDelay(c.attempt); got != c.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

// TestDefaultBackoff_LiveCtxWaitsThenTrue covers DefaultBackoff's success path
// (the time.After branch): a live ctx returns true after waiting backoffDelay
// (≈100ms for attempt 1). The cancelled-ctx false path is covered elsewhere.
func TestDefaultBackoff_LiveCtxWaitsThenTrue(t *testing.T) {
	t.Parallel()
	start := time.Now()
	ok := DefaultBackoff(context.Background(), 1)
	elapsed := time.Since(start)
	if !ok {
		t.Fatal("DefaultBackoff(live ctx, attempt=1) = false, want true")
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("DefaultBackoff returned after %v, want ≥100ms (the attempt-1 backoff)", elapsed)
	}
}

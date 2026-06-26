package registryx

import (
	"testing"
	"time"
)

// TestBackoffDelay_ClampAndOverflow pins the exponential back-off values AND the
// int64-overflow regression (R13F1): without the pre-shift exponent clamp,
// 1<<uint(attempt) overflows time.Duration (int64 ns) around attempt>=34,
// yielding a negative/zero delay that slips past the `> cap` upper guard and
// turns the registration retry loop into a no-backoff CPU/RPC spin. Every result
// must stay in (0, 30s]. Pure values, no timers.
func TestBackoffDelay_ClampAndOverflow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second},    // 2^5=32s capped to 30s
		{10, 30 * time.Second},   // within the old safe window
		{34, 30 * time.Second},   // old overflow boundary (was NEGATIVE pre-fix)
		{60, 30 * time.Second},   // deep overflow window (was 0 pre-fix)
		{1000, 30 * time.Second}, // far overflow
	}
	for _, c := range cases {
		got := backoffDelay(c.attempt)
		if got != c.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", c.attempt, got, c.want)
		}
		if got <= 0 {
			t.Errorf("backoffDelay(%d) = %v: non-positive delay would cause a no-backoff spin (overflow regression)", c.attempt, got)
		}
	}
}

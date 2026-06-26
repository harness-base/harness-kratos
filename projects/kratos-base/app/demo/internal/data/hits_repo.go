package data

import (
	"context"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// HitsRepo is the exported concrete repo type so tests can reference it without
// importing biz through a detour. The public API is through biz.HitsRepo.
type HitsRepo struct {
	data    *Data
	breaker circuitbreaker.CircuitBreaker
}

// NewHitsRepo constructs a HitsRepo with a production SRE breaker and returns
// it as biz.HitsRepo.
func NewHitsRepo(d *Data) biz.HitsRepo {
	return &HitsRepo{data: d, breaker: sre.NewBreaker()}
}

// NewHitsRepoWithBreaker constructs a HitsRepo with the given breaker.
// It is intended for tests that need to control breaker thresholds.
func NewHitsRepoWithBreaker(d *Data, b circuitbreaker.CircuitBreaker) *HitsRepo {
	return &HitsRepo{data: d, breaker: b}
}

// Incr atomically increments the counter at key in Redis and returns the new
// value.
//
//   - If the circuit is open the call fails fast with errs.DBUnavailable.
//   - Redis connection/command errors trip the breaker and return errs.DBUnavailable.
//   - A caller-cancelled ctx (client disconnect) is NOT a Redis fault: it returns
//     errs.DBUnavailable but leaves the breaker untouched (R10F4) so a burst of
//     client disconnects cannot open the circuit against a healthy Redis.
//   - On success the breaker is marked successful.
func (r *HitsRepo) Incr(ctx context.Context, key string) (int64, error) {
	// Fast-fail when the breaker is open.
	if err := r.breaker.Allow(); err != nil {
		return 0, errs.DBUnavailable(err)
	}

	client, err := r.data.Redis(ctx)
	if err != nil {
		r.markBackendFailure(ctx, err)
		return 0, errs.DBUnavailable(err)
	}

	val, err := client.Incr(ctx, key).Result()
	if err != nil {
		r.markBackendFailure(ctx, err)
		return 0, errs.DBUnavailable(err)
	}

	r.breaker.MarkSuccess()
	return val, nil
}

// markBackendFailure trips the breaker for a backend error UNLESS the error came
// from the caller cancelling ctx (client disconnect). A caller cancellation is
// not evidence Redis is sick, so counting it as a failure would let a wave of
// client disconnects open the circuit against a healthy backend (R10F4). It is
// passed the CALLER's ctx so an internally-derived deadline (a slow-Redis
// timeout) is NOT mistaken for a cancellation — that still counts as a failure.
func (r *HitsRepo) markBackendFailure(ctx context.Context, err error) {
	if errs.IsCallerCanceled(ctx, err) {
		return
	}
	r.breaker.MarkFailed()
}

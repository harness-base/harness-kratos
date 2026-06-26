package data

import (
	"context"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/app/demo/internal/data/ent"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// GreetRepo is the exported concrete repo type so tests can reference it
// without importing the internal biz package through a detour.
// The public API is through biz.GreetRepo; this type is exported only for
// testing convenience (NewGreetRepoWithBreaker).
type GreetRepo struct {
	data    *Data
	breaker circuitbreaker.CircuitBreaker
}

// NewGreetRepo constructs a GreetRepo with a production SRE breaker and
// returns it as biz.GreetRepo.
func NewGreetRepo(d *Data) biz.GreetRepo {
	return &GreetRepo{data: d, breaker: sre.NewBreaker()}
}

// NewGreetRepoWithBreaker constructs a GreetRepo with the given breaker.
// It is intended for tests that need to control breaker thresholds.
func NewGreetRepoWithBreaker(d *Data, b circuitbreaker.CircuitBreaker) *GreetRepo {
	return &GreetRepo{data: d, breaker: b}
}

// Get fetches a Greet by id.
//
//   - If the circuit is open the call fails fast with errs.DBUnavailable.
//   - Infrastructure errors (pool, driver) trip the breaker and return
//     errs.DBUnavailable.
//   - A missing row is a business condition: the breaker is NOT tripped and
//     errs.NotFound is returned.
//   - A caller-cancelled ctx (client disconnect) is NOT a DB fault: it returns
//     errs.DBUnavailable but leaves the breaker untouched (R10F4) so a burst of
//     client disconnects cannot open the circuit against a healthy DB.
//   - On success the breaker is marked successful.
func (r *GreetRepo) Get(ctx context.Context, id int64) (*biz.Greet, error) {
	// Fast-fail when the breaker is open.
	if err := r.breaker.Allow(); err != nil {
		return nil, errs.DBUnavailable(err)
	}

	client, err := r.data.Ent(ctx)
	if err != nil {
		r.markBackendFailure(ctx, err)
		return nil, errs.DBUnavailable(err)
	}

	// ent.GreetClient.Get expects int (not int64).
	g, err := client.Greet.Get(ctx, int(id))
	if err != nil {
		if ent.IsNotFound(err) {
			// Absent row is expected business behaviour – not a DB fault.
			r.breaker.MarkSuccess()
			return nil, errs.NotFound("greet %d not found", id)
		}
		// All other errors are infrastructure failures.
		r.markBackendFailure(ctx, err)
		return nil, errs.DBUnavailable(err)
	}

	r.breaker.MarkSuccess()
	return &biz.Greet{
		ID:        int64(g.ID),
		Content:   g.Content,
		CreatedAt: g.CreatedAt,
	}, nil
}

// markBackendFailure trips the breaker for a backend error UNLESS the error came
// from the caller cancelling ctx (client disconnect). A caller cancellation is
// not evidence the DB is sick, so counting it as a failure would let a wave of
// client disconnects open the circuit against a healthy backend (R10F4). It is
// passed the CALLER's ctx so an internally-derived deadline (a slow-DB timeout)
// is NOT mistaken for a cancellation — that still counts as a failure.
func (r *GreetRepo) markBackendFailure(ctx context.Context, err error) {
	if errs.IsCallerCanceled(ctx, err) {
		return
	}
	r.breaker.MarkFailed()
}

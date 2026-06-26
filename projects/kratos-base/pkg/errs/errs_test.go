package errs_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"

	v1 "github.com/z-mate/kratos-base/api/demo/v1"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// TestConstructors_ReasonCodeMapping is the table-driven contract test:
// each helper must yield the proto-declared reason + HTTP code, and the matching
// generated Is* discriminator (and only that one) must report true.
func TestConstructors_ReasonCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason string
		wantCode   int
		// match is the generated discriminator that MUST return true for err.
		match func(error) bool
	}{
		{
			name:       "DBUnavailable",
			err:        errs.DBUnavailable(stderrors.New("dial tcp: connection refused")),
			wantReason: v1.ErrorReason_DB_UNAVAILABLE.String(), // "DB_UNAVAILABLE"
			wantCode:   503,
			match:      errs.IsDBUnavailable,
		},
		{
			name:       "DBUnavailable_nil_cause",
			err:        errs.DBUnavailable(nil),
			wantReason: v1.ErrorReason_DB_UNAVAILABLE.String(),
			wantCode:   503,
			match:      errs.IsDBUnavailable,
		},
		{
			name:       "NotFound",
			err:        errs.NotFound("greet %d not found", 7),
			wantReason: v1.ErrorReason_NOT_FOUND.String(), // "NOT_FOUND"
			wantCode:   404,
			match:      errs.IsNotFound,
		},
		{
			name:       "InvalidArgument",
			err:        errs.InvalidArgument("id must be positive, got %d", -1),
			wantReason: v1.ErrorReason_INVALID_ARGUMENT.String(), // "INVALID_ARGUMENT"
			wantCode:   400,
			match:      errs.IsInvalidArgument,
		},
	}

	// allDiscriminators lets each row assert the OTHER discriminators stay false,
	// so a wrong reason/code can't silently pass.
	allDiscriminators := map[string]func(error) bool{
		"IsDBUnavailable":   errs.IsDBUnavailable,
		"IsNotFound":        errs.IsNotFound,
		"IsInvalidArgument": errs.IsInvalidArgument,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// kratos errors.FromError extracts code/reason across wrapped errors.
			ke := kerrors.FromError(tt.err)
			if ke == nil {
				t.Fatalf("FromError returned nil for %v", tt.err)
			}
			if got := ke.Reason; got != tt.wantReason {
				t.Errorf("reason = %q, want %q", got, tt.wantReason)
			}
			if got := int(ke.Code); got != tt.wantCode {
				t.Errorf("code = %d, want %d", got, tt.wantCode)
			}

			// The matching generated discriminator must be true...
			if !tt.match(tt.err) {
				t.Errorf("expected discriminator to match err (reason=%q code=%d)", ke.Reason, ke.Code)
			}
			// ...and exactly one discriminator matches (so a wrong reason/code
			// can't silently pass by also tripping another discriminator).
			matches := 0
			for name, fn := range allDiscriminators {
				if fn(tt.err) {
					matches++
					if !tt.match(tt.err) {
						t.Errorf("unexpected discriminator %s matched", name)
					}
				}
			}
			if matches != 1 {
				t.Errorf("expected exactly 1 discriminator to match, got %d", matches)
			}
		})
	}
}

// TestDBUnavailable_WrapsCause verifies WithCause keeps the underlying error on
// the chain (errors.Is finds it) without leaking into the public reason/code.
func TestDBUnavailable_WrapsCause(t *testing.T) {
	sentinel := stderrors.New("pgx: pool exhausted")
	err := errs.DBUnavailable(sentinel)

	if !stderrors.Is(err, sentinel) {
		t.Fatalf("errors.Is should find the wrapped cause on the chain")
	}
	if unwrapped := stderrors.Unwrap(err); unwrapped != sentinel {
		t.Errorf("Unwrap = %v, want sentinel cause %v", unwrapped, sentinel)
	}
	// Public contract unchanged by the cause.
	if got := kerrors.Reason(err); got != v1.ErrorReason_DB_UNAVAILABLE.String() {
		t.Errorf("reason = %q, want %q", got, v1.ErrorReason_DB_UNAVAILABLE.String())
	}
	if got := kerrors.Code(err); got != 503 {
		t.Errorf("code = %d, want 503", got)
	}
}

// canceledCtx returns a ctx that is already cancelled via cancel().
func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// deadlineDoneCtx returns a ctx that is already DeadlineExceeded.
func deadlineDoneCtx(t *testing.T) context.Context {
	t.Helper()
	// A deadline in the past makes ctx.Err() == DeadlineExceeded immediately.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	t.Cleanup(cancel)
	// Confirm the deadline has fired so the table row is exercising the real path.
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("setup: expected DeadlineExceeded, got %v", ctx.Err())
	}
	return ctx
}

// TestIsCallerCanceled is the mutation-self-proving contract for the breaker
// guard added in R10F4: a CALLER cancellation (errors.Is Canceled, or the
// caller's ctx is Canceled) must report true so the call site skips MarkFailed;
// every other condition — including a DeadlineExceeded (slow backend = real
// fault) — must report false so MarkFailed still fires.
func TestIsCallerCanceled(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{
			name: "err is context.Canceled",
			ctx:  context.Background(),
			err:  context.Canceled,
			want: true,
		},
		{
			name: "err wraps context.Canceled",
			ctx:  context.Background(),
			err:  fmt.Errorf("rocketmq: producer Start cancelled: %w", context.Canceled),
			want: true,
		},
		{
			name: "caller ctx canceled, err unrelated",
			ctx:  canceledCtx(),
			err:  stderrors.New("dial tcp: connection refused"),
			want: true,
		},
		{
			name: "caller ctx canceled, err nil",
			ctx:  canceledCtx(),
			err:  nil,
			want: true,
		},
		{
			// A blown deadline is a slow-backend fault, NOT a caller cancellation.
			name: "err is context.DeadlineExceeded",
			ctx:  context.Background(),
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			// Caller ctx hit its deadline (our own timeout firing on a slow
			// backend): still a fault, must NOT be treated as cancellation.
			name: "caller ctx DeadlineExceeded, err DeadlineExceeded",
			ctx:  deadlineDoneCtx(t),
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "ordinary backend error, live ctx",
			ctx:  context.Background(),
			err:  stderrors.New("redis: ECONNRESET"),
			want: false,
		},
		{
			name: "nil ctx and nil err",
			ctx:  nil,
			err:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errs.IsCallerCanceled(tt.ctx, tt.err); got != tt.want {
				t.Errorf("IsCallerCanceled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDiscriminators_NilAndForeign guards the boundary behavior of the reused
// generated Is* helpers.
func TestDiscriminators_NilAndForeign(t *testing.T) {
	foreign := stderrors.New("some unrelated error")
	tests := []struct {
		name string
		fn   func(error) bool
	}{
		{"IsDBUnavailable", errs.IsDBUnavailable},
		{"IsNotFound", errs.IsNotFound},
		{"IsInvalidArgument", errs.IsInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fn(nil) {
				t.Errorf("%s(nil) = true, want false", tt.name)
			}
			if tt.fn(foreign) {
				t.Errorf("%s(foreign) = true, want false", tt.name)
			}
		})
	}
}

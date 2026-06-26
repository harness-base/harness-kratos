package biz

import (
	"context"

	"github.com/z-mate/kratos-base/pkg/errs"
)

// DefaultHitsKey is the Redis key used by the demo hit counter when the caller
// does not supply one. Exported so callers/tests can assert the default-fallback
// contract without re-typing the literal.
const DefaultHitsKey = "demo:hits"

// maxHitsKeyLen bounds a caller-supplied counter key. A key is a Redis key, so
// an unbounded/garbage value would pollute the keyspace; 128 bytes is generous
// for any legitimate counter name while keeping rogue keys small.
const maxHitsKeyLen = 128

// HitsRepo is the repository interface for hit-counter persistence.
// Implementations live in the data layer (backed by Redis INCR).
type HitsRepo interface {
	Incr(ctx context.Context, key string) (int64, error)
}

// HitsUsecase orchestrates business logic for the hit counter.
type HitsUsecase struct {
	repo HitsRepo
}

// NewHitsUsecase constructs a HitsUsecase backed by the given repo.
func NewHitsUsecase(repo HitsRepo) *HitsUsecase {
	return &HitsUsecase{repo: repo}
}

// Hit increments the hit counter at key and returns the new value.
//
// An empty key falls back to DefaultHitsKey ("demo:hits"). A non-empty key is
// validated (see validateHitsKey) and used verbatim, so ?key=foo increments the
// "foo" counter — honoring the proto contract. Returns errs.InvalidArgument
// (HTTP 400) for a malformed key and errs.DBUnavailable (HTTP 503) when Redis is
// unavailable.
func (uc *HitsUsecase) Hit(ctx context.Context, key string) (int64, error) {
	if key == "" {
		key = DefaultHitsKey
	} else if err := validateHitsKey(key); err != nil {
		return 0, err
	}
	return uc.repo.Incr(ctx, key)
}

// validateHitsKey enforces a minimal sanity check on a caller-supplied counter
// key before it becomes a Redis key. It rejects over-long keys and any key
// containing control characters or ASCII whitespace (space, tab, CR, LF, etc.),
// which have no place in a legitimate counter name and would otherwise let a
// caller smuggle structure into the Redis keyspace. Allowed keys pass through
// verbatim — this is a guardrail, not a transformer.
func validateHitsKey(key string) error {
	if len(key) > maxHitsKeyLen {
		return errs.InvalidArgument("key too long: %d > %d", len(key), maxHitsKeyLen)
	}
	for _, r := range key {
		if r <= ' ' || r == 0x7f {
			return errs.InvalidArgument("key contains an illegal control or whitespace character")
		}
	}
	return nil
}

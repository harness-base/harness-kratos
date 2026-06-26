package biz_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// fakeHitsRepo is a simple stub HitsRepo for testing HitsUsecase. It records the
// last key it was asked to increment and keeps a per-key count, so tests can
// assert exactly which Redis key the usecase routed the increment to.
type fakeHitsRepo struct {
	err     error
	lastKey string
	counts  map[string]int64
}

func (f *fakeHitsRepo) Incr(_ context.Context, key string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.lastKey = key
	if f.counts == nil {
		f.counts = map[string]int64{}
	}
	f.counts[key]++
	return f.counts[key], nil
}

// TestHitsUsecase_Hit_Increment verifies that consecutive Hit calls increment
// the counter and return the updated value.
func TestHitsUsecase_Hit_Increment(t *testing.T) {
	repo := &fakeHitsRepo{}
	uc := biz.NewHitsUsecase(repo)

	for i := int64(1); i <= 5; i++ {
		got, err := uc.Hit(context.Background(), "")
		if err != nil {
			t.Fatalf("Hit() unexpected error on call %d: %v", i, err)
		}
		if got != i {
			t.Fatalf("Hit() call %d: want %d, got %d", i, i, got)
		}
	}
}

// TestHitsUsecase_Hit_EmptyKeyUsesDefault proves the empty-key fallback: Hit("")
// must route the increment to biz.DefaultHitsKey ("demo:hits"), not to "".
// Mutation guard: if the `key = DefaultHitsKey` fallback were dropped, lastKey
// would be "" and this fails.
func TestHitsUsecase_Hit_EmptyKeyUsesDefault(t *testing.T) {
	repo := &fakeHitsRepo{}
	uc := biz.NewHitsUsecase(repo)

	if _, err := uc.Hit(context.Background(), ""); err != nil {
		t.Fatalf("Hit(empty) unexpected error: %v", err)
	}
	if repo.lastKey != biz.DefaultHitsKey {
		t.Fatalf("Hit(empty) incremented key %q, want default %q", repo.lastKey, biz.DefaultHitsKey)
	}
}

// TestHitsUsecase_Hit_CustomKeyPassesThrough proves the contract the proto
// promises: a non-empty key is used verbatim. The two keys are counted
// independently, so the per-key counts pin down that the usecase did NOT collapse
// everything onto the default key. Mutation guard: if Hit ignored its key and
// always used DefaultHitsKey, foo/bar would never be incremented and this fails.
func TestHitsUsecase_Hit_CustomKeyPassesThrough(t *testing.T) {
	repo := &fakeHitsRepo{}
	uc := biz.NewHitsUsecase(repo)

	got, err := uc.Hit(context.Background(), "foo")
	if err != nil {
		t.Fatalf("Hit(foo) unexpected error: %v", err)
	}
	if repo.lastKey != "foo" {
		t.Fatalf("Hit(foo) incremented key %q, want %q", repo.lastKey, "foo")
	}
	if got != 1 {
		t.Fatalf("Hit(foo) = %d, want 1", got)
	}

	// A second distinct key must have its own count, and the default key must
	// stay untouched — proving keys are not aliased onto a single counter.
	if _, err := uc.Hit(context.Background(), "bar"); err != nil {
		t.Fatalf("Hit(bar) unexpected error: %v", err)
	}
	if repo.counts["foo"] != 1 || repo.counts["bar"] != 1 {
		t.Fatalf("per-key counts = foo:%d bar:%d, want 1/1", repo.counts["foo"], repo.counts["bar"])
	}
	if _, seen := repo.counts[biz.DefaultHitsKey]; seen {
		t.Fatalf("default key %q was incremented but no empty-key call was made", biz.DefaultHitsKey)
	}
}

// TestHitsUsecase_Hit_RejectsInvalidKey proves the minimal keyspace guard: a key
// with a control/whitespace character or one over the length bound is rejected
// with INVALID_ARGUMENT (HTTP 400) BEFORE touching the repo. Mutation guard: if
// validateHitsKey were removed, these would reach the repo and return no error.
func TestHitsUsecase_Hit_RejectsInvalidKey(t *testing.T) {
	cases := map[string]string{
		"space":   "foo bar",
		"newline": "foo\nbar",
		"tab":     "foo\tbar",
		"toolong": strings.Repeat("x", 129),
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeHitsRepo{}
			uc := biz.NewHitsUsecase(repo)

			_, err := uc.Hit(context.Background(), key)
			if err == nil {
				t.Fatalf("Hit(%q): expected error, got nil", key)
			}
			if !errs.IsInvalidArgument(err) {
				t.Fatalf("Hit(%q): expected InvalidArgument, got %T: %v", key, err, err)
			}
			if ke := errs.FromError(err); ke.Code != 400 {
				t.Fatalf("Hit(%q): expected HTTP 400, got %d", key, ke.Code)
			}
			// The guard must short-circuit before the repo runs.
			if repo.lastKey != "" {
				t.Fatalf("Hit(%q): repo was called with %q despite invalid key", key, repo.lastKey)
			}
		})
	}
}

// TestHitsUsecase_Hit_AllowsMaxLenKey is the boundary complement of the toolong
// case: a key of exactly maxHitsKeyLen (128) bytes is accepted, proving the
// length check uses `>` not `>=`.
func TestHitsUsecase_Hit_AllowsMaxLenKey(t *testing.T) {
	repo := &fakeHitsRepo{}
	uc := biz.NewHitsUsecase(repo)

	key := strings.Repeat("x", 128)
	if _, err := uc.Hit(context.Background(), key); err != nil {
		t.Fatalf("Hit(128-byte key) unexpected error: %v", err)
	}
	if repo.lastKey != key {
		t.Fatalf("Hit(128-byte key) incremented %q, want the supplied key", repo.lastKey)
	}
}

// TestHitsUsecase_Hit_PropagatesError verifies that errors from the repo are
// propagated unchanged.
func TestHitsUsecase_Hit_PropagatesError(t *testing.T) {
	sentinel := errors.New("redis: connection refused")
	repo := &fakeHitsRepo{err: sentinel}
	uc := biz.NewHitsUsecase(repo)

	_, err := uc.Hit(context.Background(), "")
	if err == nil {
		t.Fatal("Hit() expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("Hit() expected sentinel error, got %v", err)
	}
}

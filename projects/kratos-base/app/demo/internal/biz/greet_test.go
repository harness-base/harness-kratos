package biz_test

import (
	"context"
	"errors"
	"testing"
	"time"

	kErrors "github.com/go-kratos/kratos/v2/errors"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// fakeGreetRepo is a hand-written stub for biz.GreetRepo.
type fakeGreetRepo struct {
	greet *biz.Greet
	err   error
}

func (f *fakeGreetRepo) Get(_ context.Context, _ int64) (*biz.Greet, error) {
	return f.greet, f.err
}

func TestGreetUsecase_Ping(t *testing.T) {
	uc := biz.NewGreetUsecase(&fakeGreetRepo{})
	if got := uc.Ping(); got != "pong" {
		t.Fatalf("Ping() = %q, want %q", got, "pong")
	}
}

func TestGreetUsecase_Get_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   int64
	}{
		{"zero", 0},
		{"negative", -1},
		{"very negative", -100},
	}
	uc := biz.NewGreetUsecase(&fakeGreetRepo{})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Get(context.Background(), tc.id)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			ke := kErrors.FromError(err)
			if ke.Code != 400 {
				t.Fatalf("expected HTTP 400, got %d", ke.Code)
			}
			if !errs.IsInvalidArgument(err) {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestGreetUsecase_Get_Passthrough(t *testing.T) {
	want := &biz.Greet{ID: 1, Content: "hello", CreatedAt: time.Now()}
	uc := biz.NewGreetUsecase(&fakeGreetRepo{greet: want})

	got, err := uc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || got.Content != want.Content {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}
}

func TestGreetUsecase_Get_RepoError(t *testing.T) {
	repoErr := errs.NotFound("greet 42 not found")
	uc := biz.NewGreetUsecase(&fakeGreetRepo{err: repoErr})

	_, err := uc.Get(context.Background(), 42)
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repo error to be passed through, got %v", err)
	}
}

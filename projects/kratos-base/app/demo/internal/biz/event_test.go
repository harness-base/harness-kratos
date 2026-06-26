package biz_test

import (
	"context"
	"errors"
	"testing"

	kErrors "github.com/go-kratos/kratos/v2/errors"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// fakeEventRepo is a hand-written stub for biz.EventRepo.
type fakeEventRepo struct {
	id  string
	err error
}

func (f *fakeEventRepo) Emit(_ context.Context, _ string) (string, error) {
	return f.id, f.err
}

// TestEventUsecase_EmptyPayload verifies that an empty payload returns
// errs.InvalidArgument (HTTP 400).
func TestEventUsecase_EmptyPayload(t *testing.T) {
	uc := biz.NewEventUsecase(&fakeEventRepo{id: "some-id"})

	_, err := uc.Publish(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 400 {
		t.Fatalf("expected HTTP 400, got %d", ke.Code)
	}
	if !errs.IsInvalidArgument(err) {
		t.Fatalf("expected IsInvalidArgument, got %v", err)
	}
}

// TestEventUsecase_Success verifies that a valid payload is forwarded to the
// repo and the returned id is passed back unchanged.
func TestEventUsecase_Success(t *testing.T) {
	uc := biz.NewEventUsecase(&fakeEventRepo{id: "event-123"})

	id, err := uc.Publish(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "event-123" {
		t.Fatalf("id = %q, want event-123", id)
	}
}

// TestEventUsecase_RepoError verifies that repo errors are passed through.
func TestEventUsecase_RepoError(t *testing.T) {
	repoErr := errs.DBUnavailable(errors.New("broker down"))
	uc := biz.NewEventUsecase(&fakeEventRepo{err: repoErr})

	_, err := uc.Publish(context.Background(), "payload")
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repoErr, got %v", err)
	}
}

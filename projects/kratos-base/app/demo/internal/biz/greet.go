// Package biz contains the business-logic layer for the demo service.
// It is intentionally free of any infrastructure dependencies.
package biz

import (
	"context"
	"time"

	"github.com/z-mate/kratos-base/pkg/errs"
)

// Greet is the business-layer representation of a greeting entity.
type Greet struct {
	ID        int64
	Content   string
	CreatedAt time.Time
}

// GreetRepo is the repository interface for Greet persistence.
// Implementations live in the data layer.
type GreetRepo interface {
	Get(ctx context.Context, id int64) (*Greet, error)
}

// GreetUsecase orchestrates business logic for the Greet entity.
type GreetUsecase struct {
	repo GreetRepo
}

// NewGreetUsecase constructs a GreetUsecase backed by the given repo.
func NewGreetUsecase(repo GreetRepo) *GreetUsecase {
	return &GreetUsecase{repo: repo}
}

// Ping returns the fixed health-check string "pong".
func (uc *GreetUsecase) Ping() string {
	return "pong"
}

// Get retrieves a Greet by ID.
// Returns errs.InvalidArgument when id <= 0; otherwise delegates to the repo.
func (uc *GreetUsecase) Get(ctx context.Context, id int64) (*Greet, error) {
	if id <= 0 {
		return nil, errs.InvalidArgument("id must be positive, got %d", id)
	}
	return uc.repo.Get(ctx, id)
}

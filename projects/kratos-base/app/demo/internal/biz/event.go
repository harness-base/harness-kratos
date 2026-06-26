// Package biz – event business logic.
package biz

import (
	"context"

	"github.com/z-mate/kratos-base/pkg/errs"
)

// EventRepo is the repository interface for event publishing.
// Implementations live in the data layer (backed by MQ).
type EventRepo interface {
	// Emit publishes payload to the message broker and returns a generated id.
	// Returns errs.DBUnavailable when the broker is unreachable or MQ is disabled.
	Emit(ctx context.Context, payload string) (string, error)
}

// EventUsecase orchestrates business logic for event publishing.
type EventUsecase struct {
	repo EventRepo
}

// NewEventUsecase constructs an EventUsecase backed by the given repo.
func NewEventUsecase(repo EventRepo) *EventUsecase {
	return &EventUsecase{repo: repo}
}

// Publish validates payload and delegates to the repo.
// Returns errs.InvalidArgument (HTTP 400) when payload is empty.
func (uc *EventUsecase) Publish(ctx context.Context, payload string) (string, error) {
	if payload == "" {
		return "", errs.InvalidArgument("payload must not be empty")
	}
	return uc.repo.Emit(ctx, payload)
}

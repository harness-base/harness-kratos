// Package data – EventRepo implementation for the demo service.
package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/mq"
)

// EventRepo implements biz.EventRepo backed by the MQ publisher.
type EventRepo struct {
	data *Data
}

// NewEventRepo constructs an EventRepo backed by d.
func NewEventRepo(d *Data) biz.EventRepo {
	return &EventRepo{data: d}
}

// Emit publishes payload to the configured MQ topic and returns a generated id.
//
// When MQ is not enabled (kind == "") this returns errs.DBUnavailable to keep
// the "dependency unavailable" semantics consistent with PG/Redis outage paths.
// Callers (service layer) see HTTP 503.
//
// The publisher's internal circuit-breaker handles consecutive failures; errors
// from Publish are already wrapped as errs.DBUnavailable — no second wrapping
// layer is added here.
func (r *EventRepo) Emit(ctx context.Context, payload string) (string, error) {
	pub := r.data.MQPublisher()
	if pub == nil {
		// MQ not enabled — treat as "dependency unavailable" (HTTP 503).
		// We use DBUnavailable rather than InvalidArgument because the payload
		// is valid; the issue is a missing infrastructure dependency.
		return "", errs.DBUnavailable(fmt.Errorf("mq not enabled"))
	}

	id := newEventID()

	msg := mq.Message{
		Topic: r.data.MQTopic(),
		Key:   id,
		Body:  []byte(payload),
	}
	if err := pub.Publish(ctx, msg); err != nil {
		// Publisher already wraps broker errors as errs.DBUnavailable; pass through.
		return "", err
	}
	return id, nil
}

// newEventID generates a cryptographically random 16-byte hex event identifier.
// We use crypto/rand (always available, no new dependency) rather than uuid
// to avoid an import purely for this purpose; the module already has uuid in
// go.mod so either choice is valid, but this keeps the data layer self-contained.
func newEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is exceedingly rare on supported platforms;
		// fall back to a fixed sentinel rather than panicking.
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

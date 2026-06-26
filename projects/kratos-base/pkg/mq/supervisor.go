package mq

import (
	"context"
	"math"
	"time"
)

// Delivery represents a single message delivery from a broker.
// Ack and Nack are broker-specific acknowledgement callbacks.
type Delivery struct {
	Msg  Message
	Ack  func() error
	Nack func() error
}

// ConnectFn establishes one subscribe session.
// On success it returns a channel of deliveries and a cleanup function.
// The channel being closed signals a disconnection.
// On failure it returns a non-nil error.
type ConnectFn func(ctx context.Context, topic string) (<-chan Delivery, func(), error)

// Backoff is called between reconnect attempts. attempt starts at 1.
// Return true to continue; return false (e.g. because ctx is done) to exit.
type Backoff func(ctx context.Context, attempt int) bool

// RunSupervised drives the supervised consumer loop:
//
//  1. Call connect; on failure call backoff(attempt) and retry.
//  2. On success reset attempt to 0 and drain the delivery channel,
//     calling h for each message (Ack on nil, Nack on error).
//  3. When the delivery channel is closed (disconnect), call backoff(attempt)
//     and reconnect.
//  4. When ctx is cancelled, return ctx.Err() cleanly.
func RunSupervised(
	ctx context.Context,
	topic string,
	h Handler,
	connect ConnectFn,
	backoff Backoff,
) error {
	attempt := 0

	for {
		// Respect context before each connect attempt.
		if err := ctx.Err(); err != nil {
			return err
		}

		deliveries, cleanup, err := connect(ctx, topic)
		if err != nil {
			attempt++
			if !backoff(ctx, attempt) {
				return ctx.Err()
			}
			continue
		}

		// Successful connection: reset attempt counter.
		attempt = 0

		// Drain deliveries until disconnect or ctx cancel.
		disconnected := drainDeliveries(ctx, deliveries, h)

		// The session is over either way — release its resources NOW.
		// Never defer this in the loop: a long-running consumer reconnects
		// indefinitely and deferred cleanups would pile up (leaking one
		// broker channel per reconnect) until the function returns.
		if cleanup != nil {
			cleanup()
		}

		if !disconnected {
			// ctx was cancelled.
			return ctx.Err()
		}

		// Disconnected: back off before reconnecting.
		attempt++
		if !backoff(ctx, attempt) {
			return ctx.Err()
		}
	}
}

// drainDeliveries processes messages from the channel until it is closed
// (returns true) or ctx is cancelled (returns false).
func drainDeliveries(ctx context.Context, ch <-chan Delivery, h Handler) (disconnected bool) {
	for {
		select {
		case <-ctx.Done():
			return false
		case d, ok := <-ch:
			if !ok {
				return true
			}
			// Restore any W3C trace context the producer injected into the message
			// headers so the handler joins the producer's trace across the async
			// hop (R9F6). No trace headers → ctx is returned unchanged.
			hctx := ExtractTrace(ctx, d.Msg.Headers)
			if err := h(hctx, d.Msg); err != nil {
				_ = d.Nack()
			} else {
				_ = d.Ack()
			}
		}
	}
}

// backoffDelay computes the exponential reconnect wait for a 1-based attempt:
// base * 2^(attempt-1), with the exponent bounded by maxExp and an absolute 30s
// cap_. With maxExp=8 the effective ceiling is 2^8*100ms ≈ 25.6s (so cap_ is a
// defensive absolute bound that only binds if base/maxExp are raised). Split out
// as a pure function so the exponential/clamp values are unit-testable without
// waiting out real timers. Backs DefaultBackoff for both the rabbitmq and
// rocketmq consumer reconnect loops.
func backoffDelay(attempt int) time.Duration {
	const (
		base   = 100 * time.Millisecond
		cap_   = 30 * time.Second
		maxExp = 8 // 2^8 * 100ms ≈ 25.6 s effective ceiling; cap_ below is a defensive absolute bound
	)
	exp := attempt - 1
	if exp > maxExp {
		exp = maxExp
	}
	wait := time.Duration(math.Pow(2, float64(exp))) * base
	if wait > cap_ {
		wait = cap_
	}
	return wait
}

// DefaultBackoff waits backoffDelay(attempt) (exponential, ≈25.6s effective
// ceiling within a 30s absolute cap) and returns true, or returns false
// immediately if ctx is cancelled.
func DefaultBackoff(ctx context.Context, attempt int) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(backoffDelay(attempt)):
		return true
	}
}

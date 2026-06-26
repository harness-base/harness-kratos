// Package mq defines the adapter-agnostic message-queue interfaces and shared
// types used by all broker backends (RabbitMQ, RocketMQ, …).
package mq

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

// traceProp is the W3C trace-context propagator used to carry trace state across
// the async MQ hop. It is a self-contained propagation.TraceContext{} and
// deliberately does NOT consult otel's global propagator, so trace propagation
// works even in a process that never called otel.SetTextMapPropagator. Both
// broker adapters share it so inject/extract use identical wire keys.
var traceProp = propagation.TraceContext{}

// InjectTrace returns a copy of headers with the current span's W3C traceparent
// (and tracestate, if any) added, so an async publish carries the trace to the
// consumer. The caller's map is never mutated. When ctx carries no span context
// and headers is nil, it returns nil (range over nil is a no-op for publishers).
func InjectTrace(ctx context.Context, headers map[string]string) map[string]string {
	carrier := propagation.MapCarrier{}
	traceProp.Inject(ctx, carrier)
	if len(carrier) == 0 && headers == nil {
		return nil
	}
	out := make(map[string]string, len(headers)+len(carrier))
	for k, v := range headers {
		out[k] = v
	}
	for k, v := range carrier {
		out[k] = v
	}
	return out
}

// ExtractTrace returns ctx with any W3C trace context found in headers restored,
// so a handler invoked for a received message joins the producer's trace. With
// no trace headers it returns ctx unchanged.
func ExtractTrace(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return traceProp.Extract(ctx, propagation.MapCarrier(headers))
}

// Message is a single unit of data exchanged over a message broker.
type Message struct {
	// Topic is the destination exchange, topic, or queue name.
	Topic string
	// Key is an optional routing key or partition key.
	Key string
	// Body is the raw message payload.
	Body []byte
	// Headers carries optional metadata (e.g. trace context, content-type).
	Headers map[string]string
}

// Handler processes a single delivered message.
// Return nil to acknowledge the message; return an error to nack/reject.
type Handler func(ctx context.Context, m Message) error

// Publisher sends messages to a broker.
type Publisher interface {
	// Publish sends m to the broker. Returns errs.DBUnavailable when the
	// broker is unreachable or the circuit-breaker is open.
	Publish(ctx context.Context, m Message) error
	// Close releases any broker connections held by this publisher.
	Close() error
}

// Consumer receives messages from a broker.
type Consumer interface {
	// Subscribe starts the supervised consumer loop for topic, calling h for
	// each delivery. It blocks until ctx is cancelled. The implementation is
	// responsible for reconnecting on transient broker failures.
	Subscribe(ctx context.Context, topic string, h Handler) error
	// Close releases any broker connections held by this consumer.
	Close() error
}

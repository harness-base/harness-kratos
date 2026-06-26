// Package data – background consumer runner for the demo service.
// ConsumerRunner starts a supervised MQ consumer in the background and
// integrates with the kratos.App lifecycle via BeforeStart / AfterStop hooks.
package data

import (
	"context"
	"sync/atomic"

	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/z-mate/kratos-base/pkg/mq"
)

// ConsumerRunner manages the lifecycle of the background MQ consumer.
// It is a no-op when MQ is disabled (kind == "").
type ConsumerRunner struct {
	data    *Data
	logger  kratoslog.Logger
	cancel  context.CancelFunc
	done    chan struct{}
	counter atomic.Int64
}

// NewConsumerRunner constructs a ConsumerRunner.
// When d.MQEnabled() is false, Start and Stop are no-ops.
func NewConsumerRunner(d *Data, logger kratoslog.Logger) *ConsumerRunner {
	return &ConsumerRunner{
		data:   d,
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Start launches the consumer goroutine.
// Should be called from a kratos.BeforeStart hook (or equivalent lifecycle).
// The hook signature is func(context.Context) error; Start always returns nil.
// If MQ is disabled, Start returns nil immediately.
func (r *ConsumerRunner) Start(ctx context.Context) error {
	if !r.data.MQEnabled() {
		close(r.done) // mark done immediately so Stop doesn't block
		return nil
	}

	con := r.data.MQConsumer()
	topic := r.data.MQTopic()
	logger := r.logger
	counter := &r.counter

	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	handler := func(ctx context.Context, m mq.Message) error {
		n := counter.Add(1)
		_ = kratoslog.WithContext(ctx, logger).Log(kratoslog.LevelInfo,
			"consumer", "received",
			"topic", m.Topic,
			"key", m.Key,
			"body_len", len(m.Body),
			"total_received", n,
		)
		// Return nil to Ack; return non-nil to Nack (broker will redeliver).
		return nil
	}

	go func() {
		defer close(r.done)
		err := con.Subscribe(runCtx, topic, handler)
		if err != nil && runCtx.Err() == nil {
			// Unexpected exit — not caused by context cancellation.
			_ = logger.Log(kratoslog.LevelError,
				"consumer", "Subscribe exited unexpectedly",
				"topic", topic,
				"err", err.Error(),
			)
		}
	}()
	return nil
}

// Stop cancels the consumer context and waits for the goroutine to exit.
// Should be called from a kratos.AfterStop hook (or equivalent lifecycle).
func (r *ConsumerRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	<-r.done
}

// Received returns the total number of messages successfully processed.
// Useful for observability and tests.
func (r *ConsumerRunner) Received() int64 {
	return r.counter.Load()
}

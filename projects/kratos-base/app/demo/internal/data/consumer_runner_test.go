package data_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/z-mate/kratos-base/app/demo/internal/data"
	"github.com/z-mate/kratos-base/pkg/mq"
)

// ─────────────────────────────────────────────────────────────────────────────
// stubs
// ─────────────────────────────────────────────────────────────────────────────

// syncConsumer records Subscribe calls and blocks until ctx is cancelled.
// The subscribed channel is written when Subscribe is first called.
type syncConsumer struct {
	subscribed chan struct{} // closed when Subscribe is called
}

func newSyncConsumer() *syncConsumer {
	return &syncConsumer{subscribed: make(chan struct{})}
}

func (s *syncConsumer) Subscribe(ctx context.Context, _ string, _ mq.Handler) error {
	close(s.subscribed) // signal that Subscribe was called
	<-ctx.Done()
	return ctx.Err()
}

func (s *syncConsumer) Close() error { return nil }

// nopLogger satisfies kratoslog.Logger by discarding all messages.
type nopLogger struct{}

func (nopLogger) Log(_ kratoslog.Level, _ ...interface{}) error { return nil }

// failConsumer's Subscribe returns a fixed non-context error immediately, while
// ctx is still live — exercising the "unexpected exit" alarm branch in
// ConsumerRunner.Start (runtime err != nil && runCtx.Err() == nil).
type failConsumer struct {
	err error
}

func (c *failConsumer) Subscribe(_ context.Context, _ string, _ mq.Handler) error {
	return c.err
}
func (c *failConsumer) Close() error { return nil }

// logEntry captures one Log call's level and key/value pairs.
type logEntry struct {
	level   kratoslog.Level
	keyvals []interface{}
}

// captureLogger records every Log call so tests can assert on level and fields.
// errCh (when non-nil) is closed the first time an ERROR-level entry is logged,
// letting tests synchronize without sleeps or racing Stop's cancel.
type captureLogger struct {
	mu       sync.Mutex
	entries  []logEntry
	errCh    chan struct{}
	errFired bool
}

func (l *captureLogger) Log(level kratoslog.Level, keyvals ...interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, logEntry{level: level, keyvals: append([]interface{}(nil), keyvals...)})
	if level == kratoslog.LevelError && l.errCh != nil && !l.errFired {
		l.errFired = true
		close(l.errCh)
	}
	return nil
}

// find returns the first entry at the given level, or false.
func (l *captureLogger) find(level kratoslog.Level) (logEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.level == level {
			return e, true
		}
	}
	return logEntry{}, false
}

// field extracts the value following the given key in a flat keyvals slice.
func (e logEntry) field(key string) (interface{}, bool) {
	for i := 0; i+1 < len(e.keyvals); i += 2 {
		if k, ok := e.keyvals[i].(string); ok && k == key {
			return e.keyvals[i+1], true
		}
	}
	return nil, false
}

// ─────────────────────────────────────────────────────────────────────────────
// tests
// ─────────────────────────────────────────────────────────────────────────────

// TestConsumerRunner_Disabled verifies that Start and Stop are no-ops when
// MQ is not enabled.
func TestConsumerRunner_Disabled(t *testing.T) {
	// NewForTest with nil publisher = MQ disabled.
	d := data.NewForTest(nil, nil, "demo.events")
	cr := data.NewConsumerRunner(d, nopLogger{})

	ctx := context.Background()
	if err := cr.Start(ctx); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}
	// Stop must return immediately (no goroutine was started).
	cr.Stop()
}

// TestConsumerRunner_Start_SubscribeIsCalled verifies that Start launches a
// goroutine that calls consumer.Subscribe with the configured topic.
func TestConsumerRunner_Start_SubscribeIsCalled(t *testing.T) {
	con := newSyncConsumer()
	pub := &fakePublisher{}
	d := data.NewForTest(pub, con, "demo.topic")
	cr := data.NewConsumerRunner(d, nopLogger{})

	ctx := context.Background()
	if err := cr.Start(ctx); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	// Wait for Subscribe to be called (no time.Sleep — channel sync).
	<-con.subscribed

	// Stop triggers ctx cancellation and waits for goroutine to exit.
	cr.Stop()
}

// TestConsumerRunner_Stop_CleanExit verifies that Stop returns after the
// consumer goroutine exits cleanly.
func TestConsumerRunner_Stop_CleanExit(t *testing.T) {
	con := newSyncConsumer()
	pub := &fakePublisher{}
	d := data.NewForTest(pub, con, "events")
	cr := data.NewConsumerRunner(d, nopLogger{})

	ctx := context.Background()
	if err := cr.Start(ctx); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	// Wait until Subscribe is definitely running.
	<-con.subscribed

	// Stop must unblock (not hang) because it cancels the consumer ctx.
	done := make(chan struct{})
	go func() {
		cr.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-ctx.Done():
		t.Fatal("Stop did not return: context cancelled externally")
	}
}

// TestConsumerRunner_ContextCancel_GracefulExit verifies that cancelling the
// parent context causes the consumer goroutine to exit gracefully.
func TestConsumerRunner_ContextCancel_GracefulExit(t *testing.T) {
	con := newSyncConsumer()
	pub := &fakePublisher{}
	d := data.NewForTest(pub, con, "events")
	cr := data.NewConsumerRunner(d, nopLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	if err := cr.Start(ctx); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	<-con.subscribed

	// Cancel the parent context — the consumer goroutine should exit.
	cancel()

	// Stop still needs to be called to drain the done channel.
	cr.Stop()
}

// TestConsumerRunner_UnexpectedExit_LogsError verifies the alarm branch
// (consumer_runner.go L69-76): when Subscribe returns a non-context error while
// the run context is still live, the runner logs an ERROR-level entry carrying
// the topic and err fields. This guards against silent consumer death.
func TestConsumerRunner_UnexpectedExit_LogsError(t *testing.T) {
	subscribeErr := errors.New("broker connection reset")
	con := &failConsumer{err: subscribeErr}
	pub := &fakePublisher{}
	const topic = "demo.unexpected"
	d := data.NewForTest(pub, con, topic)

	log := &captureLogger{errCh: make(chan struct{})}
	cr := data.NewConsumerRunner(d, log)

	// Parent ctx stays live throughout — so runCtx.Err() is nil when Subscribe
	// returns, which is exactly what makes the exit "unexpected". We must NOT
	// call Stop() before the log is emitted: Stop cancels the run context, which
	// would flip runCtx.Err() and suppress the very branch under test. So wait
	// on the logger's error channel first, then Stop to drain the goroutine.
	if err := cr.Start(context.Background()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	select {
	case <-log.errCh:
		// ERROR log observed.
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unexpected-exit ERROR log")
	}
	cr.Stop()

	entry, ok := log.find(kratoslog.LevelError)
	if !ok {
		t.Fatal("expected an ERROR-level log for unexpected consumer exit, got none")
	}

	gotTopic, ok := entry.field("topic")
	if !ok {
		t.Fatal("error log missing 'topic' field")
	}
	if gotTopic != topic {
		t.Errorf("error log topic = %v, want %q", gotTopic, topic)
	}

	gotErr, ok := entry.field("err")
	if !ok {
		t.Fatal("error log missing 'err' field")
	}
	if gotErr != subscribeErr.Error() {
		t.Errorf("error log err = %v, want %q", gotErr, subscribeErr.Error())
	}
}

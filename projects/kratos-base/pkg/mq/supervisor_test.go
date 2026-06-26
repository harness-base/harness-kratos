package mq_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/z-mate/kratos-base/pkg/mq"
)

// syncBackoff is a Backoff that unblocks callers and records invocations.
// It never blocks on ctx — purely synchronous so tests don't need time.Sleep.
func syncBackoff(record *[]int, mu *sync.Mutex) mq.Backoff {
	return func(ctx context.Context, attempt int) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		mu.Lock()
		*record = append(*record, attempt)
		mu.Unlock()
		return true
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test ①: happy-path with reconnect
//
// First session: delivers 2 messages then closes the channel (simulates
// disconnect). Each message is Ack-ed. After close, RunSupervised reconnects
// (ConnectFn called a second time). Second session: ctx is cancelled → return.
// ─────────────────────────────────────────────────────────────────────────────
func TestRunSupervised_HappyPathReconnect(t *testing.T) {
	t.Parallel()

	var connectCalls atomic.Int32
	var ackCount atomic.Int32

	// cancelCtx will be cancelled after the second session is established.
	ctx, cancel := context.WithCancel(context.Background())

	connectFn := func(_ context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		call := connectCalls.Add(1)
		ch := make(chan mq.Delivery, 2)
		cleanup := func() {}

		if call == 1 {
			// Feed 2 messages then close — simulates disconnect.
			for i := range 2 {
				_ = i
				msg := mq.Message{Topic: "t", Body: []byte("hi")}
				ch <- mq.Delivery{
					Msg:  msg,
					Ack:  func() error { ackCount.Add(1); return nil },
					Nack: func() error { return nil },
				}
			}
			close(ch)
		} else {
			// Second session: cancel ctx immediately so RunSupervised exits.
			cancel()
			close(ch)
		}
		return ch, cleanup, nil
	}

	var backoffCalls []int
	var mu sync.Mutex
	bf := syncBackoff(&backoffCalls, &mu)

	err := mq.RunSupervised(ctx, "t", func(_ context.Context, _ mq.Message) error {
		return nil
	}, connectFn, bf)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if n := connectCalls.Load(); n < 2 {
		t.Fatalf("expected connect called >=2 times, got %d", n)
	}
	if n := ackCount.Load(); n != 2 {
		t.Fatalf("expected 2 acks, got %d", n)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test ②: connect errors and attempt counter reset
//
// First 3 connect calls return an error → backoff called with attempt 1, 2, 3.
// 4th call succeeds and delivers 0 messages before disconnect. Reconnect
// happens (5th call): attempt counter must reset to 1.
// We verify via the backoff records.
// ─────────────────────────────────────────────────────────────────────────────
func TestRunSupervised_ConnectErrorAttemptReset(t *testing.T) {
	t.Parallel()

	var connectCalls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	connectFn := func(_ context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		call := connectCalls.Add(1)
		switch {
		case call <= 3:
			return nil, nil, errors.New("dial refused")
		case call == 4:
			// Success: immediately close channel to trigger reconnect.
			ch := make(chan mq.Delivery)
			close(ch)
			return ch, func() {}, nil
		default:
			// 5th call: cancel ctx so we exit.
			cancel()
			ch := make(chan mq.Delivery)
			close(ch)
			return ch, func() {}, nil
		}
	}

	var attempts []int
	var mu sync.Mutex
	bf := syncBackoff(&attempts, &mu)

	_ = mq.RunSupervised(ctx, "t", func(_ context.Context, _ mq.Message) error {
		return nil
	}, connectFn, bf)

	mu.Lock()
	defer mu.Unlock()

	// First three connect failures → attempts [1,2,3].
	if len(attempts) < 3 {
		t.Fatalf("expected at least 3 backoff calls, got %d: %v", len(attempts), attempts)
	}
	if attempts[0] != 1 || attempts[1] != 2 || attempts[2] != 3 {
		t.Fatalf("expected attempts [1,2,3] prefix, got %v", attempts)
	}

	// After success (call 4) and then immediate disconnect (call 5),
	// the next backoff attempt must reset to 1.
	// attempts[3] is the backoff after the disconnect of call 4.
	if len(attempts) >= 4 && attempts[3] != 1 {
		t.Fatalf("expected attempt reset to 1 after successful connect, got %d (attempts=%v)", attempts[3], attempts)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test ③: handler error → Nack; success → Ack
// ─────────────────────────────────────────────────────────────────────────────
func TestRunSupervised_HandlerNack(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	var ackCount, nackCount atomic.Int32

	makeDelivery := func(idx int) mq.Delivery {
		return mq.Delivery{
			Msg:  mq.Message{Topic: "t", Body: []byte{byte(idx)}},
			Ack:  func() error { ackCount.Add(1); return nil },
			Nack: func() error { nackCount.Add(1); return nil },
		}
	}

	var connectCalls atomic.Int32
	connectFn := func(_ context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		call := connectCalls.Add(1)
		if call > 1 {
			// Subsequent reconnect: cancel and return empty closed channel.
			cancel()
			ch := make(chan mq.Delivery)
			close(ch)
			return ch, func() {}, nil
		}
		ch := make(chan mq.Delivery, 3)
		for i := range 3 {
			ch <- makeDelivery(i)
		}
		close(ch)
		return ch, func() {}, nil
	}

	var mu sync.Mutex
	var attempts []int
	bf := syncBackoff(&attempts, &mu)

	// Handler: reject message index 1 (body[0]==1), accept the rest.
	err := mq.RunSupervised(ctx, "t", func(_ context.Context, m mq.Message) error {
		if len(m.Body) > 0 && m.Body[0] == 1 {
			return errors.New("reject this one")
		}
		return nil
	}, connectFn, bf)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if n := ackCount.Load(); n != 2 {
		t.Fatalf("expected 2 acks, got %d", n)
	}
	if n := nackCount.Load(); n != 1 {
		t.Fatalf("expected 1 nack, got %d", n)
	}
}

// TestRunSupervised_ExtractsTraceIntoHandlerCtx is the R9F6 consumer-side test
// for the shared supervisor driver (used by rabbitmq and any RunSupervised-based
// adapter): a delivery whose Msg.Headers carry a W3C traceparent must produce a
// handler ctx that joins the producer's trace. We build the traceparent the way a
// producer would (mq.InjectTrace), ship it as Msg.Headers, and assert the handler
// ctx carries the same trace id.
//
// Mutation self-proof: revert drainDeliveries to call h(ctx, d.Msg) instead of
// h(mq.ExtractTrace(ctx, d.Msg.Headers), d.Msg) and the handler ctx has no span
// context, failing the trace-id assertion below.
func TestRunSupervised_ExtractsTraceIntoHandlerCtx(t *testing.T) {
	t.Parallel()

	const traceHex = "aabbccddeeff00112233445566778899"
	const spanHex = "aabbccddeeff0011"

	tid, err := oteltrace.TraceIDFromHex(traceHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	sid, err := oteltrace.SpanIDFromHex(spanHex)
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	producerCtx := oteltrace.ContextWithSpanContext(context.Background(),
		oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
			TraceID: tid, SpanID: sid, TraceFlags: oteltrace.FlagsSampled, Remote: true,
		}))
	headers := mq.InjectTrace(producerCtx, nil)
	if _, ok := headers["traceparent"]; !ok {
		t.Fatalf("test setup: no traceparent injected: %+v", headers)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var connectCalls atomic.Int32
	connectFn := func(_ context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		if connectCalls.Add(1) > 1 {
			cancel()
			ch := make(chan mq.Delivery)
			close(ch)
			return ch, func() {}, nil
		}
		ch := make(chan mq.Delivery, 1)
		ch <- mq.Delivery{
			Msg:  mq.Message{Topic: "t", Body: []byte("x"), Headers: headers},
			Ack:  func() error { return nil },
			Nack: func() error { return nil },
		}
		close(ch)
		return ch, func() {}, nil
	}

	var mu sync.Mutex
	var attempts []int
	bf := syncBackoff(&attempts, &mu)

	var gotTrace, gotParent string
	var handled atomic.Int32
	rerr := mq.RunSupervised(ctx, "t", func(hctx context.Context, _ mq.Message) error {
		sc := oteltrace.SpanContextFromContext(hctx)
		gotTrace = sc.TraceID().String()
		gotParent = sc.SpanID().String()
		handled.Add(1)
		return nil
	}, connectFn, bf)

	if !errors.Is(rerr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", rerr)
	}
	if handled.Load() != 1 {
		t.Fatalf("expected handler called once, got %d", handled.Load())
	}
	if gotTrace != traceHex {
		t.Fatalf("handler ctx trace id = %q, want %q (supervisor did not extract trace — R9F6)", gotTrace, traceHex)
	}
	if gotParent != spanHex {
		t.Fatalf("handler ctx parent span id = %q, want %q", gotParent, spanHex)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test ④-a: ctx cancel during backoff wait
// ─────────────────────────────────────────────────────────────────────────────
func TestRunSupervised_CancelDuringBackoff(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	// ConnectFn always fails so RunSupervised enters backoff loop.
	var connectCalls atomic.Int32
	connectFn := func(_ context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		connectCalls.Add(1)
		return nil, nil, errors.New("always fails")
	}

	// Backoff: on first attempt cancel ctx; return false so RunSupervised exits.
	bf := func(bCtx context.Context, attempt int) bool {
		cancel() // cancel the supervisor context
		<-bCtx.Done()
		return false
	}

	done := make(chan error, 1)
	go func() {
		done <- mq.RunSupervised(ctx, "t", func(_ context.Context, _ mq.Message) error {
			return nil
		}, connectFn, bf)
	}()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test ④-b: ctx cancel while consuming messages (no goroutine leak)
// ─────────────────────────────────────────────────────────────────────────────
func TestRunSupervised_CancelDuringConsume(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	// Channel that blocks after one message so we can cancel from the handler.
	ch := make(chan mq.Delivery, 1)
	ch <- mq.Delivery{
		Msg:  mq.Message{Topic: "t"},
		Ack:  func() error { return nil },
		Nack: func() error { return nil },
	}
	// Do NOT close ch — RunSupervised must respect ctx cancellation.

	connectFn := func(_ context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		return ch, func() {}, nil
	}

	var mu sync.Mutex
	var attempts []int
	bf := syncBackoff(&attempts, &mu)

	done := make(chan error, 1)
	go func() {
		done <- mq.RunSupervised(ctx, "t", func(_ context.Context, _ mq.Message) error {
			// After handling the one queued message, cancel.
			cancel()
			return nil
		}, connectFn, bf)
	}()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

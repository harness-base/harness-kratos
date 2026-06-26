package main

import (
	"errors"
	"testing"
)

// spyConsumer records how many times Stop was called and the call order.
type spyConsumer struct {
	stops int
	seq   *[]string
}

func (s *spyConsumer) Stop() {
	s.stops++
	if s.seq != nil {
		*s.seq = append(*s.seq, "consumer.Stop")
	}
}

// spyData records how many times Close was called, the call order, and returns
// a configurable error so we can assert error propagation.
type spyData struct {
	closes int
	seq    *[]string
	retErr error
}

func (s *spyData) Close() error {
	s.closes++
	if s.seq != nil {
		*s.seq = append(*s.seq, "data.Close")
	}
	return s.retErr
}

// TestAppTeardown_StopsConsumerThenClosesData proves the graceful-shutdown
// sequence run from the AfterStop hook: the consumer is stopped exactly once,
// the data layer is closed exactly once, and the consumer Stop happens BEFORE
// the data Close (consumption must cease before MQ connections are torn down).
//
// Mutation self-justification:
//   - Drop `d.Close()` from appTeardown          → closes==0, fails.
//   - Drop `cr.Stop()` from appTeardown          → stops==0, fails.
//   - Swap the order (Close before Stop)         → order assertion fails.
//   - Call either twice                          → count assertion fails.
func TestAppTeardown_StopsConsumerThenClosesData(t *testing.T) {
	var order []string
	cr := &spyConsumer{seq: &order}
	d := &spyData{seq: &order}

	if err := appTeardown(cr, d); err != nil {
		t.Fatalf("appTeardown returned unexpected error: %v", err)
	}

	if cr.stops != 1 {
		t.Errorf("consumer Stop called %d times, want exactly 1", cr.stops)
	}
	if d.closes != 1 {
		t.Errorf("data Close called %d times, want exactly 1", d.closes)
	}

	want := []string{"consumer.Stop", "data.Close"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Errorf("teardown order = %v, want %v (consumer must stop before data closes)", order, want)
	}
}

// TestAppTeardown_PropagatesCloseError proves appTeardown surfaces the
// data-layer Close error to the lifecycle (so a failed resource teardown is not
// silently swallowed). The consumer is still stopped.
//
// Mutation self-justification:
//   - Return nil instead of d.Close()'s error    → err==nil, fails.
//   - Skip cr.Stop() on the error path           → stops==0, fails.
func TestAppTeardown_PropagatesCloseError(t *testing.T) {
	sentinel := errors.New("pg pool close failed")
	cr := &spyConsumer{}
	d := &spyData{retErr: sentinel}

	err := appTeardown(cr, d)
	if !errors.Is(err, sentinel) {
		t.Fatalf("appTeardown error = %v, want it to wrap/return %v", err, sentinel)
	}
	if cr.stops != 1 {
		t.Errorf("consumer Stop called %d times on error path, want exactly 1", cr.stops)
	}
}

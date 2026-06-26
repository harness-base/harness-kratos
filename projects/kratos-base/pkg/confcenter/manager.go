// Package confcenter provides a generic runtime-config manager with hot-reload
// support, validate-on-write semantics, and rollback on validation failure.
//
// Design:
//   - Manager[T] is the single source of truth for the current config version.
//   - Publish validates a candidate; only on success does it bump the version
//     and fan-out to subscribers.
//   - BindKratosWatch wires a kratos config.Config watcher to a Manager so that
//     file (or remote) changes flow through automatically.
package confcenter

import (
	"context"
	"log/slog"
	"sync"

	kratosconfig "github.com/go-kratos/kratos/v2/config"

	"github.com/z-mate/kratos-base/pkg/resource"
)

// Snapshot is a versioned point-in-time view of a config value.
type Snapshot[T any] struct {
	Version uint64
	Value   T
}

// Manager is a thread-safe generic config holder with validation and fan-out.
type Manager[T any] struct {
	mu       sync.RWMutex
	cur      Snapshot[T]
	validate func(T) error
	subs     []chan Snapshot[T]
}

// NewManager creates a Manager with initial as the starting value.
// validate is called on every Publish (and on the initial value); it must not
// be nil (use func(T) error { return nil } for unconditional acceptance).
// Returns an error if validate(initial) fails.
func NewManager[T any](initial T, validate func(T) error) (*Manager[T], error) {
	if err := validate(initial); err != nil {
		return nil, err
	}
	return &Manager[T]{
		cur:      Snapshot[T]{Version: 1, Value: initial},
		validate: validate,
	}, nil
}

// Current returns the latest validated snapshot.
func (m *Manager[T]) Current() Snapshot[T] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cur
}

// Publish validates next and, if valid, atomically replaces the current
// snapshot (Version incremented) and notifies all subscribers.
// If validate returns an error the current snapshot is unchanged.
// Subscriber channels that are full are skipped (non-blocking send).
func (m *Manager[T]) Publish(next T) error {
	if err := m.validate(next); err != nil {
		return err
	}

	m.mu.Lock()
	m.cur = Snapshot[T]{
		Version: m.cur.Version + 1,
		Value:   next,
	}
	snap := m.cur
	subs := m.subs // snapshot the slice under lock
	m.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- snap:
		default:
			// subscriber is slow; skip to avoid blocking
		}
	}
	return nil
}

// Subscribe returns a new buffered (capacity 1) channel that receives a
// Snapshot[T] each time Publish succeeds.
func (m *Manager[T]) Subscribe() <-chan Snapshot[T] {
	ch := make(chan Snapshot[T], 1)
	m.mu.Lock()
	m.subs = append(m.subs, ch)
	m.mu.Unlock()
	return ch
}

// resourceAdapter adapts Manager[T] to resource.Source.
type resourceAdapter[T any] struct {
	m *Manager[T]
}

func (a *resourceAdapter[T]) Current() resource.Snapshot {
	s := a.m.Current()
	return resource.Snapshot{Version: s.Version, Value: s.Value}
}

// ResourceSource returns a resource.Source adapter whose Current() reflects
// the Manager's current version and value (as any).
func (m *Manager[T]) ResourceSource() resource.Source {
	return &resourceAdapter[T]{m: m}
}

// BindKratosWatch registers observers on the given kratos config.Config for
// each key in keys.  When any key changes, reload(c) is called to produce a
// new T; the result is passed to m.Publish.  If reload or Publish fails a
// warning is logged and the previous config is retained (no panic).
//
// ctx is used only for potential future cancellation; the underlying kratos
// Watch mechanism does not accept a context directly.
func BindKratosWatch[T any](
	_ context.Context,
	c kratosconfig.Config,
	keys []string,
	reload func(kratosconfig.Config) (T, error),
	m *Manager[T],
	logger *slog.Logger,
) error {
	for _, key := range keys {
		key := key // capture
		observer := func(_ string, _ kratosconfig.Value) {
			next, err := reload(c)
			if err != nil {
				logger.Warn("confcenter: reload failed; retaining previous config",
					slog.String("key", key),
					slog.String("error", err.Error()),
				)
				return
			}
			if pubErr := m.Publish(next); pubErr != nil {
				logger.Warn("confcenter: Publish failed; retaining previous config",
					slog.String("key", key),
					slog.String("error", pubErr.Error()),
				)
				return
			}
			// Positive signal for e2e: a new version was validated and applied.
			// Emitted only on the success path; failure paths above log
			// "retaining previous config" and return before reaching here.
			logger.Info("confcenter: config applied",
				slog.String("key", key),
				slog.Uint64("version", m.Current().Version),
			)
		}
		if err := c.Watch(key, observer); err != nil {
			return err
		}
	}
	return nil
}

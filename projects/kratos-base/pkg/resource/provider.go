package resource

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrNotReady is returned when the provider has no live resource and Build fails.
var ErrNotReady = errors.New("resource: not ready")

// Adapter describes how to create, close, fingerprint, and health-check a resource.
type Adapter[T any] struct {
	// Build creates a new resource handle from the given config. Must be non-nil.
	Build func(ctx context.Context, cfg any) (T, error)
	// Close tears down a resource handle. May be nil (no-op close).
	Close func(T) error
	// Fingerprint extracts the connection-relevant subset of cfg into a string.
	// If nil, fingerprinting is skipped (empty string used).
	Fingerprint func(cfg any) string
	// Health probes liveness. If nil the resource is always considered healthy.
	Health func(ctx context.Context, t T) error
}

// Snapshot is a point-in-time view of the source configuration.
type Snapshot struct {
	Version uint64
	Value   any
}

// Source provides the current configuration snapshot.
type Source interface{ Current() Snapshot }

// Provider lazily builds and caches a resource, automatically rebuilding when
// the source version or connection fingerprint changes. On transient Build
// failures it continues to serve the last known-good value (self-healing).
type Provider[T any] struct {
	mu      sync.Mutex
	src     Source
	ad      Adapter[T]
	cur     T
	ready   bool
	ver     uint64
	fp      string
	lastErr error
}

// New creates a Provider backed by src using ad.
func New[T any](src Source, ad Adapter[T]) *Provider[T] {
	return &Provider[T]{src: src, ad: ad}
}

// Get returns a live resource handle.
//
// It checks whether the source has a new version or fingerprint; if so, it
// rebuilds. On Build failure it falls back to the last good handle when one
// exists, otherwise it returns the zero value and an error.
func (p *Provider[T]) Get(ctx context.Context) (T, error) {
	snap := p.src.Current()

	fp := ""
	if p.ad.Fingerprint != nil {
		fp = p.ad.Fingerprint(snap.Value)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ready && p.ver == snap.Version && p.fp == fp {
		return p.cur, nil
	}

	next, err := p.ad.Build(ctx, snap.Value)
	if err != nil {
		p.lastErr = err
		if p.ready {
			// Self-heal: return stale handle rather than surfacing the error.
			return p.cur, nil
		}
		var z T
		return z, fmt.Errorf("%w: %w", ErrNotReady, err)
	}

	// Build succeeded: close old handle and install new one.
	if p.ready && p.ad.Close != nil {
		_ = p.ad.Close(p.cur)
	}
	p.cur = next
	p.ready = true
	p.ver = snap.Version
	p.fp = fp
	p.lastErr = nil
	return p.cur, nil
}

// Healthy drives a refresh (via Get) and then runs the Health probe if one is
// configured. It is safe to call concurrently; it does not hold p.mu while
// calling Health.
//
// When the Health probe returns an error, Healthy marks the provider as not
// ready so that the next Get call triggers a fresh Build. This allows
// automatic recovery after a resource (e.g. a network connection) becomes
// unavailable and then comes back.
func (p *Provider[T]) Healthy(ctx context.Context) error {
	t, err := p.Get(ctx)
	if err != nil {
		return err
	}
	if p.ad.Health != nil {
		if herr := p.ad.Health(ctx, t); herr != nil {
			// Mark not-ready so that the next Get will attempt a fresh Build.
			// This enables self-healing when a live resource (connection) goes
			// dead without a config-version change. Close the dead handle now —
			// the rebuild path in Get only closes the old value when ready is
			// still true, so skipping this would leak one handle per outage.
			//
			// TOCTOU guard: we probed t after releasing p.mu, so a concurrent
			// Get may have rebuilt (version/fingerprint bump) and installed a
			// fresh LIVE handle as p.cur — and that rebuild already closed the
			// old t. Only tear down when p.cur is STILL the handle we probed;
			// otherwise we'd destroy a working connection and falsely flip
			// ready=false, churning a healthy resource. (T is always a
			// pointer/interface here, so == identity comparison is valid.)
			p.mu.Lock()
			if p.ready && any(p.cur) == any(t) {
				if p.ad.Close != nil {
					_ = p.ad.Close(p.cur)
				}
				p.ready = false
			}
			p.mu.Unlock()
			return herr
		}
	}
	return nil
}

// Close tears down the current resource (if any) and marks the provider as
// not ready, so the next Get will trigger a fresh Build.
func (p *Provider[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready && p.ad.Close != nil {
		_ = p.ad.Close(p.cur)
	}
	p.ready = false
	return nil
}

// LastErr returns the most recent Build error, or nil if the last Build
// succeeded. Useful for surfacing why a resource is currently degraded
// (e.g. readiness details) without forcing a rebuild.
func (p *Provider[T]) LastErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

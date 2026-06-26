package resource

import (
	"context"
	"sync"
)

// Check is a readiness probe function.
type Check func(ctx context.Context) error

// Registry holds named readiness checks and can report aggregate health.
type Registry struct {
	mu     sync.Mutex
	checks map[string]Check
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{checks: make(map[string]Check)}
}

// Register adds or replaces the named check.
func (r *Registry) Register(name string, c Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = c
}

// Ready runs every registered check concurrently and returns an aggregate
// result. ok is true only when all checks pass. details contains every
// non-nil error, keyed by check name. The registry lock is held only long
// enough to snapshot the check map; checks themselves run without the lock.
func (r *Registry) Ready(ctx context.Context) (ok bool, details map[string]error) {
	r.mu.Lock()
	snapshot := make(map[string]Check, len(r.checks))
	for name, c := range r.checks {
		snapshot[name] = c
	}
	r.mu.Unlock()

	details = make(map[string]error)
	var (
		mu     sync.Mutex
		anyErr bool
	)
	var wg sync.WaitGroup
	for name, c := range snapshot {
		name, c := name, c
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := c(ctx)
			if err != nil {
				mu.Lock()
				details[name] = err
				anyErr = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	ok = !anyErr
	return ok, details
}

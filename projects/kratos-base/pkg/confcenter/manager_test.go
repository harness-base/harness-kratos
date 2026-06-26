package confcenter_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	kratosconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"

	"github.com/z-mate/kratos-base/pkg/confcenter"
)

// noValidate is a validator that always passes.
func noValidate[T any](_ T) error { return nil }

// rejectAll is a validator that always fails.
func rejectAll[T any](_ T) error { return errors.New("invalid config") }

type testCfg struct {
	Value string
}

func TestNewManager_InvalidInitial(t *testing.T) {
	t.Parallel()
	_, err := confcenter.NewManager(testCfg{Value: "bad"}, rejectAll[testCfg])
	if err == nil {
		t.Fatal("expected error for invalid initial config, got nil")
	}
}

func TestNewManager_ValidInitial(t *testing.T) {
	t.Parallel()
	m, err := confcenter.NewManager(testCfg{Value: "ok"}, noValidate[testCfg])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	snap := m.Current()
	if snap.Version != 1 {
		t.Errorf("initial Version = %d, want 1", snap.Version)
	}
	if snap.Value.Value != "ok" {
		t.Errorf("initial Value = %q, want %q", snap.Value.Value, "ok")
	}
}

func TestPublish_Valid(t *testing.T) {
	t.Parallel()
	m, err := confcenter.NewManager(testCfg{Value: "v1"}, noValidate[testCfg])
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	sub := m.Subscribe()

	if err := m.Publish(testCfg{Value: "v2"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	snap := m.Current()
	if snap.Version != 2 {
		t.Errorf("Version after Publish = %d, want 2", snap.Version)
	}
	if snap.Value.Value != "v2" {
		t.Errorf("Value after Publish = %q, want %q", snap.Value.Value, "v2")
	}

	// Subscriber must have received the new snapshot (buffered channel, no sleep needed).
	select {
	case got := <-sub:
		if got.Version != 2 {
			t.Errorf("subscriber Version = %d, want 2", got.Version)
		}
		if got.Value.Value != "v2" {
			t.Errorf("subscriber Value = %q, want %q", got.Value.Value, "v2")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber notification")
	}
}

func TestPublish_Invalid(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("bad value")
	validate := func(c testCfg) error {
		if c.Value == "bad" {
			return sentinel
		}
		return nil
	}

	m, err := confcenter.NewManager(testCfg{Value: "ok"}, validate)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	pubErr := m.Publish(testCfg{Value: "bad"})
	if !errors.Is(pubErr, sentinel) {
		t.Fatalf("Publish error = %v, want %v", pubErr, sentinel)
	}

	// cur must be unchanged.
	snap := m.Current()
	if snap.Version != 1 {
		t.Errorf("Version after failed Publish = %d, want 1", snap.Version)
	}
	if snap.Value.Value != "ok" {
		t.Errorf("Value after failed Publish = %q, want %q", snap.Value.Value, "ok")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	t.Parallel()
	m, err := confcenter.NewManager(testCfg{Value: "init"}, noValidate[testCfg])
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	sub1 := m.Subscribe()
	sub2 := m.Subscribe()

	if err := m.Publish(testCfg{Value: "broadcast"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	for i, sub := range []<-chan confcenter.Snapshot[testCfg]{sub1, sub2} {
		select {
		case got := <-sub:
			if got.Value.Value != "broadcast" {
				t.Errorf("sub%d: Value = %q, want %q", i+1, got.Value.Value, "broadcast")
			}
		case <-time.After(time.Second):
			t.Fatalf("sub%d: timed out waiting for notification", i+1)
		}
	}
}

func TestResourceSource(t *testing.T) {
	t.Parallel()
	m, err := confcenter.NewManager(testCfg{Value: "res"}, noValidate[testCfg])
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	src := m.ResourceSource()
	snap := src.Current()
	if snap.Version != 1 {
		t.Errorf("ResourceSource Version = %d, want 1", snap.Version)
	}
	cfg, ok := snap.Value.(testCfg)
	if !ok {
		t.Fatalf("ResourceSource Value type = %T, want testCfg", snap.Value)
	}
	if cfg.Value != "res" {
		t.Errorf("ResourceSource Value.Value = %q, want %q", cfg.Value, "res")
	}

	// After Publish, ResourceSource reflects the new version.
	if err := m.Publish(testCfg{Value: "res2"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	snap2 := src.Current()
	if snap2.Version != 2 {
		t.Errorf("ResourceSource Version after Publish = %d, want 2", snap2.Version)
	}
}

func TestBindKratosWatch_ReloadSuccess(t *testing.T) {
	t.Parallel()

	// Write a temporary YAML config file.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runtime.yaml")
	writeYAML := func(val string) {
		t.Helper()
		content := "app:\n  value: " + val + "\n"
		if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write yaml: %v", err)
		}
	}
	writeYAML("initial")

	c := kratosconfig.New(kratosconfig.WithSource(file.NewSource(cfgPath)))
	if err := c.Load(); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	defer c.Close() //nolint:errcheck

	type AppCfg struct{ Value string }

	reload := func(cfg kratosconfig.Config) (AppCfg, error) {
		var v AppCfg
		if err := cfg.Value("app").Scan(&v); err != nil {
			return AppCfg{}, err
		}
		return v, nil
	}

	initial, err := reload(c)
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}

	m, err := confcenter.NewManager(initial, noValidate[AppCfg])
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	sub := m.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := confcenter.BindKratosWatch(ctx, c, []string{"app"}, reload, m, logger); err != nil {
		t.Fatalf("BindKratosWatch: %v", err)
	}

	// Update the config file — the file watcher will fire.
	writeYAML("updated")

	// Wait for the subscriber notification (channel/select, no sleep).
	select {
	case got := <-sub:
		if got.Value.Value != "updated" {
			t.Errorf("after watch: Value = %q, want %q", got.Value.Value, "updated")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for BindKratosWatch to propagate change")
	}
}

func TestBindKratosWatch_PublishRejectRetainsPrev(t *testing.T) {
	t.Parallel()

	// Headline guarantee composed THROUGH the watcher: a hot config change that
	// reloads cleanly (parses) but FAILS domain validation must be rejected by
	// Publish, leaving the previous config serving (version + value unchanged).
	// The building blocks (Publish-validate-reject in TestPublish_Invalid, and
	// watcher-reload in the tests above) are covered in isolation; this asserts
	// the observer's warn+retain branch (manager.go BindKratosWatch) end-to-end.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(cfgPath, []byte("app:\n  value: orig\n"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	c := kratosconfig.New(kratosconfig.WithSource(file.NewSource(cfgPath)))
	if err := c.Load(); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	defer c.Close() //nolint:errcheck

	type AppCfg struct{ Value string }

	// reload always succeeds (parses fine) — the rejection must come from
	// validation, not from reload, so this exercises the Publish-reject path.
	reload := func(cfg kratosconfig.Config) (AppCfg, error) {
		var v AppCfg
		if err := cfg.Value("app").Scan(&v); err != nil {
			return AppCfg{}, err
		}
		return v, nil
	}

	// validate accepts the initial value ("orig") but rejects the post-change
	// value ("bad"); it signals on rejection so the test is deterministic (no
	// sleep): receiving on `rejected` proves the observer reloaded AND called
	// Publish, which hit the warn+retain branch.
	rejected := make(chan struct{}, 1)
	validate := func(v AppCfg) error {
		if v.Value == "bad" {
			select {
			case rejected <- struct{}{}:
			default:
			}
			return errors.New("domain rule: value 'bad' not allowed")
		}
		return nil
	}

	initial, err := reload(c)
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	m, err := confcenter.NewManager(initial, validate)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.Current().Value.Value != "orig" {
		t.Fatalf("precondition: initial Value = %q, want orig", m.Current().Value.Value)
	}
	prevVersion := m.Current().Version

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := confcenter.BindKratosWatch(ctx, c, []string{"app"}, reload, m, logger); err != nil {
		t.Fatalf("BindKratosWatch: %v", err)
	}

	// Hot change to a value that reloads fine but fails validation.
	if err := os.WriteFile(cfgPath, []byte("app:\n  value: bad\n"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// Wait until validate has rejected the candidate (proves the observer
	// reloaded + called Publish, hitting the reject branch) — no sleep.
	select {
	case <-rejected:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher never delivered the invalid change to Publish within 5s")
	}

	// Previous config must still be serving: version + value unchanged.
	cur := m.Current()
	if cur.Version != prevVersion {
		t.Errorf("after rejected hot update: Version = %d, want unchanged %d", cur.Version, prevVersion)
	}
	if cur.Value.Value != "orig" {
		t.Errorf("after rejected hot update: Value = %q, want retained %q", cur.Value.Value, "orig")
	}
}

func TestBindKratosWatch_ReloadFailRetainsPrev(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(cfgPath, []byte("app:\n  value: orig\n"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	c := kratosconfig.New(kratosconfig.WithSource(file.NewSource(cfgPath)))
	if err := c.Load(); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	defer c.Close() //nolint:errcheck

	type AppCfg struct{ Value string }

	// Initial load succeeds.
	reloadOK := func(cfg kratosconfig.Config) (AppCfg, error) {
		var v AppCfg
		if err := cfg.Value("app").Scan(&v); err != nil {
			return AppCfg{}, err
		}
		return v, nil
	}
	initial, err := reloadOK(c)
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	m, err := confcenter.NewManager(initial, noValidate[AppCfg])
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// The watch uses a reload that always fails, signalling when it has run.
	// This lets the test assert deterministically: we wait until the failing
	// reload has actually executed (proving the watcher fired — no false
	// positive) instead of sleeping for a fixed window.
	reloaded := make(chan struct{}, 1)
	reloadFail := func(kratosconfig.Config) (AppCfg, error) {
		select {
		case reloaded <- struct{}{}:
		default:
		}
		return AppCfg{}, errors.New("reload failed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := confcenter.BindKratosWatch(ctx, c, []string{"app"}, reloadFail, m, logger); err != nil {
		t.Fatalf("BindKratosWatch: %v", err)
	}

	// Trigger a file change; the watcher must fire and call reloadFail.
	if err := os.WriteFile(cfgPath, []byte("app:\n  value: changed\n"), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// Wait until the failing reload has run (proves the watcher fired).
	select {
	case <-reloaded:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher never fired within 5s")
	}

	// Bad config rejected: version and value stay at the initial snapshot.
	snap := m.Current()
	if snap.Version != 1 {
		t.Errorf("Version after failed reload = %d, want 1", snap.Version)
	}
	if snap.Value.Value != "orig" {
		t.Errorf("Value after failed reload = %q, want %q", snap.Value.Value, "orig")
	}
}

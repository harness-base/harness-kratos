package resource_test

import (
	"context"
	"errors"
	"testing"

	"github.com/z-mate/kratos-base/pkg/resource"
)

func TestRegistry_AllPass(t *testing.T) {
	reg := resource.NewRegistry()
	reg.Register("db", func(_ context.Context) error { return nil })
	reg.Register("cache", func(_ context.Context) error { return nil })

	ok, details := reg.Ready(context.Background())
	if !ok {
		t.Error("expected ok=true when all checks pass")
	}
	for name, err := range details {
		if err != nil {
			t.Errorf("check %q should have nil error, got: %v", name, err)
		}
	}
}

func TestRegistry_OneFails(t *testing.T) {
	errDB := errors.New("db: connection refused")
	reg := resource.NewRegistry()
	reg.Register("db", func(_ context.Context) error { return errDB })
	reg.Register("cache", func(_ context.Context) error { return nil })

	ok, details := reg.Ready(context.Background())
	if ok {
		t.Error("expected ok=false when a check fails")
	}
	if details["db"] == nil {
		t.Error("expected details[db] to be non-nil")
	}
	if !errors.Is(details["db"], errDB) {
		t.Errorf("expected details[db]=%v, got %v", errDB, details["db"])
	}
}

func TestRegistry_AllFail(t *testing.T) {
	reg := resource.NewRegistry()
	reg.Register("a", func(_ context.Context) error { return errors.New("err a") })
	reg.Register("b", func(_ context.Context) error { return errors.New("err b") })

	ok, details := reg.Ready(context.Background())
	if ok {
		t.Error("expected ok=false when all checks fail")
	}
	if details["a"] == nil {
		t.Error("expected details[a] non-nil")
	}
	if details["b"] == nil {
		t.Error("expected details[b] non-nil")
	}
}

func TestRegistry_Empty(t *testing.T) {
	reg := resource.NewRegistry()
	ok, details := reg.Ready(context.Background())
	if !ok {
		t.Error("empty registry should be ok=true")
	}
	if len(details) != 0 {
		t.Errorf("expected empty details, got %v", details)
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	errNew := errors.New("new error")
	reg := resource.NewRegistry()
	reg.Register("svc", func(_ context.Context) error { return nil })
	// Overwrite with failing check
	reg.Register("svc", func(_ context.Context) error { return errNew })

	ok, details := reg.Ready(context.Background())
	if ok {
		t.Error("expected ok=false after overwrite with failing check")
	}
	if !errors.Is(details["svc"], errNew) {
		t.Errorf("expected overwritten check to be used, got %v", details["svc"])
	}
}

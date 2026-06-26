package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	transhttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/app/demo/internal/conf"
	"github.com/z-mate/kratos-base/app/demo/internal/server"
	"github.com/z-mate/kratos-base/app/demo/internal/service"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// fakeRepo implements biz.GreetRepo for the DemoService without touching a DB.
type fakeRepo struct{}

func (f *fakeRepo) Get(_ context.Context, _ int64) (*biz.Greet, error) {
	return nil, errors.New("not implemented in test")
}

// fakeHitsRepo implements biz.HitsRepo for the DemoService without touching Redis.
type fakeHitsRepo struct{}

func (f *fakeHitsRepo) Incr(_ context.Context, _ string) (int64, error) {
	return 0, errors.New("not implemented in test")
}

// fakeEventRepo implements biz.EventRepo for tests without touching a broker.
type fakeEventRepo struct{}

func (f *fakeEventRepo) Emit(_ context.Context, _ string) (string, error) {
	return "", errors.New("not implemented in test")
}

// buildServer is a test helper that creates a NewHTTPServer with fake
// dependencies.  reg is caller-supplied so each test can control readiness.
func buildServer(t *testing.T, reg *resource.Registry) *transhttp.Server {
	t.Helper()
	uc := biz.NewGreetUsecase(&fakeRepo{})
	hitsUc := biz.NewHitsUsecase(&fakeHitsRepo{})
	eventUc := biz.NewEventUsecase(&fakeEventRepo{})
	svc := service.NewDemoService(uc, hitsUc, eventUc)
	rt := conf.Runtime{}
	rt.Server.HTTP.Addr = ":0" // ephemeral port, not used with httptest
	return server.NewHTTPServer(rt, svc, reg, nil)
}

func TestHealthzAlways200(t *testing.T) {
	reg := resource.NewRegistry()
	srv := buildServer(t, reg)

	// Use the kratos server as an http.Handler via httptest (no real listen).
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz: want 200, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("healthz: body %q missing status:ok", body)
	}
}

func TestReadyzAllChecksPassing(t *testing.T) {
	reg := resource.NewRegistry()
	reg.Register("db", func(_ context.Context) error { return nil }) // always passes

	srv := buildServer(t, reg)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("readyz all-pass: want 200, got %d; body=%q", res.StatusCode, body)
	}
}

func TestReadyzWithFailingCheck(t *testing.T) {
	reg := resource.NewRegistry()
	reg.Register("postgres", func(_ context.Context) error {
		return errors.New("connection refused")
	})

	srv := buildServer(t, reg)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz failing check: want 503, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "connection refused") {
		t.Fatalf("readyz failing check: body %q should contain error text", body)
	}
}

func TestReadyzMixedChecks(t *testing.T) {
	reg := resource.NewRegistry()
	reg.Register("ok-svc", func(_ context.Context) error { return nil })
	reg.Register("bad-svc", func(_ context.Context) error {
		return errors.New("timeout")
	})

	srv := buildServer(t, reg)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz mixed: want 503, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "timeout") {
		t.Fatalf("readyz mixed: body %q should contain 'timeout'", body)
	}
}

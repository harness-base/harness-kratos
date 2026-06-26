package obs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sampleHasValue reports whether the Prometheus text output contains a sample
// line for metricName (with or without labels) whose trailing value is want.
// It ignores HELP/TYPE comment lines and tolerates the optional {labels} block.
func sampleHasValue(out, metricName, want string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, metricName) {
			continue
		}
		// Must be the exact metric (name followed by '{' for labels or ' ' for value),
		// not a longer metric that shares this prefix.
		rest := line[len(metricName):]
		if rest == "" || (rest[0] != '{' && rest[0] != ' ') {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[len(fields)-1] == want {
			return true
		}
	}
	return false
}

func TestRegistryAndHandlerSingleton(t *testing.T) {
	if Registry() == nil {
		t.Fatal("Registry() returned nil")
	}
	// Singleton: repeated calls return the same registry instance.
	r1 := Registry()
	if r1 != Registry() {
		t.Fatal("Registry() is not a stable singleton")
	}
	if MeterProvider() == nil {
		t.Fatal("MeterProvider() returned nil")
	}
	if Handler() == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestHandlerServesPrometheusText(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	ct := res.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want prometheus text/plain", ct)
	}
	_ = body // body emptiness is exercised by the counter test below.
}

func TestMeterCounterAppearsInOutput(t *testing.T) {
	const metricName = "obs_test_requests_total"

	// Register a counter through the global otel meter (installed by setup via
	// MeterProvider) and record a value; it must surface in the Prometheus output.
	meter := MeterProvider().Meter("obs-test")
	counter, err := meter.Int64Counter(metricName)
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(context.Background(), 3)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	out := string(body)

	// The otel prometheus exporter renders the metric with its TYPE line and a
	// sample line carrying otel_scope_* labels and the recorded value, e.g.:
	//   # TYPE obs_test_requests_total counter
	//   obs_test_requests_total{otel_scope_name="obs-test",...} 3
	if !strings.Contains(out, "# TYPE "+metricName+" counter") {
		t.Fatalf("metric %q TYPE line not found in /metrics output:\n%s", metricName, out)
	}
	if !sampleHasValue(out, metricName, "3") {
		t.Errorf("expected a %q sample with value 3 in output:\n%s", metricName, out)
	}
	// Scope labels prove the sample came through the otel meter (not a raw
	// prometheus collector), i.e. the otel→prometheus bridge is wired.
	if !strings.Contains(out, `otel_scope_name="obs-test"`) {
		t.Errorf("expected otel_scope_name label on the otel-sourced metric:\n%s", out)
	}
}

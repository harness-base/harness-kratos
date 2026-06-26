package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/trace"
)

// decode parses a single JSON log line into a map for field assertions.
func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("log output is not valid JSON: %v\nraw: %q", err, b)
	}
	return m
}

func TestNewHandlerFixedFields(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		svc       string
		ver       string
		env       string
		logLevel  slog.Level
		wantEmit  bool
		wantField map[string]string
	}{
		{
			name:     "info emits fixed fields",
			level:    "info",
			svc:      "demo",
			ver:      "1.2.3",
			env:      "prod",
			logLevel: slog.LevelInfo,
			wantEmit: true,
			wantField: map[string]string{
				"service": "demo",
				"version": "1.2.3",
				"env":     "prod",
				"level":   "INFO",
				"msg":     "hello",
			},
		},
		{
			name:     "debug below info level is dropped",
			level:    "info",
			svc:      "demo",
			ver:      "1.0.0",
			env:      "dev",
			logLevel: slog.LevelDebug,
			wantEmit: false,
		},
		{
			name:     "debug level emits debug",
			level:    "debug",
			svc:      "svc",
			ver:      "0.1.0",
			env:      "test",
			logLevel: slog.LevelDebug,
			wantEmit: true,
			wantField: map[string]string{
				"service": "svc",
				"version": "0.1.0",
				"env":     "test",
				"level":   "DEBUG",
			},
		},
		{
			name:     "unknown level defaults to info (warn still emits)",
			level:    "not-a-level",
			svc:      "s",
			ver:      "v",
			env:      "e",
			logLevel: slog.LevelWarn,
			wantEmit: true,
			wantField: map[string]string{
				"service": "s",
				"level":   "WARN",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := newHandler(&buf, tc.level, tc.svc, tc.ver, tc.env)
			l.Log(context.Background(), tc.logLevel, "hello")

			if !tc.wantEmit {
				if buf.Len() != 0 {
					t.Fatalf("expected no output for level below threshold, got: %q", buf.String())
				}
				return
			}
			if buf.Len() == 0 {
				t.Fatal("expected log output, got none")
			}
			m := decode(t, buf.Bytes())
			for k, want := range tc.wantField {
				got, ok := m[k]
				if !ok {
					t.Errorf("missing field %q in %v", k, m)
					continue
				}
				if gs, _ := got.(string); gs != want {
					t.Errorf("field %q = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestNewHandlerOmitsEmptyFields(t *testing.T) {
	var buf bytes.Buffer
	// Empty svc/ver/env must not appear as empty-string fields.
	l := newHandler(&buf, "info", "", "", "")
	l.Info("x")
	m := decode(t, buf.Bytes())
	for _, k := range []string{"service", "version", "env"} {
		if _, ok := m[k]; ok {
			t.Errorf("did not expect field %q when empty, got %v", k, m[k])
		}
	}
}

// validSpanContext builds a deterministic, valid (sampled) span context for
// trace enrichment assertions.
func validSpanContext(t *testing.T) trace.SpanContext {
	t.Helper()
	tid, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	sid, err := trace.SpanIDFromHex("1112131415161718")
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
}

func TestWithTraceFields(t *testing.T) {
	var buf bytes.Buffer
	// Point the package Default at our buffer so With(ctx) is observable.
	Default = newHandler(&buf, "info", "demo", "1.0.0", "test")

	tests := []struct {
		name        string
		ctx         context.Context
		wantTrace   bool
		wantTraceID string
		wantSpanID  string
	}{
		{
			name:        "valid span context injects trace_id and span_id",
			ctx:         trace.ContextWithSpanContext(context.Background(), validSpanContext(t)),
			wantTrace:   true,
			wantTraceID: "0102030405060708090a0b0c0d0e0f10",
			wantSpanID:  "1112131415161718",
		},
		{
			name:      "no span context omits trace fields",
			ctx:       context.Background(),
			wantTrace: false,
		},
		{
			name:      "invalid (zero) span context omits trace fields",
			ctx:       trace.ContextWithSpanContext(context.Background(), trace.SpanContext{}),
			wantTrace: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			With(tc.ctx).Info("event")
			m := decode(t, buf.Bytes())

			// Fixed fields always present regardless of trace.
			if m["service"] != "demo" {
				t.Errorf("service = %v, want demo", m["service"])
			}

			tid, hasTID := m["trace_id"]
			sid, hasSID := m["span_id"]
			if !tc.wantTrace {
				if hasTID || hasSID {
					t.Fatalf("did not expect trace fields, got trace_id=%v span_id=%v", tid, sid)
				}
				return
			}
			if !hasTID || !hasSID {
				t.Fatalf("expected trace fields, got %v", m)
			}
			if tid != tc.wantTraceID {
				t.Errorf("trace_id = %v, want %v", tid, tc.wantTraceID)
			}
			if sid != tc.wantSpanID {
				t.Errorf("span_id = %v, want %v", sid, tc.wantSpanID)
			}
			// trace_id is a 16-byte (32 hex char) ID; span_id is 8 bytes (16 hex chars).
			if s, _ := tid.(string); len(s) != 32 {
				t.Errorf("trace_id %q length = %d, want 32 hex chars", tid, len(s))
			}
			if s, _ := sid.(string); len(s) != 16 {
				t.Errorf("span_id %q length = %d, want 16 hex chars", sid, len(s))
			}
		})
	}
}

func TestKratosAdapter(t *testing.T) {
	tests := []struct {
		name      string
		level     log.Level
		keyvals   []any
		wantLevel string
		wantMsg   string
		wantPairs map[string]any
	}{
		{
			name:      "info with even keyvals",
			level:     log.LevelInfo,
			keyvals:   []any{"k1", "v1", "n", float64(2)},
			wantLevel: "INFO",
			wantMsg:   "INFO",
			wantPairs: map[string]any{"k1": "v1", "n": float64(2)},
		},
		{
			name:      "error level maps to ERROR",
			level:     log.LevelError,
			keyvals:   []any{"reason", "boom"},
			wantLevel: "ERROR",
			wantMsg:   "ERROR",
			wantPairs: map[string]any{"reason": "boom"},
		},
		{
			name:      "odd keyvals keeps dangling key with marker",
			level:     log.LevelWarn,
			keyvals:   []any{"orphan"},
			wantLevel: "WARN",
			wantMsg:   "WARN",
			wantPairs: map[string]any{"orphan": "!MISSING"},
		},
		{
			name:      "non-string key becomes BADKEY",
			level:     log.LevelInfo,
			keyvals:   []any{123, "v"},
			wantLevel: "INFO",
			wantMsg:   "INFO",
			wantPairs: map[string]any{"!BADKEY": "v"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			adapter := KratosAdapter(newHandler(&buf, "debug", "demo", "1.0.0", "test"))
			if err := adapter.Log(tc.level, tc.keyvals...); err != nil {
				t.Fatalf("Log returned error: %v", err)
			}
			m := decode(t, buf.Bytes())
			if m["level"] != tc.wantLevel {
				t.Errorf("level = %v, want %v", m["level"], tc.wantLevel)
			}
			if m["msg"] != tc.wantMsg {
				t.Errorf("msg = %v, want %v", m["msg"], tc.wantMsg)
			}
			for k, want := range tc.wantPairs {
				if got := m[k]; got != want {
					t.Errorf("pair %q = %v (%T), want %v (%T)", k, got, got, want, want)
				}
			}
		})
	}
}

func TestKratosAdapterImplementsInterface(t *testing.T) {
	// Compile-time + runtime check that the adapter satisfies kratos log.Logger.
	_ = log.Logger(KratosAdapter(slog.Default()))
}

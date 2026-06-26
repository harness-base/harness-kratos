// Package logx provides the service's structured logging: a slog JSON logger
// with fixed service/version/env fields, a bridge to Kratos's log.Logger
// interface, and trace-aware enrichment (trace_id / span_id) pulled from an
// OpenTelemetry span context.
//
// Single source of structure: every log line is JSON on stdout so the platform
// can ship it as-is; trace_id ties a line back to its distributed trace (AC5).
package logx

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/trace"
)

// Default is the package-level logger used by With when no logger is threaded
// explicitly. New sets it so application wiring (New once at boot) makes
// With(ctx) usable everywhere. It starts as a stdout JSON logger at info so the
// package is safe to use before New runs (e.g. in tests of other packages).
var Default = newHandler(os.Stdout, "info", "", "", "")

// New builds the application's root slog.Logger: a JSON handler writing to
// os.Stdout at the given level, with fixed service/version/env fields attached
// to every record. It also installs the result as the package Default so
// With(ctx) enriches the same logger. Call once during startup.
func New(level string, svc, ver, env string) *slog.Logger {
	l := newHandler(os.Stdout, level, svc, ver, env)
	Default = l
	return l
}

// newHandler is the testable core of New: it writes to an arbitrary io.Writer
// so tests can capture output in a buffer. Production code goes through New
// (os.Stdout); tests pass their own writer.
func newHandler(w io.Writer, level, svc, ver, env string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)})
	l := slog.New(h)
	// Only attach fields that were provided, so the bare Default (empty svc/ver/env)
	// doesn't emit empty strings.
	var attrs []any
	if svc != "" {
		attrs = append(attrs, slog.String("service", svc))
	}
	if ver != "" {
		attrs = append(attrs, slog.String("version", ver))
	}
	if env != "" {
		attrs = append(attrs, slog.String("env", env))
	}
	if len(attrs) > 0 {
		l = l.With(attrs...)
	}
	return l
}

// parseLevel maps a textual level (case-insensitive) to a slog.Level,
// defaulting to Info for unknown/empty input.
func parseLevel(level string) slog.Level {
	var l slog.Level
	// slog.Level.UnmarshalText accepts "DEBUG"/"INFO"/"WARN"/"ERROR"
	// (case-insensitive) and "INFO+2"-style offsets.
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}
	return l
}

// With returns a logger derived from the package Default, enriched with
// trace_id and span_id when ctx carries a valid OpenTelemetry span context.
// Without a valid span it returns the base logger unchanged (no trace fields),
// so callers can always use With(ctx) regardless of whether a span is active.
func With(ctx context.Context) *slog.Logger {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return Default
	}
	return Default.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)
}

// KratosAdapter adapts a slog.Logger to Kratos's log.Logger interface so the
// Kratos framework, middleware and helpers all emit through the same structured
// pipeline. Kratos passes alternating key/value pairs; this bridges them to
// slog attributes and maps Kratos levels onto slog levels.
func KratosAdapter(l *slog.Logger) log.Logger {
	return &kratosLogger{l: l}
}

type kratosLogger struct {
	l *slog.Logger
}

// Log implements log.Logger. keyvals is a flat, even-length sequence of
// key/value pairs; an odd trailing key gets a placeholder value so nothing is
// silently dropped.
func (k *kratosLogger) Log(level log.Level, keyvals ...any) error {
	lvl := toSlogLevel(level)
	if !k.l.Enabled(context.Background(), lvl) {
		return nil
	}
	attrs := make([]slog.Attr, 0, len(keyvals)/2+1)
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = "!BADKEY"
		}
		if i+1 < len(keyvals) {
			attrs = append(attrs, slog.Any(key, keyvals[i+1]))
		} else {
			// Odd number of keyvals: keep the dangling key with an explicit marker.
			attrs = append(attrs, slog.Any(key, "!MISSING"))
		}
	}
	// Use the Kratos level's String() as the message so the line is self-describing
	// even when the structured level field is filtered out downstream.
	k.l.LogAttrs(context.Background(), lvl, level.String(), attrs...)
	return nil
}

// toSlogLevel maps Kratos log levels onto slog levels. Kratos Fatal has no slog
// equivalent (slog tops out at Error), so it maps to Error+4 to remain distinct
// and above Error.
func toSlogLevel(level log.Level) slog.Level {
	switch level {
	case log.LevelDebug:
		return slog.LevelDebug
	case log.LevelInfo:
		return slog.LevelInfo
	case log.LevelWarn:
		return slog.LevelWarn
	case log.LevelError:
		return slog.LevelError
	case log.LevelFatal:
		return slog.LevelError + 4
	default:
		return slog.LevelInfo
	}
}

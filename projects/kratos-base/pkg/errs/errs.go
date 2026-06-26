// Package errs centralizes the demo domain's error constructors.
//
// 它**复用** protoc-gen-go-errors 在 api/demo/v1 生成的构造器（Error*）与判定（Is*），
// 只在其上补 WithCause 包装与统一的默认消息——不手写魔法字符串重复 reason。
// 这样 reason 字符串、HTTP code、Is* 判定全部来自同一份 proto 契约（单一事实源）。
//
// 映射（见 api/demo/v1/error_reason.proto）：
//
//	DB_UNAVAILABLE   → HTTP 503
//	NOT_FOUND        → HTTP 404
//	INVALID_ARGUMENT → HTTP 400
package errs

import (
	"context"
	"errors"

	kerrors "github.com/go-kratos/kratos/v2/errors"

	v1 "github.com/z-mate/kratos-base/api/demo/v1"
)

// DBUnavailable reports that a backing datastore (e.g. PostgreSQL) is
// unreachable. reason "DB_UNAVAILABLE", HTTP 503. cause（底层驱动/连接错误）通过
// WithCause 挂在错误链上，便于日志/排查，但不改变对外暴露的 reason/code。
//
// cause 可为 nil（例如懒加载尚未连接、无底层错误）。
func DBUnavailable(cause error) error {
	return v1.ErrorDbUnavailable("database unavailable").WithCause(cause)
}

// NotFound reports that the requested resource does not exist.
// reason "NOT_FOUND", HTTP 404. format/args 描述缺失的资源。
func NotFound(format string, args ...any) error {
	return v1.ErrorNotFound(format, args...)
}

// InvalidArgument reports a malformed or out-of-range caller argument.
// reason "INVALID_ARGUMENT", HTTP 400. format/args 描述非法参数。
func InvalidArgument(format string, args ...any) error {
	return v1.ErrorInvalidArgument(format, args...)
}

// 复用生成的判定，供调用方按 reason+code 判别错误（支持错误链）。
// 直接转发 protoc-gen-go-errors 生成的 Is*，避免在别处重复 reason 字符串。
var (
	// IsDBUnavailable reports whether err is (or wraps) a DB_UNAVAILABLE error.
	IsDBUnavailable = v1.IsDbUnavailable
	// IsNotFound reports whether err is (or wraps) a NOT_FOUND error.
	IsNotFound = v1.IsNotFound
	// IsInvalidArgument reports whether err is (or wraps) an INVALID_ARGUMENT error.
	IsInvalidArgument = v1.IsInvalidArgument
)

// FromError converts any error into a *kratos errors.Error (supports wrapped
// errors), exposing Code/Reason/Metadata. Thin re-export of kratos errors.FromError
// so callers depend on this package rather than reaching for kratos directly.
func FromError(err error) *kerrors.Error { return kerrors.FromError(err) }

// IsCallerCanceled reports whether err (against the caller-supplied ctx) is the
// result of the *caller* explicitly cancelling the request — e.g. an HTTP client
// disconnecting — rather than a backend fault.
//
// Why this matters for circuit breakers: a repo/publisher trips the breaker
// (MarkFailed) on backend errors so a sick datastore is shed quickly. But a
// burst of clients hanging up mid-request is NOT a backend fault; counting those
// as failures would open the breaker against a perfectly healthy backend (R10F4).
// Callers therefore consult IsCallerCanceled before MarkFailed and, when it is
// true, neither mark failure nor success — the request simply never produced a
// verdict about backend health.
//
// It returns true when EITHER:
//   - err's chain contains context.Canceled (errors.Is), OR
//   - the caller's ctx was itself cancelled (ctx.Err() == context.Canceled).
//
// It deliberately returns false for context.DeadlineExceeded: a deadline blown
// by a slow backend IS a backend fault and must still trip the breaker. Pass the
// CALLER's ctx here, never an internally-derived ctx with a tighter timeout —
// that derived ctx's DeadlineExceeded is our own bound firing on a slow backend,
// which must remain a failure.
func IsCallerCanceled(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	return ctx != nil && ctx.Err() == context.Canceled
}

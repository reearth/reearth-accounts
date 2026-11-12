// Package applog is the only way accounts writes logs. Every entry goes
// through the request-scoped logger when the context carries one, and every
// field comes from the fixed constructors in fields.go (ADR-0003 decision 9).
package applog

import (
	"context"

	"github.com/reearth/reearthx/log"
	"go.opentelemetry.io/otel/trace"
)

type level int

const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelError
	levelFatal
	levelPanic
)

// With returns a context whose logger adds fields to every entry written
// through it.
func With(ctx context.Context, fields ...Field) context.Context {
	return log.ContextWith(ctx, base(ctx).With(kv(fields)...))
}

// base is the logger for ctx without caller skip, so every exported function
// below adds exactly one frame.
func base(ctx context.Context) *log.Logger {
	if l := log.GetLoggerFromContext(ctx); l != nil {
		return l
	}
	// The reearthx global logger already skips one frame for its own wrappers.
	return log.GetLoggerFromContextOrDefault(context.Background()).AddCallerSkip(-1)
}

func kv(fields []Field) []any {
	res := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		res = append(res, f.key, f.value)
	}
	return res
}

// traceFields links the entry to the current span. With a GCP project known,
// it uses the Cloud Logging keys so the entry shows under its trace.
func traceFields(ctx context.Context) []any {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	if t := log.GoogleTraceFromTraceID(sc.TraceID().String(), ""); t != "" {
		return []any{
			"logging.googleapis.com/trace", t,
			"logging.googleapis.com/spanId", sc.SpanID().String(),
			"logging.googleapis.com/trace_sampled", sc.IsSampled(),
		}
	}
	return []any{"trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String()}
}

func write(ctx context.Context, depth int, lv level, msg string, fields []Field) {
	if ctx == nil {
		ctx = context.Background()
	}
	l := base(ctx).AddCallerSkip(depth + 2)
	args := append(kv(fields), traceFields(ctx)...)
	switch lv {
	case levelDebug:
		l.Debugw(msg, args...)
	case levelInfo:
		l.Infow(msg, args...)
	case levelWarn:
		l.Warnw(msg, args...)
	case levelError:
		l.Errorw(msg, args...)
	case levelFatal:
		l.Fatalw(msg, args...)
	case levelPanic:
		l.Panicw(msg, args...)
	}
}

func Debug(ctx context.Context, msg string, fields ...Field) { write(ctx, 0, levelDebug, msg, fields) }
func Info(ctx context.Context, msg string, fields ...Field)  { write(ctx, 0, levelInfo, msg, fields) }
func Warn(ctx context.Context, msg string, fields ...Field)  { write(ctx, 0, levelWarn, msg, fields) }
func Error(ctx context.Context, msg string, fields ...Field) { write(ctx, 0, levelError, msg, fields) }
func Fatal(ctx context.Context, msg string, fields ...Field) { write(ctx, 0, levelFatal, msg, fields) }
func Panic(ctx context.Context, msg string, fields ...Field) { write(ctx, 0, levelPanic, msg, fields) }

// WarnDepth and ErrorDepth attribute the entry to the caller depth frames
// above the function that calls them.
func WarnDepth(ctx context.Context, depth int, msg string, fields ...Field) {
	write(ctx, depth, levelWarn, msg, fields)
}

func ErrorDepth(ctx context.Context, depth int, msg string, fields ...Field) {
	write(ctx, depth, levelError, msg, fields)
}

func ErrorWithCallerLogging(ctx context.Context, msg string, err error) error {
	ErrorDepth(ctx, 1, msg, Err(err))
	return err
}

func WarnWithCallerLogging(ctx context.Context, msg string) {
	WarnDepth(ctx, 1, msg)
}

package log

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// Logger is the application logger type (zerolog).
type Logger = zerolog.Logger

// WithFields appends key-value pairs to a zerolog event.
func WithFields(evt *zerolog.Event, fields ...interface{}) *zerolog.Event {
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}

		switch v := fields[i+1].(type) {
		case string:
			evt = evt.Str(key, v)
		case int:
			evt = evt.Int(key, v)
		case int64:
			evt = evt.Int64(key, v)
		case bool:
			evt = evt.Bool(key, v)
		case error:
			evt = evt.Err(v)
		case fmt.Stringer:
			evt = evt.Str(key, v.String())
		default:
			evt = evt.Interface(key, v)
		}
	}

	return evt
}

// InfoCtx logs at info level using logger from ctx.
func InfoCtx(ctx context.Context, msg string, fields ...interface{}) {
	l := FromContext(ctx)
	WithFields(l.Info(), fields...).Msg(msg)
}

// ErrorCtx logs at error level using logger from ctx.
func ErrorCtx(ctx context.Context, msg string, fields ...interface{}) {
	l := FromContext(ctx)
	WithFields(l.Error(), fields...).Msg(msg)
}

// WarnCtx logs at warn level using logger from ctx.
func WarnCtx(ctx context.Context, msg string, fields ...interface{}) {
	l := FromContext(ctx)
	WithFields(l.Warn(), fields...).Msg(msg)
}

// DebugCtx logs at debug level using logger from ctx.
func DebugCtx(ctx context.Context, msg string, fields ...interface{}) {
	l := FromContext(ctx)
	WithFields(l.Debug(), fields...).Msg(msg)
}

// Info logs at info level using global logger.
func Info(msg string, fields ...interface{}) {
	InfoCtx(context.Background(), msg, fields...)
}

// Error logs at error level using global logger.
func Error(msg string, fields ...interface{}) {
	ErrorCtx(context.Background(), msg, fields...)
}

// Warn logs at warn level using global logger.
func Warn(msg string, fields ...interface{}) {
	WarnCtx(context.Background(), msg, fields...)
}

// Debug logs at debug level using global logger.
func Debug(msg string, fields ...interface{}) {
	DebugCtx(context.Background(), msg, fields...)
}

package log

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey struct{}

// Config configures the root logger.
type Config struct {
	Env         string // local | dev | staging | prod
	ServiceName string
	Level       string // debug | info | warn | error
	// VectorURL sends JSON logs to Vector http_server (VictoriaLogs local stack).
	// Empty in local/dev → http://127.0.0.1:8687; "disabled" turns forwarding off.
	VectorURL string
}

type options struct {
	writer io.Writer
}

// Option configures Setup.
type Option func(*options)

// WithWriter overrides log output (tests).
func WithWriter(w io.Writer) Option {
	return func(o *options) {
		o.writer = w
	}
}

var (
	global   zerolog.Logger
	globalMu sync.RWMutex
)

func init() {
	global = zerolog.Nop()
}

// Setup creates the root logger and stores it as global fallback.
func Setup(cfg Config, opts ...Option) (zerolog.Logger, error) {
	o := options{writer: os.Stdout}
	for _, opt := range opts {
		opt(&o)
	}

	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}

	out := buildOutput(o.writer, cfg)

	l := zerolog.New(out).
		Level(level).
		With().
		Timestamp().
		Str("service", cfg.ServiceName).
		Logger()

	globalMu.Lock()
	global = l
	globalMu.Unlock()

	return l, nil
}

func buildOutput(base io.Writer, cfg Config) io.Writer {
	writers := make([]io.Writer, 0, 2)

	if isConsoleEnv(cfg.Env) {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        base,
			TimeFormat: time.RFC3339,
		})
	} else {
		writers = append(writers, base)
	}

	if url := resolveVectorURL(cfg); url != "" {
		writers = append(writers, newVectorHTTPWriter(url))
	}

	if len(writers) == 1 {
		return writers[0]
	}

	return zerolog.MultiLevelWriter(writers...)
}

func isConsoleEnv(env string) bool {
	switch strings.ToLower(env) {
	case "local", "dev", "development":
		return true
	default:
		return false
	}
}

// WithContext stores logger in ctx.
func WithContext(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns logger from ctx or global/nop fallback.
func FromContext(ctx context.Context) zerolog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(zerolog.Logger); ok {
			return l
		}
	}
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

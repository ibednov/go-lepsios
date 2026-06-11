package log_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ibednov/go-lepsios/log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestSetupConsole(t *testing.T) {
	var buf bytes.Buffer
	l, err := log.Setup(log.Config{
		Env:         "local",
		ServiceName: "test",
		Level:       "debug",
	}, log.WithWriter(&buf))
	require.NoError(t, err)

	l.Info().Msg("hello")
	require.Contains(t, buf.String(), "hello")
	require.Contains(t, buf.String(), "test")
}

func TestFromContextFallback(t *testing.T) {
	var buf bytes.Buffer
	_, err := log.Setup(log.Config{
		Env:         "prod",
		ServiceName: "svc",
		Level:       "info",
	}, log.WithWriter(&buf))
	require.NoError(t, err)

	l := log.FromContext(context.Background())
	l.Info().Msg("global")
	require.True(t, strings.Contains(buf.String(), "global"))
}

func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	l, err := log.Setup(log.Config{
		Env:         "prod",
		ServiceName: "svc",
		Level:       "info",
	}, log.WithWriter(&buf))
	require.NoError(t, err)

	child := l.With().Str("trace", "abc").Logger()
	ctx := log.WithContext(context.Background(), child)
	fromCtx := log.FromContext(ctx)
	fromCtx.Info().Msg("ctx")
	require.Contains(t, buf.String(), "abc")
}

func TestFromContextEmptyReturnsNopOrGlobal(t *testing.T) {
	l := log.FromContext(context.Background())
	require.NotPanics(t, func() {
		l.Log().Msg("noop")
	})
	_ = zerolog.Nop() // ensure import used
}

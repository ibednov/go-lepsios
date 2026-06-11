package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibednov/go-lepsios/httpx"
	"github.com/ibednov/go-lepsios/i18n"
	"github.com/ibednov/go-lepsios/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProvideEngineAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bundle, err := i18n.NewBundle("en")
	require.NoError(t, err)

	logger, err := log.Setup(log.Config{Env: "local", ServiceName: "test", Level: "error"})
	require.NoError(t, err)

	engine := httpx.ProvideEngine(httpx.EngineConfig{
		ServiceName: "test",
	}, bundle, logger)

	httpx.RegisterHealth(engine,
		httpx.NewChecker("live", func(ctx context.Context) error { return nil }),
		httpx.NewChecker("db", func(ctx context.Context) error { return nil }),
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

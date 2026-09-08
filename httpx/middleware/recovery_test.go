package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/httpx/middleware"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/stretchr/testify/require"
)

func TestRecoveryReturnsInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.GET("/boom", func(c *gin.Context) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var body response.ErrorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "INTERNAL_ERROR", body.Error)
	require.Equal(t, "internal server error", body.Message)
	require.Equal(t, http.StatusInternalServerError, body.Code)
}

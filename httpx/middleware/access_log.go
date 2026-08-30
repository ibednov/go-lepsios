package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Gin context keys that AccessLog optionally reads after c.Next().
const (
	CtxKeyUserID    = "log_user_id"
	CtxKeyRequestID = "request_id"
)

// AccessLog logs HTTP requests (skips configured path prefixes).
func AccessLog(logger zerolog.Logger, skipPaths ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		for _, p := range skipPaths {
			if strings.HasPrefix(path, p) {
				return
			}
		}

		evt := logger.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Str("route", routeArea(path)).
			Int("status", c.Writer.Status()).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Int("bytes_out", c.Writer.Size()).
			Str("ip", c.ClientIP())

		if rid := requestID(c); rid != "" {
			evt = evt.Str("request_id", rid)
		}
		if uid, ok := c.Get(CtxKeyUserID); ok {
			if s, ok := uid.(string); ok && s != "" {
				evt = evt.Str("user_id", s)
			}
		}
		if len(c.Errors) > 0 {
			evt = evt.Str("error", c.Errors.String())
		}

		evt.Msg("http.request")
	}
}

func requestID(c *gin.Context) string {
	if v := c.GetHeader("X-Request-ID"); v != "" {
		return v
	}
	if v := c.Writer.Header().Get("X-Request-ID"); v != "" {
		return v
	}
	if v, ok := c.Get(CtxKeyRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// routeArea maps /api/v1/<module>/... → module for dashboard grouping.
func routeArea(path string) string {
	const prefix = "/api/v1/"
	if !strings.HasPrefix(path, prefix) {
		if path == "/" || path == "" {
			return "root"
		}
		return "other"
	}
	rest := strings.TrimPrefix(path, prefix)
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	if rest == "" {
		return "api"
	}
	return rest
}

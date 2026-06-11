package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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

		logger.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Str("ip", c.ClientIP()).
			Msg("http.request")
	}
}

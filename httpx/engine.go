package httpx

import (
	"github.com/ibednov/go-lepsios/i18n"
	"github.com/ibednov/go-lepsios/httpx/middleware"
	"github.com/ibednov/go-lepsios/log"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// EngineConfig configures the Gin engine.
type EngineConfig struct {
	Mode         string
	CORSOrigins  []string
	ServiceName  string
	SkipLogPaths []string
	LocaleFallback string
}

// ProvideEngine builds a Gin engine with global middleware.
func ProvideEngine(cfg EngineConfig, bundle *i18n.Bundle, logger zerolog.Logger) *gin.Engine {
	if cfg.Mode != "" {
		gin.SetMode(cfg.Mode)
	}

	r := gin.New()
	skip := append([]string{"/health", "/health/live", "/health/ready"}, cfg.SkipLogPaths...)

	r.Use(middleware.Recovery())
	r.Use(middleware.TraceID())
	r.Use(middleware.Locale(bundle, cfg.LocaleFallback))
	r.Use(middleware.AccessLog(logger, skip...))
	r.Use(middleware.CORS(cfg.CORSOrigins))

	return r
}

// WithRequestLogger attaches zerolog logger to request context (optional helper).
func WithRequestLogger(c *gin.Context, logger zerolog.Logger) {
	ctx := log.WithContext(c.Request.Context(), logger)
	c.Request = c.Request.WithContext(ctx)
}

package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker performs a readiness/liveness probe.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type checkerFunc struct {
	name string
	fn   func(ctx context.Context) error
}

// NewChecker wraps a check function.
func NewChecker(name string, fn func(ctx context.Context) error) Checker {
	return checkerFunc{name: name, fn: fn}
}

func (c checkerFunc) Name() string { return c.name }

func (c checkerFunc) Check(ctx context.Context) error {
	if c.fn == nil {
		return nil
	}
	return c.fn(ctx)
}

// RegisterHealth registers /health, /health/live, /health/ready.
func RegisterHealth(r *gin.Engine, liveness, readiness Checker) {
	r.GET("/health/live", func(c *gin.Context) {
		if err := runCheck(c.Request.Context(), liveness); err != nil {
			c.String(http.StatusServiceUnavailable, "Not alive: %v", err)
			return
		}
		c.String(http.StatusOK, "Alive")
	})

	r.GET("/health/ready", func(c *gin.Context) {
		if err := runCheck(c.Request.Context(), readiness); err != nil {
			c.String(http.StatusServiceUnavailable, "Not ready: %v", err)
			return
		}
		c.String(http.StatusOK, "Ready")
	})

	r.GET("/health", func(c *gin.Context) {
		status := gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"services":  gin.H{},
		}
		services := status["services"].(gin.H)

		if err := runCheck(c.Request.Context(), liveness); err != nil {
			status["status"] = "degraded"
			services["liveness"] = err.Error()
		} else {
			services["liveness"] = "healthy"
		}

		if readiness != nil {
			if err := runCheck(c.Request.Context(), readiness); err != nil {
				status["status"] = "degraded"
				services[readiness.Name()] = err.Error()
			} else {
				services[readiness.Name()] = "healthy"
			}
		}

		c.JSON(http.StatusOK, status)
	})
}

func runCheck(ctx context.Context, chk Checker) error {
	if chk == nil {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return chk.Check(cctx)
}

package httpx

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterMetrics exposes Prometheus metrics at GET /metrics.
func RegisterMetrics(r gin.IRoutes) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

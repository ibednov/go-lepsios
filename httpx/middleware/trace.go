package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const traceHeader = "X-Trace-ID"

// TraceID ensures every request has a trace id.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		trace := c.GetHeader(traceHeader)
		if trace == "" {
			trace = newTraceID()
		}
		c.Header(traceHeader, trace)
		c.Set(traceHeader, trace)
		c.Next()
	}
}

func newTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

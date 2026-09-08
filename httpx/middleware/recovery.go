package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/log"
)

// Recovery recovers panics, logs stack, and returns 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				path := ""
				method := ""
				ctx := context.Background()
				if c.Request != nil {
					path = c.Request.URL.Path
					method = c.Request.Method
					ctx = c.Request.Context()
				}
				log.ErrorCtx(ctx, "http.panic",
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()),
					"path", path,
					"method", method,
				)
				response.Internal(c, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}

package middleware

import (
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/gin-gonic/gin"
)

// Recovery recovers panics and returns 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				response.Internal(c, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}

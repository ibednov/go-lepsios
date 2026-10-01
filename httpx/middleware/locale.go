package middleware

import (
	"github.com/ibednov/go-lepsios/i18n"
	"github.com/gin-gonic/gin"
)

// Locale resolves Accept-Language and stores locale + localizer in request context.
func Locale(bundle *i18n.Bundle, fallback string) gin.HandlerFunc {
	if fallback == "" {
		fallback = "en"
	}
	return func(c *gin.Context) {
		acceptLanguage := c.GetHeader("Accept-Language")
		if acceptLanguage == "" {
			acceptLanguage = fallback
		}
		loc := bundle.Localizer(acceptLanguage)
		ctx := i18n.SetLocale(c.Request.Context(), loc.Locale())
		ctx = i18n.SetLocalizer(ctx, loc)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

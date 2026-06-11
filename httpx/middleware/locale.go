package middleware

import (
	"strings"

	"github.com/ibednov/go-lepsios/i18n"
	"github.com/gin-gonic/gin"
)

// Locale resolves Accept-Language and stores locale + localizer in request context.
func Locale(bundle *i18n.Bundle, fallback string) gin.HandlerFunc {
	if fallback == "" {
		fallback = "en"
	}
	return func(c *gin.Context) {
		lang := c.GetHeader("Accept-Language")
		if lang == "" {
			lang = fallback
		}
		lang = strings.Split(lang, ",")[0]
		lang = strings.TrimSpace(strings.Split(lang, ";")[0])

		loc := bundle.Localizer(lang)
		ctx := i18n.SetLocale(c.Request.Context(), lang)
		ctx = i18n.SetLocalizer(ctx, loc)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

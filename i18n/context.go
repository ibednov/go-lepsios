package i18n

import "context"

// SetLocale stores locale in ctx.
func SetLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, ctxLocaleKey{}, locale)
}

// SetLocalizer stores localizer in ctx.
func SetLocalizer(ctx context.Context, l *Localizer) context.Context {
	return context.WithValue(ctx, ctxLocalizerKey{}, l)
}

// LocaleFromContext returns locale from ctx.
func LocaleFromContext(ctx context.Context) string {
	if ctx == nil {
		return "en"
	}
	if v, ok := ctx.Value(ctxLocaleKey{}).(string); ok && v != "" {
		return v
	}
	return "en"
}

// LocalizerFromContext returns localizer from ctx.
func LocalizerFromContext(ctx context.Context) *Localizer {
	if ctx == nil {
		return &Localizer{}
	}
	if l, ok := ctx.Value(ctxLocalizerKey{}).(*Localizer); ok && l != nil {
		return l
	}
	return &Localizer{}
}

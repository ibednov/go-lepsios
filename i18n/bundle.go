package i18n

import (
	"encoding/json"
	"fmt"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type ctxLocaleKey struct{}
type ctxLocalizerKey struct{}

// Bundle holds loaded translation messages.
type Bundle struct {
	bundle        *goi18n.Bundle
	defaultLocale string
}

// Option configures Bundle.
type Option func(*Bundle)

// NewBundle creates a bundle with default locale.
func NewBundle(defaultLocale string, _ ...Option) (*Bundle, error) {
	tag, err := language.Parse(defaultLocale)
	if err != nil {
		return nil, fmt.Errorf("i18n: invalid default locale: %w", err)
	}
	b := goi18n.NewBundle(tag)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	return &Bundle{bundle: b, defaultLocale: tag.String()}, nil
}

// LoadMessages loads JSON messages for a locale.
func (b *Bundle) LoadMessages(locale string, data []byte) error {
	_, err := b.bundle.ParseMessageFileBytes(data, locale+".json")
	return err
}

// Localizer returns a localizer for locale (falls back to default).
type Localizer struct {
	inner *goi18n.Localizer
	locale string
}

// LocalizerFor creates a localizer for locale string.
func (b *Bundle) Localizer(locale string) *Localizer {
	if b == nil || b.bundle == nil {
		return &Localizer{}
	}
	tags := b.bundle.LanguageTags()
	defaultTag, err := language.Parse(b.defaultLocale)
	if err == nil {
		ordered := []language.Tag{defaultTag}
		for _, tag := range tags {
			if tag != defaultTag {
				ordered = append(ordered, tag)
			}
		}
		tags = ordered
	}
	matcher := language.NewMatcher(tags)
	preferred, _, parseErr := language.ParseAcceptLanguage(locale)
	if parseErr != nil || len(preferred) == 0 {
		preferred = []language.Tag{language.Make(locale)}
	}
	tag, _, _ := matcher.Match(preferred...)
	return &Localizer{inner: goi18n.NewLocalizer(b.bundle, tag.String()), locale: tag.String()}
}

// Locale returns the selected supported locale.
func (l *Localizer) Locale() string {
	if l == nil || l.locale == "" {
		return "en"
	}
	return l.locale
}

// T translates a message key.
func (l *Localizer) T(key string, args ...any) string {
	if l == nil || l.inner == nil {
		return key
	}
	var templateData any
	if len(args) == 1 {
		templateData = args[0]
	} else if len(args) > 1 {
		templateData = args
	}
	msg, err := l.inner.Localize(&goi18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil || msg == "" {
		return key
	}
	return msg
}

// LocalizedString is a JSONB-friendly localized map.
type LocalizedString map[string]string

// Get returns value for locale with fallback to en then any value.
func (ls LocalizedString) Get(locale string) string {
	if ls == nil {
		return ""
	}
	if v, ok := ls[locale]; ok && v != "" {
		return v
	}
	if v, ok := ls["en"]; ok && v != "" {
		return v
	}
	for _, v := range ls {
		return v
	}
	return ""
}

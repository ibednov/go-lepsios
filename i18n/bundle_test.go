package i18n_test

import (
	"context"
	"testing"

	"github.com/ibednov/go-lepsios/i18n"
	"github.com/stretchr/testify/require"
)

const enJSON = `[
  {"id": "errors.auth.not_found", "translation": "User not found"}
]`

const ruJSON = `[
  {"id": "errors.auth.not_found", "translation": "Пользователь не найден"}
]`

func TestBundleLocalize(t *testing.T) {
	b, err := i18n.NewBundle("en")
	require.NoError(t, err)
	require.NoError(t, b.LoadMessages("en", []byte(enJSON)))
	require.NoError(t, b.LoadMessages("ru", []byte(ruJSON)))

	loc := b.Localizer("ru")
	require.Equal(t, "Пользователь не найден", loc.T("errors.auth.not_found"))
	require.Equal(t, "errors.missing", loc.T("errors.missing"))
}

func TestBundleLocalizerMatchesAcceptLanguagePriorities(t *testing.T) {
	b, err := i18n.NewBundle("ru")
	require.NoError(t, err)
	require.NoError(t, b.LoadMessages("en", []byte(enJSON)))
	require.NoError(t, b.LoadMessages("ru", []byte(ruJSON)))

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "regional Russian header with weighted fallbacks",
			header: "ru,en-US;q=0.9,en;q=0.8,ru-BY;q=0.7,ru-RU;q=0.6",
			want:   "ru",
		},
		{
			name:   "higher quality English beats earlier Russian",
			header: "ru;q=0.5,en;q=0.9",
			want:   "en",
		},
		{
			name:   "unsupported language falls back to configured default",
			header: "fr-CA",
			want:   "ru",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, b.Localizer(tt.header).Locale())
		})
	}
}

func TestLocalizedString(t *testing.T) {
	ls := i18n.LocalizedString{"en": "Hello", "ru": "Привет"}
	require.Equal(t, "Привет", ls.Get("ru"))
	require.Equal(t, "Hello", ls.Get("de"))
}

func TestContextLocale(t *testing.T) {
	ctx := i18n.SetLocale(context.Background(), "ru")
	require.Equal(t, "ru", i18n.LocaleFromContext(ctx))
}

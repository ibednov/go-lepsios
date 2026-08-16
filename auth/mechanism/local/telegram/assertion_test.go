package telegram_test

import (
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/mechanism/local/telegram"
	"github.com/stretchr/testify/require"
)

func TestBotAssertionRoundTrip(t *testing.T) {
	token := "test-bot-token"
	in := telegram.NewAssertionNow(token, 42, 99)
	require.NoError(t, telegram.VerifyBotAssertion(token, in))

	bad := in
	bad.Assertion = "deadbeef"
	require.ErrorIs(t, telegram.VerifyBotAssertion(token, bad), telegram.ErrInvalidAssertion)

	expired := in
	expired.AuthDate = time.Now().Add(-10 * time.Minute).Unix()
	expired.Assertion = telegram.SignBotAssertion(token, expired.TelegramUserID, expired.TelegramChatID, expired.AuthDate)
	require.ErrorIs(t, telegram.VerifyBotAssertion(token, expired), telegram.ErrExpiredAssertion)
}

func TestNormalizePurposeAndCode(t *testing.T) {
	require.Equal(t, telegram.ChallengePurposeLogin, telegram.NormalizePurpose(""))
	require.Equal(t, telegram.ChallengePurposeLink, telegram.NormalizePurpose("link"))
	require.Equal(t, "", telegram.NormalizePurpose("x"))

	code, err := telegram.RandomCode(8)
	require.NoError(t, err)
	require.Len(t, code, 8)
}

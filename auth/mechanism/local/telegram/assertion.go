package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidAssertion = errors.New("invalid telegram bot assertion")
	ErrExpiredAssertion = errors.New("telegram bot assertion expired")
)

// BotIdentity is Telegram user data proven by the bot via HMAC assertion.
// Reusable across services that trust the same bot token.
type BotIdentity struct {
	TelegramUserID   int64  `json:"telegram_user_id"`
	TelegramChatID   int64  `json:"telegram_chat_id"`
	TelegramUsername string `json:"telegram_username"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	AuthDate         int64  `json:"auth_date"`
	Assertion        string `json:"assertion"`
}

const AssertionTTL = 5 * time.Minute

// SignBotAssertion builds HMAC-SHA256(bot_token, "user_id:chat_id:auth_date").
func SignBotAssertion(botToken string, telegramUserID, chatID, authDate int64) string {
	mac := hmac.New(sha256.New, []byte(botToken))
	_, _ = mac.Write([]byte(assertionPayload(telegramUserID, chatID, authDate)))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyBotAssertion checks HMAC and auth_date freshness.
func VerifyBotAssertion(botToken string, in BotIdentity) error {
	if botToken == "" {
		return ErrInvalidAssertion
	}
	if in.TelegramUserID == 0 || in.AuthDate == 0 || strings.TrimSpace(in.Assertion) == "" {
		return ErrInvalidAssertion
	}
	authAt := time.Unix(in.AuthDate, 0).UTC()
	now := time.Now().UTC()
	if authAt.After(now.Add(time.Minute)) || now.Sub(authAt) > AssertionTTL {
		return ErrExpiredAssertion
	}
	expected := SignBotAssertion(botToken, in.TelegramUserID, in.TelegramChatID, in.AuthDate)
	got := strings.ToLower(strings.TrimSpace(in.Assertion))
	if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(got)) {
		return ErrInvalidAssertion
	}
	return nil
}

// NewAssertionNow is a helper for bot workers.
func NewAssertionNow(botToken string, telegramUserID, chatID int64) BotIdentity {
	authDate := time.Now().UTC().Unix()
	return BotIdentity{
		TelegramUserID: telegramUserID,
		TelegramChatID: chatID,
		AuthDate:       authDate,
		Assertion:      SignBotAssertion(botToken, telegramUserID, chatID, authDate),
	}
}

func assertionPayload(telegramUserID, chatID, authDate int64) string {
	return strings.Join([]string{
		strconv.FormatInt(telegramUserID, 10),
		strconv.FormatInt(chatID, 10),
		strconv.FormatInt(authDate, 10),
	}, ":")
}

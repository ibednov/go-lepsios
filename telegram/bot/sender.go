package bot

import (
	"context"
	"net/http"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type senderAdapter struct {
	b *bot.Bot
}

func NewSender(token string) (Sender, error) {
	b, err := bot.New(token, bot.WithHTTPClient(15*time.Second, http.DefaultClient))
	if err != nil {
		return nil, err
	}
	return &senderAdapter{b: b}, nil
}

func (a *senderAdapter) SendText(ctx context.Context, chatID int64, text string) error {
	_, err := a.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func (a *senderAdapter) SendTextWithKeyboard(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) error {
	_, err := a.SendTextWithKeyboardID(ctx, chatID, text, keyboard)
	return err
}

func (a *senderAdapter) SendTextWithKeyboardID(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	return sendKeyboard(ctx, a.b, chatID, text, keyboard)
}

func (a *senderAdapter) EditText(ctx context.Context, chatID int64, messageID int, text string, keyboard *models.InlineKeyboardMarkup) error {
	return editText(ctx, a.b, chatID, messageID, text, keyboard)
}

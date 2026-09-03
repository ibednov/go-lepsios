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
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if keyboard != nil {
		params.ReplyMarkup = keyboard
	}
	_, err := a.b.SendMessage(ctx, params)
	return err
}

package bot

import (
	"context"
	"errors"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func sendKeyboard(ctx context.Context, b *bot.Bot, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if keyboard != nil {
		params.ReplyMarkup = keyboard
	}
	msg, err := b.SendMessage(ctx, params)
	if err != nil {
		return 0, err
	}
	if msg == nil {
		return 0, nil
	}
	return msg.ID, nil
}

func editText(ctx context.Context, b *bot.Bot, chatID int64, messageID int, text string, keyboard *models.InlineKeyboardMarkup) error {
	markup := keyboard
	if markup == nil {
		markup = &models.InlineKeyboardMarkup{}
	}
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: markup,
	})
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
		return ErrNotModified
	}
	return err
}

// IsNotModified reports edit no-ops.
func IsNotModified(err error) bool {
	return errors.Is(err, ErrNotModified)
}

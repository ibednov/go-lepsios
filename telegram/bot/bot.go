package bot

import (
	"context"
	"net/http"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type UpdateHandler func(ctx context.Context, update *models.Update) error

type Bot interface {
	Start(ctx context.Context) error
	Sender
	AnswerCallback(ctx context.Context, callbackID, text string) error
}

type Sender interface {
	SendText(ctx context.Context, chatID int64, text string) error
	SendTextWithKeyboard(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) error
}

type Config struct {
	Token              string
	LongPollTimeoutSec int
}

type adapter struct {
	b       *bot.Bot
	handler UpdateHandler
}

func New(cfg Config, handler UpdateHandler) (Bot, error) {
	timeout := cfg.LongPollTimeoutSec
	if timeout <= 0 {
		timeout = 30
	}

	a := &adapter{handler: handler}

	opts := []bot.Option{
		bot.WithDefaultHandler(a.onUpdate),
		bot.WithHTTPClient(time.Duration(timeout)*time.Second, http.DefaultClient),
		bot.WithNotAsyncHandlers(),
	}

	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		return nil, err
	}
	a.b = b
	return a, nil
}

func (a *adapter) onUpdate(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if a.handler == nil || update == nil {
		return
	}
	_ = a.handler(ctx, update)
}

func (a *adapter) Start(ctx context.Context) error {
	a.b.Start(ctx)
	return ctx.Err()
}

func (a *adapter) SendText(ctx context.Context, chatID int64, text string) error {
	_, err := a.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func (a *adapter) SendTextWithKeyboard(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) error {
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

func (a *adapter) AnswerCallback(ctx context.Context, callbackID, text string) error {
	_, err := a.b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	})
	return err
}

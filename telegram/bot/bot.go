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
	// SendTextWithKeyboardID is SendTextWithKeyboard plus the new message id.
	SendTextWithKeyboardID(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) (int, error)
	// EditText replaces text and the inline keyboard of a message this bot sent.
	// A nil keyboard clears the buttons. Same text and keyboard returns ErrNotModified.
	EditText(ctx context.Context, chatID int64, messageID int, text string, keyboard *models.InlineKeyboardMarkup) error
}

// ErrNotModified is returned when edit would not change the message.
var ErrNotModified = errNotModified("message is not modified")

type errNotModified string

func (e errNotModified) Error() string { return string(e) }

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
	_, err := a.SendTextWithKeyboardID(ctx, chatID, text, keyboard)
	return err
}

func (a *adapter) SendTextWithKeyboardID(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	return sendKeyboard(ctx, a.b, chatID, text, keyboard)
}

func (a *adapter) EditText(ctx context.Context, chatID int64, messageID int, text string, keyboard *models.InlineKeyboardMarkup) error {
	return editText(ctx, a.b, chatID, messageID, text, keyboard)
}

func (a *adapter) AnswerCallback(ctx context.Context, callbackID, text string) error {
	_, err := a.b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	})
	return err
}

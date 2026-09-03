package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
	"github.com/ibednov/go-lepsios/log"
	"github.com/ibednov/go-lepsios/telegram/bot"
)

type Identity struct {
	UserID string
	ChatID int64
}

type Directory interface {
	FindChatID(ctx context.Context, userID string) (chatID int64, ok bool, err error)
	ListChatIDs(ctx context.Context) ([]Identity, error)
}

type KeyboardFunc func(payload map[string]any) *models.InlineKeyboardMarkup

type Bridge struct {
	Sender   bot.Sender
	Dir      Directory
	Keyboard KeyboardFunc
}

func FormatLocalized(title, body map[string]string) string {
	t := PickLocalized(title)
	b := PickLocalized(body)
	switch {
	case t != "" && b != "":
		return fmt.Sprintf("%s\n\n%s", t, b)
	case t != "":
		return t
	default:
		return b
	}
}

func PickLocalized(m map[string]string) string {
	if m == nil {
		return ""
	}
	if v := strings.TrimSpace(m["ru"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(m["en"]); v != "" {
		return v
	}
	for _, v := range m {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (b *Bridge) PushUser(ctx context.Context, userID string, title, body map[string]string, payload map[string]any) bool {
	if b == nil || b.Sender == nil || b.Dir == nil {
		return false
	}
	chatID, ok, err := b.Dir.FindChatID(ctx, userID)
	if err != nil || !ok || chatID == 0 {
		return false
	}
	text := FormatLocalized(title, body)
	var kb *models.InlineKeyboardMarkup
	if b.Keyboard != nil {
		kb = b.Keyboard(payload)
	}
	if err := b.Sender.SendTextWithKeyboard(ctx, chatID, text, kb); err != nil {
		log.WarnCtx(ctx, "notifier.telegram.push.finished",
			"component", "notifier",
			"user_id", userID,
			"chat_id", chatID,
			"success", false,
			"error", err.Error(),
		)
		return false
	}
	log.InfoCtx(ctx, "notifier.telegram.push.finished",
		"component", "notifier",
		"user_id", userID,
		"chat_id", chatID,
		"success", true,
	)
	return true
}

func (b *Bridge) PushBroadcast(ctx context.Context, title, body map[string]string) (ok, fail int) {
	if b == nil || b.Sender == nil || b.Dir == nil {
		return 0, 0
	}
	rows, err := b.Dir.ListChatIDs(ctx)
	if err != nil {
		log.WarnCtx(ctx, "notifier.telegram.broadcast.finished",
			"component", "notifier",
			"success", false,
			"error", err.Error(),
		)
		return 0, 0
	}
	text := FormatLocalized(title, body)
	var kb *models.InlineKeyboardMarkup
	if b.Keyboard != nil {
		kb = b.Keyboard(nil)
	}
	for _, row := range rows {
		if row.ChatID == 0 {
			continue
		}
		if err := b.Sender.SendTextWithKeyboard(ctx, row.ChatID, text, kb); err != nil {
			fail++
			log.WarnCtx(ctx, "notifier.telegram.broadcast_send_failed",
				"component", "notifier",
				"user_id", row.UserID,
				"chat_id", row.ChatID,
				"error", err.Error(),
			)
			continue
		}
		ok++
	}
	log.InfoCtx(ctx, "notifier.telegram.broadcast.finished",
		"component", "notifier",
		"telegram_ok", ok,
		"telegram_fail", fail,
		"success", fail == 0,
	)
	return ok, fail
}

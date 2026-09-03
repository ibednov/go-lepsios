package session

import (
	"context"
	"time"
)

type Flow string

const FlowNone Flow = ""

type Session struct {
	ChatID                      int64     `json:"chat_id"`
	TelegramUserID              int64     `json:"telegram_user_id,omitempty"`
	TelegramUsername            string    `json:"telegram_username,omitempty"`
	TelegramFirstName           string    `json:"telegram_first_name,omitempty"`
	TelegramLastName            string    `json:"telegram_last_name,omitempty"`
	AccessToken                 string    `json:"access_token,omitempty"`
	RefreshToken                string    `json:"refresh_token,omitempty"`
	UserID                      string    `json:"user_id,omitempty"`
	Email                       string    `json:"email,omitempty"`
	DefaultWishlistID           string    `json:"default_wishlist_id,omitempty"`
	DefaultWishlistName         string    `json:"default_wishlist_name,omitempty"`
	Flow                        Flow      `json:"flow,omitempty"`
	PendingEmail                string    `json:"pending_email,omitempty"`
	PendingDuplicateCandidateID string    `json:"pending_duplicate_candidate_id,omitempty"`
	PendingDuplicateWishID      string    `json:"pending_duplicate_wish_id,omitempty"`
	PendingQuickAddJSON         string    `json:"pending_quick_add_json,omitempty"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

func (s *Session) IsAuthenticated() bool {
	return s != nil && s.AccessToken != ""
}

func (s *Session) CanSilentAuth() bool {
	return s != nil && s.IdentityUserID() != 0 && s.ChatID != 0
}

func (s *Session) IdentityUserID() int64 {
	if s.TelegramUserID != 0 {
		return s.TelegramUserID
	}
	return s.ChatID
}

type SessionStore interface {
	Get(ctx context.Context, chatID int64) (*Session, error)
	Save(ctx context.Context, session *Session) error
	Delete(ctx context.Context, chatID int64) error
}

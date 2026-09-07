// Package chat provides a reusable multi-tenant chat domain:
// rooms (users_chats), members (users_chats_members) and messages
// (users_chats_messages) with soft-delete on messages (and optionally rooms/members).
//
// Storage is owned by the consumer via Repository. This package carries the
// shared entity shapes, repository contract and membership/send/delete rules.
package chat

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Service implements shared chat use-cases on top of Repository.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a chat service.
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// EnsureChat returns an existing room for (kind, externalID) or creates one
// with the given members. Members are added only on create; existing rooms
// are returned as-is (call AddMember explicitly to extend membership).
func (s *Service) EnsureChat(ctx context.Context, in CreateChatInput) (Chat, error) {
	if s == nil || s.repo == nil {
		return Chat{}, ErrInvalidInput
	}
	kind := Kind(strings.TrimSpace(string(in.Kind)))
	ext := strings.TrimSpace(in.ExternalID)
	if kind == "" || ext == "" {
		return Chat{}, ErrInvalidInput
	}

	existing, err := s.repo.FindChatByRef(ctx, kind, ext)
	if err == nil && existing.ID != "" && existing.DeletedAt == nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Chat{}, err
	}

	now := s.now()
	created, err := s.repo.CreateChat(ctx, Chat{
		Kind:       kind,
		ExternalID: ext,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return s.repo.FindChatByRef(ctx, kind, ext)
		}
		return Chat{}, err
	}

	for _, m := range in.Members {
		if _, err := s.addMemberValidated(ctx, created.ID, m); err != nil {
			return Chat{}, err
		}
	}
	return created, nil
}

// AddMember adds an active participant to a room.
func (s *Service) AddMember(ctx context.Context, chatID string, in AddMemberInput) (Member, error) {
	if s == nil || s.repo == nil {
		return Member{}, ErrInvalidInput
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return Member{}, ErrInvalidInput
	}
	if _, err := s.requireChat(ctx, chatID); err != nil {
		return Member{}, err
	}
	return s.addMemberValidated(ctx, chatID, in)
}

func (s *Service) addMemberValidated(ctx context.Context, chatID string, in AddMemberInput) (Member, error) {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return Member{}, ErrInvalidInput
	}
	role := in.Role
	if role == "" {
		role = RoleMember
	}
	return s.repo.AddMember(ctx, Member{
		ChatID:   chatID,
		UserID:   userID,
		Role:     role,
		JoinedAt: s.now(),
	})
}

// IsMember reports whether userID is an active member of chatID.
func (s *Service) IsMember(ctx context.Context, chatID, userID string) (bool, error) {
	if s == nil || s.repo == nil {
		return false, ErrInvalidInput
	}
	chatID = strings.TrimSpace(chatID)
	userID = strings.TrimSpace(userID)
	if chatID == "" || userID == "" {
		return false, ErrInvalidInput
	}
	return s.repo.IsMember(ctx, chatID, userID)
}

// ListMembers returns active members of a room.
func (s *Service) ListMembers(ctx context.Context, chatID string) ([]Member, error) {
	if s == nil || s.repo == nil {
		return nil, ErrInvalidInput
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, ErrInvalidInput
	}
	if _, err := s.requireChat(ctx, chatID); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, chatID)
}

// GetChat returns a non-deleted chat by id.
func (s *Service) GetChat(ctx context.Context, chatID string) (Chat, error) {
	if s == nil || s.repo == nil {
		return Chat{}, ErrInvalidInput
	}
	return s.requireChat(ctx, chatID)
}

// GetChatByRef returns a non-deleted chat by (kind, externalID).
func (s *Service) GetChatByRef(ctx context.Context, kind Kind, externalID string) (Chat, error) {
	if s == nil || s.repo == nil {
		return Chat{}, ErrInvalidInput
	}
	kind = Kind(strings.TrimSpace(string(kind)))
	externalID = strings.TrimSpace(externalID)
	if kind == "" || externalID == "" {
		return Chat{}, ErrInvalidInput
	}
	c, err := s.repo.FindChatByRef(ctx, kind, externalID)
	if err != nil {
		return Chat{}, err
	}
	if c.DeletedAt != nil {
		return Chat{}, ErrNotFound
	}
	return c, nil
}

// SendMessage posts a message. Actor must be an active member.
// Body may be empty only when AttachmentFileID is set.
func (s *Service) SendMessage(ctx context.Context, in SendMessageInput) (Message, error) {
	if s == nil || s.repo == nil {
		return Message{}, ErrInvalidInput
	}
	chatID := strings.TrimSpace(in.ChatID)
	sender := strings.TrimSpace(in.SenderUserID)
	body := strings.TrimSpace(in.Body)
	if chatID == "" || sender == "" {
		return Message{}, ErrInvalidInput
	}
	if body == "" && (in.AttachmentFileID == nil || strings.TrimSpace(*in.AttachmentFileID) == "") {
		return Message{}, ErrEmptyMessage
	}
	if _, err := s.requireChat(ctx, chatID); err != nil {
		return Message{}, err
	}
	ok, err := s.repo.IsMember(ctx, chatID, sender)
	if err != nil {
		return Message{}, err
	}
	if !ok {
		return Message{}, ErrNotMember
	}

	now := s.now()
	var attachment *string
	if in.AttachmentFileID != nil {
		v := strings.TrimSpace(*in.AttachmentFileID)
		if v != "" {
			attachment = &v
		}
	}
	return s.repo.CreateMessage(ctx, Message{
		ChatID:           chatID,
		SenderUserID:     sender,
		Body:             body,
		AttachmentFileID: attachment,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
}

// SoftDeleteMessage marks a message deleted. Only the original sender may delete.
// History row is kept (deleted_at / deleted_by).
func (s *Service) SoftDeleteMessage(ctx context.Context, in SoftDeleteMessageInput) error {
	if s == nil || s.repo == nil {
		return ErrInvalidInput
	}
	chatID := strings.TrimSpace(in.ChatID)
	messageID := strings.TrimSpace(in.MessageID)
	actor := strings.TrimSpace(in.ActorID)
	if chatID == "" || messageID == "" || actor == "" {
		return ErrInvalidInput
	}
	if _, err := s.requireChat(ctx, chatID); err != nil {
		return err
	}
	msg, err := s.repo.FindMessage(ctx, chatID, messageID)
	if err != nil {
		return err
	}
	if msg.DeletedAt != nil {
		return nil
	}
	if msg.SenderUserID != actor {
		return ErrForbidden
	}
	return s.repo.SoftDeleteMessage(ctx, chatID, messageID, actor)
}

// ListMessages returns page of messages for a room. Caller must be a member.
// Soft-deleted messages are excluded unless IncludeDeleted is set.
func (s *Service) ListMessages(ctx context.Context, actorID string, f ListMessagesFilter) ([]Message, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, ErrInvalidInput
	}
	chatID := strings.TrimSpace(f.ChatID)
	actorID = strings.TrimSpace(actorID)
	if chatID == "" || actorID == "" {
		return nil, 0, ErrInvalidInput
	}
	if _, err := s.requireChat(ctx, chatID); err != nil {
		return nil, 0, err
	}
	ok, err := s.repo.IsMember(ctx, chatID, actorID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, ErrNotMember
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	f.ChatID = chatID
	return s.repo.ListMessages(ctx, f)
}

func (s *Service) requireChat(ctx context.Context, chatID string) (Chat, error) {
	c, err := s.repo.FindChatByID(ctx, chatID)
	if err != nil {
		return Chat{}, err
	}
	if c.DeletedAt != nil {
		return Chat{}, ErrNotFound
	}
	return c, nil
}

package chat

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memRepo struct {
	mu       sync.Mutex
	chats    map[string]Chat
	byRef    map[string]string
	members  map[string]Member // chatID+"|"+userID
	messages map[string]Message
	seq      int
}

func newMemRepo() *memRepo {
	return &memRepo{
		chats:    map[string]Chat{},
		byRef:    map[string]string{},
		members:  map[string]Member{},
		messages: map[string]Message{},
	}
}

func (m *memRepo) nextID(prefix string) string {
	m.seq++
	return prefix + strconv.Itoa(m.seq)
}

func refKey(kind Kind, externalID string) string {
	return string(kind) + "|" + externalID
}

func memberKey(chatID, userID string) string {
	return chatID + "|" + userID
}

func (m *memRepo) CreateChat(_ context.Context, c Chat) (Chat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := refKey(c.Kind, c.ExternalID)
	if id, ok := m.byRef[k]; ok {
		if existing := m.chats[id]; existing.DeletedAt == nil {
			return Chat{}, ErrAlreadyExists
		}
	}
	if c.ID == "" {
		c.ID = m.nextID("c")
	}
	m.chats[c.ID] = c
	m.byRef[k] = c.ID
	return c, nil
}

func (m *memRepo) FindChatByID(_ context.Context, id string) (Chat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.chats[id]
	if !ok {
		return Chat{}, ErrNotFound
	}
	return c, nil
}

func (m *memRepo) FindChatByRef(_ context.Context, kind Kind, externalID string) (Chat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byRef[refKey(kind, externalID)]
	if !ok {
		return Chat{}, ErrNotFound
	}
	c := m.chats[id]
	if c.DeletedAt != nil {
		return Chat{}, ErrNotFound
	}
	return c, nil
}

func (m *memRepo) SoftDeleteChat(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.chats[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	c.DeletedAt = &now
	m.chats[id] = c
	return nil
}

func (m *memRepo) AddMember(_ context.Context, mem Member) (Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.members[memberKey(mem.ChatID, mem.UserID)] = mem
	return mem, nil
}

func (m *memRepo) ListMembers(_ context.Context, chatID string) ([]Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Member, 0)
	for _, mem := range m.members {
		if mem.ChatID == chatID && mem.DeletedAt == nil {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (m *memRepo) IsMember(_ context.Context, chatID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.members[memberKey(chatID, userID)]
	return ok && mem.DeletedAt == nil, nil
}

func (m *memRepo) SoftDeleteMember(_ context.Context, chatID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := memberKey(chatID, userID)
	mem, ok := m.members[k]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	mem.DeletedAt = &now
	m.members[k] = mem
	return nil
}

func (m *memRepo) CreateMessage(_ context.Context, msg Message) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg.ID == "" {
		msg.ID = m.nextID("m")
	}
	m.messages[msg.ID] = msg
	return msg, nil
}

func (m *memRepo) FindMessage(_ context.Context, chatID, messageID string) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg, ok := m.messages[messageID]
	if !ok || msg.ChatID != chatID {
		return Message{}, ErrNotFound
	}
	return msg, nil
}

func (m *memRepo) ListMessages(_ context.Context, f ListMessagesFilter) ([]Message, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]Message, 0)
	for _, msg := range m.messages {
		if msg.ChatID != f.ChatID {
			continue
		}
		if !f.IncludeDeleted && msg.DeletedAt != nil {
			continue
		}
		if f.BeforeCreatedAt != nil && !msg.CreatedAt.Before(*f.BeforeCreatedAt) {
			continue
		}
		all = append(all, msg)
	}
	total := int64(len(all))
	if f.Offset >= len(all) {
		return nil, total, nil
	}
	end := f.Offset + f.Limit
	if end > len(all) {
		end = len(all)
	}
	return all[f.Offset:end], total, nil
}

func (m *memRepo) SoftDeleteMessage(_ context.Context, chatID, messageID, deletedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg, ok := m.messages[messageID]
	if !ok || msg.ChatID != chatID {
		return ErrNotFound
	}
	now := time.Now().UTC()
	msg.DeletedAt = &now
	msg.DeletedBy = &deletedBy
	msg.UpdatedAt = now
	m.messages[messageID] = msg
	return nil
}

func TestEnsureChatCreateAndIdempotent(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	c1, err := svc.EnsureChat(ctx, CreateChatInput{
		Kind:       KindReservation,
		ExternalID: "res-1",
		Members: []AddMemberInput{
			{UserID: "owner", Role: RoleOwner},
			{UserID: "giver", Role: RoleGiver},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, c1.ID)

	c2, err := svc.EnsureChat(ctx, CreateChatInput{
		Kind:       KindReservation,
		ExternalID: "res-1",
		Members:    []AddMemberInput{{UserID: "other", Role: RoleMember}},
	})
	require.NoError(t, err)
	require.Equal(t, c1.ID, c2.ID)

	members, err := svc.ListMembers(ctx, c1.ID)
	require.NoError(t, err)
	require.Len(t, members, 2)
}

func TestSendAndSoftDeleteMessage(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	chat, err := svc.EnsureChat(ctx, CreateChatInput{
		Kind:       KindDM,
		ExternalID: "dm-1",
		Members: []AddMemberInput{
			{UserID: "u1", Role: RoleMember},
			{UserID: "u2", Role: RoleMember},
		},
	})
	require.NoError(t, err)

	_, err = svc.SendMessage(ctx, SendMessageInput{
		ChatID:       chat.ID,
		SenderUserID: "stranger",
		Body:         "nope",
	})
	require.ErrorIs(t, err, ErrNotMember)

	_, err = svc.SendMessage(ctx, SendMessageInput{
		ChatID:       chat.ID,
		SenderUserID: "u1",
	})
	require.ErrorIs(t, err, ErrEmptyMessage)

	msg, err := svc.SendMessage(ctx, SendMessageInput{
		ChatID:       chat.ID,
		SenderUserID: "u1",
		Body:         "hello",
	})
	require.NoError(t, err)
	require.Equal(t, "hello", msg.Body)

	err = svc.SoftDeleteMessage(ctx, SoftDeleteMessageInput{
		ChatID:    chat.ID,
		MessageID: msg.ID,
		ActorID:   "u2",
	})
	require.ErrorIs(t, err, ErrForbidden)

	err = svc.SoftDeleteMessage(ctx, SoftDeleteMessageInput{
		ChatID:    chat.ID,
		MessageID: msg.ID,
		ActorID:   "u1",
	})
	require.NoError(t, err)

	list, total, err := svc.ListMessages(ctx, "u2", ListMessagesFilter{ChatID: chat.ID, Limit: 50})
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.Empty(t, list)

	list, total, err = svc.ListMessages(ctx, "u2", ListMessagesFilter{
		ChatID:         chat.ID,
		Limit:          50,
		IncludeDeleted: true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].DeletedAt)
	require.Equal(t, "u1", *list[0].DeletedBy)
}

func TestListMessagesRequiresMembership(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo)
	ctx := context.Background()

	chat, err := svc.EnsureChat(ctx, CreateChatInput{
		Kind:       KindReservation,
		ExternalID: "res-2",
		Members:    []AddMemberInput{{UserID: "u1"}},
	})
	require.NoError(t, err)

	_, _, err = svc.ListMessages(ctx, "outsider", ListMessagesFilter{ChatID: chat.ID})
	require.ErrorIs(t, err, ErrNotMember)
}

func TestInvalidEnsureChat(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.EnsureChat(context.Background(), CreateChatInput{Kind: KindDM})
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestNilService(t *testing.T) {
	var svc *Service
	_, err := svc.EnsureChat(context.Background(), CreateChatInput{Kind: KindDM, ExternalID: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
}

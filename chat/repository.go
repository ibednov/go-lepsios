package chat

import "context"

// Repository persists chats, members and messages.
// Consumers own the storage schema (see schema/users_chats.sql).
type Repository interface {
	CreateChat(ctx context.Context, c Chat) (Chat, error)
	FindChatByID(ctx context.Context, id string) (Chat, error)
	FindChatByRef(ctx context.Context, kind Kind, externalID string) (Chat, error)
	SoftDeleteChat(ctx context.Context, id string) error

	AddMember(ctx context.Context, m Member) (Member, error)
	ListMembers(ctx context.Context, chatID string) ([]Member, error)
	IsMember(ctx context.Context, chatID, userID string) (bool, error)
	SoftDeleteMember(ctx context.Context, chatID, userID string) error

	CreateMessage(ctx context.Context, m Message) (Message, error)
	FindMessage(ctx context.Context, chatID, messageID string) (Message, error)
	FindLatestMessage(ctx context.Context, chatID string) (Message, error)
	ListMessages(ctx context.Context, f ListMessagesFilter) ([]Message, int64, error)
	SoftDeleteMessage(ctx context.Context, chatID, messageID, deletedBy string) error

	ListChatsByMember(ctx context.Context, f ListChatsByMemberFilter) ([]Chat, int64, error)
}

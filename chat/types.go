package chat

import "time"

// Kind classifies a chat room for the consuming product.
// Products may introduce their own kind strings; library constants are examples.
type Kind string

const (
	KindReservation Kind = "reservation"
	KindDM          Kind = "dm"
)

// Role is a member's role inside a chat room.
type Role string

const (
	RoleMember Role = "member"
	RoleOwner  Role = "owner"
	RoleGiver  Role = "giver"
)

// Chat is a generic conversation room keyed by (Kind, ExternalID).
type Chat struct {
	ID         string
	Kind       Kind
	ExternalID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

// Member is a participant of a chat room.
type Member struct {
	ChatID    string
	UserID    string
	Role      Role
	JoinedAt  time.Time
	DeletedAt *time.Time
}

// Message is one chat message. Soft-delete sets DeletedAt / DeletedBy.
type Message struct {
	ID               string
	ChatID           string
	SenderUserID     string
	Body             string
	AttachmentFileID *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
	DeletedBy        *string
}

// CreateChatInput creates or describes a new room.
type CreateChatInput struct {
	Kind       Kind
	ExternalID string
	Members    []AddMemberInput
}

// AddMemberInput adds a participant to a room.
type AddMemberInput struct {
	UserID string
	Role   Role
}

// SendMessageInput posts a new message.
type SendMessageInput struct {
	ChatID           string
	SenderUserID     string
	Body             string
	AttachmentFileID *string
}

// SoftDeleteMessageInput marks a message deleted without removing the row.
type SoftDeleteMessageInput struct {
	MessageID string
	ChatID    string
	ActorID   string
}

// ListMessagesFilter pages messages inside a room.
type ListMessagesFilter struct {
	ChatID          string
	Limit           int
	Offset          int
	IncludeDeleted  bool
	BeforeCreatedAt *time.Time
}

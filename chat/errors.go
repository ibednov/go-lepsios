package chat

import "errors"

var (
	// ErrInvalidInput is returned when required fields are missing or empty.
	ErrInvalidInput = errors.New("chat: invalid input")
	// ErrNotFound is returned when a chat or message does not exist (or is soft-deleted).
	ErrNotFound = errors.New("chat: not found")
	// ErrNotMember is returned when the actor is not an active member of the room.
	ErrNotMember = errors.New("chat: not a member")
	// ErrForbidden is returned when the actor may not perform the action.
	ErrForbidden = errors.New("chat: forbidden")
	// ErrEmptyMessage is returned when body and attachment are both empty.
	ErrEmptyMessage = errors.New("chat: empty message")
	// ErrAlreadyExists is returned when a room with the same (kind, external_id) exists.
	ErrAlreadyExists = errors.New("chat: already exists")
)

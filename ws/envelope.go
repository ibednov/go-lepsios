package ws

import (
	"encoding/json"
	"time"
)

const (
	TypeAuth        = "auth"
	TypeSubscribe   = "subscribe"
	TypeUnsubscribe = "unsubscribe"
	TypePing        = "ping"
	TypeReady       = "ready"
	TypePong        = "pong"
	TypeError       = "error"
)

const (
	ErrAuthRequired = "auth_required"
	ErrAuthInvalid  = "auth_invalid"
	ErrAccessDenied = "access_denied"
	ErrBadRequest   = "bad_request"
	ErrRateLimited  = "rate_limited"
	ErrInternal     = "internal"
)

type Envelope struct {
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

type RoomRef struct {
	Kind  string `json:"kind"`
	RefID string `json:"ref_id"`
}

type AuthPayload struct {
	Token string `json:"token"`
}

type ReadyPayload struct {
	UserID string `json:"user_id"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewEnvelope(typ string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Type:    typ,
		TS:      time.Now().UTC(),
		Payload: raw,
	}, nil
}

func MustMarshalEnvelope(typ string, payload any) []byte {
	env, err := NewEnvelope(typ, payload)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil
	}
	return b
}

// RoomFromEnvelope extracts kind/ref_id when present (room-scoped events).
func RoomFromEnvelope(env Envelope) (RoomRef, bool) {
	var room RoomRef
	if err := json.Unmarshal(env.Payload, &room); err != nil {
		return RoomRef{}, false
	}
	if room.Kind == "" || room.RefID == "" {
		return RoomRef{}, false
	}
	return room, true
}

package audit

import (
	"encoding/json"
	"strconv"
	"time"
)

// HTTPEvent is the stable, product-neutral JSON representation of an audit
// event. Product APIs may add optional display fields, but must not rename or
// reinterpret these fields.
type HTTPEvent struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	ActorID    string          `json:"actor_id"`
	ActorKind  ActorKind       `json:"actor_kind"`
	ActorEmail string          `json:"actor_email,omitempty"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// HTTPListData is the data value in the standard {"data": ...} response
// envelope returned by an audit list endpoint.
type HTTPListData struct {
	Items  []HTTPEvent `json:"items"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// ToHTTPEvent converts the shared domain record to its stable JSON form.
// IDs are strings on the wire so consumers can support both integer and UUID
// identifiers without precision loss in JavaScript.
func ToHTTPEvent(event Event) HTTPEvent {
	id := ""
	if event.ID != 0 {
		id = strconv.FormatInt(event.ID, 10)
	}
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return HTTPEvent{
		ID:         id,
		Type:       event.Type,
		ActorID:    event.ActorID,
		ActorKind:  event.ActorKind,
		ActorEmail: event.ActorEmail,
		TargetType: event.TargetType,
		TargetID:   event.TargetID,
		Payload:    payload,
		OccurredAt: event.OccurredAt.UTC(),
	}
}

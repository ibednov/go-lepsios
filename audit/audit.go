// Package audit provides a reusable actor-action audit log ("who did what").
//
// Event types, target enums and schemas are product-specific and stay in the
// consumer. This package carries the generic record, filter, store interface
// and the best-effort write path shared by services with admin/audit needs.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ibednov/go-lepsios/log"
)

// ActorKind is the performing side of an action.
type ActorKind string

const (
	ActorKindAdmin     ActorKind = "admin"
	ActorKindUser      ActorKind = "user"
	ActorKindModerator ActorKind = "moderator"
	ActorKindSystem    ActorKind = "system"
)

// Event is one recorded action.
type Event struct {
	ID         int64
	Type       string
	ActorID    string
	ActorKind  ActorKind
	TargetType string
	TargetID   string
	Payload    json.RawMessage
	OccurredAt time.Time
}

// AppendInput describes a new audited action before it is persisted.
type AppendInput struct {
	Type       string
	ActorID    string
	ActorKind  ActorKind
	TargetType string
	TargetID   string
	Payload    any
}

// ListFilter filters stored events.
type ListFilter struct {
	ActorID    string
	ActorKind  string
	TargetType string
	TargetID   string
	Type       string // exact type, or prefix when ending with '*'
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// Repository persists audit events. Consumers provide the implementation and
// own the storage schema.
type Repository interface {
	Append(ctx context.Context, e Event) error
	List(ctx context.Context, f ListFilter) ([]Event, int64, error)
}

// Service writes and lists audit events.
type Service struct {
	repo Repository
}

// NewService creates an audit service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Append records an action. Incomplete inputs (missing actor/type/target) are
// skipped silently. ActorKind defaults to admin.
func (s *Service) Append(ctx context.Context, in AppendInput) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if in.ActorID == "" || in.Type == "" || in.TargetType == "" || in.TargetID == "" {
		return nil
	}
	payload := json.RawMessage("{}")
	if in.Payload != nil {
		if raw, err := json.Marshal(in.Payload); err == nil {
			payload = raw
		}
	}
	kind := in.ActorKind
	if kind == "" {
		kind = ActorKindAdmin
	}
	return s.repo.Append(ctx, Event{
		Type:       in.Type,
		ActorID:    in.ActorID,
		ActorKind:  kind,
		TargetType: in.TargetType,
		TargetID:   in.TargetID,
		Payload:    payload,
		OccurredAt: time.Now().UTC(),
	})
}

// AppendBestEffort writes an event; failures are logged, never propagated.
func (s *Service) AppendBestEffort(ctx context.Context, in AppendInput) {
	if err := s.Append(ctx, in); err != nil {
		log.Warn("audit.append_failed",
			"type", in.Type,
			"actor_id", in.ActorID,
			"target_type", in.TargetType,
			"target_id", in.TargetID,
			"error", err.Error(),
		)
	}
}

// List returns matching events and their total count.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Event, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, nil
	}
	return s.repo.List(ctx, f)
}
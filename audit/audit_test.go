package audit

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	appended []Event
	appendFn func(_ context.Context, e Event) error
}

func (f *fakeRepo) Append(ctx context.Context, e Event) error {
	if f.appendFn != nil {
		return f.appendFn(ctx, e)
	}
	f.appended = append(f.appended, e)
	return nil
}

func (f *fakeRepo) List(_ context.Context, _ ListFilter) ([]Event, int64, error) {
	return f.appended, int64(len(f.appended)), nil
}

func TestAppendSkipsIncomplete(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.Append(context.Background(), AppendInput{Type: "admin.user.deleted"}); err != nil {
		t.Fatalf("incomplete append should be skipped, got %v", err)
	}
	if len(repo.appended) != 0 {
		t.Fatalf("expected no events, got %d", len(repo.appended))
	}
}

func TestAppendDefaults(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	err := svc.Append(context.Background(), AppendInput{
		Type:       "admin.user.deleted",
		ActorID:    "u1",
		TargetType: "user",
		TargetID:   "u2",
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if len(repo.appended) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.appended))
	}
	e := repo.appended[0]
	if e.ActorKind != ActorKindAdmin {
		t.Fatalf("actor kind should default to admin, got %s", e.ActorKind)
	}
	if string(e.Payload) != "{}" {
		t.Fatalf("empty payload should be {}, got %s", e.Payload)
	}
}

func TestNilServiceIsSafe(t *testing.T) {
	var svc *Service
	if err := svc.Append(context.Background(), AppendInput{}); err != nil {
		t.Fatalf("nil service append should be safe, got %v", err)
	}
	svc.AppendBestEffort(context.Background(), AppendInput{})
	if _, _, err := svc.List(context.Background(), ListFilter{}); err != nil {
		t.Fatalf("nil service list should be safe, got %v", err)
	}
}

func TestRepositoryFailurePropagates(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	repo.appendFn = func(_ context.Context, e Event) error { return errors.New("db down") }
	if err := svc.Append(context.Background(), AppendInput{Type: "x", ActorID: "a", TargetType: "t", TargetID: "i"}); err == nil {
		t.Fatal("expected error to propagate")
	}
}
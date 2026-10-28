package recovery_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"sync"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/recovery"
	"github.com/ibednov/go-lepsios/mailer"
)

type memoryTx struct{ committed bool }

type memoryStore struct {
	mu              sync.Mutex
	policy          recovery.Policy
	reservation     recovery.Reservation
	active          map[string]recovery.Subject
	lastTokenHash   []byte
	accepted        bool
	failed          recovery.DeliveryFailure
	markAcceptedErr error
	confirmErr      error
	cleanupRecorded bool
	rejected        []string
}

func (s *memoryStore) Policy(context.Context) (recovery.Policy, error) { return s.policy, nil }
func (s *memoryStore) Reserve(_ context.Context, command recovery.ReservationCommand) (recovery.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !command.Enabled || s.reservation.Recipient == "" {
		return recovery.Reservation{}, nil
	}
	s.active[string(command.TokenHash)] = recovery.Subject{ID: "opaque-account"}
	s.lastTokenHash = append([]byte(nil), command.TokenHash...)
	s.reservation.TokenID = command.TokenID
	s.reservation.EventID = command.EventID
	s.reservation.Reserved = true
	return s.reservation, nil
}
func (s *memoryStore) MarkAccepted(_ context.Context, eventID string, receipt mailer.Receipt) error {
	if s.markAcceptedErr != nil {
		return s.markAcceptedErr
	}
	s.accepted = eventID != "" && receipt.Provider == "test" && receipt.MessageID != ""
	return nil
}
func (s *memoryStore) MarkFailed(_ context.Context, tokenID, eventID string, failure recovery.DeliveryFailure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, string(s.lastTokenHash))
	s.failed = failure
	return nil
}
func (s *memoryStore) FindActive(_ context.Context, tokenHash []byte) (recovery.Subject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if subject, ok := s.active[string(tokenHash)]; ok {
		return subject, nil
	}
	return recovery.Subject{}, recovery.ErrInvalidToken
}
func (s *memoryStore) RecordRejected(_ context.Context, _ string, reason, _ string, _ []byte) error {
	s.rejected = append(s.rejected, reason)
	return nil
}
func (s *memoryStore) Confirm(_ context.Context, command recovery.ConfirmationCommand, apply func(context.Context, memoryTx, recovery.Subject) error) (recovery.Subject, error) {
	s.mu.Lock()
	subject, ok := s.active[string(command.TokenHash)]
	s.mu.Unlock()
	if !ok {
		return recovery.Subject{}, recovery.ErrInvalidToken
	}
	tx := memoryTx{}
	if err := apply(context.Background(), tx, subject); err != nil {
		return recovery.Subject{}, err
	}
	tx.committed = true
	s.mu.Lock()
	delete(s.active, string(command.TokenHash))
	s.mu.Unlock()
	return subject, nil
}
func (s *memoryStore) RecordSessionInvalidationFailure(context.Context, string, []byte) error {
	s.cleanupRecorded = true
	return nil
}

type memoryLimiter struct {
	operations   []string
	fingerprints [][]byte
	err          error
}

func (l *memoryLimiter) Allow(_ context.Context, operation string, fingerprint []byte, max int, window time.Duration) error {
	if max != 3 || window != time.Hour {
		return errors.New("unexpected limiter policy")
	}
	l.operations = append(l.operations, operation)
	l.fingerprints = append(l.fingerprints, append([]byte(nil), fingerprint...))
	return l.err
}

type memoryComposer struct{ delivery recovery.Delivery }

func (c *memoryComposer) Compose(_ context.Context, delivery recovery.Delivery) (mailer.Message, error) {
	c.delivery = delivery
	return mailer.Message{To: delivery.Recipient, Subject: "Recover", Body: delivery.RawToken, IdempotencyKey: "password-reset/" + delivery.TokenID}, nil
}

type memorySender struct {
	message mailer.Message
	err     error
}

func (s *memorySender) Send(_ context.Context, strategy, senderAddress string, message mailer.Message) (mailer.Receipt, error) {
	if strategy != "test" {
		return mailer.Receipt{}, errors.New("unexpected strategy")
	}
	if senderAddress != "noreply@example.test" {
		return mailer.Receipt{}, errors.New("unexpected sender address")
	}
	s.message = message
	if s.err != nil {
		return mailer.Receipt{}, s.err
	}
	return mailer.Receipt{Provider: "test", MessageID: "message-1"}, nil
}

type invalidator struct {
	userID string
	err    error
}

func (i *invalidator) Invalidate(_ context.Context, userID string) error {
	i.userID = userID
	return i.err
}

func newFixture(t *testing.T, store *memoryStore, limiter *memoryLimiter, composer *memoryComposer, sender *memorySender, invalidator recovery.SessionInvalidator) *recovery.Service[memoryTx] {
	t.Helper()
	randomBytes := make([]byte, 32+16+16)
	for i := range randomBytes {
		randomBytes[i] = byte(i + 1)
	}
	return recovery.NewService[memoryTx](store, limiter, composer, sender, invalidator, recovery.Options{Random: &bytesReader{data: randomBytes}})
}

type bytesReader struct {
	mu   sync.Mutex
	data []byte
}

func (r *bytesReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.data) < len(p) {
		return 0, errors.New("deterministic random source exhausted")
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestRequestPreservesTokenFormatAndUsesReservationDeliveryData(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{
		policy:      recovery.Policy{Enabled: true, RequestAttemptsPerClientPerHour: 3, ConfirmAttemptsPerClientPerHour: 3, TokenTTL: 30 * time.Minute},
		reservation: recovery.Reservation{Recipient: "restore@example.test", Sender: "noreply@example.test", Strategy: "test", LinkBaseURL: "https://example.test/reset", ExpiresAt: now.Add(30 * time.Minute)},
		active:      make(map[string]recovery.Subject),
	}
	composer, sender := &memoryComposer{}, &memorySender{}
	limiter := &memoryLimiter{}
	service := newFixture(t, store, limiter, composer, sender, nil)
	result, err := service.Request(context.Background(), "product-hint", recovery.Client{Fingerprint: []byte("opaque-client"), RateLimitFingerprint: []byte("legacy-limit-key")})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || !store.accepted {
		t.Fatalf("accepted=%v, delivery recorded=%v", result.Accepted, store.accepted)
	}
	if len(limiter.fingerprints) != 1 || string(limiter.fingerprints[0]) != "legacy-limit-key" {
		t.Fatalf("limiter fingerprint = %q, want independently supplied value", limiter.fingerprints)
	}
	raw, err := base64.RawURLEncoding.DecodeString(composer.delivery.RawToken)
	if err != nil || len(raw) != 32 {
		t.Fatalf("recovery token must keep 32-byte raw-url-base64 format: len=%d err=%v", len(raw), err)
	}
	if !mailAddress(sender.message.To) || sender.message.IdempotencyKey != "password-reset/"+composer.delivery.TokenID {
		t.Fatalf("unexpected delivery contract: %+v", sender.message)
	}
	wantHash := sha256.Sum256([]byte(composer.delivery.RawToken))
	if _, err := service.Validate(context.Background(), composer.delivery.RawToken, "reset-state", recovery.Client{Fingerprint: []byte("opaque-client")}); err != nil {
		t.Fatalf("stored token hash does not match legacy SHA-256 contract %x: %v", wantHash, err)
	}
}

func mailAddress(value string) bool {
	_, err := mail.ParseAddress(value)
	return err == nil
}

func TestSuppressedRequestDoesNotSendOrRevealAccountEligibility(t *testing.T) {
	store := &memoryStore{policy: recovery.Policy{Enabled: true, RequestAttemptsPerClientPerHour: 3}, active: make(map[string]recovery.Subject)}
	composer, sender := &memoryComposer{}, &memorySender{}
	service := newFixture(t, store, &memoryLimiter{}, composer, sender, nil)
	result, err := service.Request(context.Background(), "unknown-account", recovery.Client{Fingerprint: []byte("opaque-client")})
	if err != nil || !result.Accepted {
		t.Fatalf("result=%+v err=%v, want generic accepted response", result, err)
	}
	if sender.message.To != "" || composer.delivery.RawToken != "" {
		t.Fatal("suppressed request attempted delivery")
	}
}

func TestConfirmLimitRunsBeforeDisabledPolicyAudit(t *testing.T) {
	store := &memoryStore{policy: recovery.Policy{Enabled: false, ConfirmAttemptsPerClientPerHour: 3}, active: make(map[string]recovery.Subject)}
	limiter := &memoryLimiter{err: recovery.ErrRateLimited}
	service := newFixture(t, store, limiter, &memoryComposer{}, &memorySender{}, nil)
	_, err := service.Confirm(context.Background(), "token", recovery.Client{Fingerprint: []byte("client")}, func(context.Context, recovery.Subject) (func(context.Context, memoryTx, recovery.Subject) error, error) {
		t.Fatal("confirmation callback ran for a limited request")
		return nil, nil
	})
	if !errors.Is(err, recovery.ErrRateLimited) {
		t.Fatalf("confirm error = %v, want rate limit", err)
	}
	if len(store.rejected) != 0 {
		t.Fatalf("disabled audit rows = %d, want none before rate limit", len(store.rejected))
	}
}

func TestRequestFailureInvalidatesReservationAndRecordsFailure(t *testing.T) {
	now := time.Now().UTC()
	store := &memoryStore{
		policy:      recovery.Policy{Enabled: true, RequestAttemptsPerClientPerHour: 3, TokenTTL: time.Hour},
		reservation: recovery.Reservation{Recipient: "restore@example.test", Sender: "noreply@example.test", Strategy: "test", LinkBaseURL: "https://example.test/reset", ExpiresAt: now.Add(time.Hour)},
		active:      make(map[string]recovery.Subject),
	}
	sender := &memorySender{err: errors.New("temporary transport failure")}
	service := newFixture(t, store, &memoryLimiter{}, &memoryComposer{}, sender, nil)
	if _, err := service.Request(context.Background(), "hint", recovery.Client{Fingerprint: []byte("client")}); err == nil {
		t.Fatal("expected provider failure")
	}
	if store.failed.Reason != "provider_failure" || len(store.active) != 0 {
		t.Fatalf("failure was not made unusable and recorded: reason=%q active=%d", store.failed.Reason, len(store.active))
	}
}

func TestAcceptedStatusFailureKeepsPossiblyDeliveredTokenUsable(t *testing.T) {
	now := time.Now().UTC()
	store := &memoryStore{
		policy:          recovery.Policy{Enabled: true, RequestAttemptsPerClientPerHour: 3, TokenTTL: time.Hour},
		reservation:     recovery.Reservation{Recipient: "restore@example.test", Sender: "noreply@example.test", Strategy: "test", LinkBaseURL: "https://example.test/reset", ExpiresAt: now.Add(time.Hour)},
		active:          make(map[string]recovery.Subject),
		markAcceptedErr: errors.New("event update failed"),
	}
	service := newFixture(t, store, &memoryLimiter{}, &memoryComposer{}, &memorySender{}, nil)
	if _, err := service.Request(context.Background(), "hint", recovery.Client{Fingerprint: []byte("client")}); err == nil {
		t.Fatal("expected accepted-state persistence failure")
	}
	if len(store.active) != 1 {
		t.Fatalf("active reservations = %d, want the delivered token retained", len(store.active))
	}
}

func TestConfirmRunsTypedProductMutationBeforeInvalidation(t *testing.T) {
	now := time.Now().UTC()
	store := &memoryStore{
		policy:      recovery.Policy{Enabled: true, RequestAttemptsPerClientPerHour: 3, ConfirmAttemptsPerClientPerHour: 3},
		reservation: recovery.Reservation{Recipient: "restore@example.test", Sender: "noreply@example.test", Strategy: "test", ExpiresAt: now.Add(time.Hour)},
		active:      make(map[string]recovery.Subject),
	}
	composer, sender := &memoryComposer{}, &memorySender{}
	service := newFixture(t, store, &memoryLimiter{}, composer, sender, nil)
	if _, err := service.Request(context.Background(), "hint", recovery.Client{Fingerprint: []byte("client")}); err != nil {
		t.Fatal(err)
	}
	invalidator := &invalidator{}
	service = newFixture(t, store, &memoryLimiter{}, composer, sender, invalidator)
	mutated := false
	subject, err := service.Confirm(context.Background(), composer.delivery.RawToken, recovery.Client{Fingerprint: []byte("client")}, func(_ context.Context, got recovery.Subject) (func(context.Context, memoryTx, recovery.Subject) error, error) {
		if got.ID != "opaque-account" {
			t.Fatalf("unexpected preflight subject: %+v", got)
		}
		return func(_ context.Context, tx memoryTx, transactionSubject recovery.Subject) error {
			if transactionSubject.ID != got.ID || tx.committed {
				t.Fatalf("unexpected transaction callback values: tx=%+v subject=%+v", tx, transactionSubject)
			}
			mutated = true
			return nil
		}, nil
	})
	if err != nil || !mutated || subject.ID != "opaque-account" || invalidator.userID != subject.ID {
		t.Fatalf("subject=%+v mutated=%v invalidated=%q err=%v", subject, mutated, invalidator.userID, err)
	}
	if _, err := service.Validate(context.Background(), composer.delivery.RawToken, "reset-state", recovery.Client{Fingerprint: []byte("client")}); !errors.Is(err, recovery.ErrInvalidToken) {
		t.Fatalf("confirmed token remained valid: %v", err)
	}
}

func TestSessionInvalidationFailureIsRecordedAfterSuccessfulConfirm(t *testing.T) {
	now := time.Now().UTC()
	store := &memoryStore{
		policy:      recovery.Policy{Enabled: true, RequestAttemptsPerClientPerHour: 3, ConfirmAttemptsPerClientPerHour: 3},
		reservation: recovery.Reservation{Recipient: "restore@example.test", Sender: "noreply@example.test", Strategy: "test", ExpiresAt: now.Add(time.Hour)},
		active:      make(map[string]recovery.Subject),
	}
	composer, sender := &memoryComposer{}, &memorySender{}
	service := newFixture(t, store, &memoryLimiter{}, composer, sender, nil)
	if _, err := service.Request(context.Background(), "hint", recovery.Client{Fingerprint: []byte("client")}); err != nil {
		t.Fatal(err)
	}
	cleanup := &invalidator{err: errors.New("redis unavailable")}
	service = newFixture(t, store, &memoryLimiter{}, composer, sender, cleanup)
	if _, err := service.Confirm(context.Background(), composer.delivery.RawToken, recovery.Client{Fingerprint: []byte("client")}, func(context.Context, recovery.Subject) (func(context.Context, memoryTx, recovery.Subject) error, error) {
		return func(context.Context, memoryTx, recovery.Subject) error { return nil }, nil
	}); err != nil {
		t.Fatalf("committed recovery should succeed despite best-effort Redis cleanup failure: %v", err)
	}
	if !store.cleanupRecorded {
		t.Fatal("failed session invalidation was not persisted")
	}
}

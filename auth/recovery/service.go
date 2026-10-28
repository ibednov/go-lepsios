package recovery

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ibednov/go-lepsios/mailer"
)

const (
	requestOperation      = "request"
	requestWindow         = time.Hour
	confirmWindow         = time.Hour
	sessionCleanupTimeout = 5 * time.Second
)

type Service[Tx any] struct {
	store       Store[Tx]
	limiter     Limiter
	composer    Composer
	sender      Sender
	invalidator SessionInvalidator
	random      io.Reader
	randomMu    sync.Mutex
}

type Options struct {
	Random io.Reader
}

func NewService[Tx any](store Store[Tx], limiter Limiter, composer Composer, sender Sender, invalidator SessionInvalidator, options Options) *Service[Tx] {
	randomSource := options.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &Service[Tx]{store: store, limiter: limiter, composer: composer, sender: sender, invalidator: invalidator, random: randomSource}
}

// Request always returns an accepted result for an ineligible account or a
// client request limit, leaving public account-enumeration policy to the caller.
func (s *Service[Tx]) Request(ctx context.Context, accountHint string, client Client) (RequestResult, error) {
	if s == nil || s.store == nil {
		return RequestResult{}, errors.New("recovery store is unavailable")
	}
	policy, err := s.store.Policy(ctx)
	if err != nil {
		return RequestResult{}, fmt.Errorf("load recovery policy: %w", err)
	}
	command := ReservationCommand{AccountHint: accountHint, Fingerprint: cloneBytes(client.Fingerprint), Enabled: policy.Enabled, TTL: policy.TokenTTL, Limits: policy.Limits}
	var rawToken string
	if policy.Enabled {
		if s.limiter == nil {
			return RequestResult{}, errors.New("recovery request limiter is unavailable")
		}
		if err := s.limiter.Allow(ctx, requestOperation, limiterFingerprint(client), policy.RequestAttemptsPerClientPerHour, requestWindow); err != nil {
			if errors.Is(err, ErrRateLimited) {
				return RequestResult{Accepted: true}, nil
			}
			return RequestResult{}, fmt.Errorf("apply recovery request limit: %w", err)
		}
		raw, err := s.randomBytes(32)
		if err != nil {
			return RequestResult{}, fmt.Errorf("generate recovery token: %w", err)
		}
		rawToken = base64.RawURLEncoding.EncodeToString(raw)
		command.TokenHash = HashToken(rawToken)
		command.TokenID, err = s.newID()
		if err != nil {
			return RequestResult{}, fmt.Errorf("generate recovery token id: %w", err)
		}
		command.EventID, err = s.newID()
		if err != nil {
			return RequestResult{}, fmt.Errorf("generate recovery event id: %w", err)
		}
	}
	reservation, err := s.store.Reserve(ctx, command)
	if err != nil {
		return RequestResult{}, fmt.Errorf("reserve recovery request: %w", err)
	}
	if !reservation.Reserved {
		return RequestResult{Accepted: true}, nil
	}
	if rawToken == "" || reservation.TokenID == "" || reservation.EventID == "" {
		return RequestResult{}, errors.New("recovery store returned an incomplete reservation")
	}
	if s.composer == nil || s.sender == nil {
		return RequestResult{}, s.failDelivery(ctx, reservation, DeliveryFailure{Reason: "delivery_unavailable"}, errors.New("recovery delivery is unavailable"))
	}
	message, err := s.composer.Compose(ctx, Delivery{
		Recipient: reservation.Recipient, Sender: reservation.Sender, RawToken: rawToken, TokenID: reservation.TokenID,
		LinkBaseURL: reservation.LinkBaseURL, ExpiresAt: reservation.ExpiresAt,
	})
	if err != nil {
		failure := deliveryFailure(err)
		if failure.Reason == "provider_failure" {
			failure.Reason = "message_configuration"
		}
		return RequestResult{}, s.failDelivery(ctx, reservation, failure, fmt.Errorf("compose recovery message: %w", err))
	}
	receipt, err := s.sender.Send(ctx, reservation.Strategy, reservation.Sender, message)
	if err != nil {
		return RequestResult{}, s.failDelivery(ctx, reservation, deliveryFailure(err), fmt.Errorf("send recovery message: %w", err))
	}
	statusCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionCleanupTimeout)
	defer cancel()
	if err := s.store.MarkAccepted(statusCtx, reservation.EventID, receipt); err != nil {
		// Sending already succeeded. Keep the reserved token usable, matching the
		// durable partial-failure state so the accepted link is not invalidated.
		return RequestResult{}, fmt.Errorf("record accepted recovery delivery: %w", err)
	}
	return RequestResult{Accepted: true}, nil
}

// Validate checks the enabled policy, client limit, and active one-time token.
func (s *Service[Tx]) Validate(ctx context.Context, rawToken, operation string, client Client) (Subject, error) {
	if s == nil || s.store == nil {
		return Subject{}, errors.New("recovery store is unavailable")
	}
	if rawToken == "" {
		return Subject{}, ErrInvalidToken
	}
	policy, err := s.store.Policy(ctx)
	if err != nil {
		return Subject{}, fmt.Errorf("load recovery policy: %w", err)
	}
	if s.limiter == nil {
		return Subject{}, errors.New("recovery confirmation limiter is unavailable")
	}
	if err := s.limiter.Allow(ctx, operation, limiterFingerprint(client), policy.ConfirmAttemptsPerClientPerHour, confirmWindow); err != nil {
		if errors.Is(err, ErrRateLimited) {
			return Subject{}, ErrRateLimited
		}
		return Subject{}, fmt.Errorf("apply recovery confirmation limit: %w", err)
	}
	if !policy.Enabled {
		return Subject{}, ErrDisabled
	}
	subject, err := s.store.FindActive(ctx, HashToken(rawToken))
	if err != nil {
		return Subject{}, err
	}
	if subject.ID == "" {
		return Subject{}, ErrInvalidToken
	}
	return subject, nil
}

// Confirm delegates the atomic token/credential/event transition to the
// product store while keeping its transaction handle strongly typed.
func (s *Service[Tx]) Confirm(ctx context.Context, rawToken string, client Client, prepare ConfirmationAction[Tx]) (Subject, error) {
	if s == nil || s.store == nil {
		return Subject{}, errors.New("recovery store is unavailable")
	}
	if rawToken == "" || prepare == nil {
		return Subject{}, ErrInvalidToken
	}
	policy, err := s.store.Policy(ctx)
	if err != nil {
		return Subject{}, fmt.Errorf("load recovery policy: %w", err)
	}
	if s.limiter == nil {
		return Subject{}, errors.New("recovery confirmation limiter is unavailable")
	}
	if err := s.limiter.Allow(ctx, "reset-confirm", limiterFingerprint(client), policy.ConfirmAttemptsPerClientPerHour, confirmWindow); err != nil {
		if errors.Is(err, ErrRateLimited) {
			return Subject{}, ErrRateLimited
		}
		return Subject{}, fmt.Errorf("apply recovery confirmation limit: %w", err)
	}
	if !policy.Enabled {
		_ = s.store.RecordRejected(ctx, "confirm", "disabled", "", client.Fingerprint)
		return Subject{}, ErrDisabled
	}
	tokenHash := HashToken(rawToken)
	subject, err := s.store.FindActive(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			_ = s.store.RecordRejected(ctx, "confirm", "invalid_token", "", client.Fingerprint)
		}
		return Subject{}, err
	}
	apply, err := prepare(ctx, subject)
	if err != nil {
		return Subject{}, err
	}
	if apply == nil {
		return Subject{}, errors.New("recovery confirmation action is unavailable")
	}
	subject, err = s.store.Confirm(ctx, ConfirmationCommand{
		TokenHash: tokenHash, Fingerprint: cloneBytes(client.Fingerprint),
		ClientLimit: policy.ConfirmAttemptsPerClientPerHour,
	}, apply)
	if err != nil {
		return Subject{}, err
	}
	if s.invalidator != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionCleanupTimeout)
		defer cancel()
		if err := s.invalidator.Invalidate(cleanupCtx, subject.ID); err != nil {
			_ = s.store.RecordSessionInvalidationFailure(cleanupCtx, subject.ID, client.Fingerprint)
		}
	}
	return subject, nil
}

func HashToken(raw string) []byte {
	hash := sha256.Sum256([]byte(raw))
	return hash[:]
}

func (s *Service[Tx]) failDelivery(ctx context.Context, reservation Reservation, failure DeliveryFailure, cause error) error {
	if failure.Provider == "" {
		failure.Provider = reservation.Strategy
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionCleanupTimeout)
	defer cancel()
	if err := s.store.MarkFailed(cleanupCtx, reservation.TokenID, reservation.EventID, failure); err != nil {
		return errors.Join(cause, fmt.Errorf("mark failed recovery delivery: %w", err))
	}
	return cause
}

func deliveryFailure(err error) DeliveryFailure {
	var deliveryErr *DeliveryError
	if errors.As(err, &deliveryErr) {
		return deliveryErr.Failure
	}
	failure := DeliveryFailure{Reason: "provider_failure", Diagnostic: []byte(`{"category":"transport_or_client_error"}`)}
	var providerErr *mailer.ProviderError
	if errors.As(err, &providerErr) {
		diagnostic, marshalErr := json.Marshal(struct {
			StatusCode int    `json:"status_code"`
			Name       string `json:"name,omitempty"`
			Message    string `json:"message,omitempty"`
			RetryAfter string `json:"retry_after,omitempty"`
		}{providerErr.StatusCode, providerErr.DiagnosticName(), providerErr.DiagnosticMessage(), providerErr.RetryAfter})
		if marshalErr == nil {
			failure.Diagnostic = diagnostic
		}
	}
	return failure
}

func (s *Service[Tx]) randomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	s.randomMu.Lock()
	defer s.randomMu.Unlock()
	_, err := io.ReadFull(s.random, buf)
	return buf, err
}

func limiterFingerprint(client Client) []byte {
	if len(client.RateLimitFingerprint) > 0 {
		return client.RateLimitFingerprint
	}
	return client.Fingerprint
}

func (s *Service[Tx]) newID() (string, error) {
	buf, err := s.randomBytes(16)
	if err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], buf[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], buf[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], buf[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], buf[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], buf[10:16])
	return string(encoded), nil
}

func cloneBytes(value []byte) []byte { return append([]byte(nil), value...) }

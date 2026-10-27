package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

type ResendConfig struct {
	APIKey string
	From   string
	Client *http.Client
}

type Resend struct {
	apiKey string
	from   string
	client *http.Client
}

func NewResend(cfg ResendConfig) (*Resend, error) {
	if strings.TrimSpace(cfg.APIKey) == "" || strings.ContainsAny(cfg.APIKey, "\r\n") {
		return nil, errors.New("mailer: valid Resend API key is required")
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		return nil, errors.New("mailer: valid Resend from address is required")
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Resend{apiKey: cfg.APIKey, from: cfg.From, client: client}, nil
}

// ProviderError classifies a failed provider request without exposing its body.
type ProviderError struct {
	StatusCode int
	RetryAfter string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("mailer: Resend returned HTTP %d", e.StatusCode)
}

func (r *Resend) Deliver(ctx context.Context, m Message) (Receipt, error) {
	if _, err := mail.ParseAddress(m.To); err != nil {
		return Receipt{}, errors.New("mailer: valid recipient is required")
	}
	if m.Subject == "" || strings.ContainsAny(m.Subject, "\r\n") || (m.Body == "" && m.HTML == "") {
		return Receipt{}, errors.New("mailer: subject and text or HTML body are required")
	}
	if len(m.IdempotencyKey) > 256 || strings.ContainsAny(m.IdempotencyKey, "\r\n") {
		return Receipt{}, errors.New("mailer: invalid idempotency key")
	}
	payload, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text,omitempty"`
		HTML    string   `json:"html,omitempty"`
	}{r.from, []string{m.To}, m.Subject, m.Body, m.HTML})
	if err != nil {
		return Receipt{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(payload))
	if err != nil {
		return Receipt{}, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if m.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	}
	response, err := r.client.Do(req)
	if err != nil {
		return Receipt{}, fmt.Errorf("mailer: Resend request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Receipt{}, &ProviderError{StatusCode: response.StatusCode, RetryAfter: response.Header.Get("Retry-After")}
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil || result.ID == "" {
		return Receipt{}, errors.New("mailer: invalid Resend response")
	}
	return Receipt{Provider: "resend", MessageID: result.ID}, nil
}

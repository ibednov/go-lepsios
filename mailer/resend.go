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
	"regexp"
	"strings"
	"time"
	"unicode"
)

const resendEndpoint = "https://api.resend.com/emails"

const maxProviderErrorMessageLength = 512

var (
	providerEmailPattern = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+`)
	providerURLPattern   = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)
)

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

// ProviderError contains bounded, scrubbed diagnostics from a provider failure.
// Error intentionally omits these details so callers must opt in to structured logging.
type ProviderError struct {
	StatusCode int
	RetryAfter string
	Name       string
	Message    string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("mailer: Resend returned HTTP %d", e.StatusCode)
}

// DiagnosticName exposes the sanitized provider error name to callers that opt in.
func (e *ProviderError) DiagnosticName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

// DiagnosticMessage exposes the sanitized provider error message to callers that opt in.
func (e *ProviderError) DiagnosticMessage() string {
	if e == nil {
		return ""
	}
	return e.Message
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
		var providerResponse struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&providerResponse)
		return Receipt{}, &ProviderError{
			StatusCode: response.StatusCode,
			RetryAfter: sanitizeProviderHeader(response.Header.Get("Retry-After"), 128),
			Name:       sanitizeProviderDiagnostic(providerResponse.Name, 128, r.apiKey),
			Message:    sanitizeProviderDiagnostic(providerResponse.Message, maxProviderErrorMessageLength, r.apiKey),
		}
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil || result.ID == "" {
		return Receipt{}, errors.New("mailer: invalid Resend response")
	}
	return Receipt{Provider: "resend", MessageID: result.ID}, nil
}

func sanitizeProviderDiagnostic(value string, maxLength int, secrets ...string) string {
	for _, secret := range secrets {
		if len(secret) >= 8 {
			value = strings.ReplaceAll(value, secret, "[redacted-secret]")
		}
	}
	value = providerEmailPattern.ReplaceAllString(value, "[redacted-email]")
	value = providerURLPattern.ReplaceAllString(value, "[redacted-url]")
	return sanitizeProviderText(value, maxLength)
}

func sanitizeProviderHeader(value string, maxLength int) string {
	return sanitizeProviderText(value, maxLength)
}

func sanitizeProviderText(value string, maxLength int) string {
	value = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value))
	if len(value) <= maxLength {
		return value
	}
	return strings.ToValidUTF8(value[:maxLength], "")
}

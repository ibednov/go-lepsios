package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxResponseBytes = 1 << 20
const maxErrorExcerptBytes = 2 << 10

var secretPattern = regexp.MustCompile(`(?i)\b(?:sk-or-v1-|sk-)[A-Za-z0-9_-]{12,}\b`)
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// HTTPConfig supplies provider endpoints and a host-configured HTTP client.
// Empty endpoint fields use the vendors' public API defaults.
type HTTPConfig struct {
	Client              *http.Client
	GigaChatConfigError error
	GigaChatOAuthURL    string
	GigaChatChatURL     string
	YandexChatURL       string
	OpenRouterChatURL   string
	OpenAIResponsesURL  string
}

func (c HTTPConfig) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func (c HTTPConfig) endpoint(configured, fallback string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	return fallback
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, payload any, response any, provider string, secrets ...string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed", provider)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerHTTPError(provider, resp, secrets...)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(response); err != nil {
		return fmt.Errorf("invalid %s response", provider)
	}
	return nil
}

func providerHTTPError(provider string, response *http.Response, secrets ...string) error {
	data, err := io.ReadAll(io.LimitReader(response.Body, maxErrorExcerptBytes+1))
	if err != nil {
		return fmt.Errorf("%s returned HTTP %d (failed to read error response)", provider, response.StatusCode)
	}
	truncated := len(data) > maxErrorExcerptBytes
	if truncated {
		data = data[:maxErrorExcerptBytes]
	}
	excerpt := sanitizeExcerpt(data)
	if json.Valid(data) {
		var value any
		if json.Unmarshal(data, &value) == nil {
			redactSecrets(value)
			if sanitized, marshalErr := json.Marshal(value); marshalErr == nil {
				excerpt = string(sanitized)
			}
		}
	}
	excerpt = secretPattern.ReplaceAllString(excerpt, "[REDACTED]")
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			excerpt = strings.ReplaceAll(excerpt, secret, "[REDACTED]")
		}
	}
	if truncated {
		const marker = "… [truncated]"
		excerpt = truncateExcerpt(excerpt, maxErrorExcerptBytes-len(marker)) + marker
	} else {
		excerpt = truncateExcerpt(excerpt, maxErrorExcerptBytes)
	}
	if excerpt == "" {
		return fmt.Errorf("%s returned HTTP %d with an empty error response", provider, response.StatusCode)
	}
	requestID := strings.TrimSpace(response.Header.Get("X-Request-Id"))
	if requestID == "" {
		requestID = strings.TrimSpace(response.Header.Get("X-OpenRouter-Request-Id"))
	}
	if requestIDPattern.MatchString(requestID) {
		return fmt.Errorf("%s returned HTTP %d (request_id=%s): %s", provider, response.StatusCode, requestID, excerpt)
	}
	return fmt.Errorf("%s returned HTTP %d: %s", provider, response.StatusCode, excerpt)
}

// ProviderHTTPError returns a bounded HTTP error excerpt with credentials redacted.
func ProviderHTTPError(provider string, response *http.Response, secrets ...string) error {
	return providerHTTPError(provider, response, secrets...)
}

func sanitizeExcerpt(data []byte) string {
	text := string(data)
	if !utf8.Valid(data) {
		text = strings.ToValidUTF8(text, "�")
	}
	text = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) && char != '\n' && char != '\r' && char != '\t' {
			return -1
		}
		return char
	}, text)
	return strings.TrimSpace(text)
}

func truncateExcerpt(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func redactSecrets(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
			if strings.Contains(normalized, "apikey") || strings.Contains(normalized, "token") ||
				strings.Contains(normalized, "secret") || strings.Contains(normalized, "password") ||
				strings.Contains(normalized, "authorization") || normalized == "key" {
				typed[key] = "[REDACTED]"
				continue
			}
			redactSecrets(nested)
		}
	case []any:
		for _, nested := range typed {
			redactSecrets(nested)
		}
	}
}

var errEmptyProviderResponse = errors.New("empty provider response")

type chatCompletionsRequest struct {
	Model           string              `json:"model"`
	Messages        []chatMessage       `json:"messages"`
	MaxTokens       int                 `json:"max_tokens"`
	ReasoningEffort string              `json:"reasoning_effort,omitempty"`
	ResponseFormat  *chatResponseFormat `json:"response_format,omitempty"`
}

type chatResponseFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning"`
}

func messages(request Request) []chatMessage {
	result := []chatMessage{{Role: "system", Content: request.SystemPrompt}}
	for _, turn := range request.History {
		if turn.Role == "user" || turn.Role == "assistant" {
			result = append(result, chatMessage{Role: turn.Role, Content: turn.Content})
		}
	}
	return append(result, chatMessage{Role: "user", Content: request.Message})
}

type chatCompletionsResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message            chatMessage `json:"message"`
		FinishReason       string      `json:"finish_reason"`
		NativeFinishReason string      `json:"native_finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

func maxTokens(value int) int {
	if value < 1 {
		return 1000
	}
	return value
}

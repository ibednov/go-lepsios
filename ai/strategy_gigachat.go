package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const ProviderGigaChat = "gigachat"
const CredentialGigaChatAuthorizationKey = "authorization_key"

type gigaChatStrategy struct {
	config HTTPConfig
	cache  *gigaChatTokenCache
}

type cachedAccessToken struct {
	value     string
	expiresAt time.Time
}

type gigaChatTokenCache struct {
	mu     sync.Mutex
	tokens map[string]cachedAccessToken
}

type gigaChatProvider struct {
	client           *http.Client
	oauthURL         string
	chatURL          string
	authorizationKey string
	cache            *gigaChatTokenCache
}

// NewGigaChatStrategy creates the GigaChat credential and HTTP strategy.
func NewGigaChatStrategy(config HTTPConfig) Strategy {
	return gigaChatStrategy{config: config, cache: &gigaChatTokenCache{tokens: make(map[string]cachedAccessToken)}}
}

func (gigaChatStrategy) Name() string { return ProviderGigaChat }

func (s gigaChatStrategy) New(credentials Credentials) (Provider, error) {
	if s.config.GigaChatConfigError != nil {
		return nil, s.config.GigaChatConfigError
	}
	key := strings.TrimSpace(credentials[CredentialGigaChatAuthorizationKey])
	if key == "" {
		return nil, fmt.Errorf("%w: %s", ErrMissingCredential, CredentialGigaChatAuthorizationKey)
	}
	if s.cache == nil {
		s.cache = &gigaChatTokenCache{tokens: make(map[string]cachedAccessToken)}
	}
	return &gigaChatProvider{client: s.config.client(),
		oauthURL:         s.config.endpoint(s.config.GigaChatOAuthURL, "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"),
		chatURL:          s.config.endpoint(s.config.GigaChatChatURL, "https://api.giga.chat/v1/chat/completions"),
		authorizationKey: key, cache: s.cache}, nil
}

func (*gigaChatProvider) Name() string { return ProviderGigaChat }

func (*gigaChatProvider) ListModels(context.Context) ([]Model, error) {
	return []Model{{ID: "GigaChat", Name: "GigaChat"}, {ID: "GigaChat-2-Pro", Name: "GigaChat 2 Pro"},
		{ID: "GigaChat-2-Max", Name: "GigaChat 2 Max"}, {ID: "GigaChat-3-Ultra", Name: "GigaChat 3 Ultra"}}, nil
}

func (p *gigaChatProvider) Chat(ctx context.Context, request Request) (Result, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return Result{}, err
	}
	payload := chatCompletionsRequest{Model: request.Model, Messages: messages(request), MaxTokens: maxTokens(request.MaxTokens)}
	var response chatCompletionsResponse
	if err := postJSON(ctx, p.client, p.chatURL, map[string]string{"Authorization": "Bearer " + token}, payload,
		&response, "GigaChat", token); err != nil {
		return Result{}, err
	}
	if len(response.Choices) == 0 {
		return Result{}, fmt.Errorf("GigaChat: %w", errEmptyProviderResponse)
	}
	return Result{Text: response.Choices[0].Message.Content,
		InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CompletionTokens}, nil
}

func (p *gigaChatProvider) accessToken(ctx context.Context) (string, error) {
	digest := sha256.Sum256([]byte(p.authorizationKey))
	cacheKey := fmt.Sprintf("%x", digest[:])
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()
	if token, ok := p.cache.tokens[cacheKey]; ok && time.Now().Add(time.Minute).Before(token.expiresAt) {
		return token.value, nil
	}
	requestID, err := newRequestID()
	if err != nil {
		return "", errors.New("GigaChat authentication failed")
	}
	form := url.Values{"scope": {"GIGACHAT_API_PERS"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.oauthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("GigaChat authentication failed")
	}
	req.Header.Set("Authorization", "Basic "+p.authorizationKey)
	req.Header.Set("RqUID", requestID)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", errors.New("GigaChat authentication failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", providerHTTPError("GigaChat authentication", resp, p.authorizationKey)
	}
	var response struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&response); err != nil || response.AccessToken == "" {
		return "", errors.New("invalid GigaChat authentication response")
	}
	expiresAt := time.Unix(response.ExpiresAt, 0)
	if response.ExpiresAt == 0 {
		expiresAt = time.Now().Add(25 * time.Minute)
	}
	p.cache.tokens[cacheKey] = cachedAccessToken{value: response.AccessToken, expiresAt: expiresAt}
	return response.AccessToken, nil
}

func newRequestID() (string, error) {
	var data [16]byte
	if _, err := io.ReadFull(rand.Reader, data[:]); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(data[0:4]), hex.EncodeToString(data[4:6]),
		hex.EncodeToString(data[6:8]), hex.EncodeToString(data[8:10]), hex.EncodeToString(data[10:16])), nil
}

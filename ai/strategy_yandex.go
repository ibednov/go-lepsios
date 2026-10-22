package ai

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const ProviderYandex = "yandex"
const CredentialYandexAPIKey = "api_key"
const CredentialYandexFolderID = "folder_id"

type yandexStrategy struct{ config HTTPConfig }
type yandexProvider struct {
	client   *http.Client
	endpoint string
	apiKey   string
	folderID string
}

// NewYandexStrategy creates the Yandex Foundation Models strategy.
func NewYandexStrategy(config HTTPConfig) Strategy { return yandexStrategy{config: config} }

func (yandexStrategy) Name() string { return ProviderYandex }

func (s yandexStrategy) New(credentials Credentials) (Provider, error) {
	key := strings.TrimSpace(credentials[CredentialYandexAPIKey])
	folderID := strings.TrimSpace(credentials[CredentialYandexFolderID])
	if key == "" {
		return nil, fmt.Errorf("%w: %s", ErrMissingCredential, CredentialYandexAPIKey)
	}
	if folderID == "" {
		return nil, fmt.Errorf("%w: %s", ErrMissingCredential, CredentialYandexFolderID)
	}
	return &yandexProvider{client: s.config.client(),
		endpoint: s.config.endpoint(s.config.YandexChatURL, "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"),
		apiKey:   key, folderID: folderID}, nil
}

func (*yandexProvider) Name() string { return ProviderYandex }

func (*yandexProvider) ListModels(context.Context) ([]Model, error) {
	return []Model{{ID: "yandexgpt/latest", Name: "YandexGPT Pro"},
		{ID: "yandexgpt-lite/latest", Name: "YandexGPT Lite"}}, nil
}

func (p *yandexProvider) Chat(ctx context.Context, request Request) (Result, error) {
	modelURI := request.Model
	if !strings.HasPrefix(modelURI, "gpt://") {
		modelURI = "gpt://" + p.folderID + "/" + modelURI
	}
	payload := yandexRequest{ModelURI: modelURI}
	payload.CompletionOptions.Temperature = request.Temperature
	payload.CompletionOptions.MaxTokens = request.MaxTokens
	payload.Messages = append(payload.Messages, yandexMessage{Role: "system", Text: request.SystemPrompt})
	for _, turn := range request.History {
		payload.Messages = append(payload.Messages, yandexMessage{Role: turn.Role, Text: turn.Content})
	}
	payload.Messages = append(payload.Messages, yandexMessage{Role: "user", Text: request.Message})
	var response yandexResponse
	if err := postJSON(ctx, p.client, p.endpoint, map[string]string{"Authorization": "Api-Key " + p.apiKey},
		payload, &response, "Yandex AI", p.apiKey); err != nil {
		return Result{}, err
	}
	if len(response.Result.Alternatives) == 0 {
		return Result{}, fmt.Errorf("Yandex AI: %w", errEmptyProviderResponse)
	}
	inputTokens, _ := strconv.Atoi(response.Result.Usage.InputTextTokens)
	outputTokens, _ := strconv.Atoi(response.Result.Usage.CompletionTokens)
	return Result{Text: response.Result.Alternatives[0].Message.Text,
		InputTokens: inputTokens, OutputTokens: outputTokens}, nil
}

type yandexRequest struct {
	ModelURI          string `json:"modelUri"`
	CompletionOptions struct {
		Stream      bool     `json:"stream"`
		Temperature *float64 `json:"temperature,omitempty"`
		MaxTokens   int      `json:"maxTokens"`
	} `json:"completionOptions"`
	Messages []yandexMessage `json:"messages"`
}

type yandexMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type yandexResponse struct {
	Result struct {
		Alternatives []struct {
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
		} `json:"alternatives"`
		Usage struct {
			InputTextTokens  string `json:"inputTextTokens"`
			CompletionTokens string `json:"completionTokens"`
		} `json:"usage"`
	} `json:"result"`
}

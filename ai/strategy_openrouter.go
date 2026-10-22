package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const ProviderOpenRouter = "openrouter"
const CredentialOpenRouterAPIKey = "openrouter_api_key"

type openRouterStrategy struct{ config HTTPConfig }
type openRouterProvider struct {
	client   *http.Client
	endpoint string
	apiKey   string
}

// NewOpenRouterStrategy creates the OpenRouter chat-completions strategy.
func NewOpenRouterStrategy(config HTTPConfig) Strategy { return openRouterStrategy{config: config} }

func (openRouterStrategy) Name() string { return ProviderOpenRouter }

func (s openRouterStrategy) New(credentials Credentials) (Provider, error) {
	key := strings.TrimSpace(credentials[CredentialOpenRouterAPIKey])
	if key == "" {
		return nil, fmt.Errorf("%w: %s", ErrMissingCredential, CredentialOpenRouterAPIKey)
	}
	return &openRouterProvider{client: s.config.client(),
		endpoint: s.config.endpoint(s.config.OpenRouterChatURL, "https://openrouter.ai/api/v1/chat/completions"), apiKey: key}, nil
}

func (*openRouterProvider) Name() string { return ProviderOpenRouter }

func (*openRouterProvider) ListModels(context.Context) ([]Model, error) {
	// Model IDs change frequently on OpenRouter, so host applications accept an ID directly.
	return nil, nil
}

func (p *openRouterProvider) Chat(ctx context.Context, request Request) (Result, error) {
	effort := request.ReasoningEffort
	if effort == "" {
		effort = "low"
	}
	payload := chatCompletionsRequest{Model: request.Model, Messages: messages(request), MaxTokens: maxTokens(request.MaxTokens), ReasoningEffort: effort}
	if request.StructuredJSON || request.StructuredOutput != nil {
		payload.ResponseFormat = &chatResponseFormat{Type: "json_object"}
	}
	var response chatCompletionsResponse
	if err := postJSON(ctx, p.client, p.endpoint, map[string]string{"Authorization": "Bearer " + p.apiKey},
		payload, &response, "OpenRouter", p.apiKey); err != nil {
		return Result{}, err
	}
	if len(response.Choices) == 0 {
		return Result{}, fmt.Errorf("OpenRouter: %w", errEmptyProviderResponse)
	}
	choice := response.Choices[0]
	return Result{Text: choice.Message.Content, InputTokens: response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens, ResponseID: response.ID, ResponseModel: response.Model,
		FinishReason: choice.FinishReason, NativeFinishReason: choice.NativeFinishReason,
		ReasoningLength: len(choice.Message.Reasoning), ReasoningTokens: response.Usage.CompletionTokensDetails.ReasoningTokens}, nil
}

package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const ProviderOpenAI = "openai"
const CredentialOpenAIAPIKey = "openai_api_key"

type openAIStrategy struct{ config HTTPConfig }
type openAIProvider struct {
	client   *http.Client
	endpoint string
	apiKey   string
}

// NewOpenAIStrategy creates the OpenAI Responses API strategy.
func NewOpenAIStrategy(config HTTPConfig) Strategy { return openAIStrategy{config: config} }

func (openAIStrategy) Name() string { return ProviderOpenAI }

func (s openAIStrategy) New(credentials Credentials) (Provider, error) {
	key := strings.TrimSpace(credentials[CredentialOpenAIAPIKey])
	if key == "" {
		return nil, fmt.Errorf("%w: %s", ErrMissingCredential, CredentialOpenAIAPIKey)
	}
	return &openAIProvider{client: s.config.client(),
		endpoint: s.config.endpoint(s.config.OpenAIResponsesURL, "https://api.openai.com/v1/responses"), apiKey: key}, nil
}

func (*openAIProvider) Name() string { return ProviderOpenAI }

func (*openAIProvider) ListModels(context.Context) ([]Model, error) {
	// Model availability is project-specific, so host applications can accept an ID directly.
	return nil, nil
}

func (p *openAIProvider) Chat(ctx context.Context, request Request) (Result, error) {
	payload := openAIResponsesRequest{Model: request.Model, Instructions: request.SystemPrompt,
		Input: responseInput(request), MaxOutputTokens: request.MaxTokens, Store: false}
	if request.StructuredOutput != nil {
		payload.Text = &openAIResponseText{Format: openAIResponseFormat{Type: "json_schema",
			Name: request.StructuredOutput.Name, Strict: request.StructuredOutput.Strict, Schema: request.StructuredOutput.Schema}}
	}
	if effort := strings.TrimSpace(request.ReasoningEffort); effort != "" && effort != "none" {
		payload.Reasoning = &openAIReasoning{Effort: effort}
	}
	var response openAIResponsesResponse
	if err := postJSON(ctx, p.client, p.endpoint, map[string]string{"Authorization": "Bearer " + p.apiKey},
		payload, &response, "OpenAI", p.apiKey); err != nil {
		return Result{}, err
	}
	var text strings.Builder
	for _, output := range response.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" {
				text.WriteString(content.Text)
			}
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return Result{}, fmt.Errorf("OpenAI: %w", errEmptyProviderResponse)
	}
	return Result{Text: text.String(), InputTokens: response.Usage.InputTokens, OutputTokens: response.Usage.OutputTokens,
		ResponseID: response.ID, ResponseModel: response.Model, FinishReason: response.Status,
		ReasoningTokens: response.Usage.OutputTokensDetails.ReasoningTokens}, nil
}

type openAIResponsesRequest struct {
	Model           string                `json:"model"`
	Instructions    string                `json:"instructions"`
	Input           []openAIResponseInput `json:"input"`
	MaxOutputTokens int                   `json:"max_output_tokens"`
	Store           bool                  `json:"store"`
	Reasoning       *openAIReasoning      `json:"reasoning,omitempty"`
	Text            *openAIResponseText   `json:"text,omitempty"`
}

type openAIReasoning struct {
	Effort string `json:"effort"`
}

type openAIResponseText struct {
	Format openAIResponseFormat `json:"format"`
}

type openAIResponseFormat struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Strict bool   `json:"strict"`
	Schema any    `json:"schema"`
}

type openAIResponseInput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func responseInput(request Request) []openAIResponseInput {
	input := make([]openAIResponseInput, 0, len(request.History)+1)
	for _, turn := range request.History {
		if turn.Role == "user" || turn.Role == "assistant" {
			input = append(input, openAIResponseInput{Role: turn.Role, Content: turn.Content})
		}
	}
	return append(input, openAIResponseInput{Role: "user", Content: request.Message})
}

type openAIResponsesResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		OutputTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

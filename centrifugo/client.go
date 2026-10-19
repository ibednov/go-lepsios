package centrifugo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrInvalidConfig = errors.New("centrifugo: invalid config")

type Config struct {
	APIURL     string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type Client struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	apiURL := strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	if apiURL == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrInvalidConfig
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{apiURL: apiURL, apiKey: cfg.APIKey, client: httpClient}, nil
}

func (c *Client) Publish(ctx context.Context, channel string, data any) error {
	if strings.TrimSpace(channel) == "" {
		return fmt.Errorf("centrifugo: publish: channel is required")
	}
	payload := struct {
		Channel string `json:"channel"`
		Data    any    `json:"data"`
	}{Channel: channel, Data: data}
	return c.post(ctx, "publish", payload, 0)
}

func (c *Client) Broadcast(ctx context.Context, channels []string, data any) error {
	if len(channels) == 0 {
		return fmt.Errorf("centrifugo: broadcast: at least one channel is required")
	}
	for _, channel := range channels {
		if strings.TrimSpace(channel) == "" {
			return fmt.Errorf("centrifugo: broadcast: channel is required")
		}
	}
	payload := struct {
		Channels []string `json:"channels"`
		Data     any      `json:"data"`
	}{Channels: channels, Data: data}
	return c.post(ctx, "broadcast", payload, len(channels))
}

func (c *Client) post(ctx context.Context, method string, payload any, expectedResponses int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("centrifugo: encode %s request: %w", method, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("centrifugo: create %s request: %w", method, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Key", c.apiKey)

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("centrifugo: %s: %w", method, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("centrifugo: %s returned %s: %s", method, response.Status, strings.TrimSpace(string(message)))
	}

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil {
		return fmt.Errorf("centrifugo: read %s response: %w", method, err)
	}
	if len(responseBody) > 4096 {
		return fmt.Errorf("centrifugo: %s response exceeds 4096 bytes", method)
	}
	var apiResponse struct {
		Result json.RawMessage `json:"result"`
		Error  *apiError       `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return fmt.Errorf("centrifugo: decode %s response: %w", method, err)
	}
	if apiResponse.Error != nil {
		return fmt.Errorf("centrifugo: %s API error %d: %s", method, apiResponse.Error.Code, apiResponse.Error.Message)
	}
	if len(apiResponse.Result) == 0 {
		return fmt.Errorf("centrifugo: %s response is missing result", method)
	}
	if method == "broadcast" {
		var result struct {
			Responses *[]struct {
				Error *apiError `json:"error"`
			} `json:"responses"`
		}
		if err := json.Unmarshal(apiResponse.Result, &result); err != nil {
			return fmt.Errorf("centrifugo: decode broadcast responses: %w", err)
		}
		if result.Responses == nil {
			return fmt.Errorf("centrifugo: broadcast response is missing per-channel responses")
		}
		if len(*result.Responses) != expectedResponses {
			return fmt.Errorf("centrifugo: broadcast returned %d responses for %d channels", len(*result.Responses), expectedResponses)
		}
		for i, item := range *result.Responses {
			if item.Error != nil {
				return fmt.Errorf("centrifugo: broadcast response %d API error %d: %s", i, item.Error.Code, item.Error.Message)
			}
		}
	}
	return nil
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

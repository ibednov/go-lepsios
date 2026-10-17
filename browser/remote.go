package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const remotePathFetchInPage = "/internal/v1/fetch-in-page"
const remotePathCaptureJSON = "/internal/v1/capture-json"

// RemoteClient talks to a separate browser container. It does not start Chromium.
type RemoteClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type remoteFetchRequest struct {
	WarmupURL  string `json:"warmup_url"`
	RequestURL string `json:"request_url"`
}

type remoteFetchResponse struct {
	Body []byte `json:"body"`
	HTML []byte `json:"html"`
}

type remoteCaptureRequest struct {
	PageURL string `json:"page_url"`
	URLGlob string `json:"url_glob"`
}

type remoteCaptureResponse struct {
	Body []byte `json:"body"`
}

type remoteErrorResponse struct {
	Error string `json:"error"`
}

func NewRemote(baseURL, token string, timeout time.Duration) *RemoteClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &RemoteClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		http: &http.Client{
			Timeout: timeout + 5*time.Second,
		},
	}
}

func (c *RemoteClient) FetchInPage(ctx context.Context, warmupURL, requestURL string) (PageSnapshot, error) {
	if c == nil || c.baseURL == "" {
		return PageSnapshot{}, ErrRemoteUnavailable
	}
	if warmupURL == "" {
		return PageSnapshot{}, ErrEmptyPageURL
	}
	if requestURL == "" {
		return PageSnapshot{}, ErrEmptyRequestURL
	}
	if err := CheckAllowedURL(warmupURL); err != nil {
		return PageSnapshot{}, err
	}
	if err := CheckAllowedURL(requestURL); err != nil {
		return PageSnapshot{}, err
	}

	var out remoteFetchResponse
	if err := c.post(ctx, remotePathFetchInPage, remoteFetchRequest{
		WarmupURL:  warmupURL,
		RequestURL: requestURL,
	}, &out); err != nil {
		return PageSnapshot{}, err
	}
	return PageSnapshot{Body: out.Body, HTML: out.HTML}, nil
}

func (c *RemoteClient) CaptureJSON(ctx context.Context, pageURL, urlGlob string) ([]byte, error) {
	if c == nil || c.baseURL == "" {
		return nil, ErrRemoteUnavailable
	}
	if pageURL == "" {
		return nil, ErrEmptyPageURL
	}
	if urlGlob == "" {
		return nil, ErrEmptyGlob
	}
	if err := CheckAllowedURL(pageURL); err != nil {
		return nil, err
	}

	var out remoteCaptureResponse
	if err := c.post(ctx, remotePathCaptureJSON, remoteCaptureRequest{
		PageURL: pageURL,
		URLGlob: urlGlob,
	}, &out); err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (c *RemoteClient) post(ctx context.Context, path string, payload, dest any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", ErrRemoteUnavailable, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		var wrapped remoteErrorResponse
		_ = json.Unmarshal(body, &wrapped)
		if wrapped.Error != "" {
			return fmt.Errorf("%w: %s", ErrHostNotAllowed, wrapped.Error)
		}
		return ErrHostNotAllowed
	}
	if resp.StatusCode >= 400 {
		var wrapped remoteErrorResponse
		_ = json.Unmarshal(body, &wrapped)
		if wrapped.Error != "" {
			return fmt.Errorf("browser sidecar HTTP %d: %s", resp.StatusCode, wrapped.Error)
		}
		return fmt.Errorf("browser sidecar HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("browser sidecar decode: %w", err)
	}
	return nil
}

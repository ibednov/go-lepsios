package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type diagnosticCaptureKey struct{}

// DiagnosticCapture retains only the most recent HTTP response observed in a
// single parser operation. It is opt-in and stores a bounded body copy.
type DiagnosticCapture struct {
	mu       sync.Mutex
	response *DiagnosticResponse
}

func NewDiagnosticCapture() *DiagnosticCapture { return &DiagnosticCapture{} }

func WithDiagnosticCapture(ctx context.Context, capture *DiagnosticCapture) context.Context {
	return context.WithValue(ctx, diagnosticCaptureKey{}, capture)
}

func (c *Client) captureResponse(ctx context.Context, rawURL string, response *http.Response, body []byte, readErr error) {
	capture, _ := ctx.Value(diagnosticCaptureKey{}).(*DiagnosticCapture)
	if capture == nil || response == nil {
		return
	}
	bytesRead := int64(len(body))
	hash := CapturedBodyHash(body)
	var tooLarge *BodyTooLargeError
	var bodyReadErr *BodyReadError
	if errors.As(readErr, &tooLarge) {
		bytesRead, hash = tooLarge.BytesRead, tooLarge.BodyHash
	} else if errors.As(readErr, &bodyReadErr) {
		bytesRead = bodyReadErr.BytesRead
	}
	diagnostic := DiagnosticResponse{
		URL: sanitizeDiagnosticURL(rawURL), StatusCode: response.StatusCode,
		ContentType: response.Header.Get("Content-Type"), ReceivedAt: time.Now().UTC(),
		BodyHash: hash, BytesRead: bytesRead, BodyTruncated: IsBodyTruncated(readErr),
	}
	if len(body) > MaxDiagnosticBodyBytes {
		diagnostic.Body = append([]byte(nil), body[:MaxDiagnosticBodyBytes]...)
		diagnostic.StorageTruncated = true
	} else {
		diagnostic.Body = append([]byte(nil), body...)
	}
	capture.mu.Lock()
	capture.response = &diagnostic
	capture.mu.Unlock()
}

func (c *DiagnosticCapture) Last() (DiagnosticResponse, bool) {
	if c == nil {
		return DiagnosticResponse{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.response == nil {
		return DiagnosticResponse{}, false
	}
	response := *c.response
	response.Body = append([]byte(nil), c.response.Body...)
	return response, true
}

func sanitizeDiagnosticURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid-url>"
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

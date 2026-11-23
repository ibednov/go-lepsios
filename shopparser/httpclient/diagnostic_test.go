package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiagnosticCaptureBoundsAndSanitizesResponse(t *testing.T) {
	body := strings.Repeat("x", 200<<10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	capture := NewDiagnosticCapture()
	ctx := WithDiagnosticCapture(context.Background(), capture)
	client := New(Config{})
	_, response, err := client.GetWithHeaders(ctx, server.URL+"/product?token=secret#frag", nil)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("response=%v err=%v", response, err)
	}
	diagnostic, ok := capture.Last()
	if !ok {
		t.Fatal("diagnostic response not captured")
	}
	if strings.Contains(diagnostic.URL, "secret") || strings.Contains(diagnostic.URL, "#") || strings.Contains(diagnostic.URL, "?") {
		t.Fatalf("unsafe diagnostic URL %q", diagnostic.URL)
	}
	if len(diagnostic.Body) != MaxDiagnosticBodyBytes || !diagnostic.StorageTruncated || diagnostic.BodyTruncated {
		t.Fatalf("body length=%d storage_truncated=%v body_truncated=%v", len(diagnostic.Body), diagnostic.StorageTruncated, diagnostic.BodyTruncated)
	}
	if diagnostic.BytesRead != int64(len(body)) || diagnostic.BodyHash != CapturedBodyHash([]byte(body)) {
		t.Fatalf("read metadata bytes=%d hash_matches=%v", diagnostic.BytesRead, diagnostic.BodyHash == CapturedBodyHash([]byte(body)))
	}
	copy := diagnostic
	copy.Body[0] = 'y'
	again, _ := capture.Last()
	if again.Body[0] != 'x' {
		t.Fatal("Last returned a mutable reference to the retained body")
	}
}

func TestDiagnosticCaptureMarksHTTPReadTruncation(t *testing.T) {
	body := strings.Repeat("tea", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer server.Close()

	capture := NewDiagnosticCapture()
	ctx := WithDiagnosticCapture(context.Background(), capture)
	client := New(Config{MaxBodyBytes: 20})
	_, _, err := client.GetWithHeaders(ctx, server.URL+"/product", nil)
	if err == nil {
		t.Fatal("oversized response should fail")
	}
	diagnostic, ok := capture.Last()
	if !ok || !diagnostic.BodyTruncated || diagnostic.StorageTruncated || diagnostic.BytesRead != 21 {
		t.Fatalf("diagnostic=%+v found=%v", diagnostic, ok)
	}
	if diagnostic.BodyHash != CapturedBodyHash([]byte(body[:21])) {
		t.Fatal("truncated body hash does not cover the bytes actually read")
	}
}

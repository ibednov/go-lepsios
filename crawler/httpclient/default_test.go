package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultMaxBodyBytesAllowsTenMebibytes(t *testing.T) {
	if DefaultMaxBodyBytes != 10<<20 {
		t.Fatalf("DefaultMaxBodyBytes=%d, want 10 MiB", DefaultMaxBodyBytes)
	}
	payload := make([]byte, 5<<20)
	for i := range payload {
		payload[i] = 'x'
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	body, _, err := New(Config{DefaultRPS: 1000}).Get(t.Context(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(body)) != int64(len(payload)) {
		t.Fatalf("read %d bytes, want %d", len(body), len(payload))
	}
}

func TestDefaultMaxBodyBytesStillCapsOversizedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.CopyN(w, zeroReader{}, DefaultMaxBodyBytes+1)
	}))
	defer server.Close()
	_, _, err := New(Config{DefaultRPS: 1000}).Get(t.Context(), server.URL)
	var tooLarge *BodyTooLargeError
	if err == nil {
		t.Fatal("expected truncation")
	}
	if !IsBodyTruncated(err) {
		t.Fatalf("err=%v, want truncation", err)
	}
	_ = tooLarge
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

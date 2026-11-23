package httpclient

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBodyLimitReportsReadBytesAndPrefix(t *testing.T) {
	fullBody := []byte("0123456789abcdef")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(fullBody)
	}))
	defer server.Close()

	body, response, err := New(Config{MaxBodyBytes: 8}).Get(context.Background(), server.URL)
	var tooLarge *BodyTooLargeError
	if !errors.As(err, &tooLarge) || response == nil {
		t.Fatalf("expected typed size error and response, got response=%v err=%v", response, err)
	}
	if !bytes.Equal(body, fullBody[:8]) {
		t.Fatalf("returned body=%q, expected bounded prefix", body)
	}
	if tooLarge.BytesRead != 9 || tooLarge.BodyHash != CapturedBodyHash(fullBody[:9]) {
		t.Fatalf("read metadata bytes=%d hash_matches=%v", tooLarge.BytesRead, tooLarge.BodyHash == CapturedBodyHash(fullBody[:9]))
	}
}

func TestSameOriginRedirectPolicyRejectsCrossOriginBeforeFollowing(t *testing.T) {
	var followed bool
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		followed = true
		w.WriteHeader(http.StatusTeapot)
	}))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL, http.StatusFound)
	}))
	defer first.Close()

	cli := New(Config{SameOriginRedirectsOnly: true})
	_, response, err := cli.Get(context.Background(), first.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect response, got %d", response.StatusCode)
	}
	if followed {
		t.Fatal("client followed redirect to another origin")
	}
}

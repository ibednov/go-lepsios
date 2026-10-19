package centrifugo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublishReturnsAPIErrorOnHTTP200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/publish" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"error":{"code":102,"message":"unknown channel"}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{APIURL: server.URL + "/api", APIKey: "test"})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Publish(context.Background(), "wishimi:user_1", map[string]string{"type": "ping"})
	if err == nil || !strings.Contains(err.Error(), "unknown channel") {
		t.Fatalf("expected Centrifugo API error, got %v", err)
	}
}

func TestBroadcastReturnsPerChannelAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/broadcast" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"responses":[{}, {"error":{"code":102,"message":"unknown channel"}}]}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{APIURL: server.URL + "/api", APIKey: "test"})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Broadcast(context.Background(), []string{"wishimi:user_1", "wishimi:user_2"}, map[string]string{"type": "inbox.changed"})
	if err == nil || !strings.Contains(err.Error(), "unknown channel") {
		t.Fatalf("expected per-channel Centrifugo API error, got %v", err)
	}
}

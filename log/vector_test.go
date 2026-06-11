package log_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/log"
	"github.com/stretchr/testify/require"
)

func TestSetupForwardsToVectorInLocalEnv(t *testing.T) {
	var (
		mu    sync.Mutex
		body  string
		ready = make(chan struct{})
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		payload, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		mu.Lock()
		body = string(payload)
		mu.Unlock()
		close(ready)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	l, err := log.Setup(log.Config{
		Env:         "local",
		ServiceName: "test",
		Level:       "info",
		VectorURL:   server.URL,
	}, log.WithWriter(io.Discard))
	require.NoError(t, err)

	l.Info().Str("event", "ping").Msg("hello vector")

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("vector sink did not receive log")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Contains(t, body, "hello vector")
	require.Contains(t, body, "ping")
}

func TestVectorURLDisabled(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	l, err := log.Setup(log.Config{
		Env:         "local",
		ServiceName: "test",
		Level:       "info",
		VectorURL:   "disabled",
	}, log.WithWriter(io.Discard))
	require.NoError(t, err)

	l.Info().Msg("no forward")
	time.Sleep(100 * time.Millisecond)
	require.False(t, called)
}

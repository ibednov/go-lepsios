package log

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

type vectorHTTPWriter struct {
	url    string
	client *http.Client
}

func newVectorHTTPWriter(url string) io.Writer {
	return &vectorHTTPWriter{
		url:    strings.TrimRight(strings.TrimSpace(url), "/"),
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (w *vectorHTTPWriter) Write(p []byte) (int, error) {
	if w.url == "" || len(p) == 0 {
		return len(p), nil
	}

	payload := bytes.TrimSpace(p)
	go func() {
		req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(payload))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		_, _ = w.client.Do(req)
	}()

	return len(p), nil
}

func resolveVectorURL(cfg Config) string {
	value := strings.TrimSpace(cfg.VectorURL)
	if value != "" {
		switch strings.ToLower(value) {
		case "false", "disabled", "off", "0":
			return ""
		default:
			return value
		}
	}

	if isConsoleEnv(cfg.Env) {
		return "http://127.0.0.1:8687"
	}

	return ""
}

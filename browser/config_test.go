package browser

import (
	"testing"
	"time"
)

func TestConfigFromEnvAliases(t *testing.T) {
	t.Setenv("BROWSER_LISTEN", "")
	t.Setenv("PARSER_BROWSER_LISTEN", ":9000")
	t.Setenv("BROWSER_TOKEN", "")
	t.Setenv("PARSER_BROWSER_TOKEN", "tok")
	t.Setenv("PARSER_BROWSER_TIMEOUT", "30s")
	t.Setenv("PARSER_BROWSER_WARMUP", "2s")
	t.Setenv("PARSER_BROWSER_HEADLESS", "false")
	t.Setenv("PARSER_BROWSER_ALLOWED_HOSTS", "example.com")
	t.Setenv("BROWSER_HTTP_PROXY", "")
	t.Setenv("PARSER_HTTP_PROXY", "http://user:pass@proxy.example:8000")

	cfg := ConfigFromEnv()
	if cfg.Listen != ":9000" || cfg.Token != "tok" || cfg.Headless || cfg.Timeout != 30*time.Second || cfg.Warmup != 2*time.Second {
		t.Fatalf("%+v", cfg)
	}
	if len(cfg.AllowedHosts) != 1 || cfg.AllowedHosts[0] != "example.com" {
		t.Fatalf("%v", cfg.AllowedHosts)
	}
	if cfg.ProxyURL != "http://user:pass@proxy.example:8000" {
		t.Fatalf("proxy: %s", cfg.ProxyURL)
	}
}

func TestProxyFromURL(t *testing.T) {
	if proxyFromURL("") != nil {
		t.Fatal("empty")
	}
	p := proxyFromURL("http://user:secret@127.0.0.1:8000")
	if p == nil || p.Server != "http://127.0.0.1:8000" || p.Username == nil || *p.Username != "user" {
		t.Fatalf("%+v", p)
	}
}

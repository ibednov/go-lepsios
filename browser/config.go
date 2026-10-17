package browser

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is env for the standalone sidecar binary.
type Config struct {
	Listen         string
	Token          string
	Headless       bool
	Timeout        time.Duration
	Warmup         time.Duration
	ExecutablePath string
	AllowedHosts   []string
}

// ConfigFromEnv reads BROWSER_* keys, with PARSER_BROWSER_* aliases for Wishimi.
func ConfigFromEnv() Config {
	timeout, _ := time.ParseDuration(firstEnv("45s", "BROWSER_TIMEOUT", "PARSER_BROWSER_TIMEOUT"))
	warmup, _ := time.ParseDuration(firstEnv("8s", "BROWSER_WARMUP", "PARSER_BROWSER_WARMUP"))
	headless := parseBool(firstEnv("true", "BROWSER_HEADLESS", "PARSER_BROWSER_HEADLESS"), true)
	return Config{
		Listen:         firstEnv(":8090", "BROWSER_LISTEN", "PARSER_BROWSER_LISTEN"),
		Token:          firstEnv("", "BROWSER_TOKEN", "PARSER_BROWSER_TOKEN"),
		Headless:       headless,
		Timeout:        timeout,
		Warmup:         warmup,
		ExecutablePath: firstEnv("", "BROWSER_EXECUTABLE", "PARSER_BROWSER_EXECUTABLE"),
		AllowedHosts:   ParseAllowedHosts(firstEnv("", "BROWSER_ALLOWED_HOSTS", "PARSER_BROWSER_ALLOWED_HOSTS")),
	}
}

func firstEnv(fallback string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return fallback
}

func parseBool(raw string, fallback bool) bool {
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

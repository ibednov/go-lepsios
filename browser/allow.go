package browser

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
)

var (
	allowedMu             sync.RWMutex
	allowedHostSuffixes   = defaultAllowedHostSuffixes()
	defaultHostSuffixList = []string{
		"ozon.ru",
		"ozon.by",
		"wildberries.ru",
		"wildberries.by",
		"wb.ru",
	}
)

func defaultAllowedHostSuffixes() []string {
	out := make([]string, len(defaultHostSuffixList))
	copy(out, defaultHostSuffixList)
	return out
}

// ParseAllowedHosts splits a comma-separated host suffix list.
func ParseAllowedHosts(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		host := strings.ToLower(strings.TrimSpace(part))
		host = strings.TrimPrefix(host, ".")
		if host == "" {
			continue
		}
		out = append(out, host)
	}
	return out
}

// SetAllowedHostSuffixes replaces the SSRF allowlist. Empty resets to defaults.
func SetAllowedHostSuffixes(suffixes []string) {
	allowedMu.Lock()
	defer allowedMu.Unlock()
	if len(suffixes) == 0 {
		allowedHostSuffixes = defaultAllowedHostSuffixes()
		return
	}
	allowedHostSuffixes = append([]string(nil), suffixes...)
}

// CheckAllowedURL allows only http(s) URLs whose host matches the allowlist.
// The sidecar is an internal fetch proxy: without an allowlist this is SSRF.
func CheckAllowedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%w: invalid url", ErrHostNotAllowed)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: scheme %s", ErrHostNotAllowed, u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || net.ParseIP(host) != nil {
		return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}

	allowedMu.RLock()
	suffixes := allowedHostSuffixes
	allowedMu.RUnlock()

	for _, suffix := range suffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
}

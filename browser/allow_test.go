package browser

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckAllowedURL(t *testing.T) {
	tests := []struct {
		raw     string
		wantErr bool
	}{
		{"https://ozon.by/product/x-1/", false},
		{"https://www.ozon.ru/product/x-1/", false},
		{"https://www.wildberries.ru/catalog/1/detail.aspx", false},
		{"https://card.wb.ru/cards/v4/detail", false},
		{"https://example.com/", true},
		{"https://127.0.0.1/x", true},
		{"file:///etc/passwd", true},
		{"not a url", true},
	}
	for _, tt := range tests {
		err := CheckAllowedURL(tt.raw)
		if tt.wantErr && err == nil {
			t.Fatalf("%s: expected deny", tt.raw)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("%s: %v", tt.raw, err)
		}
		if tt.wantErr && err != nil && !errors.Is(err, ErrHostNotAllowed) {
			t.Fatalf("%s: want ErrHostNotAllowed, got %v", tt.raw, err)
		}
	}
}

func TestSetAllowedHostSuffixes(t *testing.T) {
	t.Cleanup(func() { SetAllowedHostSuffixes(nil) })

	SetAllowedHostSuffixes([]string{"example.com"})
	if err := CheckAllowedURL("https://shop.example.com/x"); err != nil {
		t.Fatalf("custom allow: %v", err)
	}
	if err := CheckAllowedURL("https://ozon.by/x"); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("default dropped: %v", err)
	}

	SetAllowedHostSuffixes(nil)
	if err := CheckAllowedURL("https://ozon.by/x"); err != nil {
		t.Fatalf("reset: %v", err)
	}
}

func TestLooksLikeChallengeAndReady(t *testing.T) {
	if !LooksLikeChallenge(`<html>Antibot Challenge Page</html>`) {
		t.Fatal("challenge html")
	}
	if !LooksLikeChallenge(`{"incidentId":"x","challengeURL":"/c"}`) {
		t.Fatal("challenge json")
	}
	ready := `{"widgetStates":{"webProductHeading":"{}"}}`
	if LooksLikeChallenge(ready) || !LooksLikeReady(ready) {
		t.Fatal("ready composer")
	}
	shell := strings.Repeat("x", 9000) + `<meta property="og:title" content="Ozon">`
	if LooksLikeReady(shell) {
		t.Fatal("shell html is not a product")
	}
}

func TestResponseMatchesRequest(t *testing.T) {
	req := "https://ozon.by/api/composer-api.bx/page/json/v2?url=/product/x-1/"
	if !responseMatchesRequest("https://www.ozon.by/api/composer-api.bx/page/json/v2?url=/product/x-1/&__rr=1", req) {
		t.Fatal("composer glob")
	}
	if !responseMatchesRequest(req, req) {
		t.Fatal("exact")
	}
	if responseMatchesRequest("https://ozon.by/product/x-1/", req) {
		t.Fatal("html is not composer")
	}
}

func TestOriginHome(t *testing.T) {
	if got := originHome("https://ozon.by/product/x-1/?q=1"); got != "https://ozon.by/" {
		t.Fatalf("home: %s", got)
	}
}

func TestNetworkCaptureSkipsChallenge(t *testing.T) {
	var heard networkCapture
	heard.put(200, []byte(`{"incidentId":"x","challengeURL":"/c"}`))
	if heard.has() {
		t.Fatal("challenge captured")
	}
	heard.put(200, []byte(`{"widgetStates":{"webProductHeading":"{}"}}`))
	if !heard.has() {
		t.Fatal("ready not captured")
	}
}

func TestParseAllowedHosts(t *testing.T) {
	got := ParseAllowedHosts(" ozon.by, .wb.ru,,Example.COM ")
	if len(got) != 3 || got[0] != "ozon.by" || got[1] != "wb.ru" || got[2] != "example.com" {
		t.Fatalf("%v", got)
	}
}

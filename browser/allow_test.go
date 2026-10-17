package browser

import (
	"errors"
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

func TestParseAllowedHosts(t *testing.T) {
	got := ParseAllowedHosts(" ozon.by, .wb.ru,,Example.COM ")
	if len(got) != 3 || got[0] != "ozon.by" || got[1] != "wb.ru" || got[2] != "example.com" {
		t.Fatalf("%v", got)
	}
}

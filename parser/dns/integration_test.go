package dns

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestIntegrationParseLiveProduct(t *testing.T) {
	if os.Getenv("DNS_INTEGRATION") == "" {
		t.Skip("set DNS_INTEGRATION=1 to run live DNS parse test")
	}

	s := NewStrategy()
	pageURL := "https://dns-shop.by/ru/product/0d7f3e4e5691ed20/klaviatura-provodnaa-besprovodnaa-8bitdo-retro-mechanical-keyboard-n-edition/"

	result, err := s.Parse(context.Background(), pageURL)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if result.Title == "" {
		t.Fatal("empty title")
	}
	if result.Price == nil || *result.Price <= 0 {
		t.Fatal("missing price")
	}
	if result.ImageURL == nil || *result.ImageURL == "" {
		t.Fatal("missing image")
	}
	if !strings.Contains(*result.ImageURL, "c.dns-shop.ru") {
		t.Fatalf("unexpected image url: %s", *result.ImageURL)
	}
}

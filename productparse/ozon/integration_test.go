//go:build integration

package ozon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

func TestIntegrationParseCard(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := NewStrategy(nil)
	urls := []string{
		"https://ozon.by/product/umnyy-kontroller-rele-smart-life-s-golosovym-upravleniem-alisa-16a-3255813434/",
		"https://ozon.by/t/BSawbtL",
	}

	for _, pageURL := range urls {
		t.Run(pageURL, func(t *testing.T) {
			result, err := s.ParseCard(ctx, pageURL)
			if err != nil {
				if errors.Is(err, errAccessDenied) ||
					strings.Contains(strings.ToLower(err.Error()), "access denied") ||
					strings.Contains(strings.ToLower(err.Error()), "antibot") ||
					strings.Contains(err.Error(), "status 403") {
					t.Skip("ozon antibot blocked server-side request; parser works with browser session")
				}
				t.Fatalf("ParseCard failed: %v", err)
			}

			if result.Title == "" {
				t.Fatal("expected non-empty title")
			}
			if result.Price == nil || *result.Price <= 0 {
				t.Fatal("expected positive price")
			}
			if result.Currency != currency.BYN {
				t.Fatalf("expected BYN currency, got %s", result.Currency)
			}
		})
	}
}

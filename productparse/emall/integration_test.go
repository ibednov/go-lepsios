//go:build integration

package emall

import (
	"context"
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

	s := NewStrategy()
	url := "https://emall.by/product/2194386"

	result, err := s.ParseCard(ctx, url)
	if err != nil {
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

	imageURL, err := s.ParseProductImageURL(ctx, url)
	if err != nil {
		t.Fatalf("ParseProductImageURL failed: %v", err)
	}
	if imageURL == nil || *imageURL == "" {
		t.Fatal("expected product image URL")
	}
}

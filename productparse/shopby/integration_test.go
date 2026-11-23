//go:build integration

package shopby

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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	s := NewStrategy()
	url := "https://shop.by/multimedia_proigryvateli/apple_tv_4k_64gb_3_e_pokolenie/"

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

package steam

import (
	"context"
	"os"
	"testing"
)

func TestIntegrationParseLiveProduct(t *testing.T) {
	if os.Getenv("STEAM_INTEGRATION") == "" {
		t.Skip("set STEAM_INTEGRATION=1 to run live Steam parse test")
	}

	s := NewStrategy()
	pageURL := "https://store.steampowered.com/app/2322010/God_of_War_Ragnaryok/"

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
}

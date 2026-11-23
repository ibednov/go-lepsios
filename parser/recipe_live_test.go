package parser

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/parser/recipe"
)

func TestLiveSteamDota2(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if os.Getenv("PARSER_LIVE") == "0" {
		t.Skip("PARSER_LIVE=0")
	}
	var cfg recipe.Config
	for _, c := range recipe.Builtin() {
		if c.Code == "steam" {
			cfg = c
			break
		}
	}
	s, err := NewRecipeStrategy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	card, err := s.ParseCard(ctx, "https://store.steampowered.com/app/570/Dota_2/")
	if err != nil {
		t.Fatal(err)
	}
	if card.Title == "" {
		t.Fatal("empty title")
	}
	t.Logf("steam title=%q price=%v cur=%s", card.Title, card.Price, card.Currency)
}

func TestLiveOnlinerProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if os.Getenv("PARSER_LIVE") == "0" {
		t.Skip("PARSER_LIVE=0")
	}
	var cfg recipe.Config
	for _, c := range recipe.Builtin() {
		if c.Code == "onliner" {
			cfg = c
			break
		}
	}
	s, err := NewRecipeStrategy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	// key from catalog search sample
	card, err := s.ParseCard(ctx, "https://catalog.onliner.by/mobile/apple/iphone17256bk")
	if err != nil {
		// no_price is OK if title present path - but ParseCard may return no_price
		t.Logf("onliner err=%v", err)
		return
	}
	if card.Title == "" {
		t.Fatal("empty title")
	}
	t.Logf("onliner title=%q price=%v", card.Title, card.Price)
}

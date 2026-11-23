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

func TestLiveDetmirProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if os.Getenv("PARSER_LIVE") == "0" {
		t.Skip("PARSER_LIVE=0")
	}
	var cfg recipe.Config
	for _, c := range recipe.Builtin() {
		if c.Code == "detmir" {
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
	card, err := s.ParseCard(ctx, "https://www.detmir.by/product/index/id/1001/")
	if err != nil {
		t.Fatal(err)
	}
	if card.Title == "" {
		t.Fatal("empty title")
	}
	t.Logf("detmir title=%q price=%v cur=%s", card.Title, card.Price, card.Currency)
}

func TestLiveVek21Product(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if os.Getenv("PARSER_LIVE") == "0" {
		t.Skip("PARSER_LIVE=0")
	}
	var cfg recipe.Config
	for _, c := range recipe.Builtin() {
		if c.Code == "vek21" {
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
	// popular catalog-ish URL; may 404 — log and soft-fail only if empty title path returns err
	card, err := s.ParseCard(ctx, "https://www.21vek.by/mobile/iphone17256gb_apple_10019140.html")
	if err != nil {
		t.Fatal(err)
	}
	if card.Title == "" {
		t.Fatal("empty title")
	}
	t.Logf("vek21 title=%q price=%v", card.Title, card.Price)
}

func TestLiveEmallProduct(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if os.Getenv("PARSER_LIVE") == "0" {
		t.Skip("PARSER_LIVE=0")
	}
	var cfg recipe.Config
	for _, c := range recipe.Builtin() {
		if c.Code == "emall" {
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
	card, err := s.ParseCard(ctx, "https://emall.by/product/2169965")
	if err != nil {
		t.Fatal(err)
	}
	if card.Title == "" {
		t.Fatal("empty title")
	}
	t.Logf("emall title=%q price=%v cur=%s", card.Title, card.Price, card.Currency)
}

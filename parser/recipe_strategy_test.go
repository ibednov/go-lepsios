package parser

import (
	"testing"

	"github.com/ibednov/go-lepsios/parser/recipe"
)

func TestRecipeStrategiesBuiltin(t *testing.T) {
	strats, err := RecipeStrategies(recipe.Builtin()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(strats) != 8 {
		t.Fatalf("len=%d", len(strats))
	}
	names := map[string]bool{}
	for _, s := range strats {
		if n, ok := s.(Named); ok {
			names[n.Name()] = true
		}
	}
	for _, want := range []string{"emall", "shopby", "ozby", "dns", "vek21", "steam", "onliner", "detmir"} {
		if !names[want] {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
}

func TestLastSegmentKey(t *testing.T) {
	s, err := NewRecipeStrategy(recipe.Config{
		Code: "onliner", Hosts: []string{"catalog.onliner.by"},
		PathKeyMode: "last_segment",
		RejectPathRoots: []string{"cat", "search"},
		StripPathSuffixes: []string{"prices", "reviews"},
		APIURLTemplate: "https://x/{id}",
		APITitlePaths: []string{"name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := s.pathProductID("https://catalog.onliner.by/mobile/apple/iphone15")
	if id != "iphone15" {
		t.Fatalf("id=%q", id)
	}
	id = s.pathProductID("https://catalog.onliner.by/mobile/apple/iphone15/prices")
	if id != "iphone15" {
		t.Fatalf("prices id=%q", id)
	}
	if err := s.validateURL("https://catalog.onliner.by/cat/foo"); err == nil {
		t.Fatal("expected reject cat")
	}
}

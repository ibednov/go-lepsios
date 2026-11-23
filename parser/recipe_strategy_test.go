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
	if len(strats) != 5 {
		t.Fatalf("len=%d", len(strats))
	}
	names := map[string]bool{}
	for _, s := range strats {
		if n, ok := s.(Named); ok {
			names[n.Name()] = true
		}
	}
	for _, want := range []string{"emall", "shopby", "ozby", "dns", "vek21"} {
		if !names[want] {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
}

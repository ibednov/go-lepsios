package storeapi

import (
	"testing"
)

func TestWeightFromVariationParsesGarbataSlugs(t *testing.T) {
	for _, tc := range []struct {
		attr string
		want int
	}{
		{attr: "25-gr", want: 25},
		{attr: "50-gr", want: 50},
		{attr: "100-gr", want: 100},
		{attr: "357-gr", want: 357},
		{attr: "50 гр", want: 50},
		{attr: "Блин 357 гр", want: 357},
		{attr: "100g", want: 100},
		{attr: "25", want: 25},
	} {
		got, absent := weightFromVariation(wooVariation{Attributes: map[string]string{"attribute_pa_ves": tc.attr}})
		if absent || got == nil || *got != tc.want {
			t.Errorf("%q: weight=%v absent=%v, want %d", tc.attr, got, absent, tc.want)
		}
	}
}

func TestWeightFromVariationSkipsPricePerKgAndYears(t *testing.T) {
	got, absent := weightFromVariation(wooVariation{Attributes: map[string]string{
		"attribute_pa_cena-za-kg": "80",
		"attribute_pa_ves":        "2009г",
	}})
	if got != nil || absent {
		t.Fatalf("year/price attrs parsed as grams: %v absent=%v", got, absent)
	}
}

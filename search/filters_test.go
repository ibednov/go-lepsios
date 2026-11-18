package search

import "testing"

func TestFilterBuilderQuotesValuesAndGroupsAlternatives(t *testing.T) {
	builder := NewFilterBuilder()
	if err := builder.Equal("kind", `tea" OR kind = "coffee`); err != nil {
		t.Fatal(err)
	}
	if err := builder.Any("attributes.color", []string{"red", "green"}); err != nil {
		t.Fatal(err)
	}
	got := builder.Build()
	want := []string{`kind = "tea\" OR kind = \"coffee"`, `(attributes.color = "red" OR attributes.color = "green")`}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Build() = %#v, want %#v", got, want)
	}
}

func TestFilterBuilderRejectsInvalidAttribute(t *testing.T) {
	if err := NewFilterBuilder().Equal("kind OR true", "tea"); err == nil {
		t.Fatal("Equal accepted an invalid attribute")
	}
}

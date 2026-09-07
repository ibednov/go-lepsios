package chart

import (
	"testing"
)

func TestHorizontalBars(t *testing.T) {
	png, err := HorizontalBars("Траты", []Slice{
		{Label: "Еда и напитки", Value: 50000, Pct: 50},
		{Label: "Транспорт", Value: 30000, Pct: 30},
		{Label: "Жильё", Value: 20000, Pct: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 100 {
		t.Fatalf("png too small: %d", len(png))
	}
	if png[0] != 0x89 || png[1] != 'P' {
		t.Fatalf("not a png header")
	}
}

package dates

import (
	"testing"
	"time"
)

func TestParseRange(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	from, to, err := ParseRange("01.09.2026 - 30.09.2026", loc)
	if err != nil {
		t.Fatal(err)
	}
	wantFrom := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	wantTo := time.Date(2026, 9, 31, 0, 0, 0, 0, loc) // exclusive end
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Errorf("got from=%v to=%v, want %v/%v", from, to, wantFrom, wantTo)
	}
	if _, _, err := ParseRange("bad", loc); err == nil {
		t.Error("expected error for bad input")
	}
	// перевернулий диапазон — должен нормализоваться
	f2, t2, err := ParseRange("30.09.2026-01.09.2026", loc)
	if err != nil || !f2.Equal(wantFrom) {
		t.Errorf("reverse range: got %v/%v err=%v", f2, t2, err)
	}
}

func TestPeriod(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, loc)
	cases := map[string]struct{ from, to time.Time }{
		"today": {time.Date(2026, 9, 16, 0, 0, 0, 0, loc), time.Date(2026, 9, 17, 0, 0, 0, 0, loc)},
		"week":  {time.Date(2026, 9, 14, 0, 0, 0, 0, loc), time.Date(2026, 9, 21, 0, 0, 0, 0, loc)}, // Wed -> Mon
		"month": {time.Date(2026, 9, 1, 0, 0, 0, 0, loc), time.Date(2026, 10, 1, 0, 0, 0, 0, loc)},
		"year":  {time.Date(2025, 9, 16, 0, 0, 0, 0, loc), time.Date(2026, 9, 17, 0, 0, 0, 0, loc)},
	}
	for name, want := range cases {
		from, to := Period(name, now, loc)
		if !from.Equal(want.from) || !to.Equal(want.to) {
			t.Errorf("%s: got %v/%v want %v/%v", name, from, to, want.from, want.to)
		}
	}
}

func TestParseDay(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	got, err := ParseDay("05.09.2026", loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-09-05" {
		t.Fatalf("got %v", got)
	}
	if _, err := ParseDay("bad", loc); err == nil {
		t.Fatal("expected error")
	}
}

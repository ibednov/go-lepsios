package money

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"450", 45000, false},
		{"450.5", 45050, false},
		{"450,50", 45050, false},
		{"1 230,55", 123055, false},
		{"0.5", 50, false},
		{"-100", -10000, false},
		{"0", 0, true},
		{"abc", 0, true},
		{"1.2.3", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("Parse(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "0 BYN"},
		{45000, "450 BYN"},
		{45050, "450,50 BYN"},
		{123055, "1 230,55 BYN"},
		{-123055, "-1 230,55 BYN"},
	}
	for _, c := range cases {
		if Format(c.cents) != c.want {
			t.Errorf("Format(%d) got %q want %q", c.cents, Format(c.cents), c.want)
		}
	}
}

func TestFormatCur(t *testing.T) {
	if got := FormatCur(17000, "AZN"); got != "170 AZN" {
		t.Errorf("FormatCur AZN got %q", got)
	}
	if got := FormatCur(10050, "USD"); got != "100,50 USD" {
		t.Errorf("FormatCur USD got %q", got)
	}
}

func TestParseLocaleSpace(t *testing.T) {
	got, err := Parse("1" + "\u00a0" + "000,50")
	if err != nil || got != 100050 {
		t.Fatalf("got %d err=%v", got, err)
	}
	if !strings.Contains(Format(got), "1 000,50") {
		t.Errorf("unexpected format: %q", Format(got))
	}
}

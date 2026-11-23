package pricing

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPriceFromMinorUnits(t *testing.T) {
	cases := []struct {
		raw   string
		minor int
		want  string
	}{
		{"8600", 2, "86"},
		{"9990", 2, "99.9"},
		{"380", 1, "38"},
	}
	for _, tc := range cases {
		got, err := PriceFromMinorUnits(tc.raw, tc.minor)
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		want := decimal.RequireFromString(tc.want)
		if !got.Equal(want) {
			t.Fatalf("%s minor=%d: got %s want %s", tc.raw, tc.minor, got, want)
		}
	}
}

func TestPricePer100g(t *testing.T) {
	w25 := 25
	w357 := 357
	p := decimal.RequireFromString("22")
	got := PricePer100g(p, &w25)
	if got == nil || !got.Equal(decimal.RequireFromString("88")) {
		t.Fatalf("25g: got %v", got)
	}
	cake := decimal.RequireFromString("80")
	got = PricePer100g(cake, &w357)
	if got == nil || !got.Equal(decimal.RequireFromString("22.41")) {
		t.Fatalf("357g cake: got %v want 22.41", got)
	}
	if PricePer100g(p, nil) != nil {
		t.Fatal("nil weight should skip")
	}
}

func TestPricePer100gDecimal(t *testing.T) {
	price := decimal.RequireFromString("5")
	weight := decimal.RequireFromString("45")
	got := PricePer100gDecimal(price, &weight)
	if got == nil || !got.Equal(decimal.RequireFromString("11.11")) {
		t.Fatalf("15 × 3g: got %v, want 11.11", got)
	}
	weight = decimal.RequireFromString("37.5")
	got = PricePer100gDecimal(price, &weight)
	if got == nil || !got.Equal(decimal.RequireFromString("13.33")) {
		t.Fatalf("25 × 1.5g: got %v, want 13.33", got)
	}
	zero := decimal.Zero
	if PricePer100gDecimal(price, &zero) != nil {
		t.Fatal("zero weight must skip normalized price")
	}
}

func TestPriceFlagThreshold(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{value: "1999.99", want: ""},
		{value: "2000", want: ""},
		{value: "2000.01", want: FlagOutlier},
	} {
		value := decimal.RequireFromString(tc.value)
		if got := PriceFlag(&value); got != tc.want {
			t.Errorf("PriceFlag(%s) = %q, want %q", tc.value, got, tc.want)
		}
	}
	if got := PriceFlag(nil); got != "" {
		t.Fatalf("PriceFlag(nil) = %q", got)
	}
}

func TestParseGramsShopSlugsAndLabels(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "25-gr", want: "25"},
		{in: "50_gr", want: "50"},
		{in: "100-gr", want: "100"},
		{in: "50 гр", want: "50"},
		{in: "Блин 357 гр", want: "357"},
		{in: "100g", want: "100"},
		{in: "25", want: "25"},
		{in: "1 кг", want: "1000"},
	} {
		got := ParseGrams(tc.in)
		if got == nil || got.String() != tc.want {
			t.Errorf("ParseGrams(%q)=%v, want %s", tc.in, got, tc.want)
		}
	}
	if got := ParseGrams("2009г"); got != nil {
		t.Fatalf("year parsed as grams: %v", got)
	}
	if got := ParseGrams(""); got != nil {
		t.Fatal("empty parsed as grams")
	}
}

func TestImplausibleVsMedian(t *testing.T) {
	median := decimal.RequireFromString("27")
	low := decimal.RequireFromString("0.76")
	ok := decimal.RequireFromString("27")
	if !ImplausibleVsMedian(&low, &median) {
		t.Fatal("0.76 vs median 27 must be implausible")
	}
	if ImplausibleVsMedian(&ok, &median) {
		t.Fatal("matching median must not be implausible")
	}
	if ImplausibleVsMedian(&low, nil) || ImplausibleVsMedian(nil, &median) {
		t.Fatal("missing values must not flag")
	}
}

func TestNormalizeCurrency(t *testing.T) {
	c, err := NormalizeCurrency("BYR")
	if err != nil || c != "BYN" {
		t.Fatalf("BYR: %s %v", c, err)
	}
	c, err = NormalizeCurrency("rub")
	if err != nil || c != "RUB" {
		t.Fatalf("RUB: %s %v", c, err)
	}
	if _, err := NormalizeCurrency("KZT"); err == nil {
		t.Fatal("KZT must fail")
	}
}

func TestStaticFXConvertsRUBToBYN(t *testing.T) {
	fx := StaticFX{RUBToBYN: decimal.RequireFromString("0.04")}
	got, err := fx.ToBYN(decimal.NewFromInt(1000), "RUB")
	if err != nil || !got.Equal(decimal.NewFromInt(40)) {
		t.Fatalf("1000 RUB = %s err=%v, want 40 BYN", got, err)
	}
	got, err = fx.ToBYN(decimal.NewFromInt(10), "BYN")
	if err != nil || !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("BYN passthrough = %s err=%v", got, err)
	}
	if _, err := fx.ToBYN(decimal.NewFromInt(10), "USD"); err == nil {
		t.Fatal("USD must require a live official rate")
	}
}

func TestConvertOfficialToBYN(t *testing.T) {
	got, err := ConvertOfficialToBYN(decimal.NewFromInt(100), 100, 3.70)
	if err != nil || !got.Equal(decimal.RequireFromString("3.7")) {
		t.Fatalf("100 RUB at 3.70/100 = %s err=%v, want 3.7", got, err)
	}
}

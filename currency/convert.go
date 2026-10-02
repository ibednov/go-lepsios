package currency

import "math"

// Converter пересчитывает суммы по таблице официальных курсов относительно base.
type Converter struct {
	base       Code
	basePerOne map[Code]float64
}

// NewConverter строит конвертер для базовой валюты (BYN, AZN, …).
// rates перекрывают DefaultOfficialRatesFor(base).
func NewConverter(base Code, rates []OfficialRate) *Converter {
	if !base.IsValid() {
		base = BYN
	}
	merged := make(map[Code]float64, len(DefaultOfficialRatesFor(base))+len(rates)+1)
	merged[base] = 1
	for _, r := range DefaultOfficialRatesFor(base) {
		if r.Code.IsValid() && r.Scale > 0 && r.BasePerUnit > 0 {
			merged[r.Code] = r.BasePerUnit / float64(r.Scale)
		}
	}
	for _, r := range rates {
		if !r.Code.IsValid() || r.Scale <= 0 || r.BasePerUnit <= 0 {
			continue
		}
		merged[r.Code] = r.BasePerUnit / float64(r.Scale)
	}
	merged[base] = 1
	return &Converter{base: base, basePerOne: merged}
}

// DefaultConverter — BYN-хаб с дефолтными курсами.
func DefaultConverter() *Converter {
	return NewConverter(BYN, nil)
}

// Base returns the hub currency.
func (c *Converter) Base() Code {
	if c == nil || !c.base.IsValid() {
		return BYN
	}
	return c.base
}

func (c *Converter) basePerOneFor(code Code) (float64, error) {
	if !code.IsValid() {
		return 0, ErrUnknownCode
	}
	rate, ok := c.basePerOne[code]
	if !ok || rate <= 0 {
		return 0, ErrUnsupportedPair
	}
	return rate, nil
}

// Convert: result = amount * basePerOne(from) / basePerOne(to).
func (c *Converter) Convert(amount float64, from, to Code) (float64, error) {
	if from == to {
		return roundMoney(amount), nil
	}
	fromRate, err := c.basePerOneFor(from)
	if err != nil {
		return 0, err
	}
	toRate, err := c.basePerOneFor(to)
	if err != nil {
		return 0, err
	}
	return roundMoney(amount * fromRate / toRate), nil
}

// Convert uses the default BYN-hub converter.
func Convert(amount float64, from, to Code) (float64, error) {
	return DefaultConverter().Convert(amount, from, to)
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

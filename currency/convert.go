package currency

import "math"

// Converter пересчитывает суммы по таблице официальных курсов (хаб — BYN).
type Converter struct {
	bynPerUnit map[Code]float64
}

func NewConverter(rates []OfficialRate) *Converter {
	merged := make(map[Code]float64, len(defaultOfficialRates())+len(rates))
	for _, r := range defaultOfficialRates() {
		if r.Code.IsValid() && r.Scale > 0 && r.BYNPerUnit > 0 {
			merged[r.Code] = r.BYNPerUnit / float64(r.Scale)
		}
	}
	for _, r := range rates {
		if !r.Code.IsValid() || r.Scale <= 0 || r.BYNPerUnit <= 0 {
			continue
		}
		merged[r.Code] = r.BYNPerUnit / float64(r.Scale)
	}
	return &Converter{bynPerUnit: merged}
}

func DefaultConverter() *Converter {
	return NewConverter(nil)
}

func (c *Converter) bynPerUnitFor(code Code) (float64, error) {
	if !code.IsValid() {
		return 0, ErrUnknownCode
	}
	rate, ok := c.bynPerUnit[code]
	if !ok || rate <= 0 {
		return 0, ErrUnsupportedPair
	}
	return rate, nil
}

// Convert: result = amount * bynPerUnit(from) / bynPerUnit(to).
func (c *Converter) Convert(amount float64, from, to Code) (float64, error) {
	if from == to {
		return roundMoney(amount), nil
	}
	fromRate, err := c.bynPerUnitFor(from)
	if err != nil {
		return 0, err
	}
	toRate, err := c.bynPerUnitFor(to)
	if err != nil {
		return 0, err
	}
	return roundMoney(amount * fromRate / toRate), nil
}

func Convert(amount float64, from, to Code) (float64, error) {
	return DefaultConverter().Convert(amount, from, to)
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

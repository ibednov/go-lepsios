package pl_nbp

import (
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

// NBP table A/B JSON shapes (api.nbp.pl).
type tableDTO struct {
	Table         string    `json:"table"`
	No            string    `json:"no"`
	EffectiveDate string    `json:"effectiveDate"`
	Rates         []rateDTO `json:"rates"`
}

type rateDTO struct {
	Currency string  `json:"currency"`
	Code     string  `json:"code"`
	Mid      float64 `json:"mid"`
}

func ratesFromDTO(items []rateDTO, rateDate time.Time) []currency.OfficialRate {
	out := make([]currency.OfficialRate, 0, len(items)+1)
	for _, item := range items {
		code, err := currency.Parse(strings.TrimSpace(item.Code))
		if err != nil || !code.IsValid() {
			continue
		}
		if item.Mid <= 0 {
			continue
		}
		out = append(out, currency.OfficialRate{
			Code:        code,
			Scale:       1,
			BasePerUnit: item.Mid,
			Date:        rateDate,
		})
	}
	out = append(out, currency.OfficialRate{
		Code:        currency.PLN,
		Scale:       1,
		BasePerUnit: 1,
		Date:        rateDate,
	})
	return out
}

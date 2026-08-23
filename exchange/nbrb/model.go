package nbrb

import (
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

type rateDTO struct {
	CurAbbreviation string  `json:"Cur_Abbreviation"`
	CurScale        int     `json:"Cur_Scale"`
	CurOfficialRate float64 `json:"Cur_OfficialRate"`
	Date            string  `json:"Date"`
}

func ratesFromDTO(items []rateDTO, rateDate time.Time) []currency.OfficialRate {
	out := make([]currency.OfficialRate, 0, len(items)+1)
	for _, item := range items {
		code, err := currency.Parse(strings.TrimSpace(item.CurAbbreviation))
		if err != nil || !code.IsValid() {
			continue
		}
		if item.CurScale <= 0 || item.CurOfficialRate <= 0 {
			continue
		}
		d := rateDate
		if item.Date != "" {
			if parsed, err := time.Parse(time.RFC3339, item.Date); err == nil {
				d = parsed
			}
		}
		out = append(out, currency.OfficialRate{
			Code:       code,
			Scale:      item.CurScale,
			BYNPerUnit: item.CurOfficialRate,
			Date:       d,
		})
	}
	out = append(out, currency.OfficialRate{
		Code:       currency.BYN,
		Scale:      1,
		BYNPerUnit: 1,
		Date:       rateDate,
	})
	return out
}

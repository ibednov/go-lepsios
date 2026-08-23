package currency

import "time"

// DefaultOfficialRates — fallback при недоступности провайдера курсов (приближённые дневные курсы).
func DefaultOfficialRates() []OfficialRate {
	return defaultOfficialRates()
}

func defaultOfficialRates() []OfficialRate {
	now := time.Now().UTC()
	return []OfficialRate{
		{Code: BYN, Scale: 1, BYNPerUnit: 1, Date: now},
		{Code: USD, Scale: 1, BYNPerUnit: 3.27, Date: now},
		{Code: EUR, Scale: 1, BYNPerUnit: 3.55, Date: now},
		{Code: RUB, Scale: 100, BYNPerUnit: 3.55, Date: now},
		{Code: KZT, Scale: 1000, BYNPerUnit: 7.27, Date: now},
		{Code: CNY, Scale: 10, BYNPerUnit: 4.55, Date: now},
	}
}

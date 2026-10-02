package currency

import "time"

// DefaultOfficialRates — fallback BYN-хаба при недоступности провайдера.
func DefaultOfficialRates() []OfficialRate {
	return DefaultOfficialRatesFor(BYN)
}

// DefaultOfficialRatesFor — приближённые дневные курсы относительно base.
func DefaultOfficialRatesFor(base Code) []OfficialRate {
	now := time.Now().UTC()
	switch base {
	case AZN:
		return []OfficialRate{
			{Code: AZN, Scale: 1, BasePerUnit: 1, Date: now},
			{Code: USD, Scale: 1, BasePerUnit: 1.7, Date: now},
			{Code: EUR, Scale: 1, BasePerUnit: 1.91, Date: now},
			{Code: RUB, Scale: 100, BasePerUnit: 2.02, Date: now},
			{Code: KZT, Scale: 100, BasePerUnit: 0.384, Date: now},
			{Code: CNY, Scale: 1, BasePerUnit: 0.254, Date: now},
			{Code: BYN, Scale: 1, BasePerUnit: 0.564, Date: now},
		}
	default:
		return []OfficialRate{
			{Code: BYN, Scale: 1, BasePerUnit: 1, Date: now},
			{Code: USD, Scale: 1, BasePerUnit: 3.27, Date: now},
			{Code: EUR, Scale: 1, BasePerUnit: 3.55, Date: now},
			{Code: RUB, Scale: 100, BasePerUnit: 3.55, Date: now},
			{Code: KZT, Scale: 1000, BasePerUnit: 7.27, Date: now},
			{Code: CNY, Scale: 10, BasePerUnit: 4.55, Date: now},
			{Code: AZN, Scale: 1, BasePerUnit: 1.92, Date: now},
		}
	}
}

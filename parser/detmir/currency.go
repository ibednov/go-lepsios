package detmir

import "github.com/ibednov/go-lepsios/currency"

// currencyForAPIHost — цена с регионального API уже в валюте площадки.
func currencyForAPIHost(apiHost string) currency.Code {
	switch apiHost {
	case "api.detmir.ru":
		return currency.RUB
	case "api.detmir.kz":
		return currency.KZT
	default:
		return currency.BYN
	}
}

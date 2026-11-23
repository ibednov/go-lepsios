package wildberries

import (
	"net/url"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
)

func currencyFromURL(rawURL string) currency.Code {
	u, err := url.Parse(rawURL)
	if err != nil {
		return currency.BYN
	}
	if strings.HasSuffix(strings.ToLower(u.Hostname()), ".ru") {
		return currency.RUB
	}
	return currency.BYN
}

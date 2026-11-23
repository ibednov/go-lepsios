package dns

import (
	"net/url"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
)

// productCurrencyCode — валюта из JSON-LD priceCurrency; иначе по host/path DNS.
func productCurrencyCode(priceCurrency, pageURL string) currency.Code {
	if code, err := currency.Parse(strings.TrimSpace(priceCurrency)); err == nil {
		return code
	}

	u, err := url.Parse(pageURL)
	if err != nil {
		return currency.BYN
	}

	path := strings.ToLower(u.Path)
	host := strings.ToLower(u.Hostname())

	if strings.HasPrefix(path, "/ru/") || strings.HasSuffix(host, ".ru") {
		return currency.RUB
	}
	if strings.HasPrefix(path, "/kz/") || strings.HasSuffix(host, ".kz") {
		return currency.KZT
	}
	return currency.BYN
}

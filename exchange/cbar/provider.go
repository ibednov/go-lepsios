package cbar

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/ibednov/go-lepsios/exchange"
)

const (
	ProviderID       = "az_cbar"
	DefaultBaseURL   = "https://www.cbar.az/currencies"
	LegacyProviderID = "" // no pre-prefix key yet
)

type Provider struct {
	client *Client
}

func NewProvider(baseURL string, httpClient *http.Client) *Provider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Provider{client: NewClient(baseURL, httpClient)}
}

func (p *Provider) ID() string { return ProviderID }

func (p *Provider) LegacyIDs() []string {
	if LegacyProviderID == "" {
		return nil
	}
	return []string{LegacyProviderID}
}

func (p *Provider) Countries() []string { return []string{"AZ"} }

func (p *Provider) BaseCurrency() currency.Code { return currency.AZN }

func (p *Provider) FetchRates(ctx context.Context, date time.Time) (exchange.Snapshot, error) {
	rateDate := date
	if rateDate.IsZero() {
		rateDate = time.Now().UTC()
	}
	items, err := p.client.FetchRatesOnDate(ctx, rateDate)
	if err != nil {
		return exchange.Snapshot{}, fmt.Errorf("%w: %v", exchange.ErrFetchRates, err)
	}
	if len(items) == 0 {
		return exchange.Snapshot{}, fmt.Errorf("%w: empty rates for %s", exchange.ErrFetchRates, rateDate.Format("2006-01-02"))
	}
	return exchange.Snapshot{
		ProviderID: ProviderID,
		Date:       rateDate,
		Rates:      ratesFromXML(items, rateDate),
	}, nil
}

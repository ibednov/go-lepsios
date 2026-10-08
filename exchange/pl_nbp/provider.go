package pl_nbp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/ibednov/go-lepsios/exchange"
)

const (
	ProviderID     = "pl_nbp"
	DefaultBaseURL = "https://api.nbp.pl/api"
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

func (p *Provider) LegacyIDs() []string { return nil }

func (p *Provider) Countries() []string { return []string{"PL"} }

func (p *Provider) BaseCurrency() currency.Code { return currency.PLN }

func (p *Provider) FetchRates(ctx context.Context, date time.Time) (exchange.Snapshot, error) {
	rateDate := date
	if rateDate.IsZero() {
		rateDate = time.Now().UTC()
	}
	items, effective, err := p.client.FetchRatesOnDate(ctx, rateDate)
	if err != nil {
		return exchange.Snapshot{}, fmt.Errorf("%w: %v", exchange.ErrFetchRates, err)
	}
	if len(items) == 0 {
		return exchange.Snapshot{}, fmt.Errorf("%w: empty rates for %s", exchange.ErrFetchRates, rateDate.Format("2006-01-02"))
	}
	if !effective.IsZero() {
		rateDate = effective
	}
	return exchange.Snapshot{
		ProviderID: ProviderID,
		Date:       rateDate,
		Rates:      ratesFromDTO(items, rateDate),
	}, nil
}

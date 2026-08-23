package nbrb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ibednov/go-lepsios/exchange"
)

const ProviderID = "nbrb"

type Provider struct {
	client *Client
}

func NewProvider(baseURL string, httpClient *http.Client) *Provider {
	return &Provider{client: NewClient(baseURL, httpClient)}
}

func (p *Provider) ID() string {
	return ProviderID
}

func (p *Provider) Countries() []string {
	return []string{"BY"}
}

func (p *Provider) FetchRates(ctx context.Context, date time.Time) (exchange.Snapshot, error) {
	items, err := p.client.FetchAllRates(ctx)
	if err != nil {
		return exchange.Snapshot{}, fmt.Errorf("%w: %v", exchange.ErrFetchRates, err)
	}

	rateDate := date
	if rateDate.IsZero() {
		rateDate = time.Now().UTC()
	}

	return exchange.Snapshot{
		ProviderID: ProviderID,
		Date:       rateDate,
		Rates:      ratesFromDTO(items, rateDate),
	}, nil
}

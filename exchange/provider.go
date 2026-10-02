package exchange

import (
	"context"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

type Provider interface {
	ID() string
	// LegacyIDs are old provider keys still readable from cache/DB (may be empty).
	LegacyIDs() []string
	Countries() []string
	// BaseCurrency is the hub for this provider's OfficialRate.BasePerUnit (BYN, AZN, …).
	BaseCurrency() currency.Code
	FetchRates(ctx context.Context, date time.Time) (Snapshot, error)
}

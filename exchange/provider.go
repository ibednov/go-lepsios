package exchange

import (
	"context"
	"time"
)

type Provider interface {
	ID() string
	Countries() []string
	FetchRates(ctx context.Context, date time.Time) (Snapshot, error)
}

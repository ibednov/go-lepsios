package exchange

import (
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

type Snapshot struct {
	ProviderID string                  `json:"provider_id"`
	Date       time.Time               `json:"date"`
	Rates      []currency.OfficialRate `json:"rates"`
}

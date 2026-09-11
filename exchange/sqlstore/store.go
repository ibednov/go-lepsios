// Package sqlstore implements exchange.Store on the currency_rates table schema:
//
//	rate_date DATE, currency TEXT, rate_to_byn NUMERIC, scale INT,
//	provider TEXT, fetched_at TIMESTAMPTZ
//	PRIMARY KEY (rate_date, currency, provider)
package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

// Store is a Postgres-backed day cache for official rates.
type Store struct {
	DB *sql.DB
}

// LoadDay returns cached rates for provider (excluding missing rows).
func (s *Store) LoadDay(ctx context.Context, rateDate time.Time, provider string) ([]currency.OfficialRate, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("sqlstore: nil db")
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT currency, rate_to_byn, scale
		FROM currency_rates
		WHERE rate_date = $1 AND provider = $2
		ORDER BY currency ASC
	`, rateDate, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]currency.OfficialRate, 0)
	for rows.Next() {
		var codeRaw string
		var rateF float64
		var scale int
		if err := rows.Scan(&codeRaw, &rateF, &scale); err != nil {
			return nil, err
		}
		code, err := currency.Parse(codeRaw)
		if err != nil || rateF <= 0 || scale <= 0 {
			continue
		}
		out = append(out, currency.OfficialRate{
			Code:       code,
			Scale:      scale,
			BYNPerUnit: rateF,
			Date:       rateDate,
		})
	}
	return out, rows.Err()
}

// StoreDay upserts rates (skips BYN / invalid).
func (s *Store) StoreDay(ctx context.Context, rateDate time.Time, provider string, rates []currency.OfficialRate) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("sqlstore: nil db")
	}
	now := time.Now().UTC()
	for _, r := range rates {
		if r.Code == currency.BYN || !r.Code.IsValid() || r.Scale <= 0 || r.BYNPerUnit <= 0 {
			continue
		}
		_, err := s.DB.ExecContext(ctx, `
			INSERT INTO currency_rates (rate_date, currency, rate_to_byn, scale, provider, fetched_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (rate_date, currency, provider) DO UPDATE
			SET rate_to_byn = EXCLUDED.rate_to_byn,
			    scale = EXCLUDED.scale,
			    fetched_at = EXCLUDED.fetched_at
		`, rateDate, r.Code.String(), r.BYNPerUnit, r.Scale, provider, now)
		if err != nil {
			return err
		}
	}
	return nil
}

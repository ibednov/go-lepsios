// Package sqlstore implements exchange.Store on the currency_rates table schema:
//
//	rate_date DATE, currency TEXT, rate_to_base NUMERIC, scale INT,
//	provider TEXT, fetched_at TIMESTAMPTZ
//	PRIMARY KEY (rate_date, currency, provider)
//
// rate_to_base stores OfficialRate.BasePerUnit for the provider hub.
// Legacy column name rate_to_byn is still supported via Store.RateColumn.
package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

const (
	RateColumn       = "rate_to_base"
	LegacyRateColumn = "rate_to_byn"
)

// Store is a Postgres-backed day cache for official rates.
type Store struct {
	DB *sql.DB
	// RateColumn defaults to rate_to_base. Set LegacyRateColumn for old DBs.
	RateColumn string
}

func (s *Store) rateColumn() string {
	if s != nil && s.RateColumn != "" {
		return s.RateColumn
	}
	return RateColumn
}

// LoadDay returns cached rates for provider (excluding missing rows).
func (s *Store) LoadDay(ctx context.Context, rateDate time.Time, provider string) ([]currency.OfficialRate, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("sqlstore: nil db")
	}
	col := s.rateColumn()
	q := fmt.Sprintf(`
SELECT currency, %s, scale
FROM currency_rates
WHERE rate_date = $1 AND provider = $2
ORDER BY currency ASC
`, col)
	rows, err := s.DB.QueryContext(ctx, q, rateDate, provider)
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
			Code:        code,
			Scale:       scale,
			BasePerUnit: rateF,
			Date:        rateDate,
		})
	}
	return out, rows.Err()
}

// StoreDay upserts rates (skips identity Scale=1 BasePerUnit=1 and invalid rows).
func (s *Store) StoreDay(ctx context.Context, rateDate time.Time, provider string, rates []currency.OfficialRate) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("sqlstore: nil db")
	}
	col := s.rateColumn()
	now := time.Now().UTC()
	for _, r := range rates {
		if !r.Code.IsValid() || r.Scale <= 0 || r.BasePerUnit <= 0 {
			continue
		}
		if r.Scale == 1 && r.BasePerUnit == 1 {
			continue
		}
		q := fmt.Sprintf(`
INSERT INTO currency_rates (rate_date, currency, %s, scale, provider, fetched_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (rate_date, currency, provider) DO UPDATE
SET %s = EXCLUDED.%s,
    scale = EXCLUDED.scale,
    fetched_at = EXCLUDED.fetched_at
`, col, col, col)
		_, err := s.DB.ExecContext(ctx, q, rateDate, r.Code.String(), r.BasePerUnit, r.Scale, provider, now)
		if err != nil {
			return err
		}
	}
	return nil
}

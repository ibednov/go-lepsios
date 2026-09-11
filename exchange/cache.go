package exchange

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/ibednov/go-lepsios/log"
)

// Store is a day-keyed FX rate cache (e.g. Postgres currency_rates).
type Store interface {
	LoadDay(ctx context.Context, rateDate time.Time, provider string) ([]currency.OfficialRate, error)
	StoreDay(ctx context.Context, rateDate time.Time, provider string, rates []currency.OfficialRate) error
}

// Hooks are optional metric/telemetry callbacks.
type Hooks struct {
	OnCacheHit func()
	OnFetch    func()
	OnFallback func()
}

// Cached converts money using a Provider with Store cache-aside and optional fallback.
type Cached struct {
	Provider Provider
	Store    Store
	Fallback []currency.OfficialRate
	Hooks    Hooks
}

// Conversion is an input amount converted into a target currency.
type Conversion struct {
	AmountCents int64
	Rate        float64
	RateDate    time.Time
}

// ConvertCents converts originalCents from inputCur into targetCur for calendar day.
func (c *Cached) ConvertCents(
	ctx context.Context,
	originalCents int64,
	inputCur, targetCur string,
	day time.Time,
) (Conversion, error) {
	from, err := currency.Parse(inputCur)
	if err != nil {
		return Conversion{}, fmt.Errorf("input currency: %w", err)
	}
	to, err := currency.Parse(targetCur)
	if err != nil {
		return Conversion{}, fmt.Errorf("target currency: %w", err)
	}
	rateDate := TruncateDateUTC(day)
	if from == to {
		return Conversion{AmountCents: originalCents, Rate: 1, RateDate: rateDate}, nil
	}
	conv, err := c.converterFor(ctx, rateDate)
	if err != nil {
		return Conversion{}, err
	}
	origMoney := float64(originalCents) / 100
	outMoney, err := conv.Convert(origMoney, from, to)
	if err != nil {
		return Conversion{}, err
	}
	outCents := int64(math.Round(outMoney * 100))
	if outCents <= 0 {
		return Conversion{}, fmt.Errorf("converted amount must be > 0")
	}
	rate := 1.0
	if origMoney != 0 {
		rate = outMoney / origMoney
	}
	return Conversion{AmountCents: outCents, Rate: rate, RateDate: rateDate}, nil
}

func (c *Cached) converterFor(ctx context.Context, rateDate time.Time) (*currency.Converter, error) {
	rates, err := c.EnsureDay(ctx, rateDate)
	if err != nil {
		return nil, err
	}
	return currency.NewConverter(rates), nil
}

// EnsureDay returns rates for rateDate (cache → provider → fallback).
func (c *Cached) EnsureDay(ctx context.Context, rateDate time.Time) ([]currency.OfficialRate, error) {
	if c.Provider == nil {
		return nil, fmt.Errorf("exchange: nil provider")
	}
	providerID := c.Provider.ID()
	day := rateDate.Format("2006-01-02")

	if c.Store != nil {
		existing, err := c.Store.LoadDay(ctx, rateDate, providerID)
		if err != nil {
			return nil, err
		}
		if len(existing) > 0 {
			if c.Hooks.OnCacheHit != nil {
				c.Hooks.OnCacheHit()
			}
			log.Debug("exchange.cache.hit", "provider", providerID, "rate_date", day, "n", len(existing))
			return WithBYN(existing, rateDate), nil
		}
	}

	log.Info("exchange.cache.miss", "provider", providerID, "rate_date", day)
	snap, fetchErr := c.Provider.FetchRates(ctx, rateDate)
	if fetchErr != nil {
		if c.Hooks.OnFallback != nil {
			c.Hooks.OnFallback()
		}
		fallback := c.Fallback
		if len(fallback) == 0 {
			fallback = currency.DefaultOfficialRates()
		}
		log.Warn("exchange.fetch.failed", "provider", providerID, "rate_date", day, "error", fetchErr.Error())
		return WithBYN(fallback, rateDate), nil
	}
	if c.Store != nil {
		if err := c.Store.StoreDay(ctx, rateDate, providerID, snap.Rates); err != nil {
			log.Error("exchange.cache.store_failed", "provider", providerID, "rate_date", day, "error", err.Error())
			return nil, err
		}
	}
	if c.Hooks.OnFetch != nil {
		c.Hooks.OnFetch()
	}
	log.Info("exchange.cache.stored", "provider", providerID, "rate_date", day, "n", len(snap.Rates))
	return WithBYN(snap.Rates, rateDate), nil
}

// WithBYN ensures BYN rate=1 is present.
func WithBYN(rates []currency.OfficialRate, rateDate time.Time) []currency.OfficialRate {
	out := make([]currency.OfficialRate, 0, len(rates)+1)
	hasBYN := false
	for _, r := range rates {
		if r.Code == currency.BYN {
			hasBYN = true
		}
		out = append(out, r)
	}
	if !hasBYN {
		out = append(out, currency.OfficialRate{
			Code: currency.BYN, Scale: 1, BYNPerUnit: 1, Date: rateDate,
		})
	}
	return out
}

// TruncateDateUTC keeps Y-M-D in UTC midnight.
func TruncateDateUTC(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

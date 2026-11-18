package search

import (
	"context"

	"github.com/ibednov/go-lepsios/httpx"
)

func HealthChecker(engine Engine) httpx.Checker {
	return httpx.NewChecker("search", func(ctx context.Context) error {
		if engine == nil {
			return ErrNotConfigured
		}
		return engine.Health(ctx)
	})
}

package db

import (
	"context"
	"database/sql"

	"github.com/ibednov/go-lepsios/httpx"
)

// PingChecker returns an httpx readiness checker for the database.
func PingChecker(database *sql.DB) httpx.Checker {
	return httpx.NewChecker("database", func(ctx context.Context) error {
		if database == nil {
			return sql.ErrConnDone
		}
		return database.PingContext(ctx)
	})
}

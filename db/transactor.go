package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aarondl/sqlboiler/v4/boil"
)

type ctxTxKey struct{}

// Tx is the sqlboiler transaction executor.
type Tx interface {
	boil.ContextExecutor
}

// Transactor runs callbacks inside a SQL transaction.
type Transactor interface {
	Run(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

type transactor struct {
	db *sql.DB
}

// NewTransactor creates a Transactor over *sql.DB.
func NewTransactor(db *sql.DB) Transactor {
	return transactor{db: db}
}

// Run executes fn in a transaction.
func (t transactor) Run(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error {
	if t.db == nil {
		return fmt.Errorf("db: nil database")
	}
	sqlTx, err := t.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	txCtx := context.WithValue(ctx, ctxTxKey{}, sqlTx)
	err = func() (runErr error) {
		defer func() {
			if r := recover(); r != nil {
				_ = sqlTx.Rollback()
				panic(r)
			}
		}()
		runErr = fn(txCtx, sqlTx)
		if runErr != nil {
			_ = sqlTx.Rollback()
			return runErr
		}
		return sqlTx.Commit()
	}()
	return err
}

// TxFromContext returns the active transaction from ctx.
func TxFromContext(ctx context.Context) (Tx, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(ctxTxKey{}).(Tx)
	return tx, ok
}

// NilTransactor is a no-op transactor for tests.
type NilTransactor struct{}

// Run executes fn with nil tx.
func (NilTransactor) Run(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error {
	return fn(ctx, nil)
}

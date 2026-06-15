package db_test

import (
	"context"
	"testing"

	"github.com/ibednov/go-lepsios/db"
	"github.com/stretchr/testify/require"
)

func TestNilTransactor(t *testing.T) {
	var nilTx db.NilTransactor
	called := false
	err := nilTx.Run(context.Background(), func(ctx context.Context, tx db.Tx) error {
		called = true
		require.Nil(t, tx)
		return nil
	})
	require.NoError(t, err)
	require.True(t, called)
}

package job_test

import (
	"context"
	"testing"

	"github.com/ibednov/go-lepsios/identity"
	"github.com/ibednov/go-lepsios/job"
	"github.com/stretchr/testify/require"
)

func TestRunSetsSystemUser(t *testing.T) {
	var got identity.User
	err := job.Run(job.Config{
		ServiceName: "eco-back",
		Env:         "local",
		LogLevel:    "error",
	}, "test-job", func(ctx context.Context) error {
		u, ok := identity.UserFromContext(ctx)
		require.True(t, ok)
		got = u
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, "system:eco-back", got.ID)
	require.Equal(t, identity.ActorSystem, got.Kind)
}

func TestNewCommand(t *testing.T) {
	cmd := job.NewCommand(job.Config{
		ServiceName: "svc",
		Env:         "local",
		LogLevel:    "error",
	}, "noop", "noop job", func(context.Context) error { return nil })
	require.Equal(t, "noop", cmd.Use)
}

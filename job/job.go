package job

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ibednov/go-lepsios/identity"
	"github.com/ibednov/go-lepsios/log"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// Config configures the job runner.
type Config struct {
	ServiceName string
	Env         string
	LogLevel    string
}

// JobFunc is a background job handler.
type JobFunc func(ctx context.Context) error

// Run executes a job with signal handling and system identity.
func Run(cfg Config, name string, fn JobFunc) error {
	logger, err := log.Setup(log.Config{
		Env:         cfg.Env,
		ServiceName: cfg.ServiceName,
		Level:       cfg.LogLevel,
	})
	if err != nil {
		return err
	}
	return runWithLogger(cfg, name, fn, logger)
}

func runWithLogger(cfg Config, name string, fn JobFunc, logger zerolog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ctx = identity.WithSystemUser(ctx, cfg.ServiceName)
	ctx = log.WithContext(ctx, logger)

	start := time.Now()
	logger.Info().Str("job", name).Msg("job.start")

	if err := fn(ctx); err != nil {
		logger.Error().Err(err).Str("job", name).Dur("duration", time.Since(start)).Msg("job.failed")
		return err
	}

	logger.Info().Str("job", name).Dur("duration", time.Since(start)).Msg("job.done")
	return nil
}

// NewCommand returns a cobra command for the job.
func NewCommand(cfg Config, name, short string, fn JobFunc) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := Run(cfg, name, fn); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return err
			}
			return nil
		},
	}
}

package config_test

import (
	"os"
	"testing"

	"github.com/ibednov/go-lepsios/config"
	"github.com/stretchr/testify/require"
)

type testCfg struct {
	Port string `envconfig:"HTTP_PORT" default:"8080"`
	Name string `envconfig:"SERVICE_NAME" required:"true"`
}

func TestLoad(t *testing.T) {
	t.Setenv("SERVICE_NAME", "test-svc")
	t.Setenv("HTTP_PORT", "9090")

	var cfg testCfg
	require.NoError(t, config.Load(&cfg))
	require.Equal(t, "9090", cfg.Port)
	require.Equal(t, "test-svc", cfg.Name)
}

func TestLoadRequiredMissing(t *testing.T) {
	_ = os.Unsetenv("SERVICE_NAME")

	var cfg testCfg
	require.Error(t, config.Load(&cfg))
}

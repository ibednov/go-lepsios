package config

import "github.com/kelseyhightower/envconfig"

// Load reads environment variables into cfg.
func Load(cfg any) error {
	return envconfig.Process("", cfg)
}

// MustLoad calls Load and panics on error. Use only in main/app wiring.
func MustLoad(cfg any) {
	if err := Load(cfg); err != nil {
		panic(err)
	}
}

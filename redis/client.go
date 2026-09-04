package redis

import (
	"fmt"
	"log"
	"net"
	"strings"

	redisPkg "github.com/redis/go-redis/v9"
)

type Config struct {
	URL      string
	Addr     string
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

func NewClient(cfg Config) *redisPkg.Client {
	if cfg.URL != "" && cfg.URL != "redis://localhost:6379" {
		opt, err := redisPkg.ParseURL(cfg.URL)
		if err != nil {
			log.Printf("Warning: Failed to parse REDIS_URL, using default connection: %v", err)
			return redisPkg.NewClient(fallbackOptions(cfg))
		}
		if cfg.Password != "" {
			opt.Password = cfg.Password
		}
		if cfg.DB != 0 {
			opt.DB = cfg.DB
		}
		if cfg.PoolSize > 0 {
			opt.PoolSize = cfg.PoolSize
		}
		return redisPkg.NewClient(opt)
	}

	return redisPkg.NewClient(fallbackOptions(cfg))
}

func fallbackOptions(cfg Config) *redisPkg.Options {
	opt := &redisPkg.Options{
		Addr:     resolveAddr(cfg),
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.PoolSize > 0 {
		opt.PoolSize = cfg.PoolSize
	}
	return opt
}

func resolveAddr(cfg Config) string {
	if addr := strings.TrimSpace(cfg.Addr); addr != "" {
		return addr
	}
	host := strings.TrimSpace(cfg.Host)
	port := strings.TrimSpace(cfg.Port)
	if host != "" && port != "" {
		return net.JoinHostPort(host, port)
	}
	if host != "" {
		return fmt.Sprintf("%s:6379", host)
	}
	return "localhost:6379"
}

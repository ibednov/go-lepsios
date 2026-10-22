package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/ibednov/go-lepsios/browser"
)

func main() {
	cfg := browser.ConfigFromEnv()
	browser.SetAllowedHostSuffixes(cfg.AllowedHosts)

	pages := browser.New(browser.Options{
		Headless:       cfg.Headless,
		Timeout:        cfg.Timeout,
		Warmup:         cfg.Warmup,
		ExecutablePath: cfg.ExecutablePath,
		ProxyURL:       cfg.ProxyURL,
	})
	if cfg.ProxyURL != "" {
		log.Print("browser sidecar proxy enabled")
	}
	defer func() {
		if err := pages.Close(); err != nil {
			log.Printf("browser close failed: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	writeTimeout := cfg.Timeout + 15*time.Second
	if writeTimeout < 30*time.Second {
		writeTimeout = 30 * time.Second
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           browser.NewServer(pages, cfg.Token).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("browser sidecar listening on %s", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("browser sidecar shutdown: %v", err)
	}
	log.Print("browser sidecar stopped")
}

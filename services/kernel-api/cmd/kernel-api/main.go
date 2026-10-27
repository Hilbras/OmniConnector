package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	testintegration "github.com/hilbras/omniconnector/integrations/test-integration"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/services/kernel-api/internal/authn"
	"github.com/hilbras/omniconnector/services/kernel-api/internal/handlers"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "kernel-api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := logger.New(cfg.Log.Level)

	k, err := kernel.New(cfg, log)
	if err != nil {
		return fmt.Errorf("create kernel: %w", err)
	}
	if cfg.Security.EnableTestIntegration {
		if err := k.RegisterPackage(testintegration.NewIntegration()); err != nil {
			return fmt.Errorf("register test integration: %w", err)
		}
	}
	if err := k.Start(); err != nil {
		return fmt.Errorf("start kernel: %w", err)
	}

	if cfg.Security.DevToken == "" {
		log.Warn("KERNEL_DEV_TOKEN is not set; every authenticated endpoint will return 401")
	}
	h := handlers.New(k, log, authn.NewDevToken(cfg.Security.DevToken, "dev-user", cfg.Security.DevPermissions))

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("kernel api listening", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-sigCh:
	}

	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("http shutdown failed", "error", err)
	}
	return k.Shutdown(ctx)
}

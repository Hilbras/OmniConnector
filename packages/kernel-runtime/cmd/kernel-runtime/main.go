package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level)
	log.Info("starting kernel runtime", "version", kernel.Version)

	k, err := kernel.New(cfg, log)
	if err != nil {
		log.Error("failed to create kernel", "error", err)
		os.Exit(1)
	}

	if err := k.Start(); err != nil {
		log.Error("failed to start kernel", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("shutting down kernel")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := k.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	log.Info("kernel stopped")
}

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/query"
	"github.com/louispm/lens/internal/query/config"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	server := query.NewServer(cfg, logger)
	if err := server.Run(ctx); err != nil {
		logger.Fatal("query layer exited with error", zap.Error(err))
	}

	logger.Info("query layer shut down gracefully")
}

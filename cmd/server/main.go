package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/job-hub-kai/jobhub-auth/internal/config"
	"github.com/job-hub-kai/jobhub-auth/internal/logger"
	"github.com/job-hub-kai/jobhub-auth/internal/server"
	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad()

	log := logger.New(cfg.Env)
	defer log.Sync()

	log.Info("Starting service",
		zap.String("name", cfg.ServiceName),
		zap.String("env", cfg.Env),
		zap.String("grpc_addr", cfg.GRPC.Addr),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg, log)
	if err := srv.Run(ctx); err != nil {
		log.Fatal("server exited with error", zap.Error(err))
	}

	log.Info("service stopped gracefully")
}

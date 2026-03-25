package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/Alksndr9/jobhub-service-template/internal/config"
	"github.com/Alksndr9/jobhub-service-template/internal/logger"
	"github.com/Alksndr9/jobhub-service-template/internal/server"
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

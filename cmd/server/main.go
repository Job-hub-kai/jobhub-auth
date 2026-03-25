package main

import (
	"context"
	"database/sql"
	"os/signal"
	"syscall"

	authpb "github.com/Alksndr9/jobhub-proto/gen/go/auth"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/job-hub-kai/jobhub-auth/internal/config"
	"github.com/job-hub-kai/jobhub-auth/internal/handler"
	"github.com/job-hub-kai/jobhub-auth/internal/logger"
	pgRepo "github.com/job-hub-kai/jobhub-auth/internal/repository/postgres"
	redisRepo "github.com/job-hub-kai/jobhub-auth/internal/repository/redis"
	"github.com/job-hub-kai/jobhub-auth/internal/server"
	"github.com/job-hub-kai/jobhub-auth/internal/service"
	"github.com/job-hub-kai/jobhub-auth/internal/telemetry"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	// Telemetry (OpenTelemetry + Jaeger)
	shutdownTracer, err := telemetry.Init(cfg.ServiceName, cfg.Telemetry.JaegerEndpoint)
	if err != nil {
		log.Fatal("failed to init tracer", zap.Error(err))
	}
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			log.Error("tracer shutdown error", zap.Error(err))
		}
	}()

	// PostgreSQL
	pool, err := pgxpool.New(ctx, cfg.DB.DSN)
	if err != nil {
		log.Fatal("failed to create pgx pool", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("failed to ping postgres", zap.Error(err))
	}

	// Run migrations
	db, err := sql.Open("pgx", cfg.DB.DSN)
	if err != nil {
		log.Fatal("failed to open db for migrations", zap.Error(err))
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("goose set dialect", zap.Error(err))
	}
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatal("goose up", zap.Error(err))
	}

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("failed to ping redis", zap.Error(err))
	}

	// Repositories
	userRepo := pgRepo.NewUserRepository(pool)
	tokenRepo := redisRepo.NewTokenRepository(pool, rdb)

	// Service
	authSvc := service.NewAuthService(userRepo, tokenRepo, cfg.JWT, log)

	// Handler
	grpcHandler := handler.NewGRPCHandler(authSvc, log)

	// Server
	srv := server.New(cfg, log, func(s *grpc.Server) {
		authpb.RegisterAuthServiceServer(s, grpcHandler)
	})

	if err := srv.Run(ctx); err != nil {
		log.Fatal("server exited with error", zap.Error(err))
	}

	log.Info("service stopped gracefully")
}

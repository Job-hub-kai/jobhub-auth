package server

import (
	"context"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/job-hub-kai/jobhub-auth/internal/config"
	"github.com/job-hub-kai/jobhub-auth/internal/health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type Server struct {
	cfg  *config.Config
	log  *zap.Logger
	grpc *grpc.Server
}

func New(cfg *config.Config, log *zap.Logger, register func(*grpc.Server)) *Server {
	recoveryOpts := []recovery.Option{
		recovery.WithRecoveryHandler(func(p any) (err error) {
			log.Error("recovered from panic", zap.Any("panic", p))
			return status.Errorf(codes.Internal, "Internal server error")
		}),
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(interceptorLogger(log)),
			recovery.UnaryServerInterceptor(recoveryOpts...),
		),
		grpc.ChainStreamInterceptor(
			logging.StreamServerInterceptor(interceptorLogger(log)),
			recovery.StreamServerInterceptor(recoveryOpts...),
		),
	)

	register(grpcServer)
	reflection.Register(grpcServer)

	return &Server{cfg: cfg, log: log, grpc: grpcServer}
}

func (s *Server) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.GRPC.Addr)
	if err != nil {
		return err
	}

	go s.runMetrics()
	go health.RunHTTP(s.log)

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("gRPC server listening", zap.String("addr", s.cfg.GRPC.Addr))
		if err := s.grpc.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.log.Info("shutting down gRPC server")
		s.grpc.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) runMetrics() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	addr := s.cfg.Telemetry.MetricsAddr
	s.log.Info("metrics server listening", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		s.log.Error("metrics server error", zap.Error(err))
	}
}

func interceptorLogger(log *zap.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, level logging.Level, msg string, fields ...any) {
		switch level {
		case logging.LevelDebug:
			log.Sugar().Debugw(msg, fields...)
		case logging.LevelInfo:
			log.Sugar().Infow(msg, fields...)
		case logging.LevelWarn:
			log.Sugar().Warnw(msg, fields...)
		case logging.LevelError:
			log.Sugar().Errorw(msg, fields...)
		}
	})
}

package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvProd  = "prod"
)

func New(env string) *zap.Logger {
	var cfg zap.Config

	switch env {
	case EnvLocal:
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	case EnvDev:
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case EnvProd:
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	log, err := cfg.Build()
	if err != nil {
		panic("failed to build logger: " + err.Error())
	}

	return log
}

func With(log *zap.Logger, fields ...zap.Field) *zap.Logger {
	return log.With(fields...)
}

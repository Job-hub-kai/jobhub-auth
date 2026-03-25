package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	ServiceName string `mapstructure:"SERVICE_NAME"`
	Env         string `mapstructure:"ENV"` // local | dev | prod
	GRPC        GRPCConfig
	DB          DBConfig
	Redis       RedisConfig
	JWT         JWTConfig
	Telemetry   TelemetryConfig
}

type GRPCConfig struct {
	Addr    string        `mapstructure:"GRPC_ADDR"`
	Timeout time.Duration `mapstructure:"GRPC_TIMEOUT"`
}

type DBConfig struct {
	DSN             string        `mapstructure:"DB_DSN"`
	MaxOpenConns    int           `mapstructure:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `mapstructure:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `mapstructure:"DB_CONN_MAX_LIFETIME"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"REDIS_ADDR"`
	Password string `mapstructure:"REDIS_PASSWORD"`
	DB       int    `mapstructure:"REDIS_DB"`
}

type JWTConfig struct {
	AccessSecret  string        `mapstructure:"JWT_ACCESS_SECRET"`
	RefreshSecret string        `mapstructure:"JWT_REFRESH_SECRET"`
	AccessTTL     time.Duration `mapstructure:"JWT_ACCESS_TTL"`
	RefreshTTL    time.Duration `mapstructure:"JWT_REFRESH_TTL"`
}

type TelemetryConfig struct {
	JaegerEndpoint string `mapstructure:"JAEGER_ENDPOINT"`
	MetricsAddr    string `mapstructure:"METRICS_ADDR"`
}

func MustLoad() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	viper.SetDefault("SERVICE_NAME", "jobhub-service")
	viper.SetDefault("ENV", "local")
	viper.SetDefault("GRPC_ADDR", ":50051")
	viper.SetDefault("GRPC_TIMEOUT", "5s")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONN_MAX_LIFETIME", "5m")
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("JWT_ACCESS_TTL", "15m")
	viper.SetDefault("JWT_REFRESH_TTL", "720h")
	viper.SetDefault("METRICS_ADDR", ":9090")

	_ = viper.ReadInConfig()

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}
	if err := viper.Unmarshal(&cfg.GRPC); err != nil {
		log.Fatalf("failed to unmarshal grpc config: %v", err)
	}
	if err := viper.Unmarshal(&cfg.DB); err != nil {
		log.Fatalf("failed to unmarshal db config: %v", err)
	}
	if err := viper.Unmarshal(&cfg.Redis); err != nil {
		log.Fatalf("failed to unmarshal redis config: %v", err)
	}
	if err := viper.Unmarshal(&cfg.JWT); err != nil {
		log.Fatalf("failed to unmarshal jwt config: %v", err)
	}
	if err := viper.Unmarshal(&cfg.Telemetry); err != nil {
		log.Fatalf("failed to unmarshal telemetry config: %v", err)
	}
	return cfg
}

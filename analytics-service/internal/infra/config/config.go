package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config — конфигурация analytics-service. Формат такой же, как в основном
// сервисе (envconfig + переменные окружения), чтобы не вводить второй
// подход к конфигурации в том же репозитории.
type Config struct {
	Listen string `envconfig:"LISTEN" default:":8081"`

	// GRPCTarget — адрес gRPC основного сервиса pet-ticket.
	// В docker-compose это "app:9001" (имя сервиса, не localhost), локально — "localhost:9001".
	GRPCTarget string `envconfig:"GRPC_TARGET" default:"localhost:9001"`

	RedisAddr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB       int    `envconfig:"REDIS_DB" default:"0"`

	// CacheTTL — время жизни закешированных метрик в Redis.
	CacheTTL time.Duration `envconfig:"CACHE_TTL" default:"10m"`

	// RefreshInterval — как часто фоновый тикер пересчитывает overview/topics/timeline.
	RefreshInterval time.Duration `envconfig:"REFRESH_INTERVAL" default:"5m"`

	LogLevel  string `envconfig:"LOGGER_LEVEL" default:"info"`
	LogFormat string `envconfig:"LOGGER_FORMAT" default:"console"`

	ENV string `envconfig:"ENV" default:"local"`
}

// Load читает конфигурацию из переменных окружения.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}
	return cfg, nil
}

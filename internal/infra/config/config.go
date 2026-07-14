package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	formatJSON = "json"
)

type Config struct {
	LogLevel     string `envconfig:"LOGGER_LEVEL" default:"info"`
	LogFormat    string `envconfig:"LOGGER_FORMAT" default:"console"`
	ReportCaller bool   `envconfig:"LOG_REPORT_CALLER" default:"false"`

	Listen     string `envconfig:"LISTEN" default:":9000"`
	GrpcListen string `envconfig:"GRPC_LISTEN" default:":9001"` // пусто = gRPC не запускается

	PostgresHost             string `envconfig:"POSTGRES_HOST" default:"localhost"`
	PostgresPort             string `envconfig:"POSTGRES_PORT" default:"5432"`
	PostgresDatabase         string `envconfig:"POSTGRES_DATABASE" default:"pet_ticket"`
	PostgresUser             string `envconfig:"POSTGRES_USER" default:"postgres"`
	PostgresPassword         string `envconfig:"POSTGRES_PASSWORD" default:"postgres"`
	PostgresMaxOpenConn      int    `envconfig:"POSTGRES_MAX_OPEN_CONN" default:"100"`
	PostgresMaxIdleConn      int    `envconfig:"POSTGRES_MAX_IDLE_CONN" default:"10"`
	PostgresConnMaxLifetime  int    `envconfig:"POSTGRES_CONN_MAX_LIFETIME" default:"3600"`
	PostgresEnabledMigration bool   `envconfig:"POSTGRES_ENABLED_MIGRATION" default:"true"`
	PostgresSSLMode          string `envconfig:"POSTGRES_SSLMODE" default:"disable"`

	AutoCloseEnabled           bool   `envconfig:"AUTO_CLOSE_ENABLED" default:"true"`
	AutoCloseInactiveDays      int    `envconfig:"AUTO_CLOSE_AFTER_DAYS" default:"7"`
	AutoCloseCronSchedule      string `envconfig:"AUTO_CLOSE_CRON" default:"0 * * * *"`
	AutoCloseBatchSize         int    `envconfig:"AUTO_CLOSE_BATCH_SIZE" default:"100"`
	AutoCloseProcessingTimeout int    `envconfig:"AUTO_CLOSE_PROCESSING_TIMEOUT" default:"300"`

	// Notification outbox worker (см. cmd/notification-worker)
	NotificationWebhookURL                     string `envconfig:"NOTIFICATION_WEBHOOK_URL" default:"http://localhost:9999/webhook"`
	NotificationHTTPTimeout                    int    `envconfig:"NOTIFICATION_HTTP_TIMEOUT" default:"10"` // секунды
	NotificationPollInterval                   int    `envconfig:"NOTIFICATION_POLL_INTERVAL" default:"5"` // секунды
	NotificationBatchSize                      int    `envconfig:"NOTIFICATION_BATCH_SIZE" default:"20"`
	NotificationBaseBackoffSeconds             int    `envconfig:"NOTIFICATION_BASE_BACKOFF_SECONDS" default:"30"` // база exponential backoff
	NotificationCircuitBreakerFailureThreshold int    `envconfig:"NOTIFICATION_CB_FAILURE_THRESHOLD" default:"5"`
	NotificationCircuitBreakerSuccessThreshold int    `envconfig:"NOTIFICATION_CB_SUCCESS_THRESHOLD" default:"2"`
	NotificationCircuitBreakerTimeout          int    `envconfig:"NOTIFICATION_CB_TIMEOUT" default:"30"` // секунды, Open -> HalfOpen

	// UseNewComments — feature flag миграции комментариев на отдельную
	// таблицу ticket_comments (см. Task 11). false = читать из legacy-поля
	// tickets.comment; true = читать из ticket_comments. Запись всегда идёт
	// в оба места (dual write), независимо от флага — иначе откат флага
	// назад потерял бы свежие данные.
	UseNewComments bool `envconfig:"FEATURE_NEW_COMMENTS" default:"false"`

	// OTELExporterEndpoint — адрес OTLP/gRPC коллектора (Jaeger), куда
	// отправляются трейсы. См. internal/infra/tracing.
	OTELExporterEndpoint string `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:"localhost:4317"`

	ENV string `envconfig:"ENV" default:"local"`
}

var cfg *Config

func Load() (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Logger() zerolog.Logger {
	level := zerolog.InfoLevel
	if newLevel, err := zerolog.ParseLevel(c.LogLevel); err == nil {
		level = newLevel
	}

	var out io.Writer = os.Stdout
	if c.LogFormat != formatJSON {
		out = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}

	logger := zerolog.New(out).Level(level).With().Timestamp()
	if c.ReportCaller {
		logger = logger.Caller()
	}

	return logger.Logger()
}

func (c *Config) PostgresDSN() string {
	dsn := fmt.Sprintf("host=%s port=%s dbname=%s sslmode=%s",
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDatabase,
		c.PostgresSSLMode,
	)

	if c.PostgresUser != "" {
		dsn += fmt.Sprintf(" user=%s", c.PostgresUser)
	}

	if c.PostgresPassword != "" {
		dsn += fmt.Sprintf(" password=%s", c.PostgresPassword)
	}

	return dsn
}

func Get() *Config {
	if cfg == nil {
		var err error
		cfg, err = Load()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load config")
		}
	}
	return cfg
}

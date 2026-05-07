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

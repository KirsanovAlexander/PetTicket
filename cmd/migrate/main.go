package main

import (
	"flag"
	"time"

	"pet-ticket/internal/infra/config"
	"pet-ticket/internal/infra/migration"
	"pet-ticket/internal/infra/postgres"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()

	action := flag.String("action", "up", "Migration action: up or down")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	logger := cfg.Logger()
	log.Logger = logger

	log.Info().Str("action", *action).Msg("starting migrations")

	db, err := postgres.New(cfg.PostgresDSN(), postgres.Options{
		MaxOpenConn:     cfg.PostgresMaxOpenConn,
		MaxIdleConn:     cfg.PostgresMaxIdleConn,
		ConnMaxLifetime: time.Duration(cfg.PostgresConnMaxLifetime) * time.Second,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close database")
		}
	}()

	log.Info().Msg("database connected")

	switch *action {
	case "up":
		log.Info().Msg("running migrations up")
		if err := migration.Up(db.Conn()); err != nil {
			//nolint:gocritic // exitAfterDefer: intentional, fatal error
			log.Fatal().Err(err).Msg("failed to run migrations up")
		}
		log.Info().Msg("migrations completed successfully")
	case "down":
		log.Info().Msg("running migrations down")
		if err := migration.Down(db.Conn()); err != nil {
			//nolint:gocritic // exitAfterDefer: intentional, fatal error
			log.Fatal().Err(err).Msg("failed to run migrations down")
		}
		log.Info().Msg("migrations rolled back successfully")
	default:
		//nolint:gocritic // exitAfterDefer: intentional, fatal error
		log.Fatal().Str("action", *action).Msg("unknown action, use 'up' or 'down'")
	}
}

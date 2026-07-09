package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	appNotifications "pet-ticket/internal/app/notifications"
	"pet-ticket/internal/infra/config"
	infraNotifications "pet-ticket/internal/infra/notifications"
	"pet-ticket/internal/infra/postgres"
	"pet-ticket/pkg/logger"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("notification worker failed")
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	appLogger := logger.New(logger.Config{
		Level:        cfg.LogLevel,
		Format:       cfg.LogFormat,
		ReportCaller: cfg.ReportCaller,
	})
	log.Logger = appLogger

	log.Info().Msg("starting notification worker")

	db, err := postgres.New(cfg.PostgresDSN(), postgres.Options{
		MaxOpenConn:     cfg.PostgresMaxOpenConn,
		MaxIdleConn:     cfg.PostgresMaxIdleConn,
		ConnMaxLifetime: time.Duration(cfg.PostgresConnMaxLifetime) * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close database connection")
		}
	}()

	log.Info().Msg("connected to database")

	outboxRepo := postgres.NewOutboxRepository(db)

	breaker := infraNotifications.NewCircuitBreaker(
		cfg.NotificationCircuitBreakerFailureThreshold,
		cfg.NotificationCircuitBreakerSuccessThreshold,
		time.Duration(cfg.NotificationCircuitBreakerTimeout)*time.Second,
	)
	notifier := infraNotifications.NewHTTPNotifier(
		cfg.NotificationWebhookURL,
		time.Duration(cfg.NotificationHTTPTimeout)*time.Second,
		breaker,
	)

	baseBackoff := time.Duration(cfg.NotificationBaseBackoffSeconds) * time.Second
	sender := appNotifications.NewSender(outboxRepo, notifier, appLogger, baseBackoff)

	pollInterval := time.Duration(cfg.NotificationPollInterval) * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info().
		Str("webhook_url", cfg.NotificationWebhookURL).
		Dur("poll_interval", pollInterval).
		Int("batch_size", cfg.NotificationBatchSize).
		Dur("base_backoff", baseBackoff).
		Msg("notification worker started")

	ctx := context.Background()

	// Первый проход сразу, не дожидаясь первого тика — иначе воркер после
	// рестарта будет простаивать до pollInterval, прежде чем разгрести
	// накопившиеся pending-записи.
	processBatch(ctx, sender, cfg.NotificationBatchSize)

	for {
		select {
		case <-ticker.C:
			processBatch(ctx, sender, cfg.NotificationBatchSize)
		case sig := <-sigChan:
			log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
			log.Info().Msg("notification worker stopped gracefully")
			return nil
		}
	}
}

func processBatch(ctx context.Context, sender *appNotifications.Sender, batchSize int) {
	processed, err := sender.ProcessBatch(ctx, batchSize)
	if err != nil {
		log.Error().Err(err).Msg("failed to process notification batch")
		return
	}
	if processed > 0 {
		log.Info().Int("processed", processed).Msg("notification batch processed")
	}
}

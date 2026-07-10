package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pet-ticket/internal/app/tickets"
	"pet-ticket/internal/infra/config"
	"pet-ticket/internal/infra/postgres"
	"pet-ticket/pkg/logger"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("auto-closer worker failed")
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

	log.Info().Msg("starting auto-closer worker")

	if !cfg.AutoCloseEnabled {
		log.Info().Msg("auto-close is disabled, exiting")
		return nil
	}

	log.Info().
		Int("inactive_days", cfg.AutoCloseInactiveDays).
		Str("cron_schedule", cfg.AutoCloseCronSchedule).
		Int("batch_size", cfg.AutoCloseBatchSize).
		Msg("auto-close config loaded")

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

	repo := postgres.NewTicketsRepository(db)
	// commentsRepo/eventBus/outboxRepo не нужны: auto-closer только закрывает
	// resolved-тикеты (CloseTicket), а этот флоу не входит ни в систему
	// доменных событий, ни в уведомления о смене статуса, ни в комментарии
	// (те ветки кода UpdateTicket/AddComment здесь просто не вызываются).
	service := tickets.NewService(repo, nil, db, appLogger, nil, nil, false)
	autoCloser := tickets.NewAutoCloser(service, repo, appLogger,
		cfg.AutoCloseInactiveDays, cfg.AutoCloseBatchSize,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))

	processingTimeout := time.Duration(cfg.AutoCloseProcessingTimeout) * time.Second

	_, err = c.AddFunc(cfg.AutoCloseCronSchedule, func() {
		jobCtx, jobCancel := context.WithTimeout(ctx, processingTimeout)
		defer jobCancel()

		log.Info().Msg("cron job triggered")
		if err := autoCloser.CloseInactiveTicketsWithRetry(jobCtx, 3); err != nil {
			log.Error().Err(err).Msg("auto-close job failed")
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	c.Start()
	log.Info().Str("schedule", cfg.AutoCloseCronSchedule).Msg("cron scheduler started")

	log.Info().Msg("running initial auto-close iteration")
	initialCtx, initialCancel := context.WithTimeout(ctx, processingTimeout)
	if err := autoCloser.CloseInactiveTicketsWithRetry(initialCtx, 3); err != nil {
		log.Error().Err(err).Msg("initial auto-close failed")
	}
	initialCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Info().Str("signal", sig.String()).Msg("received shutdown signal")

	cronCtx := c.Stop()
	select {
	case <-cronCtx.Done():
		log.Info().Msg("all cron jobs completed")
	case <-time.After(30 * time.Second):
		log.Warn().Msg("shutdown timeout, forcing exit")
	}

	cancel()
	log.Info().Msg("auto-closer worker stopped gracefully")
	return nil
}

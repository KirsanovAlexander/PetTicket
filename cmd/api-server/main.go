package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"net"

	"pet-ticket/internal/app/tickets"
	domainEvents "pet-ticket/internal/domain/events"
	"pet-ticket/internal/infra/config"
	infraEvents "pet-ticket/internal/infra/events"
	"pet-ticket/internal/infra/postgres"
	grpcTransport "pet-ticket/internal/transport/grpc"
	httpTransport "pet-ticket/internal/transport/http"
	"pet-ticket/pkg/logger"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
)

func main() {
	// Загружаем .env файл для локальной разработки (игнорируем ошибки для production окружения)
	_ = godotenv.Load()

	// Запуск приложения
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("application failed")
	}
}

// run содержит всю логику приложения и возвращает ошибку вместо os.Exit
func run() error {
	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Настройка кастомного логгера
	appLogger := logger.New(logger.Config{
		Level:        cfg.LogLevel,
		Format:       cfg.LogFormat,
		ReportCaller: cfg.ReportCaller,
	})
	log.Logger = appLogger

	log.Info().Str("env", cfg.ENV).Msg("starting pet-ticket service")

	// Подключение к БД
	db, err := postgres.New(cfg.PostgresDSN(), postgres.Options{
		MaxOpenConn:     cfg.PostgresMaxOpenConn,
		MaxIdleConn:     cfg.PostgresMaxIdleConn,
		ConnMaxLifetime: time.Duration(cfg.PostgresConnMaxLifetime) * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close database")
		}
	}()

	log.Info().Msg("database connected")

	// Запуск сбора метрик БД
	db.StartMetricsCollector()
	log.Info().Msg("database metrics collector started")

	// Миграции через Goose
	if cfg.PostgresEnabledMigration {
		log.Info().Msg("running migrations with goose")
		if err := goose.Up(db.Conn(), "migrations"); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		log.Info().Msg("migrations completed")
	}

	// Инициализация слоёв (Dependency Injection)
	// Infra → App → Transport

	// Repository (реализация интерфейса из app слоя)
	ticketsRepo := postgres.NewTicketsRepository(db)

	// Event bus: HistoryHandler и MetricsHandler подписываются на все 4
	// доменных события ДО создания Service — Service будет публиковать в
	// уже настроенную шину.
	eventBus := infraEvents.NewInMemoryBus()

	historyHandler := tickets.NewHistoryHandler(ticketsRepo, appLogger)
	metricsHandler := tickets.NewMetricsHandler(appLogger)

	for _, eventName := range []string{
		domainEvents.EventTicketCreated,
		domainEvents.EventTicketStatusChanged,
		domainEvents.EventTicketCommentAdded,
		domainEvents.EventTicketAssigned,
	} {
		eventBus.Subscribe(eventName, metricsHandler.Handle)
	}
	eventBus.Subscribe(domainEvents.EventTicketCreated, historyHandler.HandleTicketCreated)
	eventBus.Subscribe(domainEvents.EventTicketStatusChanged, historyHandler.HandleTicketStatusChanged)
	eventBus.Subscribe(domainEvents.EventTicketCommentAdded, historyHandler.HandleTicketCommentAdded)
	eventBus.Subscribe(domainEvents.EventTicketAssigned, historyHandler.HandleTicketAssigned)

	// Outbox-репозиторий: сервис пишет туда уведомления при смене статуса
	// тикета (в той же транзакции), отдельный notification-worker их читает
	// и отправляет — см. cmd/notification-worker.
	outboxRepo := postgres.NewOutboxRepository(db)

	// Комментарии: dual write в ticket_comments и legacy tickets.comment,
	// откуда читать — решает FEATURE_NEW_COMMENTS (см. Task 11).
	commentsRepo := postgres.NewCommentsRepository(db)

	// Service (use cases)
	ticketsService := tickets.NewService(
		ticketsRepo, commentsRepo, db, appLogger,
		eventBus, outboxRepo, cfg.UseNewComments,
	)

	// Transport (HTTP)
	transport := httpTransport.New(ticketsService, appLogger, cfg.ENV)

	// gRPC сервер (опционально, если задан GRPC_LISTEN)
	var grpcListener net.Listener
	grpcServer, grpcListener, err := grpcTransport.Run(cfg.GrpcListen, ticketsService, appLogger)
	if err != nil {
		return fmt.Errorf("failed to start grpc server: %w", err)
	}
	if grpcServer != nil {
		defer func() {
			grpcServer.GracefulStop()
			if grpcListener != nil {
				_ = grpcListener.Close()
			}
		}()
	}

	// Канал для ошибок запуска сервера
	errCh := make(chan error, 1)

	// Запуск HTTP сервера в горутине
	go func() {
		log.Info().Str("bind", cfg.Listen).Msg("starting http server")
		if err := transport.App().Listen(cfg.Listen); err != nil {
			errCh <- fmt.Errorf("failed to start http server: %w", err)
		}
	}()

	// Канал для сигналов завершения
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Ждём либо ошибку старта, либо сигнал завершения
	select {
	case err := <-errCh:
		return err
	case sig := <-shutdown:
		log.Info().Str("signal", sig.String()).Msg("shutting down server")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := transport.App().ShutdownWithContext(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("failed to shutdown http server gracefully")
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	log.Info().Msg("server stopped gracefully")
	return nil
}

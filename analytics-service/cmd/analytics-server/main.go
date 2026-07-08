package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"analytics-service/internal/app/analytics"
	"analytics-service/internal/infra/cache"
	"analytics-service/internal/infra/config"
	analyticsgrpc "analytics-service/internal/infra/grpc"
	httpTransport "analytics-service/internal/transport/http"

	"pet-ticket/pkg/logger"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("analytics-service failed")
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	appLogger := logger.New(logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})
	log.Logger = appLogger

	log.Info().Str("env", cfg.ENV).Msg("starting analytics-service")

	// gRPC-клиент к основному сервису. Подключение ленивое — если pet-ticket
	// ещё не поднялся (обычная ситуация при старте через docker-compose),
	// сервис всё равно стартует и начнёт отвечать по мере готовности апстрима.
	ticketClient, err := analyticsgrpc.NewTicketClient(cfg.GRPCTarget)
	if err != nil {
		return fmt.Errorf("failed to create ticket grpc client: %w", err)
	}
	defer func() {
		if closeErr := ticketClient.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close grpc client")
		}
	}()
	log.Info().Str("target", cfg.GRPCTarget).Msg("grpc client configured")

	// Redis-кеш. Недоступность Redis на старте не фатальна — Service сам
	// обрабатывает ошибки Get/Set как cache miss (graceful fallback), поэтому
	// здесь только логируем предупреждение, а не падаем.
	redisCache := cache.NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.CacheTTL)
	defer func() {
		if closeErr := redisCache.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close redis connection")
		}
	}()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisCache.Ping(pingCtx); err != nil {
		log.Warn().Err(err).Str("addr", cfg.RedisAddr).Msg("redis is not reachable at startup, will retry on demand")
	} else {
		log.Info().Str("addr", cfg.RedisAddr).Msg("redis connected")
	}
	pingCancel()

	aggregator := analytics.NewAggregator(ticketClient)
	service := analytics.NewService(aggregator, redisCache, appLogger)

	// Фоновый прогрев кеша: сразу после старта и затем каждые RefreshInterval.
	refreshCtx, stopRefresh := context.WithCancel(context.Background())
	defer stopRefresh()
	go backgroundRefreshLoop(refreshCtx, service, cfg.RefreshInterval)

	transport := httpTransport.New(service, appLogger)

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("bind", cfg.Listen).Msg("starting http server")
		if err := transport.App().Listen(cfg.Listen); err != nil {
			errCh <- fmt.Errorf("failed to start http server: %w", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-shutdown:
		log.Info().Str("signal", sig.String()).Msg("shutting down analytics-service")
	}

	stopRefresh()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := transport.App().ShutdownWithContext(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("failed to shutdown http server gracefully")
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	log.Info().Msg("analytics-service stopped gracefully")
	return nil
}

// backgroundRefreshLoop пересчитывает overview/topics/timeline каждые
// interval, пока ctx не отменён. Первый прогрев запускается немедленно —
// иначе дашборд первые interval-минуты после деплоя отвечал бы только через
// живую агрегацию по каждому запросу.
func backgroundRefreshLoop(ctx context.Context, service *analytics.Service, interval time.Duration) {
	service.RefreshBackground(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.RefreshBackground(ctx)
		}
	}
}

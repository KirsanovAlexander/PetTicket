package http

import (
	"pet-ticket/internal/app/tickets"
	"pet-ticket/internal/transport/http/handlers"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Transport представляет HTTP транспортный слой
type Transport struct {
	app            *fiber.App
	ticketsHandler *handlers.TicketsHandler
	logger         zerolog.Logger
	env            string
}

// New создаёт новый экземпляр HTTP транспорта
func New(svc tickets.Service, logger zerolog.Logger, env string) *Transport {
	t := &Transport{
		ticketsHandler: handlers.NewTicketsHandler(svc, logger),
		logger:         logger.With().Str("module", "transport").Logger(),
		env:            env,
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: NewErrorHandler(logger, env),
	})

	t.app = app

	// Middleware (порядок важен!)
	app.Use(recover.New())
	app.Use(mw.RequestIDMiddleware)
	app.Use(mw.PrometheusMiddleware())
	app.Use(mw.AccessLogMiddleware(t.logger))
	app.Use(mw.AuthMiddleware(env))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type,Authorization,X-Request-ID,X-User-ID",
	}))

	t.setupRoutes()

	return t
}

func (t *Transport) setupRoutes() {
	// Метрики endpoint (без auth middleware)
	t.app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// pprof только в dev/local — нельзя открывать профилирование в production,
	// это утечка информации о рантайме и потенциальный вектор DoS
	// (например, /debug/pprof/profile блокирует горутину на 30 сек).
	if t.env != "production" {
		t.app.Use(pprof.New())
	}

	// API routes
	api := t.app.Group("/api/v1")

	// Регистрируем роуты через handler
	t.ticketsHandler.RegisterRoutes(api)
}

// App возвращает экземпляр Fiber приложения
func (t *Transport) App() *fiber.App {
	return t.app
}

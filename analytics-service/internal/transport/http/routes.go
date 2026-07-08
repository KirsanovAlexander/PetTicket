package http

import (
	"analytics-service/internal/app/analytics"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"
)

// Transport — HTTP-транспортный слой analytics-service.
type Transport struct {
	app      *fiber.App
	handlers *Handlers
}

// New создаёт Fiber-приложение и регистрирует роуты дашборда.
func New(service *analytics.Service, logger zerolog.Logger) *Transport {
	t := &Transport{
		handlers: NewHandlers(service, logger),
	}

	app := fiber.New()
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type",
	}))

	t.app = app
	t.setupRoutes()

	return t
}

func (t *Transport) setupRoutes() {
	api := t.app.Group("/api/v1")

	api.Get("/healthcheck", t.handlers.healthcheck)
	api.Get("/overview", t.handlers.getOverview)
	api.Get("/users/:id", t.handlers.getUserStats)
	api.Get("/topics", t.handlers.getTopicStats)
	api.Get("/timeline", t.handlers.getTimeline)
}

// App возвращает экземпляр Fiber-приложения.
func (t *Transport) App() *fiber.App {
	return t.app
}

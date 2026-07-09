package http

import (
	"time"

	"pet-ticket/internal/app/tickets"
	v1 "pet-ticket/internal/transport/http/handlers/v1"
	v2 "pet-ticket/internal/transport/http/handlers/v2"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// v1Lifetime — сколько живёт v1 после старта сервиса, прежде чем в
// Sunset-заголовке проставляется дата отключения. Отсчитывается от момента
// запуска процесса (не хранится нигде между рестартами) — этого достаточно,
// чтобы дать клиентам предсказуемое окно на переход на v2.
const v1Lifetime = 180 * 24 * time.Hour

// Transport представляет HTTP транспортный слой
type Transport struct {
	app       *fiber.App
	v1Handler *v1.TicketsHandler
	v2Handler *v2.TicketsHandler
	logger    zerolog.Logger
	env       string
}

// New создаёт новый экземпляр HTTP транспорта
func New(svc tickets.Service, logger zerolog.Logger, env string) *Transport {
	t := &Transport{
		v1Handler: v1.NewTicketsHandler(svc, logger),
		v2Handler: v2.NewTicketsHandler(svc, logger),
		logger:    logger.With().Str("module", "transport").Logger(),
		env:       env,
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

	// v1 — deprecated, но продолжает полностью работать. Deprecation/Sunset/
	// Link проставляются на все ответы группы, а метрики версии считаются
	// отдельно от v2 (APIVersionMetricsMiddleware) — чтобы видеть скорость
	// перехода клиентов и когда v1 реально можно выключать.
	apiV1 := t.app.Group("/api/v1")
	apiV1.Use(mw.APIVersionMetricsMiddleware("v1"))
	apiV1.Use(mw.DeprecationMiddleware(time.Now().Add(v1Lifetime), "/api/v2"))
	t.v1Handler.RegisterRoutes(apiV1)

	// v2 — тикеты с вложенными объектами (user/status/topic/comments/sla)
	// вместо плоских *_id полей v1.
	apiV2 := t.app.Group("/api/v2")
	apiV2.Use(mw.APIVersionMetricsMiddleware("v2"))
	t.v2Handler.RegisterRoutes(apiV2)
}

// App возвращает экземпляр Fiber приложения
func (t *Transport) App() *fiber.App {
	return t.app
}

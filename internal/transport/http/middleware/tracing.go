package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// TracingMiddleware создаёт span на каждый запрос и кладёт его контекст в
// UserContext, чтобы downstream middleware/handlers (в частности
// AccessLogMiddleware) могли достать trace_id через tracing.TraceIDFromCtx.
// Должен быть зарегистрирован после RequestIDMiddleware и перед остальными
// middleware, чтобы span покрывал весь запрос.
func TracingMiddleware(serviceName string) fiber.Handler {
	tracer := otel.Tracer(serviceName)

	return func(c *fiber.Ctx) error {
		// c.Method()/c.Path() во Fiber — zero-copy строки поверх буфера
		// *fiber.Ctx, который переиспользуется через sync.Pool для следующих
		// запросов. Span экспортируется асинхронно (sdktrace.WithBatcher) уже
		// после того, как этот Ctx мог быть отдан другому запросу — поэтому
		// значения атрибутов клонируем, иначе в Jaeger улетит чужой path.
		method := strings.Clone(c.Method())
		path := strings.Clone(c.Path())

		ctx, span := tracer.Start(c.Context(), fmt.Sprintf("HTTP %s %s", method, path))
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.target", path),
		)

		c.SetUserContext(ctx)
		err := c.Next()

		// c.Route() до вызова c.Next() указывает на саму TracingMiddleware
		// (глобальный Use() ещё не довёл роутинг до конкретного хендлера) —
		// поэтому шаблон маршрута читаем только сейчас, когда он уже
		// заматчен, и переименовываем span. Так "/api/v1/tickets/42" и
		// "/api/v1/tickets/17" схлопываются в одно имя "/api/v1/tickets/:id"
		// вместо тысяч уникальных span names в Jaeger. "/" отфильтровываем
		// отдельно: для несуществующего пути (404) c.Route() остаётся
		// указывать на последний сработавший Use()-middleware (у него
		// Path == "/"), а не на реальный маршрут — переименовывать в
		// "HTTP GET /" в этом случае некорректно.
		if routePath := c.Route().Path; routePath != "" && routePath != "/" && routePath != path {
			span.SetName(fmt.Sprintf("HTTP %s %s", method, strings.Clone(routePath)))
		}

		status := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", status))
		if status >= 500 {
			span.SetStatus(codes.Error, "server error")
		}

		return err
	}
}

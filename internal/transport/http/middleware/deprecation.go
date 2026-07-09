package middleware

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DeprecationMiddleware помечает все ответы группы как deprecated: заголовки
// Deprecation/Sunset/Link (см. draft-ietf-httpapi-deprecation-header и
// RFC 8594). sunsetAt формируется один раз при старте сервера (см.
// transport.go) — иначе "дедлайн" ползал бы вперёд с каждым запросом.
// successorLink — путь на замену (без Content-Negotiation, просто URL для
// клиента/документации), например "/api/v2".
func DeprecationMiddleware(sunsetAt time.Time, successorLink string) fiber.Handler {
	sunsetHeader := sunsetAt.UTC().Format(http.TimeFormat)

	return func(c *fiber.Ctx) error {
		c.Set("Deprecation", "true")
		c.Set("Sunset", sunsetHeader)
		c.Set("Link", `<`+successorLink+`>; rel="successor-version"`)
		return c.Next()
	}
}

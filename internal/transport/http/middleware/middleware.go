package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// RequestIDMiddleware добавляет уникальный request-id к каждому запросу
func RequestIDMiddleware(c *fiber.Ctx) error {
	requestID := c.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}
	c.Locals("requestId", requestID)
	c.Set("X-Request-ID", requestID)
	return c.Next()
}

// AccessLogMiddleware логирует каждый HTTP запрос
func AccessLogMiddleware(logger zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		path := c.Path()
		method := c.Method()

		// Обработка запроса
		err := c.Next()

		// Логирование после обработки
		status := c.Response().StatusCode()
		latency := time.Since(start)

		logEvent := logger.Info()
		if status >= 500 {
			logEvent = logger.Error()
		} else if status >= 400 {
			logEvent = logger.Warn()
		}

		requestID := ""
		if rid := c.Locals("requestId"); rid != nil {
			requestID = rid.(string)
		}

		logEvent.
			Str("requestId", requestID).
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Dur("latency", latency).
			Str("ip", c.IP()).
			Str("userAgent", c.Get("User-Agent")).
			Msg("request completed")

		return err
	}
}

// AuthMiddleware добавляет user_id в контекст (заглушка для dev, можно заменить на JWT)
func AuthMiddleware(env string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var userID int64

		// В dev режиме используем заглушку
		if env == "local" || env == "development" {
			// Пробуем получить из заголовка (для тестирования)
			userIDHeader := c.Get("X-User-ID")
			if userIDHeader != "" {
				// Попытка распарсить
				if parsedID, err := parseInt64(userIDHeader); err == nil {
					userID = parsedID
				} else {
					userID = 1 // Дефолтный user для dev
				}
			} else {
				userID = 1 // Дефолтный user для dev
			}
		} else {
			// В production требуем X-User-ID (или можно заменить на JWT)
			userIDHeader := c.Get("X-User-ID")
			if userIDHeader == "" {
				return fiber.NewError(fiber.StatusUnauthorized, "X-User-ID header is required")
			}

			parsedID, err := parseInt64(userIDHeader)
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "invalid X-User-ID header")
			}
			userID = parsedID
		}

		// Сохраняем в контекст
		c.Locals("userID", userID)
		return c.Next()
	}
}

// parseInt64 парсит строку в int64
func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

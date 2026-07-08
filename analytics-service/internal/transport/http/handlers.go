package http

import (
	"errors"
	"strconv"
	"strings"

	"analytics-service/internal/app/analytics"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// Handlers — REST-хендлеры дашборда аналитики.
type Handlers struct {
	service *analytics.Service
	logger  zerolog.Logger
}

// NewHandlers создаёт хендлеры поверх сервиса аналитики.
func NewHandlers(service *analytics.Service, logger zerolog.Logger) *Handlers {
	return &Handlers{
		service: service,
		logger:  logger.With().Str("module", "http_handlers").Logger(),
	}
}

func (h *Handlers) healthcheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "service": "analytics-service"})
}

// getOverview — GET /api/v1/overview
func (h *Handlers) getOverview(c *fiber.Ctx) error {
	overview, err := h.service.GetOverview(c.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get overview")
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to get overview"})
	}
	return c.JSON(overview)
}

// getUserStats — GET /api/v1/users/:id
func (h *Handlers) getUserStats(c *fiber.Ctx) error {
	userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || userID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	stats, err := h.service.GetUserStats(c.Context(), userID)
	if err != nil {
		h.logger.Error().Err(err).Int64("user_id", userID).Msg("failed to get user stats")
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to get user stats"})
	}
	return c.JSON(stats)
}

// getTopicStats — GET /api/v1/topics
func (h *Handlers) getTopicStats(c *fiber.Ctx) error {
	topics, err := h.service.GetTopicStats(c.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get topic stats")
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to get topic stats"})
	}
	return c.JSON(topics)
}

// getTimeline — GET /api/v1/timeline?period=7d|30d (по умолчанию 7d)
func (h *Handlers) getTimeline(c *fiber.Ctx) error {
	period := strings.TrimSpace(c.Query("period", analytics.Period7Days))

	timeline, err := h.service.GetTimeline(c.Context(), period)
	if err != nil {
		if errors.Is(err, analytics.ErrUnsupportedPeriod) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		h.logger.Error().Err(err).Str("period", period).Msg("failed to get timeline")
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to get timeline"})
	}
	return c.JSON(timeline)
}

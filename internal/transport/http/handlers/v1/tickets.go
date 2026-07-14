// Package v1 — обработчики v1 API. Перенесены без изменения поведения из
// прежнего единого пакета handlers при введении версионирования (Task 9):
// v1 продолжает работать как раньше, только помечен как deprecated
// (см. middleware.DeprecationMiddleware в transport.go).
package v1

import (
	"pet-ticket/internal/app/tickets"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// TicketsHandler обработчики для тикетов
type TicketsHandler struct {
	service tickets.Service
	logger  zerolog.Logger
}

// NewTicketsHandler создаёт новый handler
func NewTicketsHandler(service tickets.Service, logger zerolog.Logger) *TicketsHandler {
	return &TicketsHandler{
		service: service,
		logger:  logger.With().Str("handler", "tickets_v1").Logger(),
	}
}

func (h *TicketsHandler) healthcheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"service": "pet-ticket",
	})
}

func (h *TicketsHandler) getAllStatuses(c *fiber.Ctx) error {
	statuses, err := h.service.GetAllStatuses(c.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get statuses")
		return err
	}

	return c.JSON(statuses)
}

func (h *TicketsHandler) getAllTopics(c *fiber.Ctx) error {
	topics, err := h.service.GetAllTopics(c.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get topics")
		return err
	}

	return c.JSON(topics)
}

// RegisterRoutes регистрирует все роуты для тикетов
func (h *TicketsHandler) RegisterRoutes(api fiber.Router) {
	api.Get("/healthcheck", h.healthcheck)

	// Справочники
	api.Get("/statuses", h.getAllStatuses)
	api.Get("/topics", h.getAllTopics)

	// Тикеты
	ticketsGroup := api.Group("/tickets")
	ticketsGroup.Get("/sla-violations", h.getSLAViolations)
	ticketsGroup.Post("", mw.ValidateBody[CreateTicketRequest](), h.createTicket)
	ticketsGroup.Get("/:id", h.getTicket)
	ticketsGroup.Put("/:id", mw.ValidateBody[UpdateTicketRequest](), h.updateTicket)
	ticketsGroup.Delete("/:id", h.deleteTicket)
	ticketsGroup.Get("", h.listTickets)
	ticketsGroup.Get("/:id/history", h.getTicketHistory)
	ticketsGroup.Post("/:id/comments", mw.ValidateBody[AddCommentRequest](), h.addComment)

	// Бизнес-логика
	ticketsGroup.Post("/:id/assign", h.assignTicket)     // Взять тикет в работу (self-assign)
	ticketsGroup.Post("/:id/unassign", h.unassignTicket) // Снять назначение (только владелец)
	ticketsGroup.Post("/:id/escalate", h.escalateTicket) // Эскалация тикета
}

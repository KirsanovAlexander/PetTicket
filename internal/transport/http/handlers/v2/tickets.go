// Package v2 — обработчики v2 API: тикет возвращается с раскрытыми
// вложенными объектами (user/status/topic/comments/sla) вместо плоских
// *_id полей v1. Бизнес-действия (assign/escalate/comments/history/
// sla-violations) в v2 пока не продублированы — Task 9 про версионирование
// самого представления тикета, а не про полный паритет со всеми
// эндпоинтами v1; это осознанное сужение объёма, а не недосмотр.
package v2

import (
	"strconv"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
	"pet-ticket/internal/transport/http/dto"
	dtov2 "pet-ticket/internal/transport/http/dto/v2"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// TicketsHandler обработчики тикетов v2.
type TicketsHandler struct {
	service tickets.Service
	logger  zerolog.Logger
}

// NewTicketsHandler создаёт новый handler.
func NewTicketsHandler(service tickets.Service, logger zerolog.Logger) *TicketsHandler {
	return &TicketsHandler{
		service: service,
		logger:  logger.With().Str("handler", "tickets_v2").Logger(),
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
	return c.JSON(dto.ToStatusResponseList(statuses))
}

func (h *TicketsHandler) getAllTopics(c *fiber.Ctx) error {
	topics, err := h.service.GetAllTopics(c.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get topics")
		return err
	}
	return c.JSON(dto.ToTopicResponseList(topics))
}

// CreateTicketRequest запрос на создание тикета (v2)
type CreateTicketRequest struct {
	UserID     int64    `json:"userId" validate:"required,gte=1"`
	TopicID    int64    `json:"topicId" validate:"required,gte=1"`
	PriorityID *int     `json:"priorityId,omitempty" validate:"omitempty,gte=1,lte=4"`
	Amount     *float64 `json:"amount,omitempty" validate:"omitempty,gte=0"`
	Comment    string   `json:"comment" validate:"required,min=10,max=2000"`
}

// UpdateTicketRequest запрос на обновление тикета (v2)
type UpdateTicketRequest struct {
	StatusID *int    `json:"statusId,omitempty" validate:"omitempty,gte=1,lte=5"`
	Comment  *string `json:"comment,omitempty" validate:"omitempty,min=10,max=2000"`
}

func (h *TicketsHandler) createTicket(c *fiber.Ctx) error {
	req := mw.GetValidatedBody[CreateTicketRequest](c)

	input := tickets.CreateTicketInput{
		UserID:  req.UserID,
		TopicID: req.TopicID,
		Amount:  req.Amount,
		Comment: req.Comment,
	}
	if req.PriorityID != nil {
		priority := domain.Priority(*req.PriorityID)
		input.Priority = &priority
	}

	created, err := h.service.CreateTicket(c.Context(), input)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create ticket")
		return err
	}

	// CreateTicket возвращает плоский Ticket (*_id поля) — v2 обязан
	// отдавать вложенные объекты, поэтому дочитываем полную проекцию.
	full, err := h.service.GetTicketFull(c.Context(), created.ID)
	if err != nil {
		h.logger.Error().Err(err).Int64("ticket_id", created.ID).Msg("failed to load full ticket after create")
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(dtov2.MapTicketFullToResponse(full))
}

func (h *TicketsHandler) getTicket(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	full, err := h.service.GetTicketFull(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(dtov2.MapTicketFullToResponse(full))
}

func (h *TicketsHandler) updateTicket(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	req := mw.GetValidatedBody[UpdateTicketRequest](c)

	input := tickets.UpdateTicketInput{
		ID:      id,
		Comment: req.Comment,
	}
	if req.StatusID != nil {
		status := domain.Status(*req.StatusID)
		input.Status = &status
	}

	if _, err := h.service.UpdateTicket(c.Context(), input); err != nil {
		return err
	}

	full, err := h.service.GetTicketFull(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(dtov2.MapTicketFullToResponse(full))
}

func (h *TicketsHandler) deleteTicket(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	if err := h.service.DeleteTicket(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *TicketsHandler) listTickets(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	if limit < 1 || limit > 100 {
		return fiber.NewError(fiber.StatusBadRequest, "limit must be between 1 and 100")
	}
	if offset < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "offset must be non-negative")
	}

	var userID, topicID *int64
	var status *domain.Status
	var priority *domain.Priority

	if userIDStr := c.Query("userId"); userIDStr != "" {
		val, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid userId parameter")
		}
		userID = &val
	}

	if topicIDStr := c.Query("topicId"); topicIDStr != "" {
		val, err := strconv.ParseInt(topicIDStr, 10, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid topicId parameter")
		}
		topicID = &val
	}

	if statusIDStr := c.Query("statusId"); statusIDStr != "" {
		val, err := strconv.Atoi(statusIDStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid statusId parameter")
		}
		s := domain.Status(val)
		if !s.IsValid() {
			return fiber.NewError(fiber.StatusBadRequest, "statusId must be between 1 and 5")
		}
		status = &s
	}

	if priorityIDStr := c.Query("priorityId"); priorityIDStr != "" {
		val, err := strconv.Atoi(priorityIDStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid priorityId parameter")
		}
		p := domain.Priority(val)
		if !p.IsValid() {
			return fiber.NewError(fiber.StatusBadRequest, "priorityId must be between 1 and 4")
		}
		priority = &p
	}

	input := tickets.ListTicketsInput{
		UserID:   userID,
		TopicID:  topicID,
		Status:   status,
		Priority: priority,
		Limit:    limit,
		Offset:   offset,
		SortBy:   c.Query("sortBy", "created_at"),
		SortDesc: c.QueryBool("sortDesc", true),
	}

	list, err := h.service.ListTicketsFull(c.Context(), input)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list full tickets")
		return err
	}

	resp := dtov2.ListResponse{
		Data: dtov2.MapTicketFullListToResponse(list),
		Pagination: dtov2.PaginationResponse{
			Limit:  limit,
			Offset: offset,
			Total:  len(list),
		},
	}
	return c.JSON(resp)
}

// RegisterRoutes регистрирует роуты v2.
func (h *TicketsHandler) RegisterRoutes(api fiber.Router) {
	api.Get("/healthcheck", h.healthcheck)
	api.Get("/statuses", h.getAllStatuses)
	api.Get("/topics", h.getAllTopics)

	ticketsGroup := api.Group("/tickets")
	ticketsGroup.Post("", mw.ValidateBody[CreateTicketRequest](), h.createTicket)
	ticketsGroup.Get("/:id", h.getTicket)
	ticketsGroup.Put("/:id", mw.ValidateBody[UpdateTicketRequest](), h.updateTicket)
	ticketsGroup.Delete("/:id", h.deleteTicket)
	ticketsGroup.Get("", h.listTickets)
}

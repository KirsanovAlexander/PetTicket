package handlers

import (
	"strconv"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
	"pet-ticket/internal/transport/http/dto"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// CreateTicketRequest запрос на создание тикета
type CreateTicketRequest struct {
	UserID  int64    `json:"userId" validate:"required,gte=1"`
	TopicID int64    `json:"topicId" validate:"required,gte=1"`
	Amount  *float64 `json:"amount,omitempty" validate:"omitempty,gte=0"`
	Comment string   `json:"comment" validate:"required,min=10,max=2000"`
}

// UpdateTicketRequest запрос на обновление тикета
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

	ticket, err := h.service.CreateTicket(c.Context(), input)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create ticket")
		return err
	}

	resp := dto.ToTicketResponse(ticket)
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *TicketsHandler) getTicket(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	ticket, err := h.service.GetTicket(c.Context(), id)
	if err != nil {
		return err
	}

	resp := dto.ToTicketResponse(ticket)
	return c.JSON(resp)
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

	ticket, err := h.service.UpdateTicket(c.Context(), input)
	if err != nil {
		return err
	}

	resp := dto.ToTicketResponse(ticket)
	return c.JSON(resp)
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

	input := tickets.ListTicketsInput{
		UserID:   userID,
		TopicID:  topicID,
		Status:   status,
		Limit:    limit,
		Offset:   offset,
		SortBy:   c.Query("sortBy", "created_at"),
		SortDesc: c.QueryBool("sortDesc", true),
	}

	ticketList, err := h.service.ListTickets(c.Context(), input)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list tickets")
		return err
	}

	resp := dto.ListResponse{
		Data:  dto.ToTicketResponseList(ticketList),
		Total: int64(len(ticketList)),
	}

	return c.JSON(resp)
}

func (h *TicketsHandler) getTicketHistory(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	if limit < 1 || limit > 100 {
		return fiber.NewError(fiber.StatusBadRequest, "limit must be between 1 and 100")
	}
	if offset < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "offset must be non-negative")
	}

	history, err := h.service.GetTicketHistory(c.Context(), id, limit, offset)
	if err != nil {
		return err
	}

	resp := dto.ToTicketHistoryResponseList(history)
	return c.JSON(resp)
}

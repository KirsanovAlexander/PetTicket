package handlers

import (
	"errors"
	"strconv"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
	"pet-ticket/internal/transport/http/dto"

	"github.com/gofiber/fiber/v2"
)

// AssignTicketRequest запрос на назначение тикета оператору
type AssignTicketRequest struct {
	OperatorID int64  `json:"operatorId" validate:"required,gte=1"`
	Comment    string `json:"comment,omitempty" validate:"omitempty,max=500"`
}

// assignTicket назначает тикет на оператора (бизнес-логика)
func (h *TicketsHandler) assignTicket(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	var req AssignTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	// Получаем текущий userID из контекста (установлен AuthMiddleware)
	userID := c.Locals("userID").(int64)

	// Бизнес-правило "тикет ещё не назначен" и сам перевод в in_progress
	// теперь внутри Service.AssignTicket — оно же публикует ticket.assigned
	// (и, транзитивно через UpdateTicket, ticket.status_changed/ticket.comment_added).
	updated, err := h.service.AssignTicket(c.Context(), tickets.AssignTicketInput{
		TicketID:   ticketID,
		OperatorID: req.OperatorID,
		AssignedBy: userID,
		Comment:    req.Comment,
	})
	if err != nil {
		if errors.Is(err, tickets.ErrConflict) {
			return fiber.NewError(fiber.StatusConflict, "ticket already assigned or in progress")
		}
		return err
	}

	// Логируем назначение
	h.logger.Info().
		Int64("ticketId", ticketID).
		Int64("operatorId", req.OperatorID).
		Int64("assignedBy", userID).
		Msg("ticket assigned to operator")

	resp := dto.ToTicketResponse(updated)
	return c.JSON(fiber.Map{
		"ticket":     resp,
		"operatorId": req.OperatorID,
		"message":    "ticket successfully assigned",
	})
}

// escalateTicket повышает приоритет тикета на один уровень
func (h *TicketsHandler) escalateTicket(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	userID := c.Locals("userID").(int64)

	ticket, err := h.service.GetTicket(c.Context(), ticketID)
	if err != nil {
		return err
	}

	if ticket.Status == domain.StatusClosed || ticket.Status == domain.StatusCancelled {
		return fiber.NewError(fiber.StatusConflict, "cannot escalate closed or cancelled ticket")
	}

	updated, err := h.service.EscalateTicket(c.Context(), ticketID, userID)
	if err != nil {
		return err
	}

	h.logger.Warn().
		Int64("ticketId", ticketID).
		Int64("escalatedBy", userID).
		Int("priority", int(updated.Priority)).
		Msg("ticket escalated")

	resp := dto.ToTicketResponse(updated)
	return c.JSON(fiber.Map{
		"ticket":  resp,
		"message": "ticket successfully escalated",
	})
}

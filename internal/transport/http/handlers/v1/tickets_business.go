package v1

import (
	"errors"
	"strconv"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
	dto "pet-ticket/internal/transport/http/dto/v1"

	"github.com/gofiber/fiber/v2"
)

// AssignTicketRequest запрос на самоназначение тикета — саппорт берёт
// свободный тикет в работу на себя.
type AssignTicketRequest struct {
	AssigneeID int64 `json:"assigneeId" validate:"required,gte=1"`
}

// assignTicket назначает тикет на саппорта. Идемпотентно, конкурентно-
// безопасно (optimistic locking внутри Service.AssignTicket) — при гонке
// нескольких запросов ровно один получает 200, остальные 409.
func (h *TicketsHandler) assignTicket(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	var req AssignTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.AssigneeID <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "assigneeId is required")
	}

	if err := h.service.AssignTicket(c.Context(), ticketID, req.AssigneeID); err != nil {
		// Для "уже назначен" отдаём вместе с 409 то, кто именно и когда
		// успел раньше — клиент может показать осмысленное сообщение
		// ("уже взято в работу оператором N в 10:15") вместо голого кода
		// ошибки. Остальные ошибки уходят как есть в централизованный
		// ErrorHandler (error_handler.go) — им такое обогащение не нужно.
		if errors.Is(err, tickets.ErrTicketAlreadyAssigned) {
			if current, getErr := h.service.GetTicket(c.Context(), ticketID); getErr == nil {
				requestID := ""
				if rid := c.Locals("requestId"); rid != nil {
					requestID, _ = rid.(string)
				}
				return c.Status(fiber.StatusConflict).JSON(dto.ErrorResponse{
					Error: dto.ErrorDetail{
						Code:      "TICKET_ALREADY_ASSIGNED",
						Message:   "ticket already assigned",
						RequestID: requestID,
						Details: map[string]any{
							"assignedTo": current.AssignedTo,
							"assignedAt": current.AssignedAt,
						},
					},
				})
			}
		}
		return err
	}

	updated, err := h.service.GetTicket(c.Context(), ticketID)
	if err != nil {
		return err
	}

	h.logger.Info().
		Int64("ticketId", ticketID).
		Int64("assigneeId", req.AssigneeID).
		Msg("ticket assigned")

	return c.JSON(dto.ToTicketResponse(updated))
}

// unassignTicket снимает назначение с тикета. Снять может только текущий
// владелец (X-User-ID) — иначе 403.
func (h *TicketsHandler) unassignTicket(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	userID := c.Locals("userID").(int64)

	if err := h.service.UnassignTicket(c.Context(), ticketID, userID); err != nil {
		return err
	}

	h.logger.Info().
		Int64("ticketId", ticketID).
		Int64("userId", userID).
		Msg("ticket unassigned")

	return c.SendStatus(fiber.StatusNoContent)
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

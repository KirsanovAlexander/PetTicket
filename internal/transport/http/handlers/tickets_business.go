package handlers

import (
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

// EscalateTicketRequest запрос на эскалацию тикета
type EscalateTicketRequest struct {
	Reason   string `json:"reason" validate:"required,min=10,max=1000"`
	Priority int    `json:"priority" validate:"required,gte=1,lte=3"` // 1=низкий, 2=средний, 3=высокий
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

	// Получаем тикет
	ticket, err := h.service.GetTicket(c.Context(), ticketID)
	if err != nil {
		return err
	}

	// Проверяем, что тикет ещё не назначен (бизнес-правило)
	if ticket.Status != domain.StatusNew {
		return fiber.NewError(fiber.StatusConflict, "ticket already assigned or in progress")
	}

	// Обновляем статус на "in_progress"
	status := domain.StatusInProgress
	comment := "Assigned to operator"
	if req.Comment != "" {
		comment = req.Comment
	}

	updated, err := h.service.UpdateTicket(c.Context(), tickets.UpdateTicketInput{
		ID:      ticketID,
		Status:  &status,
		Comment: &comment,
	})
	if err != nil {
		return err
	}

	// Логируем назначение
	h.logger.Info().
		Int64("ticketId", ticketID).
		Int64("operatorId", req.OperatorID).
		Int64("assignedBy", userID).
		Msg("ticket assigned to operator")

	// Здесь можно добавить:
	// - Отправку уведомления оператору
	// - Запись в отдельную таблицу assignments
	// - Обновление метрик

	resp := dto.ToTicketResponse(updated)
	return c.JSON(fiber.Map{
		"ticket":     resp,
		"operatorId": req.OperatorID,
		"message":    "ticket successfully assigned",
	})
}

// escalateTicket эскалирует тикет на следующий уровень поддержки
func (h *TicketsHandler) escalateTicket(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	var req EscalateTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	userID := c.Locals("userID").(int64)

	// Получаем тикет
	ticket, err := h.service.GetTicket(c.Context(), ticketID)
	if err != nil {
		return err
	}

	// Проверяем бизнес-правила эскалации
	if ticket.Status == domain.StatusClosed || ticket.Status == domain.StatusCancelled {
		return fiber.NewError(fiber.StatusConflict, "cannot escalate closed or cancelled ticket")
	}

	// Добавляем комментарий об эскалации
	escalationComment := "Escalated: " + req.Reason

	updated, err := h.service.UpdateTicket(c.Context(), tickets.UpdateTicketInput{
		ID:      ticketID,
		Comment: &escalationComment,
	})
	if err != nil {
		return err
	}

	// Логируем эскалацию
	h.logger.Warn().
		Int64("ticketId", ticketID).
		Int64("escalatedBy", userID).
		Int("priority", req.Priority).
		Str("reason", req.Reason).
		Msg("ticket escalated")

	// Здесь можно добавить:
	// - Отправку уведомлений руководителю
	// - Изменение SLA тикета
	// - Автоматическое назначение на старшего специалиста
	// - Запись в таблицу escalations

	resp := dto.ToTicketResponse(updated)
	return c.JSON(fiber.Map{
		"ticket":   resp,
		"priority": req.Priority,
		"message":  "ticket successfully escalated",
	})
}

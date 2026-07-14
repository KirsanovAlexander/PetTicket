package v1

import (
	"fmt"
	"strconv"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
	dto "pet-ticket/internal/transport/http/dto/v1"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// CreateTicketRequest запрос на создание тикета
type CreateTicketRequest struct {
	UserID     int64    `json:"userId" validate:"required,gte=1"`
	TopicID    int64    `json:"topicId" validate:"required,gte=1"`
	PriorityID *int     `json:"priorityId,omitempty" validate:"omitempty,gte=1,lte=4"`
	Amount     *float64 `json:"amount,omitempty" validate:"omitempty,gte=0"`
	Comment    string   `json:"comment" validate:"required,min=10,max=2000"`
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
	if req.PriorityID != nil {
		priority := domain.Priority(*req.PriorityID)
		input.Priority = &priority
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
	// cursor-пагинация имеет приоритет: если передан cursor ИЛИ page_size,
	// клиент явно использует новый способ навигации.
	if c.Query("cursor") != "" || c.Query("page_size") != "" {
		return h.listTicketsWithCursor(c)
	}

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

	var assignedTo *int64
	unassigned := c.QueryBool("unassigned", false)
	if !unassigned {
		if assignedToStr := c.Query("assignedTo"); assignedToStr != "" {
			val, err := strconv.ParseInt(assignedToStr, 10, 64)
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "invalid assignedTo parameter")
			}
			assignedTo = &val
		}
	}

	input := tickets.ListTicketsInput{
		UserID:     userID,
		TopicID:    topicID,
		Status:     status,
		Priority:   priority,
		Limit:      limit,
		Offset:     offset,
		SortBy:     c.Query("sortBy", "created_at"),
		SortDesc:   c.QueryBool("sortDesc", true),
		AssignedTo: assignedTo,
		Unassigned: unassigned,
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

// listTicketsWithCursor обрабатывает cursor-пагинацию (см. listTickets —
// вызывается, когда клиент передал query-параметр cursor или page_size).
func (h *TicketsHandler) listTicketsWithCursor(c *fiber.Ctx) error {
	pageSize := c.QueryInt("page_size", tickets.DefaultCursorPageSize)
	if pageSize < 1 || pageSize > tickets.MaxCursorPageSize {
		return fiber.NewError(fiber.StatusBadRequest,
			fmt.Sprintf("page_size must be between 1 and %d", tickets.MaxCursorPageSize))
	}

	direction := c.Query("direction", "next")
	if direction != "next" && direction != "prev" {
		return fiber.NewError(fiber.StatusBadRequest, "direction must be 'next' or 'prev'")
	}

	var cursor *string
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		cursor = &cursorStr
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

	page, err := h.service.ListTicketsWithCursor(c.Context(), tickets.ListTicketsWithCursorInput{
		UserID:    userID,
		TopicID:   topicID,
		Status:    status,
		Priority:  priority,
		Cursor:    cursor,
		PageSize:  pageSize,
		Direction: direction,
	})
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list tickets with cursor")
		return err
	}

	resp := dto.ListResponseWithPagination{
		Data: dto.ToTicketResponseList(page.Items),
		Pagination: dto.PaginationResponse{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		},
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

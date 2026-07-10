package v1

import (
	"strconv"

	domain "pet-ticket/internal/domain/tickets"
	dto "pet-ticket/internal/transport/http/dto/v1"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// AddCommentRequest запрос на добавление комментария.
//
// IsInternal заменил IsSupportComment (Task 11: комментарии переехали в
// ticket_comments, где видимость — это IsInternal, а не "кто написал"). Это
// breaking-изменение JSON-контракта v1, но v1 уже помечен deprecated
// (DeprecationMiddleware) — новые клиенты должны использовать v2.
type AddCommentRequest struct {
	Comment    string `json:"comment" validate:"required,min=10,max=2000"`
	IsInternal bool   `json:"isInternal"`
}

func (h *TicketsHandler) getSLAViolations(c *fiber.Ctx) error {
	ticketList, err := h.service.GetSLAViolations(c.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get sla violations")
		return err
	}

	return c.JSON(fiber.Map{
		"tickets": dto.ToTicketWithSLAResponseList(ticketList),
	})
}

func (h *TicketsHandler) addComment(c *fiber.Ctx) error {
	ticketID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid ticket id")
	}

	req := mw.GetValidatedBody[AddCommentRequest](c)
	userID := c.Locals("userID").(int64)

	updated, err := h.service.AddComment(c.Context(), domain.AddCommentInput{
		TicketID:   ticketID,
		UserID:     userID,
		Content:    req.Comment,
		IsInternal: req.IsInternal,
	})
	if err != nil {
		return err
	}

	return c.JSON(dto.ToTicketWithSLAResponse(updated))
}

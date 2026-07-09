package v1

import (
	"strconv"

	"pet-ticket/internal/app/tickets"
	dto "pet-ticket/internal/transport/http/dto/v1"
	mw "pet-ticket/internal/transport/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// AddCommentRequest запрос на добавление комментария
type AddCommentRequest struct {
	Comment          string `json:"comment" validate:"required,min=10,max=2000"`
	IsSupportComment bool   `json:"isSupportComment"`
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

	updated, err := h.service.AddComment(c.Context(), tickets.AddCommentInput{
		TicketID:         ticketID,
		UserID:           userID,
		Comment:          req.Comment,
		IsSupportComment: req.IsSupportComment,
	})
	if err != nil {
		return err
	}

	return c.JSON(dto.ToTicketWithSLAResponse(updated))
}

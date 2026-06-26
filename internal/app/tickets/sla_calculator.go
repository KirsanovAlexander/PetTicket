package tickets

import (
	"context"
	"fmt"
	"time"

	"pet-ticket/internal/domain/tickets"
)

// SLARuleRepository контракт для получения SLA-правил
type SLARuleRepository interface {
	GetSLARule(ctx context.Context, topicID, priorityID int64) (*tickets.SLARule, error)
}

// SLACalculator рассчитывает SLA-дедлайны и проверяет бизнес-условия
type SLACalculator struct {
	repo SLARuleRepository
}

// NewSLACalculator создаёт калькулятор SLA
func NewSLACalculator(repo SLARuleRepository) *SLACalculator {
	return &SLACalculator{repo: repo}
}

// CalculateDeadlines возвращает дедлайны ответа и решения
func (c *SLACalculator) CalculateDeadlines(
	ctx context.Context,
	topicID, priorityID int64,
	createdAt time.Time,
) (responseDeadline, resolutionDeadline time.Time, err error) {
	rule, err := c.repo.GetSLARule(ctx, topicID, priorityID)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("get sla rule: %w", err)
	}
	if rule == nil {
		rule = &tickets.SLARule{ResponseTimeMinutes: 120, ResolutionTimeMinutes: 1440}
	}

	responseDeadline, resolutionDeadline = rule.CalculateDeadlines(createdAt)
	return responseDeadline, resolutionDeadline, nil
}

// ShouldSetFirstResponse проверяет, нужно ли зафиксировать первый ответ саппорта
func (c *SLACalculator) ShouldSetFirstResponse(ticket tickets.Ticket, isSupportComment bool) bool {
	return isSupportComment && ticket.FirstResponseAt == nil
}

// ShouldSetResolvedAt проверяет, нужно ли зафиксировать момент решения
func (c *SLACalculator) ShouldSetResolvedAt(oldStatus, newStatus tickets.Status) bool {
	return !isTerminalStatus(oldStatus) && isTerminalStatus(newStatus)
}

func isTerminalStatus(status tickets.Status) bool {
	return status == tickets.StatusResolved || status == tickets.StatusClosed
}

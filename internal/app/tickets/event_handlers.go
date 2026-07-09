package tickets

import (
	"context"
	"fmt"

	domainEvents "pet-ticket/internal/domain/events"
	domain "pet-ticket/internal/domain/tickets"

	"github.com/rs/zerolog"
)

// HistoryHandler подписывается на доменные события и пишет историю тикета
// через Repository — единственное место, которое зовёт repo.AddHistory для
// created/status_changed/comment_added/assigned. Сервис сам эти записи
// больше не создаёт (см. app/tickets/service.go): он публикует событие
// уже после коммита транзакции, а запись в ticket_history — отдельная,
// самостоятельная операция, которую выполняет этот обработчик.
//
// Методы имеют сигнатуру infraEvents.Handler и подключаются к шине в
// cmd/api-server/main.go — сам HistoryHandler ничего не знает про
// конкретную реализацию шины, только про domain-события.
type HistoryHandler struct {
	repo   Repository
	logger zerolog.Logger
}

// NewHistoryHandler создаёт обработчик истории поверх репозитория тикетов.
func NewHistoryHandler(repo Repository, logger zerolog.Logger) *HistoryHandler {
	return &HistoryHandler{
		repo:   repo,
		logger: logger.With().Str("handler", "ticket_history").Logger(),
	}
}

// HandleTicketCreated записывает историю по событию ticket.created.
func (h *HistoryHandler) HandleTicketCreated(ctx context.Context, event domainEvents.Event) error {
	e, ok := event.(domainEvents.TicketCreated)
	if !ok {
		return fmt.Errorf("history handler: unexpected event type %T for %s", event, domainEvents.EventTicketCreated)
	}

	history := domain.History{
		TicketID: e.TicketID,
		UserID:   e.UserID,
		Action:   domain.ActionCreated,
		NewValue: fmt.Sprintf("status=%s", e.Status),
	}
	if err := h.repo.AddHistory(ctx, history); err != nil {
		h.logger.Error().Err(err).Int64("ticket_id", e.TicketID).Msg("failed to record created history")
		return fmt.Errorf("failed to add history: %w", err)
	}
	return nil
}

// HandleTicketStatusChanged записывает историю по событию ticket.status_changed.
// Если событие пришло с Resolved=true, дополнительно пишет запись с
// action=resolved — так же, как раньше делал сервис прямым AddHistory-вызовом.
func (h *HistoryHandler) HandleTicketStatusChanged(ctx context.Context, event domainEvents.Event) error {
	e, ok := event.(domainEvents.TicketStatusChanged)
	if !ok {
		return fmt.Errorf("history handler: unexpected event type %T for %s", event, domainEvents.EventTicketStatusChanged)
	}

	history := domain.History{
		TicketID: e.TicketID,
		UserID:   e.ChangedBy,
		Action:   domain.ActionStatusChanged,
		OldValue: e.OldStatus,
		NewValue: e.NewStatus,
	}
	if err := h.repo.AddHistory(ctx, history); err != nil {
		h.logger.Error().Err(err).Int64("ticket_id", e.TicketID).Msg("failed to record status_changed history")
		return fmt.Errorf("failed to add history: %w", err)
	}

	if e.Resolved {
		resolvedHistory := domain.History{
			TicketID: e.TicketID,
			UserID:   e.ChangedBy,
			Action:   domain.ActionResolved,
			NewValue: e.NewStatus,
		}
		if err := h.repo.AddHistory(ctx, resolvedHistory); err != nil {
			h.logger.Error().Err(err).Int64("ticket_id", e.TicketID).Msg("failed to record resolved history")
			return fmt.Errorf("failed to add resolved history: %w", err)
		}
	}

	return nil
}

// HandleTicketCommentAdded записывает историю по событию ticket.comment_added.
func (h *HistoryHandler) HandleTicketCommentAdded(ctx context.Context, event domainEvents.Event) error {
	e, ok := event.(domainEvents.TicketCommentAdded)
	if !ok {
		return fmt.Errorf("history handler: unexpected event type %T for %s", event, domainEvents.EventTicketCommentAdded)
	}

	history := domain.History{
		TicketID: e.TicketID,
		UserID:   e.UserID,
		Action:   domain.ActionCommentAdded,
		OldValue: e.OldComment,
		NewValue: e.NewComment,
	}
	if err := h.repo.AddHistory(ctx, history); err != nil {
		h.logger.Error().Err(err).Int64("ticket_id", e.TicketID).Msg("failed to record comment_added history")
		return fmt.Errorf("failed to add history: %w", err)
	}
	return nil
}

// HandleTicketAssigned записывает историю по событию ticket.assigned.
func (h *HistoryHandler) HandleTicketAssigned(ctx context.Context, event domainEvents.Event) error {
	e, ok := event.(domainEvents.TicketAssigned)
	if !ok {
		return fmt.Errorf("history handler: unexpected event type %T for %s", event, domainEvents.EventTicketAssigned)
	}

	history := domain.History{
		TicketID: e.TicketID,
		UserID:   e.AssignedBy,
		Action:   domain.ActionAssigned,
		NewValue: fmt.Sprintf("operator=%d", e.OperatorID),
	}
	if err := h.repo.AddHistory(ctx, history); err != nil {
		h.logger.Error().Err(err).Int64("ticket_id", e.TicketID).Msg("failed to record assigned history")
		return fmt.Errorf("failed to add history: %w", err)
	}
	return nil
}

// MetricsHandler — заглушка сбора метрик по доменным событиям. Реальная
// отправка в Prometheus/StatsD и т.п. — не в скоупе этой задачи, поэтому
// обработчик просто логирует факт события структурированно, но с той же
// сигнатурой (infraEvents.Handler), что и HistoryHandler — заменить лог на
// реальный экспорт метрик в будущем можно без изменений в шине или сервисе.
type MetricsHandler struct {
	logger zerolog.Logger
}

// NewMetricsHandler создаёт обработчик метрик.
func NewMetricsHandler(logger zerolog.Logger) *MetricsHandler {
	return &MetricsHandler{logger: logger.With().Str("handler", "ticket_metrics").Logger()}
}

// Handle логирует факт получения события. Один метод подписывается сразу на
// все 4 типа события в main.go — заглушке не нужно различать их полезную
// нагрузку, только сам факт и имя события.
func (h *MetricsHandler) Handle(ctx context.Context, event domainEvents.Event) error {
	h.logger.Info().
		Str("event", event.EventName()).
		Time("occurred_at", event.OccurredAt()).
		Msg("metric event received")
	return nil
}

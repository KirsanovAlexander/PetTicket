package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// AutoCloser автоматически закрывает неактивные resolved тикеты
type AutoCloser struct {
	service      Service
	repo         Repository
	logger       zerolog.Logger
	inactiveDays int
	batchSize    int
}

// NewAutoCloser создаёт новый экземпляр AutoCloser
func NewAutoCloser(
	service Service, repo Repository, logger zerolog.Logger,
	inactiveDays int, batchSize int,
) *AutoCloser {
	return &AutoCloser{
		service:      service,
		repo:         repo,
		logger:       logger,
		inactiveDays: inactiveDays,
		batchSize:    batchSize,
	}
}

// CloseInactiveTickets находит и закрывает пачку неактивных resolved тикетов
func (ac *AutoCloser) CloseInactiveTickets(ctx context.Context) error {
	startTime := time.Now()

	ac.logger.Info().
		Int("inactive_days", ac.inactiveDays).
		Int("batch_size", ac.batchSize).
		Msg("starting auto-close job")

	ticketList, err := ac.repo.FindResolvedTicketsOlderThan(ctx, ac.inactiveDays, ac.batchSize)
	if err != nil {
		return fmt.Errorf("failed to find resolved tickets: %w", err)
	}

	if len(ticketList) == 0 {
		ac.logger.Info().Msg("no tickets to auto-close")
		return nil
	}

	ac.logger.Info().Int("count", len(ticketList)).Msg("found tickets to auto-close")

	closedCount := 0
	failedCount := 0

	for _, ticket := range ticketList {
		if ctx.Err() != nil {
			ac.logger.Warn().
				Int("closed", closedCount).Int("failed", failedCount).
				Msg("auto-close job cancelled")
			return ctx.Err()
		}

		if !ticket.IsInactiveResolved(ac.inactiveDays) {
			ac.logger.Warn().
				Int64("ticket_id", ticket.ID).
				Time("last_activity", ticket.LastUserActivityAt).
				Msg("ticket is not inactive, skipping")
			continue
		}

		_, err := ac.service.CloseTicket(ctx, CloseTicketInput{
			TicketID: ticket.ID,
			UserID:   0,
			Reason:   fmt.Sprintf("Auto-closed after %d days of inactivity", ac.inactiveDays),
		})
		if err != nil {
			ac.logger.Error().Err(err).Int64("ticket_id", ticket.ID).Msg("failed to close ticket")
			failedCount++
			continue
		}

		closedCount++
		ac.logger.Info().
			Int64("ticket_id", ticket.ID).
			Str("last_activity", ticket.LastUserActivityAt.Format(time.RFC3339)).
			Msg("ticket auto-closed")
	}

	ac.logger.Info().
		Int("closed", closedCount).Int("failed", failedCount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("auto-close job completed")

	return nil
}

// CloseInactiveTicketsWithRetry выполняет автозакрытие с exponential backoff при ошибках
func (ac *AutoCloser) CloseInactiveTicketsWithRetry(ctx context.Context, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := ac.CloseInactiveTickets(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ac.logger.Warn().Err(err).
			Int("attempt", attempt).Int("max_retries", maxRetries).
			Msg("auto-close attempt failed")

		if attempt < maxRetries {
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("auto-close failed after %d attempts: %w", maxRetries, lastErr)
}

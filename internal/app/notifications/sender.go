package notifications

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	domain "pet-ticket/internal/domain/notifications"

	"github.com/rs/zerolog"
)

const (
	// maxBackoff — верхняя граница задержки между попытками.
	maxBackoff = 12 * time.Hour
	// jitterFraction — амплитуда случайного джиттера (±20% от расчётной
	// задержки), чтобы много записей, упавших одновременно, не пытались
	// повторить отправку синхронно (thundering herd на внешний сервис).
	jitterFraction = 0.2
	// backoffMultiplier — база степени в формуле backoff: base * 5^(n-1).
	backoffMultiplier = 5
)

// Sender обрабатывает pending outbox-записи: отправляет уведомление через
// Notifier, при неудаче считает exponential backoff с jitter и планирует
// следующую попытку, при исчерпании MaxAttempts помечает запись как failed.
type Sender struct {
	repo      domain.OutboxRepository
	notifier  domain.Notifier
	logger    zerolog.Logger
	baseDelay time.Duration
}

// NewSender создаёт Sender. baseDelay — задержка перед первым retry
// (attempt=1 после первой неудачи).
func NewSender(repo domain.OutboxRepository, notifier domain.Notifier, logger zerolog.Logger, baseDelay time.Duration) *Sender {
	return &Sender{
		repo:      repo,
		notifier:  notifier,
		logger:    logger.With().Str("component", "notification_sender").Logger(),
		baseDelay: baseDelay,
	}
}

// ProcessBatch забирает до limit pending-записей и обрабатывает каждую:
// отправляет уведомление, обновляет статус в outbox. Возвращает количество
// забранных (не обязательно успешно отправленных) записей.
func (s *Sender) ProcessBatch(ctx context.Context, limit int) (int, error) {
	entries, err := s.repo.FindPending(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to find pending entries: %w", err)
	}

	for _, entry := range entries {
		s.processEntry(ctx, entry)
	}

	return len(entries), nil
}

func (s *Sender) processEntry(ctx context.Context, entry domain.OutboxEntry) {
	notification := domain.Notification{
		UserID:   entry.UserID,
		TicketID: entry.TicketID,
		Type:     entry.Type,
		Payload:  entry.Payload,
	}

	entry.Attempts++

	if err := s.notifier.Notify(ctx, notification); err != nil {
		s.handleFailure(ctx, entry, err)
		return
	}

	now := time.Now()
	entry.Status = domain.OutboxStatusSent
	entry.SentAt = &now
	entry.ErrorMessage = ""

	if err := s.repo.Update(ctx, entry); err != nil {
		s.logger.Error().Err(err).Int64("outbox_id", entry.ID).Msg("failed to update outbox entry after successful send")
		return
	}

	s.logger.Info().Int64("outbox_id", entry.ID).Int64("ticket_id", entry.TicketID).Msg("notification sent")
}

func (s *Sender) handleFailure(ctx context.Context, entry domain.OutboxEntry, sendErr error) {
	entry.ErrorMessage = sendErr.Error()

	if entry.Attempts >= entry.MaxAttempts {
		entry.Status = domain.OutboxStatusFailed
		s.logger.Warn().
			Err(sendErr).
			Int64("outbox_id", entry.ID).
			Int("attempts", entry.Attempts).
			Msg("notification permanently failed, giving up")
	} else {
		entry.Status = domain.OutboxStatusPending
		entry.NextRetryAt = time.Now().Add(s.backoff(entry.Attempts))
		s.logger.Warn().
			Err(sendErr).
			Int64("outbox_id", entry.ID).
			Int("attempts", entry.Attempts).
			Time("next_retry_at", entry.NextRetryAt).
			Msg("notification failed, scheduled for retry")
	}

	if err := s.repo.Update(ctx, entry); err != nil {
		s.logger.Error().Err(err).Int64("outbox_id", entry.ID).Msg("failed to update outbox entry after failed send")
	}
}

// backoff считает exponential backoff с jitter: base * multiplier^(attempt-1),
// ограничено maxBackoff, ±jitterFraction случайного отклонения.
func (s *Sender) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	delay := float64(s.baseDelay)
	for i := 1; i < attempt; i++ {
		delay *= backoffMultiplier
		if delay > float64(maxBackoff) {
			delay = float64(maxBackoff)
			break
		}
	}

	jitter := delay * jitterFraction * (2*rand.Float64() - 1) // [-jitterFraction, +jitterFraction]
	delay += jitter

	if delay < 0 {
		delay = 0
	}
	if delay > float64(maxBackoff) {
		delay = float64(maxBackoff)
	}

	return time.Duration(delay)
}

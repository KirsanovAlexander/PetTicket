package notifications

import (
	"context"
	"time"
)

// OutboxStatus статус обработки outbox-записи.
type OutboxStatus string

const (
	// OutboxStatusPending — запись ожидает отправки (первая попытка ещё не
	// делалась, либо предыдущая попытка провалилась и есть ещё retry).
	OutboxStatusPending OutboxStatus = "pending"
	// OutboxStatusProcessing — запись захвачена воркером (FOR UPDATE SKIP
	// LOCKED) и в моменте обрабатывается. Промежуточное состояние.
	OutboxStatusProcessing OutboxStatus = "processing"
	// OutboxStatusSent — уведомление успешно доставлено.
	OutboxStatusSent OutboxStatus = "sent"
	// OutboxStatusFailed — исчерпаны все попытки (Attempts >= MaxAttempts).
	// Терминальное состояние, воркер больше не подбирает такие записи.
	OutboxStatusFailed OutboxStatus = "failed"
)

// OutboxEntry запись в outbox-таблице — уведомление, ожидающее отправки.
// Создаётся в той же транзакции, что и бизнес-изменение (transactional
// outbox pattern): если транзакция коммитится, запись гарантированно
// появляется в базе вместе с изменением, на которое она реагирует — и
// наоборот, при откате транзакции запись тоже не создаётся. Это устраняет
// классическую dual-write проблему (обновили тикет, но упали до отправки
// уведомления — или наоборот, отправили уведомление, но откатили тикет).
type OutboxEntry struct {
	ID           int64
	UserID       int64
	TicketID     int64
	Type         NotificationType
	Payload      map[string]interface{}
	Status       OutboxStatus
	Attempts     int
	MaxAttempts  int
	NextRetryAt  time.Time
	SentAt       *time.Time
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// OutboxRepository — хранилище outbox-записей.
type OutboxRepository interface {
	// Create создаёт новую outbox-запись. Вызывается внутри той же
	// транзакции, что и бизнес-изменение — ctx должен нести значение
	// tickets.TxContextKey (см. app/tickets/service.go), иначе запись
	// уйдёт вне транзакции.
	Create(ctx context.Context, entry OutboxEntry) error

	// FindPending атомарно забирает до limit записей, готовых к обработке
	// (status=pending, NextRetryAt <= now), и помечает их processing —
	// внутри собственной транзакции с FOR UPDATE SKIP LOCKED, чтобы
	// несколько воркеров могли работать параллельно без гонок за одну и ту
	// же запись.
	FindPending(ctx context.Context, limit int) ([]OutboxEntry, error)

	// Update сохраняет новое состояние записи после попытки отправки
	// (Status, Attempts, NextRetryAt, SentAt, ErrorMessage).
	Update(ctx context.Context, entry OutboxEntry) error
}

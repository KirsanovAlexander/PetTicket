package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	apptickets "pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/notifications"

	"github.com/lib/pq"
)

// OutboxRepository реализует notifications.OutboxRepository поверх Postgres.
type OutboxRepository struct {
	db *DB
}

// NewOutboxRepository создаёт новый экземпляр репозитория outbox.
func NewOutboxRepository(db *DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// getExecutor возвращает executor: транзакцию из контекста, если она там
// есть (тот же ключ apptickets.TxContextKey, что использует
// TicketsRepository — чтобы outbox-запись реально попадала в ТУ ЖЕ
// транзакцию, что и бизнес-изменение тикета), либо пул соединений.
func (r *OutboxRepository) getExecutor(ctx context.Context) Executor {
	if tx, ok := ctx.Value(apptickets.TxContextKey).(*TxAdapter); ok && tx != nil {
		return tx.tx
	}
	return r.db.conn
}

const outboxSelectColumns = `
	id, user_id, ticket_id, notification_type, payload, status,
	attempts, max_attempts, next_retry_at, sent_at, error_message,
	created_at, updated_at
`

func scanOutboxEntry(scanner interface {
	Scan(dest ...any) error
}) (domain.OutboxEntry, error) {
	var e domain.OutboxEntry
	var notifType, status string
	var payloadJSON []byte
	var sentAt sql.NullTime
	var errorMessage sql.NullString

	err := scanner.Scan(
		&e.ID, &e.UserID, &e.TicketID, &notifType, &payloadJSON, &status,
		&e.Attempts, &e.MaxAttempts, &e.NextRetryAt, &sentAt, &errorMessage,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return domain.OutboxEntry{}, err
	}

	e.Type = domain.NotificationType(notifType)
	e.Status = domain.OutboxStatus(status)

	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
			return domain.OutboxEntry{}, fmt.Errorf("failed to unmarshal outbox payload: %w", err)
		}
	}

	if sentAt.Valid {
		t := sentAt.Time
		e.SentAt = &t
	}
	if errorMessage.Valid {
		e.ErrorMessage = errorMessage.String
	}

	return e, nil
}

// Create создаёт новую outbox-запись. Чтобы запись попала в ту же
// транзакцию, что и бизнес-изменение, вызывающая сторона обязана передать
// ctx, полученный через context.WithValue(ctx, apptickets.TxContextKey, tx).
func (r *OutboxRepository) Create(ctx context.Context, entry domain.OutboxEntry) error {
	payloadJSON, err := json.Marshal(entry.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	status := entry.Status
	if status == "" {
		status = domain.OutboxStatusPending
	}

	nextRetryAt := entry.NextRetryAt
	if nextRetryAt.IsZero() {
		nextRetryAt = time.Now()
	}

	query := `
		INSERT INTO notification_outbox (
			user_id, ticket_id, notification_type, payload, status,
			attempts, max_attempts, next_retry_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	exec := r.getExecutor(ctx)
	_, err = exec.ExecContext(ctx, query,
		entry.UserID,
		entry.TicketID,
		string(entry.Type),
		payloadJSON,
		string(status),
		entry.Attempts,
		entry.MaxAttempts,
		nextRetryAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create outbox entry: %w", err)
	}

	return nil
}

// FindPending атомарно забирает до limit pending-записей, готовых к
// обработке (next_retry_at <= now), и сразу помечает их processing —
// внутри собственной короткой транзакции с FOR UPDATE SKIP LOCKED. Так
// несколько воркеров могут одновременно вызывать FindPending и никогда не
// получат одну и ту же запись дважды: SKIP LOCKED просто пропускает строки,
// уже заблокированные другой параллельной транзакцией, вместо того чтобы
// ждать на них. Транзакция закрывается ДО отправки уведомления (сеть) —
// долгий HTTP-вызов не должен держать открытым соединение/блокировки в БД.
func (r *OutboxRepository) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	tx, err := r.db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // no-op, если транзакция уже закоммичена ниже
	}()

	query := `
		SELECT ` + outboxSelectColumns + `
		FROM notification_outbox
		WHERE status = $1 AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.QueryContext(ctx, query, string(domain.OutboxStatusPending), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox entries: %w", err)
	}

	var entries []domain.OutboxEntry
	for rows.Next() {
		entry, scanErr := scanOutboxEntry(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan outbox entry: %w", scanErr)
		}
		entries = append(entries, entry)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("failed to iterate outbox entries: %w", rowsErr)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close rows: %w", closeErr)
	}

	if len(entries) == 0 {
		if commitErr := tx.Commit(); commitErr != nil {
			return nil, fmt.Errorf("failed to commit empty claim transaction: %w", commitErr)
		}
		return nil, nil
	}

	ids := make([]int64, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE notification_outbox SET status = $1, updated_at = NOW() WHERE id = ANY($2)`,
		string(domain.OutboxStatusProcessing), pq.Array(ids),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to claim outbox entries: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit claim transaction: %w", err)
	}

	for i := range entries {
		entries[i].Status = domain.OutboxStatusProcessing
	}

	return entries, nil
}

// Update сохраняет состояние записи после попытки отправки.
func (r *OutboxRepository) Update(ctx context.Context, entry domain.OutboxEntry) error {
	query := `
		UPDATE notification_outbox
		SET status = $1, attempts = $2, next_retry_at = $3, sent_at = $4,
		    error_message = $5, updated_at = NOW()
		WHERE id = $6
	`

	exec := r.getExecutor(ctx)
	_, err := exec.ExecContext(ctx, query,
		string(entry.Status),
		entry.Attempts,
		entry.NextRetryAt,
		entry.SentAt,
		entry.ErrorMessage,
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update outbox entry: %w", err)
	}

	return nil
}

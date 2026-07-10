package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	apptickets "pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
)

// CommentsRepository реализует apptickets.CommentsRepository поверх Postgres.
type CommentsRepository struct {
	db *DB
}

// NewCommentsRepository создаёт новый экземпляр репозитория комментариев.
func NewCommentsRepository(db *DB) *CommentsRepository {
	return &CommentsRepository{db: db}
}

// getExecutor возвращает executor: транзакцию из контекста (тот же ключ
// apptickets.TxContextKey, что и у TicketsRepository/OutboxRepository —
// чтобы Create мог участвовать в транзакции AddComment) либо пул соединений.
func (r *CommentsRepository) getExecutor(ctx context.Context) Executor {
	if tx, ok := ctx.Value(apptickets.TxContextKey).(*TxAdapter); ok && tx != nil {
		return tx.tx
	}
	return r.db.conn
}

const commentSelectColumns = `id, ticket_id, user_id, content, is_internal, created_at, updated_at`

func scanTicketComment(scanner interface {
	Scan(dest ...any) error
}) (domain.TicketComment, error) {
	var c domain.TicketComment
	err := scanner.Scan(&c.ID, &c.TicketID, &c.UserID, &c.Content, &c.IsInternal, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return domain.TicketComment{}, err
	}
	return c, nil
}

// Create создаёт комментарий. Чтобы попасть в ту же транзакцию, что и
// dual-write в tickets.comment, ctx должен нести apptickets.TxContextKey
// (см. Service.AddComment).
func (r *CommentsRepository) Create(ctx context.Context, input domain.AddCommentInput) (domain.TicketComment, error) {
	query := `
		INSERT INTO ticket_comments (ticket_id, user_id, content, is_internal)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + commentSelectColumns

	exec := r.getExecutor(ctx)
	comment, err := scanTicketComment(exec.QueryRowContext(ctx, query,
		input.TicketID, input.UserID, input.Content, input.IsInternal,
	))
	if err != nil {
		return domain.TicketComment{}, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

// GetByTicketID возвращает комментарии тикета в хронологическом порядке.
func (r *CommentsRepository) GetByTicketID(ctx context.Context, filter domain.ListCommentsFilter) ([]domain.TicketComment, error) {
	query := `SELECT ` + commentSelectColumns + ` FROM ticket_comments WHERE ticket_id = $1`
	args := []interface{}{filter.TicketID}
	argPos := 2

	if !filter.IncludeInternal {
		query += " AND is_internal = false"
	}

	query += " ORDER BY created_at ASC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	exec := r.getExecutor(ctx)
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var comments []domain.TicketComment
	for rows.Next() {
		c, scanErr := scanTicketComment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", scanErr)
		}
		comments = append(comments, c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("failed to iterate comments: %w", rowsErr)
	}

	return comments, nil
}

// GetLastByTicketID возвращает последний комментарий тикета (включая
// internal) либо nil, если комментариев ещё нет.
func (r *CommentsRepository) GetLastByTicketID(ctx context.Context, ticketID int64) (*domain.TicketComment, error) {
	query := `
		SELECT ` + commentSelectColumns + `
		FROM ticket_comments
		WHERE ticket_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	exec := r.getExecutor(ctx)
	comment, err := scanTicketComment(exec.QueryRowContext(ctx, query, ticketID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last comment: %w", err)
	}

	return &comment, nil
}

// Update редактирует содержимое комментария. updated_at не выставляется
// вручную — таблица несёт триггер update_ticket_comments_updated_at
// (переиспользует update_updated_at_column() из 001_init), который
// обновляет её при ЛЮБОМ UPDATE строки, а не только через этот метод.
func (r *CommentsRepository) Update(ctx context.Context, input domain.UpdateCommentInput) error {
	query := `UPDATE ticket_comments SET content = $1 WHERE id = $2`

	exec := r.getExecutor(ctx)
	result, err := exec.ExecContext(ctx, query, input.Content, input.ID)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return apptickets.ErrNotFound
	}

	return nil
}

// Delete удаляет комментарий.
func (r *CommentsRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM ticket_comments WHERE id = $1`

	exec := r.getExecutor(ctx)
	result, err := exec.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return apptickets.ErrNotFound
	}

	return nil
}

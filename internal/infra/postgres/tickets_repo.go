package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
)

// contextKey - приватный тип для ключей контекста
type contextKey string

const (
	// txContextKey - ключ для хранения транзакции в контексте
	txContextKey contextKey = "tx"
)

// TicketsRepository реализует интерфейс tickets.Repository
type TicketsRepository struct {
	db *DB
}

// Executor интерфейс для выполнения запросов (поддерживает как *sql.DB, так и *sql.Tx)
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// NewTicketsRepository создаёт новый экземпляр репозитория
func NewTicketsRepository(db *DB) *TicketsRepository {
	return &TicketsRepository{db: db}
}

// getExecutor возвращает executor (либо транзакцию из контекста, либо базовое соединение)
func (r *TicketsRepository) getExecutor(ctx context.Context) Executor {
	if tx, ok := ctx.Value(txContextKey).(*TxAdapter); ok && tx != nil {
		return tx.tx
	}
	return r.db.conn
}

// Create создаёт новый тикет
func (r *TicketsRepository) Create(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	priority := ticket.Priority
	if !priority.IsValid() {
		priority = domain.PriorityMedium
	}

	query := `
		INSERT INTO tickets (user_id, topic_id, status_id, priority_id, amount, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	exec := r.getExecutor(ctx)
	err := exec.QueryRowContext(ctx, query,
		ticket.UserID,
		ticket.TopicID,
		int(ticket.Status),
		int(priority),
		ticket.Amount,
		ticket.Comment,
	).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt)

	if err != nil {
		return domain.Ticket{}, fmt.Errorf("failed to create ticket: %w", err)
	}

	return ticket, nil
}

// GetByID возвращает тикет по ID
func (r *TicketsRepository) GetByID(ctx context.Context, id int64) (domain.Ticket, error) {
	query := `
		SELECT id, user_id, topic_id, status_id, priority_id, amount, comment, created_at, updated_at
		FROM tickets
		WHERE id = $1
	`

	var ticket domain.Ticket
	var statusID int
	var priorityID int

	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.TopicID,
		&statusID,
		&priorityID,
		&ticket.Amount,
		&ticket.Comment,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ticket{}, tickets.ErrNotFound
		}
		return domain.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	ticket.Status = domain.Status(statusID)
	ticket.Priority = domain.Priority(priorityID)
	return ticket, nil
}

// Update обновляет существующий тикет
func (r *TicketsRepository) Update(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	query := `
		UPDATE tickets
		SET user_id = $1, topic_id = $2, status_id = $3, priority_id = $4, amount = $5, comment = $6
		WHERE id = $7
		RETURNING updated_at
	`

	exec := r.getExecutor(ctx)
	err := exec.QueryRowContext(ctx, query,
		ticket.UserID,
		ticket.TopicID,
		int(ticket.Status),
		int(ticket.Priority),
		ticket.Amount,
		ticket.Comment,
		ticket.ID,
	).Scan(&ticket.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ticket{}, tickets.ErrNotFound
		}
		return domain.Ticket{}, fmt.Errorf("failed to update ticket: %w", err)
	}

	return ticket, nil
}

// Delete удаляет тикет по ID
func (r *TicketsRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM tickets WHERE id = $1`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete ticket: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return tickets.ErrNotFound
	}

	return nil
}

// List возвращает список тикетов с фильтрацией
func (r *TicketsRepository) List(ctx context.Context, filter tickets.ListFilter) ([]domain.Ticket, error) {
	query := `
		SELECT id, user_id, topic_id, status_id, priority_id, amount, comment, created_at, updated_at
		FROM tickets
		WHERE 1=1
	`
	args := []interface{}{}
	argPos := 1

	// Фильтрация
	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argPos)
		args = append(args, *filter.UserID)
		argPos++
	}

	if filter.TopicID != nil {
		query += fmt.Sprintf(" AND topic_id = $%d", argPos)
		args = append(args, *filter.TopicID)
		argPos++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status_id = $%d", argPos)
		args = append(args, int(*filter.Status))
		argPos++
	}

	if filter.Priority != nil {
		query += fmt.Sprintf(" AND priority_id = $%d", argPos)
		args = append(args, int(*filter.Priority))
		argPos++
	}

	// Сортировка (whitelist для защиты от SQL injection)
	allowedSortFields := map[string]bool{
		"id":          true,
		"user_id":     true,
		"topic_id":    true,
		"status_id":   true,
		"priority_id": true,
		"amount":      true,
		"created_at":  true,
		"updated_at":  true,
	}

	sortBy := "created_at"
	if filter.SortBy != "" && allowedSortFields[filter.SortBy] {
		sortBy = filter.SortBy
	}

	sortOrder := "DESC"
	if !filter.SortDesc {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Пагинация
	if filter.Limit > 0 {
		query += " LIMIT $" + strconv.Itoa(argPos) //nolint:gosec // G202: argPos is controlled, not user input
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += " OFFSET $" + strconv.Itoa(argPos) //nolint:gosec // G202: argPos is controlled, not user input
		args = append(args, filter.Offset)
		// argPos++ - not needed, it's the last parameter
	}

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var ticketList []domain.Ticket
	for rows.Next() {
		var ticket domain.Ticket
		var statusID int
		var priorityID int

		err := rows.Scan(
			&ticket.ID,
			&ticket.UserID,
			&ticket.TopicID,
			&statusID,
			&priorityID,
			&ticket.Amount,
			&ticket.Comment,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}

		ticket.Status = domain.Status(statusID)
		ticket.Priority = domain.Priority(priorityID)
		ticketList = append(ticketList, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tickets: %w", err)
	}

	return ticketList, nil
}

// AddHistory добавляет запись в историю тикета
func (r *TicketsRepository) AddHistory(ctx context.Context, history domain.History) error {
	query := `
		INSERT INTO ticket_history (ticket_id, user_id, action, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5)
	`

	exec := r.getExecutor(ctx)
	_, err := exec.ExecContext(ctx, query,
		history.TicketID,
		history.UserID,
		string(history.Action),
		history.OldValue,
		history.NewValue,
	)

	if err != nil {
		return fmt.Errorf("failed to add history: %w", err)
	}

	return nil
}

// GetHistory возвращает историю изменений тикета
func (r *TicketsRepository) GetHistory(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error) {
	query := `
		SELECT id, ticket_id, user_id, action, old_value, new_value, created_at
		FROM ticket_history
		WHERE ticket_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.conn.QueryContext(ctx, query, ticketID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var historyList []domain.History
	for rows.Next() {
		var history domain.History
		var action string

		err := rows.Scan(
			&history.ID,
			&history.TicketID,
			&history.UserID,
			&action,
			&history.OldValue,
			&history.NewValue,
			&history.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}

		history.Action = domain.HistoryAction(action)
		historyList = append(historyList, history)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate history: %w", err)
	}

	return historyList, nil
}

// GetAllStatuses возвращает все статусы из справочника
func (r *TicketsRepository) GetAllStatuses(ctx context.Context) ([]tickets.StatusInfo, error) {
	query := `
		SELECT id, name
		FROM ticket_statuses
		ORDER BY id
	`

	rows, err := r.db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get statuses: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var statuses []tickets.StatusInfo
	for rows.Next() {
		var status tickets.StatusInfo
		err := rows.Scan(&status.ID, &status.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan status: %w", err)
		}
		statuses = append(statuses, status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate statuses: %w", err)
	}

	return statuses, nil
}

// GetAllTopics возвращает все темы из справочника
func (r *TicketsRepository) GetAllTopics(ctx context.Context) ([]domain.Topic, error) {
	query := `
		SELECT id, external_id, title, description
		FROM ticket_topics
		ORDER BY id
	`

	rows, err := r.db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var topics []domain.Topic
	for rows.Next() {
		var topic domain.Topic
		err := rows.Scan(&topic.ID, &topic.ExternalID, &topic.Title, &topic.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to scan topic: %w", err)
		}
		topics = append(topics, topic)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate topics: %w", err)
	}

	return topics, nil
}

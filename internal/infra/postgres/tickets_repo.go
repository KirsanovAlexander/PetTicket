package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"

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

	statusCacheMu   sync.RWMutex
	statusCache     map[string]int64
	statusCacheInit bool
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

// getStatusIDByName возвращает ID статуса по его имени, используя закешированный маппинг
// name -> id, загруженный из таблицы ticket_statuses. Кеш заполняется один раз (double-checked locking)
// и потокобезопасен благодаря statusCacheMu.
func (r *TicketsRepository) getStatusIDByName(ctx context.Context, statusName string) (int64, error) {
	r.statusCacheMu.RLock()
	if r.statusCacheInit {
		id, ok := r.statusCache[statusName]
		r.statusCacheMu.RUnlock()
		if !ok {
			return 0, fmt.Errorf("%w: unknown status %q", tickets.ErrInvalidStatus, statusName)
		}
		return id, nil
	}
	r.statusCacheMu.RUnlock()

	r.statusCacheMu.Lock()
	defer r.statusCacheMu.Unlock()

	if !r.statusCacheInit {
		if err := r.loadStatusCache(ctx); err != nil {
			return 0, err
		}
	}

	id, ok := r.statusCache[statusName]
	if !ok {
		return 0, fmt.Errorf("%w: unknown status %q", tickets.ErrInvalidStatus, statusName)
	}
	return id, nil
}

// loadStatusCache загружает маппинг name -> id из ticket_statuses одним запросом.
// Вызывающий обязан удерживать statusCacheMu на запись.
func (r *TicketsRepository) loadStatusCache(ctx context.Context) error {
	rows, err := r.db.conn.QueryContext(ctx, `SELECT id, name FROM ticket_statuses`)
	if err != nil {
		return fmt.Errorf("failed to load ticket statuses: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	cache := make(map[string]int64)
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return fmt.Errorf("failed to scan ticket status: %w", err)
		}
		cache[name] = id
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate ticket statuses: %w", err)
	}

	r.statusCache = cache
	r.statusCacheInit = true
	return nil
}

// InvalidateStatusCache сбрасывает кеш маппинга статусов (используется в тестах)
func (r *TicketsRepository) InvalidateStatusCache() {
	r.statusCacheMu.Lock()
	defer r.statusCacheMu.Unlock()
	r.statusCache = nil
	r.statusCacheInit = false
}

const ticketSelectColumns = `
	id, user_id, topic_id, status_id, priority_id, amount, comment,
	response_deadline, resolution_deadline, first_response_at, resolved_at,
	last_user_activity_at, created_at, updated_at
`

func scanTicket(scanner interface {
	Scan(dest ...any) error
}) (domain.Ticket, error) {
	var ticket domain.Ticket
	var statusID, priorityID int
	var responseDeadline, resolutionDeadline, firstResponseAt, resolvedAt sql.NullTime
	var lastUserActivityAt sql.NullTime

	err := scanner.Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.TopicID,
		&statusID,
		&priorityID,
		&ticket.Amount,
		&ticket.Comment,
		&responseDeadline,
		&resolutionDeadline,
		&firstResponseAt,
		&resolvedAt,
		&lastUserActivityAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		return domain.Ticket{}, err
	}

	ticket.Status = domain.Status(statusID)
	ticket.Priority = domain.Priority(priorityID)
	if responseDeadline.Valid {
		ticket.ResponseDeadline = responseDeadline.Time
	}
	if resolutionDeadline.Valid {
		ticket.ResolutionDeadline = resolutionDeadline.Time
	}
	if firstResponseAt.Valid {
		t := firstResponseAt.Time
		ticket.FirstResponseAt = &t
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		ticket.ResolvedAt = &t
	}
	if lastUserActivityAt.Valid {
		ticket.LastUserActivityAt = lastUserActivityAt.Time
	}

	return ticket, nil
}

// Create создаёт новый тикет
func (r *TicketsRepository) Create(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	priority := ticket.Priority
	if !priority.IsValid() {
		priority = domain.PriorityMedium
	}

	query := `
		INSERT INTO tickets (
			user_id, topic_id, status_id, priority_id, amount, comment,
			response_deadline, resolution_deadline
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
		ticket.ResponseDeadline,
		ticket.ResolutionDeadline,
	).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt)

	if err != nil {
		return domain.Ticket{}, fmt.Errorf("failed to create ticket: %w", err)
	}

	return ticket, nil
}

// GetByID возвращает тикет по ID
func (r *TicketsRepository) GetByID(ctx context.Context, id int64) (domain.Ticket, error) {
	query := `
		SELECT ` + ticketSelectColumns + `
		FROM tickets
		WHERE id = $1
	`

	ticket, err := scanTicket(r.db.conn.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ticket{}, tickets.ErrNotFound
		}
		return domain.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	return ticket, nil
}

// Update обновляет существующий тикет
func (r *TicketsRepository) Update(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	query := `
		UPDATE tickets
		SET user_id = $1, topic_id = $2, status_id = $3, priority_id = $4, amount = $5, comment = $6,
		    response_deadline = $7, resolution_deadline = $8, first_response_at = $9, resolved_at = $10
		WHERE id = $11
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
		ticket.ResponseDeadline,
		ticket.ResolutionDeadline,
		ticket.FirstResponseAt,
		ticket.ResolvedAt,
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
		SELECT ` + ticketSelectColumns + `
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
		statusName := filter.Status.String()
		statusID, err := r.getStatusIDByName(ctx, statusName)
		if err != nil {
			return nil, fmt.Errorf("failed to get status ID: %w", err)
		}
		query += fmt.Sprintf(" AND status_id = $%d", argPos)
		args = append(args, statusID)
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
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}
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

// GetSLARule возвращает правило SLA для topic + priority
func (r *TicketsRepository) GetSLARule(ctx context.Context, topicID, priorityID int64) (*domain.SLARule, error) {
	query := `
		SELECT id, topic_id, priority_id, response_time_minutes, resolution_time_minutes
		FROM sla_rules
		WHERE topic_id = $1 AND priority_id = $2
	`

	var rule domain.SLARule
	err := r.db.conn.QueryRowContext(ctx, query, topicID, priorityID).Scan(
		&rule.ID,
		&rule.TopicID,
		&rule.PriorityID,
		&rule.ResponseTimeMinutes,
		&rule.ResolutionTimeMinutes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sla rule: %w", err)
	}

	return &rule, nil
}

// FindSLAViolations возвращает тикеты с нарушенным SLA
func (r *TicketsRepository) FindSLAViolations(ctx context.Context) ([]domain.Ticket, error) {
	query := `
		SELECT ` + ticketSelectColumns + `
		FROM tickets
		WHERE
			(first_response_at IS NULL AND response_deadline < NOW())
			OR (resolved_at IS NULL AND resolution_deadline < NOW())
			OR (first_response_at IS NOT NULL AND first_response_at > response_deadline)
			OR (resolved_at IS NOT NULL AND resolved_at > resolution_deadline)
		ORDER BY priority_id DESC, created_at ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find sla violations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var ticketList []domain.Ticket
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}
		ticketList = append(ticketList, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sla violations: %w", err)
	}

	return ticketList, nil
}

// FindResolvedTicketsOlderThan возвращает resolved тикеты с неактивностью старше N дней
func (r *TicketsRepository) FindResolvedTicketsOlderThan(
	ctx context.Context, inactiveDays int, limit int,
) ([]domain.Ticket, error) {
	query := `
		SELECT ` + ticketSelectColumns + `
		FROM tickets
		WHERE
			status_id = (SELECT id FROM ticket_statuses WHERE name = 'resolved')
			AND last_user_activity_at < NOW() - INTERVAL '1 day' * $1
		ORDER BY last_user_activity_at ASC
		LIMIT $2
	`

	rows, err := r.db.conn.QueryContext(ctx, query, inactiveDays, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query resolved tickets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var ticketList []domain.Ticket
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}
		ticketList = append(ticketList, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tickets: %w", err)
	}

	return ticketList, nil
}

// UpdateLastUserActivity обновляет время последней активности пользователя
func (r *TicketsRepository) UpdateLastUserActivity(ctx context.Context, ticketID int64) error {
	query := `
		UPDATE tickets
		SET last_user_activity_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	exec := r.getExecutor(ctx)
	_, err := exec.ExecContext(ctx, query, ticketID)
	if err != nil {
		return fmt.Errorf("failed to update last user activity: %w", err)
	}
	return nil
}

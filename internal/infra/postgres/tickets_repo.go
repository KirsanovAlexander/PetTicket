package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
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

// getExecutor возвращает executor (либо транзакцию из контекста, либо базовое соединение).
// Ключ — tickets.TxContextKey (см. её комментарий в app/tickets/service.go
// про то, почему это НЕ отдельный локальный contextKey этого пакета).
func (r *TicketsRepository) getExecutor(ctx context.Context) Executor {
	if tx, ok := ctx.Value(tickets.TxContextKey).(*TxAdapter); ok && tx != nil {
		return tx.tx
	}
	return r.db.conn
}

const ticketSelectColumns = `
	id, user_id, topic_id, status_id, priority_id, amount, comment,
	response_deadline, resolution_deadline, first_response_at, resolved_at,
	last_user_activity_at, created_at, updated_at,
	assigned_to, assigned_at, version
`

func scanTicket(scanner interface {
	Scan(dest ...any) error
}) (domain.Ticket, error) {
	var ticket domain.Ticket
	var statusID, priorityID int
	var responseDeadline, resolutionDeadline, firstResponseAt, resolvedAt sql.NullTime
	var lastUserActivityAt sql.NullTime
	var assignedTo sql.NullInt64
	var assignedAt sql.NullTime

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
		&assignedTo,
		&assignedAt,
		&ticket.Version,
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
	if assignedTo.Valid {
		id := assignedTo.Int64
		ticket.AssignedTo = &id
	}
	if assignedAt.Valid {
		t := assignedAt.Time
		ticket.AssignedAt = &t
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
		RETURNING id, created_at, updated_at, version
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
	).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt, &ticket.Version)

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

// AssignWithVersion атомарно назначает тикет на assigneeID: WHERE version =
// $3 AND assigned_to IS NULL гарантирует, что при гонке нескольких саппортов
// UPDATE применится только у одного из них (у остальных к моменту их
// собственного UPDATE версия строки уже не будет совпадать с той, что они
// прочитали через GetByID). status_id переводится в in_progress той же
// командой — отдельного похода за статусом не нужно.
func (r *TicketsRepository) AssignWithVersion(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
	query := `
		UPDATE tickets
		SET assigned_to = $1, assigned_at = NOW(), status_id = $4,
		    version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3 AND assigned_to IS NULL
	`
	exec := r.getExecutor(ctx)
	result, err := exec.ExecContext(ctx, query, assigneeID, ticketID, expectedVersion, int(domain.StatusInProgress))
	if err != nil {
		return fmt.Errorf("failed to assign ticket: %w", err)
	}
	return r.checkVersionedUpdate(ctx, result, ticketID)
}

// UnassignWithVersion атомарно снимает назначение: WHERE assigned_to = $2
// AND version = $3 — снять может только текущий владелец, и только если
// версия строки не уехала с момента его GetByID. Статус не трогаем: тикет
// мог быть уже переведён дальше по воркфлоу, откатывать его в неопределённое
// состояние не наша забота.
func (r *TicketsRepository) UnassignWithVersion(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
	query := `
		UPDATE tickets
		SET assigned_to = NULL, assigned_at = NULL, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND assigned_to = $2 AND version = $3
	`
	exec := r.getExecutor(ctx)
	result, err := exec.ExecContext(ctx, query, ticketID, assigneeID, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to unassign ticket: %w", err)
	}
	return r.checkVersionedUpdate(ctx, result, ticketID)
}

// checkVersionedUpdate различает "тикета не существует" (ErrNotFound) от
// "существует, но версия/владелец уже другие" (ErrOptimisticLockConflict) —
// rowsAffected=0 сам по себе не говорит, какой из двух случаев произошёл.
// Проверка идёт через r.db.conn (пул), а не getExecutor(ctx): строка тикета
// уже существовала до начала этой транзакции (её создал не текущий вызов),
// поэтому её видимость не зависит от того, через какое соединение читать.
func (r *TicketsRepository) checkVersionedUpdate(ctx context.Context, result sql.Result, ticketID int64) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		var exists bool
		if err := r.db.conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM tickets WHERE id = $1)", ticketID).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check ticket existence: %w", err)
		}
		if !exists {
			return tickets.ErrNotFound
		}
		return tickets.ErrOptimisticLockConflict
	}
	return nil
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
		query += fmt.Sprintf(" AND status_id = $%d", argPos)
		args = append(args, int(*filter.Status))
		argPos++
	}

	if filter.Priority != nil {
		query += fmt.Sprintf(" AND priority_id = $%d", argPos)
		args = append(args, int(*filter.Priority))
		argPos++
	}

	if filter.Unassigned {
		query += " AND assigned_to IS NULL"
	} else if filter.AssignedTo != nil {
		query += fmt.Sprintf(" AND assigned_to = $%d", argPos)
		args = append(args, *filter.AssignedTo)
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

// ListWithCursor возвращает список тикетов с cursor-пагинацией.
// Для direction="next" (по умолчанию) фильтрует (created_at, id) < cursor и
// сортирует DESC — движение вглубь истории. Для direction="prev" фильтрует
// (created_at, id) > cursor, сортирует ASC (чтобы взять ближайшие к cursor
// записи через LIMIT), а затем разворачивает результат в обычный дисплейный
// порядок DESC. В обоих случаях запрашивается PageSize+1 строк: если пришла
// лишняя строка — hasMore=true, и она отбрасывается.
func (r *TicketsRepository) ListWithCursor(
	ctx context.Context, filter tickets.ListFilter,
) ([]domain.Ticket, bool, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	direction := filter.Direction
	if direction != "prev" {
		direction = "next"
	}

	query := `
		SELECT ` + ticketSelectColumns + `
		FROM tickets
		WHERE 1=1
	`
	args := []interface{}{}
	argPos := 1

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

	if filter.Unassigned {
		query += " AND assigned_to IS NULL"
	} else if filter.AssignedTo != nil {
		query += fmt.Sprintf(" AND assigned_to = $%d", argPos)
		args = append(args, *filter.AssignedTo)
		argPos++
	}

	if filter.Cursor != nil && *filter.Cursor != "" {
		createdAt, id, err := tickets.DecodeCursor(*filter.Cursor)
		if err != nil {
			return nil, false, fmt.Errorf("%w: %v", tickets.ErrInvalidCursor, err)
		}

		op := "<"
		if direction == "prev" {
			op = ">"
		}
		query += fmt.Sprintf(" AND (created_at, id) %s ($%d, $%d)", op, argPos, argPos+1)
		args = append(args, createdAt, id)
		argPos += 2
	}

	sortOrder := "DESC"
	if direction == "prev" {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY created_at %s, id %s", sortOrder, sortOrder)

	// Запрашиваем на 1 запись больше pageSize, чтобы определить hasMore
	query += " LIMIT $" + strconv.Itoa(argPos) //nolint:gosec // G202: argPos is controlled, not user input
	args = append(args, pageSize+1)

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list tickets with cursor: %w", err)
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
			return nil, false, fmt.Errorf("failed to scan ticket: %w", err)
		}
		ticketList = append(ticketList, ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("failed to iterate tickets: %w", err)
	}

	hasMore := len(ticketList) > pageSize
	if hasMore {
		ticketList = ticketList[:pageSize]
	}

	if direction == "prev" {
		reverseTicketList(ticketList)
	}

	return ticketList, hasMore, nil
}

// reverseTicketList разворачивает список тикетов на месте
func reverseTicketList(list []domain.Ticket) {
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
}

const ticketFullSelectColumns = `
	t.id, t.user_id, t.status_id, t.priority_id, t.amount, t.comment,
	t.response_deadline, t.resolution_deadline, t.first_response_at, t.resolved_at,
	t.created_at, t.updated_at, t.assigned_to, t.version,
	s.name, s.description,
	tp.id, tp.external_id, tp.title, tp.description
`

const ticketFullFromJoin = `
	FROM tickets t
	JOIN ticket_statuses s ON t.status_id = s.id
	JOIN ticket_topics tp ON t.topic_id = tp.id
`

// scanTicketFullRow сканирует строку JOIN-запроса (tickets + ticket_statuses +
// ticket_topics) в domain.TicketFull и досчитывает SLA из уже прочитанных
// дедлайнов — без дополнительных запросов к БД. Assignee читается прямо из
// t.assigned_to (Task 13) — раньше это был best-effort парсинг последней
// записи ticket_history (см. историю getTicketAssignee), теперь есть
// нормальная колонка. Comments сюда не входят: это отдельная выборка (см.
// getTicketComments), вызывающая сторона решает, нужна ли она.
func scanTicketFullRow(scanner interface {
	Scan(dest ...any) error
}) (domain.TicketFull, error) {
	var full domain.TicketFull
	var statusID, priorityID int
	var responseDeadline, resolutionDeadline, firstResponseAt, resolvedAt sql.NullTime
	var assignedTo sql.NullInt64

	err := scanner.Scan(
		&full.ID, &full.User.ID, &statusID, &priorityID, &full.Amount, &full.Comment,
		&responseDeadline, &resolutionDeadline, &firstResponseAt, &resolvedAt,
		&full.CreatedAt, &full.UpdatedAt, &assignedTo, &full.Version,
		&full.Status.Name, &full.Status.DisplayName,
		&full.Topic.ID, &full.Topic.ExternalID, &full.Topic.Title, &full.Topic.Description,
	)
	if err != nil {
		return domain.TicketFull{}, err
	}

	full.Status.ID = statusID
	full.Priority = domain.Priority(priorityID)
	if assignedTo.Valid {
		full.Assignee = &domain.User{ID: assignedTo.Int64}
	}

	var responseDeadlineTime, resolutionDeadlineTime time.Time
	if responseDeadline.Valid {
		responseDeadlineTime = responseDeadline.Time
	}
	if resolutionDeadline.Valid {
		resolutionDeadlineTime = resolutionDeadline.Time
	}

	var firstResponseAtPtr, resolvedAtPtr *time.Time
	if firstResponseAt.Valid {
		t := firstResponseAt.Time
		firstResponseAtPtr = &t
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		resolvedAtPtr = &t
	}

	metrics := domain.CalculateSLAStatus(
		full.CreatedAt, responseDeadlineTime, resolutionDeadlineTime,
		firstResponseAtPtr, resolvedAtPtr, time.Now(),
	)
	full.SLA = &domain.SLAInfo{
		ResponseDeadline:   responseDeadlineTime,
		ResolutionDeadline: resolutionDeadlineTime,
		ResponseStatus:     metrics.ResponseStatus,
		ResolutionStatus:   metrics.ResolutionStatus,
		OverallStatus:      metrics.OverallStatus,
	}

	return full, nil
}

// GetFullByID возвращает тикет со всеми связями, раскрытыми во вложенные
// объекты (v2 API).
func (r *TicketsRepository) GetFullByID(ctx context.Context, id int64) (domain.TicketFull, error) {
	query := `SELECT ` + ticketFullSelectColumns + ticketFullFromJoin + ` WHERE t.id = $1`

	full, err := scanTicketFullRow(r.db.conn.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TicketFull{}, tickets.ErrNotFound
		}
		return domain.TicketFull{}, fmt.Errorf("failed to get full ticket: %w", err)
	}

	comments, err := r.getTicketComments(ctx, id)
	if err != nil {
		return domain.TicketFull{}, err
	}
	full.Comments = comments

	return full, nil
}

// ListFull возвращает список тикетов с раскрытыми статусом/темой.
// Фильтрация и сортировка дублируют логику List() — сознательно не
// вынесены в общий хелпер, чтобы не рефакторить уже рабочий List() без
// возможности прогнать компилятор в этой сессии (см. итоговое сообщение).
func (r *TicketsRepository) ListFull(ctx context.Context, filter tickets.ListFilter) ([]domain.TicketFull, error) {
	query := `SELECT ` + ticketFullSelectColumns + ticketFullFromJoin + ` WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND t.user_id = $%d", argPos)
		args = append(args, *filter.UserID)
		argPos++
	}

	if filter.TopicID != nil {
		query += fmt.Sprintf(" AND t.topic_id = $%d", argPos)
		args = append(args, *filter.TopicID)
		argPos++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND t.status_id = $%d", argPos)
		args = append(args, int(*filter.Status))
		argPos++
	}

	if filter.Priority != nil {
		query += fmt.Sprintf(" AND t.priority_id = $%d", argPos)
		args = append(args, int(*filter.Priority))
		argPos++
	}

	if filter.Unassigned {
		query += " AND t.assigned_to IS NULL"
	} else if filter.AssignedTo != nil {
		query += fmt.Sprintf(" AND t.assigned_to = $%d", argPos)
		args = append(args, *filter.AssignedTo)
		argPos++
	}

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
	query += fmt.Sprintf(" ORDER BY t.%s %s", sortBy, sortOrder)

	if filter.Limit > 0 {
		query += " LIMIT $" + strconv.Itoa(argPos) //nolint:gosec // G202: argPos is controlled, not user input
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += " OFFSET $" + strconv.Itoa(argPos) //nolint:gosec // G202: argPos is controlled, not user input
		args = append(args, filter.Offset)
	}

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list full tickets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var list []domain.TicketFull
	for rows.Next() {
		full, err := scanTicketFullRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan full ticket: %w", err)
		}
		list = append(list, full)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate full tickets: %w", err)
	}

	return list, nil
}

// getTicketComments возвращает комментарии тикета, реконструированные из
// ticket_history (записи action=comment_added, new_value = текст).
// Отдельной таблицы комментариев в системе нет.
func (r *TicketsRepository) getTicketComments(ctx context.Context, ticketID int64) ([]domain.Comment, error) {
	query := `
		SELECT id, user_id, new_value, created_at
		FROM ticket_history
		WHERE ticket_id = $1 AND action = $2
		ORDER BY created_at ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, ticketID, string(domain.ActionCommentAdded))
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket comments: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close rows: %w", closeErr)
		}
	}()

	var comments []domain.Comment
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.UserID, &c.Text, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate comments: %w", err)
	}

	return comments, nil
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

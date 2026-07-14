package tickets

import (
	"context"

	"pet-ticket/internal/domain/tickets"
)

// Repository определяет контракт для работы с хранилищем тикетов
// Интерфейс находится в app слое (dependency inversion principle)
//
//nolint:dupl // Interface and mock have similar structure by design
type Repository interface {
	// Create создаёт новый тикет
	Create(ctx context.Context, ticket tickets.Ticket) (tickets.Ticket, error)

	// GetByID возвращает тикет по ID
	GetByID(ctx context.Context, id int64) (tickets.Ticket, error)

	// Update обновляет существующий тикет
	Update(ctx context.Context, ticket tickets.Ticket) (tickets.Ticket, error)

	// Delete удаляет тикет по ID
	Delete(ctx context.Context, id int64) error

	// List возвращает список тикетов с фильтрацией (offset-пагинация)
	List(ctx context.Context, filter ListFilter) ([]tickets.Ticket, error)

	// ListWithCursor возвращает список тикетов с cursor-пагинацией.
	// Запрашивает на 1 запись больше PageSize — если она пришла, hasMore=true
	// и лишняя запись отбрасывается.
	ListWithCursor(ctx context.Context, filter ListFilter) (items []tickets.Ticket, hasMore bool, err error)

	// GetFullByID возвращает тикет со всеми связями, раскрытыми во
	// вложенные объекты (для v2 API): статус с DisplayName, тема,
	// комментарии (из ticket_history) и, если назначен, assignee.
	GetFullByID(ctx context.Context, id int64) (tickets.TicketFull, error)

	// ListFull возвращает список тикетов с раскрытыми статусом/темой (для
	// v2 API). В отличие от GetFullByID НЕ подгружает Comments/Assignee —
	// это отдельные запросы на тикет, недопустимые в списке (N+1).
	ListFull(ctx context.Context, filter ListFilter) ([]tickets.TicketFull, error)

	// AddHistory добавляет запись в историю тикета
	AddHistory(ctx context.Context, history tickets.History) error

	// GetHistory возвращает историю изменений тикета
	GetHistory(ctx context.Context, ticketID int64, limit, offset int) ([]tickets.History, error)

	// GetAllStatuses возвращает все статусы тикетов из справочника
	GetAllStatuses(ctx context.Context) ([]StatusInfo, error)

	// GetAllTopics возвращает все темы тикетов из справочника
	GetAllTopics(ctx context.Context) ([]tickets.Topic, error)

	// GetSLARule возвращает правило SLA для topic + priority (nil, nil если не найдено)
	GetSLARule(ctx context.Context, topicID, priorityID int64) (*tickets.SLARule, error)

	// FindSLAViolations возвращает тикеты с нарушенным SLA
	FindSLAViolations(ctx context.Context) ([]tickets.Ticket, error)

	// FindResolvedTicketsOlderThan возвращает resolved тикеты с неактивностью старше N дней
	FindResolvedTicketsOlderThan(ctx context.Context, inactiveDays int, limit int) ([]tickets.Ticket, error)

	// UpdateLastUserActivity обновляет время последней активности пользователя
	UpdateLastUserActivity(ctx context.Context, ticketID int64) error

	// AssignWithVersion атомарно назначает тикет на assigneeID при условии,
	// что в БД версия строки всё ещё равна expectedVersion и тикет ещё не
	// назначен (WHERE version = $N AND assigned_to IS NULL) — так при гонке
	// нескольких саппортов ровно один UPDATE проходит. Возвращает
	// ErrNotFound, если тикета не существует, ErrOptimisticLockConflict —
	// если существует, но версия/назначение уже другие.
	AssignWithVersion(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error

	// UnassignWithVersion атомарно снимает назначение при условии, что тикет
	// всё ещё назначен на assigneeID и версия строки равна expectedVersion
	// (WHERE assigned_to = $N AND version = $M). Возвращает ErrNotFound/
	// ErrOptimisticLockConflict аналогично AssignWithVersion.
	UnassignWithVersion(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error
}

// StatusInfo представляет информацию о статусе из БД
type StatusInfo struct {
	ID   int
	Name string
}

// ListFilter определяет параметры фильтрации списка тикетов
type ListFilter struct {
	UserID   *int64
	TopicID  *int64
	Status   *tickets.Status
	Priority *tickets.Priority
	Limit    int
	Offset   int
	SortBy   string
	SortDesc bool

	// Поля для cursor-пагинации (используются только ListWithCursor)
	Cursor    *string
	PageSize  int
	Direction string

	// AssignedTo фильтрует тикеты, назначенные на конкретного саппорта.
	// Unassigned — тикеты без назначения (assigned_to IS NULL). Оба поля
	// одновременно не имеют смысла, но взаимная валидация — забота вызывающей
	// стороны (см. v1 listTickets), а не репозитория.
	AssignedTo *int64
	Unassigned bool
}

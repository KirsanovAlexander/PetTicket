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
}

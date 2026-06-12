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

	// List возвращает список тикетов с фильтрацией
	List(ctx context.Context, filter ListFilter) ([]tickets.Ticket, error)

	// AddHistory добавляет запись в историю тикета
	AddHistory(ctx context.Context, history tickets.History) error

	// GetHistory возвращает историю изменений тикета
	GetHistory(ctx context.Context, ticketID int64, limit, offset int) ([]tickets.History, error)

	// GetAllStatuses возвращает все статусы тикетов из справочника
	GetAllStatuses(ctx context.Context) ([]StatusInfo, error)

	// GetAllTopics возвращает все темы тикетов из справочника
	GetAllTopics(ctx context.Context) ([]tickets.Topic, error)
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
}

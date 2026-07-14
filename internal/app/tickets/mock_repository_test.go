package tickets

import (
	"context"
	"errors"
	"sync"

	domain "pet-ticket/internal/domain/tickets"
)

// MockRepository — stateful, потокобезопасный (sync.RWMutex) фейк
// Repository для тестов конкурентности назначения (Task 13). В отличие от
// mockRepository (func-поля, без состояния — годится для одиночных
// сценариев), MockRepository хранит тикеты в памяти и эмулирует то самое
// поведение, ради которого существует Repository.AssignWithVersion в
// Postgres: атомарный compare-and-swap по (id, version, assigned_to) под
// одним мьютексом — ровно как WHERE version = $N AND assigned_to IS NULL
// в SQL гарантирует ровно один успешный UPDATE при гонке.
type MockRepository struct {
	mu      sync.RWMutex
	tickets map[int64]domain.Ticket
	history []domain.History
}

// NewMockRepository создаёт пустой in-memory репозиторий для тестов.
func NewMockRepository() *MockRepository {
	return &MockRepository{tickets: make(map[int64]domain.Ticket)}
}

// Seed кладёт тикет в хранилище напрямую, минуя Create — для подготовки
// стартового состояния в тестах.
func (m *MockRepository) Seed(ticket domain.Ticket) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets[ticket.ID] = ticket
}

// History возвращает копию накопленных записей истории (для ассертов).
func (m *MockRepository) History() []domain.History {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.History, len(m.history))
	copy(out, m.history)
	return out
}

func (m *MockRepository) GetByID(_ context.Context, id int64) (domain.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tickets[id]
	if !ok {
		return domain.Ticket{}, ErrNotFound
	}
	return t, nil
}

// AssignWithVersion — CAS: применяется только если версия совпадает и тикет
// ещё не назначен, иначе ErrOptimisticLockConflict. Ровно как в Postgres,
// весь проверка+запись происходит под одной блокировкой — не двумя
// отдельными операциями, иначе сама эмуляция гонки была бы фиктивной.
func (m *MockRepository) AssignWithVersion(_ context.Context, ticketID, assigneeID int64, expectedVersion int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tickets[ticketID]
	if !ok {
		return ErrNotFound
	}
	if t.Version != expectedVersion || t.AssignedTo != nil {
		return ErrOptimisticLockConflict
	}

	assignee := assigneeID
	t.AssignedTo = &assignee
	t.Status = domain.StatusInProgress
	t.Version++
	m.tickets[ticketID] = t
	return nil
}

// UnassignWithVersion — CAS: применяется только если версия совпадает и
// тикет всё ещё назначен на assigneeID.
func (m *MockRepository) UnassignWithVersion(_ context.Context, ticketID, assigneeID int64, expectedVersion int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tickets[ticketID]
	if !ok {
		return ErrNotFound
	}
	if t.Version != expectedVersion || t.AssignedTo == nil || *t.AssignedTo != assigneeID {
		return ErrOptimisticLockConflict
	}

	t.AssignedTo = nil
	t.Version++
	m.tickets[ticketID] = t
	return nil
}

func (m *MockRepository) AddHistory(_ context.Context, history domain.History) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, history)
	return nil
}

// Остальные методы Repository не нужны для concurrency-тестов назначения —
// заглушки, чтобы удовлетворить интерфейс.
//
//nolint:dupl // Interface and mock have similar structure by design
func (m *MockRepository) Create(_ context.Context, _ domain.Ticket) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *MockRepository) Update(_ context.Context, _ domain.Ticket) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *MockRepository) Delete(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *MockRepository) List(_ context.Context, _ ListFilter) ([]domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) ListWithCursor(_ context.Context, _ ListFilter) ([]domain.Ticket, bool, error) {
	return nil, false, errors.New("not implemented")
}

func (m *MockRepository) GetFullByID(_ context.Context, _ int64) (domain.TicketFull, error) {
	return domain.TicketFull{}, errors.New("not implemented")
}

func (m *MockRepository) ListFull(_ context.Context, _ ListFilter) ([]domain.TicketFull, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) GetHistory(_ context.Context, _ int64, _, _ int) ([]domain.History, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) GetAllStatuses(_ context.Context) ([]StatusInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) GetAllTopics(_ context.Context) ([]domain.Topic, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) GetSLARule(_ context.Context, _, _ int64) (*domain.SLARule, error) {
	return nil, nil
}

func (m *MockRepository) FindSLAViolations(_ context.Context) ([]domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) FindResolvedTicketsOlderThan(_ context.Context, _ int, _ int) ([]domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRepository) UpdateLastUserActivity(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

package tickets

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	domainEvents "pet-ticket/internal/domain/events"
	domain "pet-ticket/internal/domain/tickets"
	infraEvents "pet-ticket/internal/infra/events"

	"github.com/rs/zerolog"
)

// mockEventBus — мок infraEvents.Bus для тестов сервиса. Subscribe — no-op
// (сервис только публикует, подписку тестируют bus_test.go и
// event_handlers_test.go), Publish запоминает все события в порядке публикации.
type mockEventBus struct {
	published []domainEvents.Event
}

func (m *mockEventBus) Subscribe(eventName string, handler infraEvents.Handler) {}

func (m *mockEventBus) Publish(ctx context.Context, event domainEvents.Event) error {
	m.published = append(m.published, event)
	return nil
}

// mockRepository — мок репозитория для тестов
//
//nolint:dupl // Interface and mock have similar structure by design
type mockRepository struct {
	createFunc                       func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error)
	getByIDFunc                      func(ctx context.Context, id int64) (domain.Ticket, error)
	updateFunc                       func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error)
	deleteFunc                       func(ctx context.Context, id int64) error
	listFunc                         func(ctx context.Context, filter ListFilter) ([]domain.Ticket, error)
	listWithCursorFunc               func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error)
	getFullByIDFunc                  func(ctx context.Context, id int64) (domain.TicketFull, error)
	listFullFunc                     func(ctx context.Context, filter ListFilter) ([]domain.TicketFull, error)
	addHistoryFunc                   func(ctx context.Context, history domain.History) error
	getHistoryFunc                   func(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error)
	getStatusesFunc                  func(ctx context.Context) ([]StatusInfo, error)
	getTopicsFunc                    func(ctx context.Context) ([]domain.Topic, error)
	getSLARuleFunc                   func(ctx context.Context, topicID, priorityID int64) (*domain.SLARule, error)
	findSLAViolationsFunc            func(ctx context.Context) ([]domain.Ticket, error)
	findResolvedTicketsOlderThanFunc func(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error)
	updateLastUserActivityFunc       func(ctx context.Context, ticketID int64) error
	assignWithVersionFunc            func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error
	unassignWithVersionFunc          func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error
}

func (m *mockRepository) Create(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, ticket)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (domain.Ticket, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockRepository) Update(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, ticket)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockRepository) List(ctx context.Context, filter ListFilter) ([]domain.Ticket, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) ListWithCursor(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
	if m.listWithCursorFunc != nil {
		return m.listWithCursorFunc(ctx, filter)
	}
	return nil, false, errors.New("not implemented")
}

func (m *mockRepository) GetFullByID(ctx context.Context, id int64) (domain.TicketFull, error) {
	if m.getFullByIDFunc != nil {
		return m.getFullByIDFunc(ctx, id)
	}
	return domain.TicketFull{}, errors.New("not implemented")
}

func (m *mockRepository) ListFull(ctx context.Context, filter ListFilter) ([]domain.TicketFull, error) {
	if m.listFullFunc != nil {
		return m.listFullFunc(ctx, filter)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) AddHistory(ctx context.Context, history domain.History) error {
	if m.addHistoryFunc != nil {
		return m.addHistoryFunc(ctx, history)
	}
	return errors.New("not implemented")
}

func (m *mockRepository) GetHistory(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error) {
	if m.getHistoryFunc != nil {
		return m.getHistoryFunc(ctx, ticketID, limit, offset)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) GetAllStatuses(ctx context.Context) ([]StatusInfo, error) {
	if m.getStatusesFunc != nil {
		return m.getStatusesFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) GetAllTopics(ctx context.Context) ([]domain.Topic, error) {
	if m.getTopicsFunc != nil {
		return m.getTopicsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) GetSLARule(ctx context.Context, topicID, priorityID int64) (*domain.SLARule, error) {
	if m.getSLARuleFunc != nil {
		return m.getSLARuleFunc(ctx, topicID, priorityID)
	}
	return nil, nil
}

func (m *mockRepository) FindSLAViolations(ctx context.Context) ([]domain.Ticket, error) {
	if m.findSLAViolationsFunc != nil {
		return m.findSLAViolationsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) FindResolvedTicketsOlderThan(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error) {
	if m.findResolvedTicketsOlderThanFunc != nil {
		return m.findResolvedTicketsOlderThanFunc(ctx, inactiveDays, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepository) UpdateLastUserActivity(ctx context.Context, ticketID int64) error {
	if m.updateLastUserActivityFunc != nil {
		return m.updateLastUserActivityFunc(ctx, ticketID)
	}
	return errors.New("not implemented")
}

func (m *mockRepository) AssignWithVersion(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
	if m.assignWithVersionFunc != nil {
		return m.assignWithVersionFunc(ctx, ticketID, assigneeID, expectedVersion)
	}
	return errors.New("not implemented")
}

func (m *mockRepository) UnassignWithVersion(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
	if m.unassignWithVersionFunc != nil {
		return m.unassignWithVersionFunc(ctx, ticketID, assigneeID, expectedVersion)
	}
	return errors.New("not implemented")
}

// mockDB — мок для работы с транзакциями
type mockDB struct {
	beginTxFunc func(ctx context.Context) (TxCommitter, error)
}

func (m *mockDB) BeginTx(ctx context.Context) (TxCommitter, error) {
	if m.beginTxFunc != nil {
		return m.beginTxFunc(ctx)
	}
	return &mockTx{}, nil
}

// mockTx — мок транзакции
type mockTx struct {
	commitFunc   func() error
	rollbackFunc func() error
}

func (m *mockTx) Commit() error {
	if m.commitFunc != nil {
		return m.commitFunc()
	}
	return nil
}

func (m *mockTx) Rollback() error {
	if m.rollbackFunc != nil {
		return m.rollbackFunc()
	}
	return nil
}

// TestGetTicket_Success — успешное получение тикета
func TestGetTicket_Success(t *testing.T) {
	// Arrange
	expectedTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "Test ticket",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			if id == 1 {
				return expectedTicket, nil
			}
			return domain.Ticket{}, ErrNotFound
		},
	}

	db := &mockDB{}
	logger := testLogger()
	svc := NewService(repo, nil, db, logger, nil, nil, false)

	// Act
	ticket, err := svc.GetTicket(context.Background(), 1)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ticket.ID != expectedTicket.ID {
		t.Errorf("expected ticket ID %d, got %d", expectedTicket.ID, ticket.ID)
	}
	if ticket.UserID != expectedTicket.UserID {
		t.Errorf("expected user ID %d, got %d", expectedTicket.UserID, ticket.UserID)
	}
}

// TestGetTicket_NotFound — тикет не найден
func TestGetTicket_NotFound(t *testing.T) {
	// Arrange
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return domain.Ticket{}, ErrNotFound
		},
	}

	db := &mockDB{}
	logger := testLogger()
	svc := NewService(repo, nil, db, logger, nil, nil, false)

	// Act
	_, err := svc.GetTicket(context.Background(), 999)

	// Assert
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestCreateTicket_Success — успешное создание тикета. История больше не
// пишется сервисом напрямую (см. TestCreateTicket_PublishesEvent) — её
// пишет HistoryHandler по событию ticket.created.
func TestCreateTicket_Success(t *testing.T) {
	// Arrange
	var capturedTicket domain.Ticket
	committed := false

	repo := &mockRepository{
		createFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			capturedTicket = ticket
			ticket.ID = 42
			return ticket, nil
		},
	}

	db := &mockDB{
		beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
			return &mockTx{
				commitFunc: func() error {
					committed = true
					return nil
				},
			}, nil
		},
	}

	logger := testLogger()
	svc := NewService(repo, nil, db, logger, nil, nil, false)

	input := CreateTicketInput{
		UserID:  100,
		TopicID: 1,
		Comment: "Test ticket",
	}

	// Act
	ticket, err := svc.CreateTicket(context.Background(), input)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ticket.ID != 42 {
		t.Errorf("expected ticket ID 42, got %d", ticket.ID)
	}
	if capturedTicket.UserID != 100 {
		t.Errorf("expected captured user ID 100, got %d", capturedTicket.UserID)
	}
	if capturedTicket.Priority != domain.PriorityMedium {
		t.Errorf("expected default priority medium, got %s", capturedTicket.Priority.String())
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestCreateTicket_PublishesEvent — CreateTicket публикует ticket.created
// ПОСЛЕ коммита транзакции, с корректными полями события.
func TestCreateTicket_PublishesEvent(t *testing.T) {
	repo := &mockRepository{
		createFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			ticket.ID = 42
			return ticket, nil
		},
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	_, err := svc.CreateTicket(context.Background(), CreateTicketInput{
		UserID:  100,
		TopicID: 7,
		Comment: "Test ticket",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(bus.published) != 1 {
		t.Fatalf("expected exactly 1 published event, got %d", len(bus.published))
	}

	event, ok := bus.published[0].(domainEvents.TicketCreated)
	if !ok {
		t.Fatalf("expected TicketCreated event, got %T", bus.published[0])
	}
	if event.TicketID != 42 || event.UserID != 100 || event.TopicID != 7 {
		t.Errorf("unexpected event fields: %+v", event)
	}
	if event.Status != domain.StatusNew.String() {
		t.Errorf("expected status 'new', got %s", event.Status)
	}
	if event.OccurredAt().IsZero() {
		t.Error("expected OccurredAt to be set")
	}
}

// TestCreateTicket_NoEventBus_DoesNotPanic — сервис с eventBus=nil
// (стандартная настройка для юнит-тестов) не публикует события и не падает.
func TestCreateTicket_NoEventBus_DoesNotPanic(t *testing.T) {
	repo := &mockRepository{
		createFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			ticket.ID = 1
			return ticket, nil
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	if _, err := svc.CreateTicket(context.Background(), CreateTicketInput{UserID: 1, TopicID: 1, Comment: "x"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// TestCreateTicket_InvalidInput — валидация входных данных
func TestCreateTicket_InvalidInput(t *testing.T) {
	// Arrange
	repo := &mockRepository{}
	db := &mockDB{}
	logger := testLogger()
	svc := NewService(repo, nil, db, logger, nil, nil, false)

	tests := []struct {
		name  string
		input CreateTicketInput
	}{
		{
			name: "invalid user_id",
			input: CreateTicketInput{
				UserID:  0, // невалидный
				TopicID: 1,
				Comment: "Test",
			},
		},
		{
			name: "invalid topic_id",
			input: CreateTicketInput{
				UserID:  1,
				TopicID: 0, // невалидный
				Comment: "Test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			_, err := svc.CreateTicket(context.Background(), tt.input)

			// Assert
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got: %v", err)
			}
		})
	}
}

// TestUpdateTicket_StatusChange — обновление статуса тикета. Запись истории
// теперь не проверяется здесь напрямую (см. TestUpdateTicket_PublishesStatusChangedEvent) —
// её пишет HistoryHandler по событию ticket.status_changed.
func TestUpdateTicket_StatusChange(t *testing.T) {
	// Arrange
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "Original comment",
	}

	committed := false

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			return ticket, nil
		},
	}

	db := &mockDB{
		beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
			return &mockTx{
				commitFunc: func() error {
					committed = true
					return nil
				},
			}, nil
		},
	}

	logger := testLogger()
	svc := NewService(repo, nil, db, logger, nil, nil, false)

	newStatus := domain.StatusInProgress
	input := UpdateTicketInput{
		ID:     1,
		Status: &newStatus,
	}

	// Act
	ticket, err := svc.UpdateTicket(context.Background(), input)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ticket.Status != domain.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", ticket.Status.String())
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestUpdateTicket_PublishesStatusChangedEvent — статус меняется -> событие
// публикуется ПОСЛЕ коммита, с корректными OldStatus/NewStatus/ChangedBy.
func TestUpdateTicket_PublishesStatusChangedEvent(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityLow,
		Comment:  "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	newStatus := domain.StatusInProgress
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Status: &newStatus}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(bus.published) != 1 {
		t.Fatalf("expected exactly 1 published event, got %d", len(bus.published))
	}
	event, ok := bus.published[0].(domainEvents.TicketStatusChanged)
	if !ok {
		t.Fatalf("expected TicketStatusChanged event, got %T", bus.published[0])
	}
	if event.TicketID != 1 || event.ChangedBy != 100 {
		t.Errorf("unexpected event fields: %+v", event)
	}
	if event.OldStatus != "new" || event.NewStatus != "in_progress" {
		t.Errorf("expected new -> in_progress, got %s -> %s", event.OldStatus, event.NewStatus)
	}
	if event.Resolved {
		t.Error("expected Resolved=false for new -> in_progress")
	}
}

// TestUpdateTicket_PublishesStatusChangedEvent_ResolvedFlag — переход,
// который SLACalculator считает моментом решения (new -> closed), помечает
// событие Resolved=true — так HistoryHandler узнаёт, что нужно дописать
// дополнительную запись с action=resolved.
func TestUpdateTicket_PublishesStatusChangedEvent_ResolvedFlag(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityLow,
		Comment:  "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	newStatus := domain.StatusClosed
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Status: &newStatus}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	event, ok := bus.published[0].(domainEvents.TicketStatusChanged)
	if !ok {
		t.Fatalf("expected TicketStatusChanged event, got %T", bus.published[0])
	}
	if !event.Resolved {
		t.Error("expected Resolved=true for new -> closed")
	}
}

// TestUpdateTicket_PublishesCommentAddedEvent — комментарий меняется ->
// событие публикуется с OldComment/NewComment.
func TestUpdateTicket_PublishesCommentAddedEvent(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityLow,
		Comment:  "old comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	newComment := "new comment"
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Comment: &newComment}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(bus.published) != 1 {
		t.Fatalf("expected exactly 1 published event, got %d", len(bus.published))
	}
	event, ok := bus.published[0].(domainEvents.TicketCommentAdded)
	if !ok {
		t.Fatalf("expected TicketCommentAdded event, got %T", bus.published[0])
	}
	if event.TicketID != 1 || event.UserID != 100 {
		t.Errorf("unexpected event fields: %+v", event)
	}
	if event.OldComment != "old comment" || event.NewComment != "new comment" {
		t.Errorf("expected 'old comment' -> 'new comment', got %s -> %s", event.OldComment, event.NewComment)
	}
}

// TestUpdateTicket_NoEventsWhenNothingChanges — UpdateTicket без Status и
// Comment не публикует ни одного события.
func TestUpdateTicket_NoEventsWhenNothingChanges(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Priority: domain.PriorityLow, Comment: "x"}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(bus.published) != 0 {
		t.Errorf("expected no events published, got %d: %+v", len(bus.published), bus.published)
	}
}

// TestUpdateTicket_TransactionRollback — откат при ошибке сохранения тикета.
// История/события больше не пишутся внутри транзакции (см. комментарий в
// service.go), поэтому здесь ошибку теперь имитирует repo.Update, а не
// AddHistory — это единственная операция с БД, которая всё ещё выполняется
// в транзакции для UpdateTicket.
func TestUpdateTicket_TransactionRollback(t *testing.T) {
	// Arrange
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "Original",
	}

	rolledBack := false

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			return domain.Ticket{}, errors.New("database error")
		},
	}

	db := &mockDB{
		beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
			return &mockTx{
				rollbackFunc: func() error {
					rolledBack = true
					return nil
				},
			}, nil
		},
	}

	logger := testLogger()
	svc := NewService(repo, nil, db, logger, nil, nil, false)

	newStatus := domain.StatusInProgress
	input := UpdateTicketInput{
		ID:     1,
		Status: &newStatus,
	}

	// Act
	_, err := svc.UpdateTicket(context.Background(), input)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !rolledBack {
		t.Error("expected transaction to be rolled back")
	}
}

// TestCreateTicket_WithPriority — создание тикета с указанным приоритетом
func TestCreateTicket_WithPriority(t *testing.T) {
	var capturedTicket domain.Ticket

	repo := &mockRepository{
		createFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			capturedTicket = ticket
			ticket.ID = 1
			return ticket, nil
		},
	}

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)
	priority := domain.PriorityHigh

	_, err := svc.CreateTicket(context.Background(), CreateTicketInput{
		UserID:   1,
		TopicID:  1,
		Priority: &priority,
		Comment:  "High priority ticket",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedTicket.Priority != domain.PriorityHigh {
		t.Errorf("expected priority high, got %s", capturedTicket.Priority.String())
	}
}

// TestUpdatePriority_Success — изменение приоритета с записью в историю
func TestUpdatePriority_Success(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityLow,
		Comment:  "Test ticket",
	}

	var capturedHistory domain.History
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			capturedHistory = history
			return nil
		},
	}

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	updated, err := svc.UpdatePriority(context.Background(), 1, domain.PriorityHigh, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.Priority != domain.PriorityHigh {
		t.Errorf("expected priority high, got %s", updated.Priority.String())
	}
	if capturedHistory.Action != domain.ActionPriorityChanged {
		t.Errorf("expected action priority_changed, got %s", capturedHistory.Action)
	}
	if capturedHistory.OldValue != "low" || capturedHistory.NewValue != "high" {
		t.Errorf("unexpected history values: %s -> %s", capturedHistory.OldValue, capturedHistory.NewValue)
	}
}

// TestEscalateTicket_Success — эскалация повышает приоритет
func TestEscalateTicket_Success(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "Test ticket",
	}

	var capturedHistory domain.History
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			capturedHistory = history
			return nil
		},
	}

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	updated, err := svc.EscalateTicket(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.Priority != domain.PriorityHigh {
		t.Errorf("expected priority high after escalation, got %s", updated.Priority.String())
	}
	if capturedHistory.Action != domain.ActionEscalated {
		t.Errorf("expected action escalated, got %s", capturedHistory.Action)
	}
}

// TestEscalateTicket_MaxPriority — эскалация на максимальном приоритете не меняет тикет
func TestEscalateTicket_MaxPriority(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityCritical,
		Comment:  "Critical ticket",
	}

	updateCalled := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			updateCalled = true
			return ticket, nil
		},
	}

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	updated, err := svc.EscalateTicket(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.Priority != domain.PriorityCritical {
		t.Errorf("expected priority critical, got %s", updated.Priority.String())
	}
	if updateCalled {
		t.Error("expected no update when priority is already critical")
	}
}

// TestAssignTicket_Success — свободный ("new") тикет назначается на
// assigneeID через AssignWithVersion (текущей версии тикета), пишет историю
// в той же "транзакции" и публикует ticket.assigned после коммита.
func TestAssignTicket_Success(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Version: 3}

	var gotTicketID, gotAssigneeID int64
	var gotVersion int
	var historyRecorded domain.History
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		assignWithVersionFunc: func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
			gotTicketID, gotAssigneeID, gotVersion = ticketID, assigneeID, expectedVersion
			return nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			historyRecorded = history
			return nil
		},
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	err := svc.AssignTicket(context.Background(), 1, 55)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotTicketID != 1 || gotAssigneeID != 55 || gotVersion != 3 {
		t.Errorf("unexpected AssignWithVersion args: ticketID=%d assigneeID=%d version=%d", gotTicketID, gotAssigneeID, gotVersion)
	}
	if historyRecorded.Action != domain.ActionAssigned || historyRecorded.TicketID != 1 || historyRecorded.UserID != 55 {
		t.Errorf("unexpected history entry: %+v", historyRecorded)
	}

	if len(bus.published) != 1 {
		t.Fatalf("expected 1 published event, got %d: %+v", len(bus.published), bus.published)
	}
	assigned, ok := bus.published[0].(domainEvents.TicketAssigned)
	if !ok {
		t.Fatalf("expected event to be TicketAssigned, got %T", bus.published[0])
	}
	if assigned.TicketID != 1 || assigned.AssigneeID != 55 {
		t.Errorf("unexpected assigned event fields: %+v", assigned)
	}
}

// TestAssignTicket_Idempotent — повторный вызов с тем же assigneeID на уже
// назначенном на него тикете — тихий успех, без похода в AssignWithVersion.
func TestAssignTicket_Idempotent(t *testing.T) {
	assignee := int64(55)
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusInProgress, AssignedTo: &assignee, Version: 4}

	assignCalled := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		assignWithVersionFunc: func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
			assignCalled = true
			return nil
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	if err := svc.AssignTicket(context.Background(), 1, 55); err != nil {
		t.Fatalf("expected no error for idempotent re-assign, got: %v", err)
	}
	if assignCalled {
		t.Error("expected AssignWithVersion NOT to be called for idempotent re-assign")
	}
}

// TestAssignTicket_AlreadyAssignedToSomeoneElse_ReturnsConflict — тикет уже
// назначен на другого саппорта -> ErrTicketAlreadyAssigned.
func TestAssignTicket_AlreadyAssignedToSomeoneElse_ReturnsConflict(t *testing.T) {
	other := int64(77)
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusInProgress, AssignedTo: &other, Version: 2}

	assignCalled := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		assignWithVersionFunc: func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
			assignCalled = true
			return nil
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	err := svc.AssignTicket(context.Background(), 1, 55)
	if !errors.Is(err, ErrTicketAlreadyAssigned) {
		t.Errorf("expected ErrTicketAlreadyAssigned, got: %v", err)
	}
	if assignCalled {
		t.Error("expected AssignWithVersion NOT to be called when ticket is already assigned to someone else")
	}
}

// TestAssignTicket_InvalidStatus_ReturnsConflict — resolved/closed/cancelled
// тикет нельзя назначить -> ErrInvalidStatusForAssignment.
func TestAssignTicket_InvalidStatus_ReturnsConflict(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusResolved, Version: 1}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	err := svc.AssignTicket(context.Background(), 1, 55)
	if !errors.Is(err, ErrInvalidStatusForAssignment) {
		t.Errorf("expected ErrInvalidStatusForAssignment, got: %v", err)
	}
}

// TestAssignTicket_OptimisticLockConflict_Propagates — конфликт версии из
// репозитория (кто-то опередил между GetByID и AssignWithVersion) доходит до
// вызывающей стороны как ErrTicketAlreadyAssigned (не "сырой"
// ErrOptimisticLockConflict — клиенту не нужно различать, на каком именно
// шаге его опередили, действие в обоих случаях одно: искать другой тикет),
// транзакция при этом откатывается.
func TestAssignTicket_OptimisticLockConflict_Propagates(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Version: 1}

	rollbackCalled := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		assignWithVersionFunc: func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
			return ErrOptimisticLockConflict
		},
	}
	db := &mockDB{
		beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
			return &mockTx{rollbackFunc: func() error { rollbackCalled = true; return nil }}, nil
		},
	}
	svc := NewService(repo, nil, db, testLogger(), nil, nil, false)

	err := svc.AssignTicket(context.Background(), 1, 55)
	if !errors.Is(err, ErrTicketAlreadyAssigned) {
		t.Errorf("expected ErrTicketAlreadyAssigned, got: %v", err)
	}
	if !rollbackCalled {
		t.Error("expected transaction to be rolled back on optimistic lock conflict")
	}
}

// TestUnassignTicket_Success — владелец снимает назначение со своего тикета.
func TestUnassignTicket_Success(t *testing.T) {
	assignee := int64(55)
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusInProgress, AssignedTo: &assignee, Version: 5}

	var historyRecorded domain.History
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		unassignWithVersionFunc: func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
			if ticketID != 1 || assigneeID != 55 || expectedVersion != 5 {
				t.Errorf("unexpected UnassignWithVersion args: ticketID=%d assigneeID=%d version=%d", ticketID, assigneeID, expectedVersion)
			}
			return nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			historyRecorded = history
			return nil
		},
	}
	bus := &mockEventBus{}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), bus, nil, false)

	if err := svc.UnassignTicket(context.Background(), 1, 55); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if historyRecorded.Action != domain.ActionUnassigned {
		t.Errorf("expected ActionUnassigned history entry, got: %+v", historyRecorded)
	}
	if len(bus.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(bus.published))
	}
	if _, ok := bus.published[0].(domainEvents.TicketUnassigned); !ok {
		t.Errorf("expected event to be TicketUnassigned, got %T", bus.published[0])
	}
}

// TestUnassignTicket_NotAssigned_ReturnsConflict — тикет никому не назначен.
func TestUnassignTicket_NotAssigned_ReturnsConflict(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Version: 1}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	err := svc.UnassignTicket(context.Background(), 1, 55)
	if !errors.Is(err, ErrTicketNotAssigned) {
		t.Errorf("expected ErrTicketNotAssigned, got: %v", err)
	}
}

// TestUnassignTicket_NotOwner_ReturnsForbidden — снять может только владелец.
func TestUnassignTicket_NotOwner_ReturnsForbidden(t *testing.T) {
	owner := int64(55)
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusInProgress, AssignedTo: &owner, Version: 2}

	unassignCalled := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		unassignWithVersionFunc: func(ctx context.Context, ticketID, assigneeID int64, expectedVersion int) error {
			unassignCalled = true
			return nil
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	err := svc.UnassignTicket(context.Background(), 1, 999)
	if !errors.Is(err, ErrNotAssignedToYou) {
		t.Errorf("expected ErrNotAssignedToYou, got: %v", err)
	}
	if unassignCalled {
		t.Error("expected UnassignWithVersion NOT to be called for non-owner")
	}
}

// TestAssignTicket_Concurrent_ExactlyOneWinner — 10 горутин одновременно
// назначают на себя один и тот же свободный тикет через реальную
// (stateful, mutex-protected) реализацию AssignWithVersion/GetByID —
// MockRepository, а не func-поля mockRepository, у которых нет своего
// состояния и поэтому нечего было бы гонять. Ожидание: ровно один вызов
// AssignTicket возвращает nil, остальные девять — ErrTicketAlreadyAssigned
// (AssignTicket ремаппит в него и ErrOptimisticLockConflict от репозитория —
// клиенту не нужно различать, на каком шаге его опередили), что маппится в
// 409 на HTTP-слое. Гоняется с -race.
func TestAssignTicket_Concurrent_ExactlyOneWinner(t *testing.T) {
	const workers = 10

	repo := NewMockRepository()
	repo.Seed(domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Version: 1})

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(assigneeID int64) {
			defer wg.Done()
			errs[assigneeID-1] = svc.AssignTicket(context.Background(), 1, assigneeID)
		}(int64(i + 1))
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrTicketAlreadyAssigned):
			// ожидаемый исход для проигравших
		default:
			t.Errorf("unexpected error from concurrent AssignTicket: %v", err)
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful assignment out of %d concurrent attempts, got %d", workers, successes)
	}

	final, err := repo.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("failed to get final ticket state: %v", err)
	}
	if final.AssignedTo == nil {
		t.Fatal("expected ticket to end up assigned")
	}
	if final.Version != 2 {
		t.Errorf("expected version to be bumped exactly once (1 -> 2), got %d", final.Version)
	}

	history := repo.History()
	if len(history) != 1 {
		t.Errorf("expected exactly 1 history entry (from the single winner), got %d: %+v", len(history), history)
	}
}

// testLogger возвращает no-op logger для тестов
func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

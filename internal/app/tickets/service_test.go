package tickets

import (
	"context"
	"errors"
	"io"
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
	svc := NewService(repo, db, logger, nil, nil)

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
	svc := NewService(repo, db, logger, nil, nil)

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
	svc := NewService(repo, db, logger, nil, nil)

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
	svc := NewService(repo, &mockDB{}, testLogger(), bus, nil)

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
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

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
	svc := NewService(repo, db, logger, nil, nil)

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
	svc := NewService(repo, db, logger, nil, nil)

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
		ID:      1,
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, &mockDB{}, testLogger(), bus, nil)

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
		ID:      1,
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, &mockDB{}, testLogger(), bus, nil)

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
		ID:      1,
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "old comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, &mockDB{}, testLogger(), bus, nil)

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
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "x"}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, &mockDB{}, testLogger(), bus, nil)

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
	svc := NewService(repo, db, logger, nil, nil)

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

	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)
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

	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

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

	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

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

	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

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

// TestAssignTicket_Success — назначение нового тикета на оператора переводит
// его в in_progress (через UpdateTicket) и публикует ticket.assigned поверх
// обычных status_changed/comment_added событий.
func TestAssignTicket_Success(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:      1,
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	bus := &mockEventBus{}
	svc := NewService(repo, &mockDB{}, testLogger(), bus, nil)

	updated, err := svc.AssignTicket(context.Background(), AssignTicketInput{
		TicketID:   1,
		OperatorID: 55,
		AssignedBy: 100,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.Status != domain.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", updated.Status.String())
	}

	// UpdateTicket публикует status_changed (new -> in_progress) и
	// comment_added (дефолтный комментарий "Assigned to operator"),
	// AssignTicket сверху публикует assigned.
	if len(bus.published) != 3 {
		t.Fatalf("expected 3 published events (status_changed, comment_added, assigned), got %d: %+v", len(bus.published), bus.published)
	}

	if _, ok := bus.published[0].(domainEvents.TicketStatusChanged); !ok {
		t.Errorf("expected event[0] to be TicketStatusChanged, got %T", bus.published[0])
	}
	if _, ok := bus.published[1].(domainEvents.TicketCommentAdded); !ok {
		t.Errorf("expected event[1] to be TicketCommentAdded, got %T", bus.published[1])
	}

	assigned, ok := bus.published[2].(domainEvents.TicketAssigned)
	if !ok {
		t.Fatalf("expected event[2] to be TicketAssigned, got %T", bus.published[2])
	}
	if assigned.TicketID != 1 || assigned.OperatorID != 55 || assigned.AssignedBy != 100 {
		t.Errorf("unexpected assigned event fields: %+v", assigned)
	}
}

// TestAssignTicket_CustomComment — переданный комментарий используется
// вместо дефолтного "Assigned to operator".
func TestAssignTicket_CustomComment(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "x"}

	var capturedComment string
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			capturedComment = ticket.Comment
			return ticket, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.AssignTicket(context.Background(), AssignTicketInput{
		TicketID:   1,
		OperatorID: 55,
		AssignedBy: 100,
		Comment:    "Please handle ASAP",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedComment != "Please handle ASAP" {
		t.Errorf("expected custom comment to be used, got %q", capturedComment)
	}
}

// TestAssignTicket_AlreadyAssigned_ReturnsConflict — тикет не в статусе
// "new" -> ErrConflict, repo.Update не вызывается.
func TestAssignTicket_AlreadyAssigned_ReturnsConflict(t *testing.T) {
	existingTicket := domain.Ticket{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusInProgress, Comment: "x"}

	updateCalled := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			updateCalled = true
			return ticket, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.AssignTicket(context.Background(), AssignTicketInput{TicketID: 1, OperatorID: 55, AssignedBy: 100})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got: %v", err)
	}
	if updateCalled {
		t.Error("expected repo.Update NOT to be called when ticket is already assigned")
	}
}

// testLogger возвращает no-op logger для тестов
func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

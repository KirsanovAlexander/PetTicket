package tickets

import (
	"context"
	"errors"
	"io"
	"testing"

	domain "pet-ticket/internal/domain/tickets"

	"github.com/rs/zerolog"
)

// mockRepository — мок репозитория для тестов
//
//nolint:dupl // Interface and mock have similar structure by design
type mockRepository struct {
	createFunc      func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error)
	getByIDFunc     func(ctx context.Context, id int64) (domain.Ticket, error)
	updateFunc      func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error)
	deleteFunc      func(ctx context.Context, id int64) error
	listFunc        func(ctx context.Context, filter ListFilter) ([]domain.Ticket, error)
	addHistoryFunc  func(ctx context.Context, history domain.History) error
	getHistoryFunc  func(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error)
	getStatusesFunc func(ctx context.Context) ([]StatusInfo, error)
	getTopicsFunc         func(ctx context.Context) ([]domain.Topic, error)
	getSLARuleFunc                  func(ctx context.Context, topicID, priorityID int64) (*domain.SLARule, error)
	findSLAViolationsFunc           func(ctx context.Context) ([]domain.Ticket, error)
	findResolvedTicketsOlderThanFunc func(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error)
	updateLastUserActivityFunc      func(ctx context.Context, ticketID int64) error
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
	svc := NewService(repo, db, logger)

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
	svc := NewService(repo, db, logger)

	// Act
	_, err := svc.GetTicket(context.Background(), 999)

	// Assert
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestCreateTicket_Success — успешное создание тикета
func TestCreateTicket_Success(t *testing.T) {
	// Arrange
	var capturedTicket domain.Ticket
	var capturedHistory domain.History
	committed := false

	repo := &mockRepository{
		createFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			capturedTicket = ticket
			ticket.ID = 42
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			capturedHistory = history
			return nil
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
	svc := NewService(repo, db, logger)

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
	if capturedHistory.Action != domain.ActionCreated {
		t.Errorf("expected history action 'created', got %s", capturedHistory.Action)
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestCreateTicket_InvalidInput — валидация входных данных
func TestCreateTicket_InvalidInput(t *testing.T) {
	// Arrange
	repo := &mockRepository{}
	db := &mockDB{}
	logger := testLogger()
	svc := NewService(repo, db, logger)

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

// TestUpdateTicket_StatusChange — обновление статуса с историей
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

	var capturedHistory domain.History
	committed := false

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
	svc := NewService(repo, db, logger)

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
	if capturedHistory.Action != domain.ActionStatusChanged {
		t.Errorf("expected history action 'status_changed', got %s", capturedHistory.Action)
	}
	if capturedHistory.OldValue != "new" {
		t.Errorf("expected old value 'new', got %s", capturedHistory.OldValue)
	}
	if capturedHistory.NewValue != "in_progress" {
		t.Errorf("expected new value 'in_progress', got %s", capturedHistory.NewValue)
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestUpdateTicket_CommentNotChanged — история НЕ добавляется при отправке того же комментария
func TestUpdateTicket_CommentNotChanged(t *testing.T) {
	// Arrange
	existingTicket := domain.Ticket{
		ID:       123,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "Исходный комментарий",
	}

	committed := false
	historyAdded := false

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			historyAdded = true
			return nil
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
	svc := NewService(repo, db, logger)

	sameComment := "Исходный комментарий"
	input := UpdateTicketInput{
		ID:      123,
		Comment: &sameComment,
	}

	// Act
	ticket, err := svc.UpdateTicket(context.Background(), input)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ticket.Comment != "Исходный комментарий" {
		t.Errorf("expected comment 'Исходный комментарий', got %s", ticket.Comment)
	}
	if historyAdded {
		t.Error("expected history NOT to be added when comment unchanged")
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestUpdateTicket_CommentChanged — история добавляется с корректными Action/OldValue/NewValue
func TestUpdateTicket_CommentChanged(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       123,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "Проблема с оплатой",
	}

	var capturedHistory domain.History
	committed := false

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
	svc := NewService(repo, db, logger)

	newComment := "Проблема решена"
	input := UpdateTicketInput{
		ID:      123,
		Comment: &newComment,
	}

	ticket, err := svc.UpdateTicket(context.Background(), input)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ticket.Comment != "Проблема решена" {
		t.Errorf("expected comment 'Проблема решена', got %s", ticket.Comment)
	}
	if capturedHistory.Action != domain.ActionCommentUpdated {
		t.Errorf("expected history action 'comment_updated', got %s", capturedHistory.Action)
	}
	if capturedHistory.OldValue != "Проблема с оплатой" {
		t.Errorf("expected old value 'Проблема с оплатой', got %s", capturedHistory.OldValue)
	}
	if capturedHistory.NewValue != "Проблема решена" {
		t.Errorf("expected new value 'Проблема решена', got %s", capturedHistory.NewValue)
	}
	if capturedHistory.TicketID != 123 {
		t.Errorf("expected ticket ID 123, got %d", capturedHistory.TicketID)
	}
	if capturedHistory.UserID != 100 {
		t.Errorf("expected user ID 100, got %d", capturedHistory.UserID)
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestUpdateTicket_CommentEmptyToValue — пустой комментарий, ставший непустым, тоже считается изменением
func TestUpdateTicket_CommentEmptyToValue(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       123,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusNew,
		Priority: domain.PriorityMedium,
		Comment:  "",
	}

	var capturedHistory domain.History
	committed := false

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
	svc := NewService(repo, db, logger)

	newComment := "Новый комментарий"
	input := UpdateTicketInput{
		ID:      123,
		Comment: &newComment,
	}

	ticket, err := svc.UpdateTicket(context.Background(), input)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ticket.Comment != "Новый комментарий" {
		t.Errorf("expected comment 'Новый комментарий', got %s", ticket.Comment)
	}
	if capturedHistory.Action != domain.ActionCommentUpdated {
		t.Errorf("expected history action 'comment_updated', got %s", capturedHistory.Action)
	}
	if capturedHistory.OldValue != "" {
		t.Errorf("expected old value '', got %s", capturedHistory.OldValue)
	}
	if capturedHistory.NewValue != "Новый комментарий" {
		t.Errorf("expected new value 'Новый комментарий', got %s", capturedHistory.NewValue)
	}
	if !committed {
		t.Error("expected transaction to be committed")
	}
}

// TestUpdateTicket_TransactionRollback — откат при ошибке истории
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
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			return errors.New("database error")
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
	svc := NewService(repo, db, logger)

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
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			return nil
		},
	}

	svc := NewService(repo, &mockDB{}, testLogger())
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

	svc := NewService(repo, &mockDB{}, testLogger())

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

	svc := NewService(repo, &mockDB{}, testLogger())

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

	svc := NewService(repo, &mockDB{}, testLogger())

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

// testLogger возвращает no-op logger для тестов
func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

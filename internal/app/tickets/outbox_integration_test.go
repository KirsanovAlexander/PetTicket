package tickets

import (
	"context"
	"errors"
	"testing"

	domain "pet-ticket/internal/domain/tickets"

	notifications "pet-ticket/internal/domain/notifications"
)

// mockOutboxRepository — мок notifications.OutboxRepository для тестов
// сервиса. Create запоминает все переданные записи и ctx, в котором они
// были созданы — тесты проверяют по нему, что запись создана ИМЕННО в
// транзакции обновления тикета (см. TestUpdateTicket_OutboxEntryUsesTxContext).
type mockOutboxRepository struct {
	createFunc func(ctx context.Context, entry notifications.OutboxEntry) error
	created    []notifications.OutboxEntry
	createdCtx []context.Context
}

func (m *mockOutboxRepository) Create(ctx context.Context, entry notifications.OutboxEntry) error {
	m.created = append(m.created, entry)
	m.createdCtx = append(m.createdCtx, ctx)
	if m.createFunc != nil {
		return m.createFunc(ctx, entry)
	}
	return nil
}

func (m *mockOutboxRepository) FindPending(ctx context.Context, limit int) ([]notifications.OutboxEntry, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOutboxRepository) Update(ctx context.Context, entry notifications.OutboxEntry) error {
	return errors.New("not implemented")
}

func TestUpdateTicket_CreatesOutboxEntryOnStatusChange(t *testing.T) {
	existingTicket := domain.Ticket{
		ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	outbox := &mockOutboxRepository{}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, outbox)

	newStatus := domain.StatusInProgress
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Status: &newStatus}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(outbox.created) != 1 {
		t.Fatalf("expected exactly 1 outbox entry created, got %d", len(outbox.created))
	}
	entry := outbox.created[0]
	if entry.TicketID != 1 {
		t.Errorf("expected TicketID 1, got %d", entry.TicketID)
	}
	if entry.UserID != 100 {
		t.Errorf("expected UserID 100, got %d", entry.UserID)
	}
	if entry.Type != notifications.NotifStatusChanged {
		t.Errorf("expected type NotifStatusChanged, got %s", entry.Type)
	}
	if entry.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts 5, got %d", entry.MaxAttempts)
	}
	if entry.Payload["message"] == nil {
		t.Error("expected payload to contain a message")
	}
}

func TestUpdateTicket_NoOutboxEntryWhenStatusUnchanged(t *testing.T) {
	existingTicket := domain.Ticket{
		ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	outbox := &mockOutboxRepository{}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, outbox)

	newComment := "Updated comment only, no status change"
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Comment: &newComment}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(outbox.created) != 0 {
		t.Errorf("expected no outbox entries when status unchanged, got %d", len(outbox.created))
	}
}

func TestUpdateTicket_NilOutboxRepo_DoesNotPanic(t *testing.T) {
	existingTicket := domain.Ticket{
		ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	newStatus := domain.StatusInProgress
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Status: &newStatus}); err != nil {
		t.Fatalf("expected no error with nil outboxRepo, got: %v", err)
	}
}

func TestUpdateTicket_OutboxCreateFailure_RollsBackTransaction(t *testing.T) {
	existingTicket := domain.Ticket{
		ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Original comment",
	}

	rolledBack := false
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
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
	outbox := &mockOutboxRepository{
		createFunc: func(ctx context.Context, entry notifications.OutboxEntry) error {
			return errors.New("db unavailable")
		},
	}
	svc := NewService(repo, db, testLogger(), nil, outbox)

	newStatus := domain.StatusInProgress
	_, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Status: &newStatus})
	if err == nil {
		t.Fatal("expected error when outbox Create fails, got nil")
	}
	if !rolledBack {
		t.Error("expected transaction to be rolled back when outbox Create fails")
	}
}

// TestUpdateTicket_OutboxEntryUsesTxContext проверяет, что Create вызывается
// с ctx, несущим TxContextKey — то есть outbox-запись реально пишется в той
// же транзакции, что и обновление тикета (а не мимо неё). Прямая проверка
// того, что фикс общего ключа транзакции (см. TxContextKey в этом файле)
// действительно используется в этом флоу.
func TestUpdateTicket_OutboxEntryUsesTxContext(t *testing.T) {
	existingTicket := domain.Ticket{
		ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Original comment",
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) { return existingTicket, nil },
		updateFunc:  func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) { return ticket, nil },
	}
	outbox := &mockOutboxRepository{}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, outbox)

	newStatus := domain.StatusInProgress
	if _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{ID: 1, Status: &newStatus}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(outbox.createdCtx) != 1 {
		t.Fatalf("expected 1 recorded ctx, got %d", len(outbox.createdCtx))
	}
	if outbox.createdCtx[0].Value(TxContextKey) == nil {
		t.Error("expected outbox.Create to be called with a ctx carrying TxContextKey")
	}
}

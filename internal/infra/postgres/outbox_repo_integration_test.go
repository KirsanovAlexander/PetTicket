//go:build integration

package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	apptickets "pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/notifications"
	domainTickets "pet-ticket/internal/domain/tickets"
)

// createTestTicket создаёт тикет-заглушку для FK-ограничения ticket_id.
func createTestTicket(t *testing.T, ticketsRepo *TicketsRepository, userID int64) int64 {
	t.Helper()
	ticket, err := ticketsRepo.Create(context.Background(), domainTickets.Ticket{
		UserID:  userID,
		TopicID: 1,
		Status:  domainTickets.StatusNew,
		Comment: "Ticket for outbox integration test",
	})
	if err != nil {
		t.Fatalf("failed to create test ticket: %v", err)
	}
	return ticket.ID
}

func TestOutboxRepository_Create_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	outboxRepo := NewOutboxRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 100)

	err := outboxRepo.Create(ctx, domain.OutboxEntry{
		UserID:      100,
		TicketID:    ticketID,
		Type:        domain.NotifStatusChanged,
		MaxAttempts: 5,
		NextRetryAt: time.Now(),
		Payload:     map[string]interface{}{"title": "test"},
	})
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}

	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to find pending entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 pending entry, got %d", len(entries))
	}
	if entries[0].TicketID != ticketID {
		t.Errorf("expected ticket_id %d, got %d", ticketID, entries[0].TicketID)
	}
	if entries[0].Payload["title"] != "test" {
		t.Errorf("expected payload title 'test', got %v", entries[0].Payload["title"])
	}
}

func TestOutboxRepository_FindPending_RespectsNextRetryAt_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	outboxRepo := NewOutboxRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 200)

	err := outboxRepo.Create(ctx, domain.OutboxEntry{
		UserID:      200,
		TicketID:    ticketID,
		Type:        domain.NotifStatusChanged,
		MaxAttempts: 5,
		NextRetryAt: time.Now().Add(1 * time.Hour), // ещё не пора
		Payload:     map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}

	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to find pending entries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (next_retry_at in future), got %d", len(entries))
	}
}

func TestOutboxRepository_FindPending_ClaimsAsProcessing_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	outboxRepo := NewOutboxRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 300)
	if err := outboxRepo.Create(ctx, domain.OutboxEntry{
		UserID: 300, TicketID: ticketID, Type: domain.NotifStatusChanged,
		MaxAttempts: 5, NextRetryAt: time.Now(), Payload: map[string]interface{}{},
	}); err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}

	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to find pending entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Status != domain.OutboxStatusProcessing {
		t.Errorf("expected status processing after claim, got %s", entries[0].Status)
	}

	// Повторный вызов не должен вернуть уже захваченную запись.
	again, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to find pending entries: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected 0 entries on second FindPending (already claimed), got %d", len(again))
	}
}

// TestOutboxRepository_FindPending_ConcurrentWorkers_Integration — ключевой
// тест на FOR UPDATE SKIP LOCKED: два "воркера" одновременно забирают
// batch из одного и того же пула pending-записей — ни одна запись не
// должна достаться обоим.
func TestOutboxRepository_FindPending_ConcurrentWorkers_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	outboxRepo := NewOutboxRepository(testDB.db)
	ctx := context.Background()

	const total = 20
	for i := 0; i < total; i++ {
		ticketID := createTestTicket(t, ticketsRepo, 400)
		if err := outboxRepo.Create(ctx, domain.OutboxEntry{
			UserID: 400, TicketID: ticketID, Type: domain.NotifStatusChanged,
			MaxAttempts: 5, NextRetryAt: time.Now(), Payload: map[string]interface{}{},
		}); err != nil {
			t.Fatalf("failed to create outbox entry %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[int64]int)

	worker := func() {
		defer wg.Done()
		entries, err := outboxRepo.FindPending(ctx, total)
		if err != nil {
			t.Errorf("worker FindPending failed: %v", err)
			return
		}
		mu.Lock()
		for _, e := range entries {
			seen[e.ID]++
		}
		mu.Unlock()
	}

	wg.Add(2)
	go worker()
	go worker()
	wg.Wait()

	claimedTotal := 0
	for id, count := range seen {
		claimedTotal += count
		if count > 1 {
			t.Errorf("entry %d claimed by %d workers, expected at most 1 (SKIP LOCKED violated)", id, count)
		}
	}
	if claimedTotal != total {
		t.Errorf("expected all %d entries claimed exactly once across both workers, got %d claims", total, claimedTotal)
	}
}

func TestOutboxRepository_Update_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	outboxRepo := NewOutboxRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 500)
	if err := outboxRepo.Create(ctx, domain.OutboxEntry{
		UserID: 500, TicketID: ticketID, Type: domain.NotifStatusChanged,
		MaxAttempts: 5, NextRetryAt: time.Now(), Payload: map[string]interface{}{},
	}); err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}

	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed to claim entry: err=%v len=%d", err, len(entries))
	}

	entry := entries[0]
	entry.Attempts = 1
	entry.Status = domain.OutboxStatusFailed
	entry.ErrorMessage = "connection refused"

	if err := outboxRepo.Update(ctx, entry); err != nil {
		t.Fatalf("failed to update outbox entry: %v", err)
	}

	// failed-запись больше не должна попадать в FindPending
	again, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to find pending entries: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("expected failed entry to be excluded from FindPending, got %d entries", len(again))
	}
}

// TestOutboxRepository_Create_WithinTransaction_Integration проверяет, что
// outbox-запись реально попадает в ту же транзакцию, что и обновление
// тикета: при откате транзакции запись не должна сохраниться. Это прямая
// проверка фикса apptickets.TxContextKey (см. app/tickets/service.go) —
// до фикса эта запись создавалась бы через пул соединений в обход tx и
// сохранялась бы, даже если тикет откатился.
func TestOutboxRepository_Create_WithinTransaction_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	outboxRepo := NewOutboxRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 600)

	tx, err := testDB.db.BeginTx(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	txCtx := context.WithValue(ctx, apptickets.TxContextKey, tx)

	if err := outboxRepo.Create(txCtx, domain.OutboxEntry{
		UserID: 600, TicketID: ticketID, Type: domain.NotifStatusChanged,
		MaxAttempts: 5, NextRetryAt: time.Now(), Payload: map[string]interface{}{},
	}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("failed to create outbox entry in transaction: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("failed to rollback transaction: %v", err)
	}

	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to find pending entries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected outbox entry to be rolled back with the transaction, but found %d pending entries", len(entries))
	}
}

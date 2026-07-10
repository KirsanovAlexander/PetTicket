//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	apptickets "pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
)

func TestCommentsRepository_Create_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 100)

	comment, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 100, Content: "First comment", IsInternal: false,
	})
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}
	if comment.ID == 0 {
		t.Error("expected non-zero comment ID")
	}
	if comment.Content != "First comment" {
		t.Errorf("expected content 'First comment', got %q", comment.Content)
	}
	if comment.IsInternal {
		t.Error("expected IsInternal=false")
	}
}

func TestCommentsRepository_GetByTicketID_FiltersInternal_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 200)

	if _, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 200, Content: "Public comment", IsInternal: false,
	}); err != nil {
		t.Fatalf("failed to create public comment: %v", err)
	}
	if _, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 300, Content: "Internal note", IsInternal: true,
	}); err != nil {
		t.Fatalf("failed to create internal comment: %v", err)
	}

	publicOnly, err := commentsRepo.GetByTicketID(ctx, domain.ListCommentsFilter{
		TicketID: ticketID, IncludeInternal: false,
	})
	if err != nil {
		t.Fatalf("failed to get comments: %v", err)
	}
	if len(publicOnly) != 1 {
		t.Fatalf("expected 1 public comment, got %d", len(publicOnly))
	}
	if publicOnly[0].Content != "Public comment" {
		t.Errorf("expected 'Public comment', got %q", publicOnly[0].Content)
	}

	all, err := commentsRepo.GetByTicketID(ctx, domain.ListCommentsFilter{
		TicketID: ticketID, IncludeInternal: true,
	})
	if err != nil {
		t.Fatalf("failed to get comments: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 comments (public+internal), got %d", len(all))
	}
}

func TestCommentsRepository_GetLastByTicketID_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 400)

	if _, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 400, Content: "Older", IsInternal: false,
	}); err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 400, Content: "Newer", IsInternal: false,
	}); err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	last, err := commentsRepo.GetLastByTicketID(ctx, ticketID)
	if err != nil {
		t.Fatalf("failed to get last comment: %v", err)
	}
	if last == nil {
		t.Fatal("expected non-nil last comment")
	}
	if last.Content != "Newer" {
		t.Errorf("expected last comment 'Newer', got %q", last.Content)
	}
}

func TestCommentsRepository_GetLastByTicketID_NoComments_ReturnsNil_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 500)

	last, err := commentsRepo.GetLastByTicketID(ctx, ticketID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if last != nil {
		t.Errorf("expected nil for ticket with no comments, got %+v", last)
	}
}

func TestCommentsRepository_Update_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 600)
	comment, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 600, Content: "Original", IsInternal: false,
	})
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	if err := commentsRepo.Update(ctx, domain.UpdateCommentInput{ID: comment.ID, Content: "Edited"}); err != nil {
		t.Fatalf("failed to update comment: %v", err)
	}

	last, err := commentsRepo.GetLastByTicketID(ctx, ticketID)
	if err != nil || last == nil {
		t.Fatalf("failed to fetch updated comment: err=%v last=%v", err, last)
	}
	if last.Content != "Edited" {
		t.Errorf("expected content 'Edited', got %q", last.Content)
	}
}

func TestCommentsRepository_Update_NotFound_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	err := commentsRepo.Update(ctx, domain.UpdateCommentInput{ID: 999999, Content: "Edited"})
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if err != apptickets.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestCommentsRepository_Delete_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 700)
	comment, err := commentsRepo.Create(ctx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 700, Content: "To delete", IsInternal: false,
	})
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	if err := commentsRepo.Delete(ctx, comment.ID); err != nil {
		t.Fatalf("failed to delete comment: %v", err)
	}

	last, err := commentsRepo.GetLastByTicketID(ctx, ticketID)
	if err != nil {
		t.Fatalf("failed to get last comment: %v", err)
	}
	if last != nil {
		t.Errorf("expected no comments after delete, got %+v", last)
	}
}

func TestCommentsRepository_Delete_NotFound_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	err := commentsRepo.Delete(ctx, 999999)
	if err != apptickets.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestCommentsRepository_Create_WithinTransaction_Integration проверяет,
// что Create участвует в транзакции, переданной через ctx (тот же
// TxContextKey, что и у OutboxRepository/TicketsRepository) — критично для
// dual write в Service.AddComment.
func TestCommentsRepository_Create_WithinTransaction_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	ticketsRepo := NewTicketsRepository(testDB.db)
	commentsRepo := NewCommentsRepository(testDB.db)
	ctx := context.Background()

	ticketID := createTestTicket(t, ticketsRepo, 800)

	tx, err := testDB.db.BeginTx(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	txCtx := context.WithValue(ctx, apptickets.TxContextKey, tx)

	if _, err := commentsRepo.Create(txCtx, domain.AddCommentInput{
		TicketID: ticketID, UserID: 800, Content: "Rolled back", IsInternal: false,
	}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("failed to create comment in transaction: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("failed to rollback transaction: %v", err)
	}

	last, err := commentsRepo.GetLastByTicketID(ctx, ticketID)
	if err != nil {
		t.Fatalf("failed to get last comment: %v", err)
	}
	if last != nil {
		t.Errorf("expected comment to be rolled back with the transaction, but found: %+v", last)
	}
}

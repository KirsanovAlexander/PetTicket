package tickets

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	domain "pet-ticket/internal/domain/tickets"

	"github.com/rs/zerolog"
)

type mockService struct {
	closeTicketFunc func(ctx context.Context, input CloseTicketInput) (domain.Ticket, error)
}

func (m *mockService) CreateTicket(ctx context.Context, input CreateTicketInput) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) GetTicket(ctx context.Context, id int64) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) UpdateTicket(ctx context.Context, input UpdateTicketInput) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) DeleteTicket(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (m *mockService) ListTickets(ctx context.Context, input ListTicketsInput) ([]domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) ListTicketsWithCursor(ctx context.Context, input ListTicketsWithCursorInput) (CursorPage, error) {
	return CursorPage{}, errors.New("not implemented")
}

func (m *mockService) GetTicketFull(ctx context.Context, id int64) (domain.TicketFull, error) {
	return domain.TicketFull{}, errors.New("not implemented")
}

func (m *mockService) ListTicketsFull(ctx context.Context, input ListTicketsInput) ([]domain.TicketFull, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) GetAllStatuses(ctx context.Context) ([]StatusInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) GetAllTopics(ctx context.Context) ([]domain.Topic, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) UpdatePriority(ctx context.Context, ticketID int64, priority domain.Priority, userID int64) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) EscalateTicket(ctx context.Context, ticketID int64, userID int64) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) AddComment(ctx context.Context, input domain.AddCommentInput) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) GetComments(ctx context.Context, filter domain.ListCommentsFilter) ([]domain.TicketComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) GetLastComment(ctx context.Context, ticketID int64) (*domain.TicketComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) UpdateComment(ctx context.Context, input domain.UpdateCommentInput) error {
	return errors.New("not implemented")
}

func (m *mockService) DeleteComment(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (m *mockService) GetSLAViolations(ctx context.Context) ([]domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (m *mockService) CloseTicket(ctx context.Context, input CloseTicketInput) (domain.Ticket, error) {
	if m.closeTicketFunc != nil {
		return m.closeTicketFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockService) AssignTicket(ctx context.Context, input AssignTicketInput) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func TestAutoCloser_CloseInactiveTickets_Success(t *testing.T) {
	oldActivity := time.Now().AddDate(0, 0, -10)
	ticketsToClose := []domain.Ticket{
		{
			ID:                 1,
			Status:             domain.StatusResolved,
			LastUserActivityAt: oldActivity,
		},
		{
			ID:                 2,
			Status:             domain.StatusResolved,
			LastUserActivityAt: oldActivity,
		},
	}

	closedIDs := make([]int64, 0)
	svc := &mockService{
		closeTicketFunc: func(ctx context.Context, input CloseTicketInput) (domain.Ticket, error) {
			closedIDs = append(closedIDs, input.TicketID)
			return domain.Ticket{ID: input.TicketID, Status: domain.StatusClosed}, nil
		},
	}

	repo := &mockRepository{
		findResolvedTicketsOlderThanFunc: func(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error) {
			return ticketsToClose, nil
		},
	}

	ac := NewAutoCloser(svc, repo, testLogger(), 7, 100)
	if err := ac.CloseInactiveTickets(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(closedIDs) != 2 {
		t.Errorf("expected 2 closed tickets, got %d", len(closedIDs))
	}
}

func TestAutoCloser_CloseInactiveTickets_NoTickets(t *testing.T) {
	closeCalled := false
	svc := &mockService{
		closeTicketFunc: func(ctx context.Context, input CloseTicketInput) (domain.Ticket, error) {
			closeCalled = true
			return domain.Ticket{}, nil
		},
	}

	repo := &mockRepository{
		findResolvedTicketsOlderThanFunc: func(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error) {
			return nil, nil
		},
	}

	ac := NewAutoCloser(svc, repo, testLogger(), 7, 100)
	if err := ac.CloseInactiveTickets(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if closeCalled {
		t.Error("expected CloseTicket not to be called")
	}
}

func TestAutoCloser_CloseInactiveTickets_SkipsActive(t *testing.T) {
	recentActivity := time.Now().AddDate(0, 0, -1)
	ticketsToClose := []domain.Ticket{
		{
			ID:                 1,
			Status:             domain.StatusResolved,
			LastUserActivityAt: recentActivity,
		},
	}

	closeCalled := false
	svc := &mockService{
		closeTicketFunc: func(ctx context.Context, input CloseTicketInput) (domain.Ticket, error) {
			closeCalled = true
			return domain.Ticket{}, nil
		},
	}

	repo := &mockRepository{
		findResolvedTicketsOlderThanFunc: func(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error) {
			return ticketsToClose, nil
		},
	}

	ac := NewAutoCloser(svc, repo, testLogger(), 7, 100)
	if err := ac.CloseInactiveTickets(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if closeCalled {
		t.Error("expected CloseTicket not to be called for active ticket")
	}
}

func TestAutoCloser_CloseInactiveTicketsWithRetry(t *testing.T) {
	attempts := 0
	repo := &mockRepository{
		findResolvedTicketsOlderThanFunc: func(ctx context.Context, inactiveDays int, limit int) ([]domain.Ticket, error) {
			attempts++
			if attempts < 2 {
				return nil, errors.New("db error")
			}
			return nil, nil
		},
	}

	ac := NewAutoCloser(&mockService{}, repo, zerolog.New(io.Discard), 7, 100)
	if err := ac.CloseInactiveTicketsWithRetry(context.Background(), 3); err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestCloseTicket_Success(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:       1,
		UserID:   100,
		TopicID:  1,
		Status:   domain.StatusResolved,
		Priority: domain.PriorityMedium,
		Comment:  "Resolved ticket",
	}

	var capturedHistory domain.History
	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
		updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
			if ticket.Status != domain.StatusClosed {
				t.Errorf("expected status closed, got %s", ticket.Status)
			}
			return ticket, nil
		},
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			capturedHistory = history
			return nil
		},
	}

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	updated, err := svc.CloseTicket(context.Background(), CloseTicketInput{
		TicketID: 1,
		UserID:   0,
		Reason:   "Auto-closed after 7 days of inactivity",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updated.Status != domain.StatusClosed {
		t.Errorf("expected status closed, got %s", updated.Status)
	}
	if capturedHistory.Action != domain.ActionAutoClosed {
		t.Errorf("expected action auto_closed, got %s", capturedHistory.Action)
	}
	if capturedHistory.OldValue != "resolved" || capturedHistory.NewValue != "closed" {
		t.Errorf("unexpected history values: %s -> %s", capturedHistory.OldValue, capturedHistory.NewValue)
	}
}

func TestCloseTicket_NotResolved(t *testing.T) {
	existingTicket := domain.Ticket{
		ID:     1,
		Status: domain.StatusInProgress,
	}

	repo := &mockRepository{
		getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return existingTicket, nil
		},
	}

	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	_, err := svc.CloseTicket(context.Background(), CloseTicketInput{TicketID: 1, UserID: 0})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

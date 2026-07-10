package tickets

import (
	"context"
	"errors"
	"testing"

	domain "pet-ticket/internal/domain/tickets"
)

func TestGetTicketFull_Success(t *testing.T) {
	expected := domain.TicketFull{
		ID:   1,
		User: domain.User{ID: 100},
		Status: domain.TicketStatusInfo{
			ID: 1, Name: "new", DisplayName: "Новый тикет",
		},
		Topic: domain.Topic{ID: 1, ExternalID: 1, Title: "Депозит"},
	}

	repo := &mockRepository{
		getFullByIDFunc: func(ctx context.Context, id int64) (domain.TicketFull, error) {
			if id == 1 {
				return expected, nil
			}
			return domain.TicketFull{}, ErrNotFound
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	full, err := svc.GetTicketFull(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if full.ID != 1 {
		t.Errorf("expected ticket id 1, got %d", full.ID)
	}
	if full.Status.DisplayName != "Новый тикет" {
		t.Errorf("expected status display name 'Новый тикет', got %q", full.Status.DisplayName)
	}
}

func TestGetTicketFull_NotFound(t *testing.T) {
	repo := &mockRepository{
		getFullByIDFunc: func(ctx context.Context, id int64) (domain.TicketFull, error) {
			return domain.TicketFull{}, ErrNotFound
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	_, err := svc.GetTicketFull(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestListTicketsFull_Success(t *testing.T) {
	expected := []domain.TicketFull{
		{ID: 1, User: domain.User{ID: 100}},
		{ID: 2, User: domain.User{ID: 100}},
	}

	userID := int64(100)
	var capturedFilter ListFilter
	repo := &mockRepository{
		listFullFunc: func(ctx context.Context, filter ListFilter) ([]domain.TicketFull, error) {
			capturedFilter = filter
			return expected, nil
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	list, err := svc.ListTicketsFull(context.Background(), ListTicketsInput{
		UserID: &userID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tickets, got %d", len(list))
	}
	if capturedFilter.UserID == nil || *capturedFilter.UserID != userID {
		t.Errorf("expected userID filter to be passed through, got %v", capturedFilter.UserID)
	}
	if capturedFilter.Limit != 10 {
		t.Errorf("expected limit 10, got %d", capturedFilter.Limit)
	}
}

func TestListTicketsFull_RepoError(t *testing.T) {
	repo := &mockRepository{
		listFullFunc: func(ctx context.Context, filter ListFilter) ([]domain.TicketFull, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewService(repo, nil, &mockDB{}, testLogger(), nil, nil, false)

	_, err := svc.ListTicketsFull(context.Background(), ListTicketsInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

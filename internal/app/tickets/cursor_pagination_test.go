package tickets

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "pet-ticket/internal/domain/tickets"
)

func TestListTicketsWithCursor_Success(t *testing.T) {
	item1 := domain.Ticket{ID: 3, CreatedAt: time.Now()}
	item2 := domain.Ticket{ID: 2, CreatedAt: time.Now().Add(-time.Minute)}

	var capturedFilter ListFilter
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			capturedFilter = filter
			return []domain.Ticket{item1, item2}, true, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	page, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
	if !page.HasMore {
		t.Error("expected HasMore to be true")
	}
	if page.NextCursor == "" {
		t.Fatal("expected non-empty NextCursor")
	}

	decodedTime, decodedID, err := DecodeCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("failed to decode NextCursor: %v", err)
	}
	if decodedID != item2.ID {
		t.Errorf("expected NextCursor built from last item (id=%d), got id=%d", item2.ID, decodedID)
	}
	if !decodedTime.Equal(item2.CreatedAt) {
		t.Errorf("expected NextCursor time %v, got %v", item2.CreatedAt, decodedTime)
	}

	// Дефолты: PageSize и Direction подставляются сервисом
	if capturedFilter.PageSize != DefaultCursorPageSize {
		t.Errorf("expected default page size %d, got %d", DefaultCursorPageSize, capturedFilter.PageSize)
	}
	if capturedFilter.Direction != "next" {
		t.Errorf("expected default direction 'next', got %q", capturedFilter.Direction)
	}
}

func TestListTicketsWithCursor_NoMore_EmptyNextCursor(t *testing.T) {
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			return []domain.Ticket{{ID: 1, CreatedAt: time.Now()}}, false, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	page, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if page.HasMore {
		t.Error("expected HasMore to be false")
	}
	if page.NextCursor != "" {
		t.Errorf("expected empty NextCursor, got %q", page.NextCursor)
	}
}

func TestListTicketsWithCursor_PageSizeClampedToMax(t *testing.T) {
	var capturedFilter ListFilter
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			capturedFilter = filter
			return nil, false, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{PageSize: 9999})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedFilter.PageSize != MaxCursorPageSize {
		t.Errorf("expected page size clamped to %d, got %d", MaxCursorPageSize, capturedFilter.PageSize)
	}
}

func TestListTicketsWithCursor_InvalidDirectionDefaultsToNext(t *testing.T) {
	var capturedFilter ListFilter
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			capturedFilter = filter
			return nil, false, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{Direction: "sideways"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedFilter.Direction != "next" {
		t.Errorf("expected direction to default to 'next', got %q", capturedFilter.Direction)
	}
}

func TestListTicketsWithCursor_PrevDirection_NextCursorFromFirstItem(t *testing.T) {
	// После разворота в дисплейный порядок (DESC) items[0] — самая новая запись,
	// продолжение в направлении "prev" должно отталкиваться именно от неё.
	newest := domain.Ticket{ID: 10, CreatedAt: time.Now()}
	oldest := domain.Ticket{ID: 9, CreatedAt: time.Now().Add(-time.Minute)}

	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			return []domain.Ticket{newest, oldest}, true, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	page, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{Direction: "prev"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, decodedID, err := DecodeCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("failed to decode NextCursor: %v", err)
	}
	if decodedID != newest.ID {
		t.Errorf("expected NextCursor built from newest item (id=%d), got id=%d", newest.ID, decodedID)
	}
}

func TestListTicketsWithCursor_RepoError(t *testing.T) {
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			return nil, false, errors.New("db error")
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListTicketsWithCursor_InvalidCursorPropagatesErrInvalidCursor(t *testing.T) {
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			return nil, false, ErrInvalidCursor
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Errorf("expected ErrInvalidCursor to propagate, got: %v", err)
	}
}

func TestListTicketsWithCursor_FiltersPassedThrough(t *testing.T) {
	userID := int64(42)
	topicID := int64(7)
	status := domain.StatusInProgress
	priority := domain.PriorityHigh
	cursor := "some-cursor"

	var capturedFilter ListFilter
	repo := &mockRepository{
		listWithCursorFunc: func(ctx context.Context, filter ListFilter) ([]domain.Ticket, bool, error) {
			capturedFilter = filter
			return nil, false, nil
		},
	}
	svc := NewService(repo, &mockDB{}, testLogger(), nil, nil)

	_, err := svc.ListTicketsWithCursor(context.Background(), ListTicketsWithCursorInput{
		UserID:   &userID,
		TopicID:  &topicID,
		Status:   &status,
		Priority: &priority,
		Cursor:   &cursor,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if capturedFilter.UserID == nil || *capturedFilter.UserID != userID {
		t.Error("expected UserID to be passed through")
	}
	if capturedFilter.TopicID == nil || *capturedFilter.TopicID != topicID {
		t.Error("expected TopicID to be passed through")
	}
	if capturedFilter.Status == nil || *capturedFilter.Status != status {
		t.Error("expected Status to be passed through")
	}
	if capturedFilter.Priority == nil || *capturedFilter.Priority != priority {
		t.Error("expected Priority to be passed through")
	}
	if capturedFilter.Cursor == nil || *capturedFilter.Cursor != cursor {
		t.Error("expected Cursor to be passed through")
	}
}

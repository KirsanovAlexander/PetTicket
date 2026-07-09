package tickets

import (
	"context"
	"errors"
	"testing"

	domainEvents "pet-ticket/internal/domain/events"
	domain "pet-ticket/internal/domain/tickets"
)

// mockRepository переиспользуется из service_test.go (тот же пакет).

func TestHistoryHandler_HandleTicketCreated(t *testing.T) {
	var captured domain.History
	repo := &mockRepository{
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			captured = history
			return nil
		},
	}
	h := NewHistoryHandler(repo, testLogger())

	event := domainEvents.TicketCreated{
		BaseEvent: domainEvents.NewBaseEvent(),
		TicketID:  1,
		UserID:    100,
		TopicID:   1,
		Status:    "new",
	}

	if err := h.HandleTicketCreated(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if captured.TicketID != 1 || captured.UserID != 100 {
		t.Errorf("unexpected history: %+v", captured)
	}
	if captured.Action != domain.ActionCreated {
		t.Errorf("expected action 'created', got %s", captured.Action)
	}
	if captured.NewValue != "status=new" {
		t.Errorf("expected new_value 'status=new', got %s", captured.NewValue)
	}
}

func TestHistoryHandler_HandleTicketCreated_WrongEventType(t *testing.T) {
	repo := &mockRepository{}
	h := NewHistoryHandler(repo, testLogger())

	err := h.HandleTicketCreated(context.Background(), domainEvents.TicketAssigned{})
	if err == nil {
		t.Error("expected error for mismatched event type, got nil")
	}
}

func TestHistoryHandler_HandleTicketStatusChanged(t *testing.T) {
	var captured domain.History
	repo := &mockRepository{
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			captured = history
			return nil
		},
	}
	h := NewHistoryHandler(repo, testLogger())

	event := domainEvents.TicketStatusChanged{
		BaseEvent: domainEvents.NewBaseEvent(),
		TicketID:  1,
		OldStatus: "new",
		NewStatus: "in_progress",
		ChangedBy: 100,
		Resolved:  false,
	}

	if err := h.HandleTicketStatusChanged(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if captured.Action != domain.ActionStatusChanged {
		t.Errorf("expected action 'status_changed', got %s", captured.Action)
	}
	if captured.OldValue != "new" || captured.NewValue != "in_progress" {
		t.Errorf("unexpected old/new value: %s -> %s", captured.OldValue, captured.NewValue)
	}
}

func TestHistoryHandler_HandleTicketStatusChanged_ResolvedWritesExtraEntry(t *testing.T) {
	var recorded []domain.History
	repo := &mockRepository{
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			recorded = append(recorded, history)
			return nil
		},
	}
	h := NewHistoryHandler(repo, testLogger())

	event := domainEvents.TicketStatusChanged{
		BaseEvent: domainEvents.NewBaseEvent(),
		TicketID:  1,
		OldStatus: "in_progress",
		NewStatus: "resolved",
		ChangedBy: 100,
		Resolved:  true,
	}

	if err := h.HandleTicketStatusChanged(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(recorded) != 2 {
		t.Fatalf("expected 2 history entries (status_changed + resolved), got %d", len(recorded))
	}
	if recorded[0].Action != domain.ActionStatusChanged {
		t.Errorf("expected first entry action 'status_changed', got %s", recorded[0].Action)
	}
	if recorded[1].Action != domain.ActionResolved {
		t.Errorf("expected second entry action 'resolved', got %s", recorded[1].Action)
	}
}

func TestHistoryHandler_HandleTicketCommentAdded(t *testing.T) {
	var captured domain.History
	repo := &mockRepository{
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			captured = history
			return nil
		},
	}
	h := NewHistoryHandler(repo, testLogger())

	event := domainEvents.TicketCommentAdded{
		BaseEvent:  domainEvents.NewBaseEvent(),
		TicketID:   1,
		UserID:     100,
		OldComment: "old",
		NewComment: "new",
	}

	if err := h.HandleTicketCommentAdded(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if captured.Action != domain.ActionCommentAdded {
		t.Errorf("expected action 'comment_added', got %s", captured.Action)
	}
	if captured.OldValue != "old" || captured.NewValue != "new" {
		t.Errorf("unexpected old/new value: %s -> %s", captured.OldValue, captured.NewValue)
	}
}

func TestHistoryHandler_HandleTicketAssigned(t *testing.T) {
	var captured domain.History
	repo := &mockRepository{
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			captured = history
			return nil
		},
	}
	h := NewHistoryHandler(repo, testLogger())

	event := domainEvents.TicketAssigned{
		BaseEvent:  domainEvents.NewBaseEvent(),
		TicketID:   1,
		OperatorID: 55,
		AssignedBy: 100,
	}

	if err := h.HandleTicketAssigned(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if captured.Action != domain.ActionAssigned {
		t.Errorf("expected action 'assigned', got %s", captured.Action)
	}
	if captured.UserID != 100 {
		t.Errorf("expected user_id 100 (assigned_by), got %d", captured.UserID)
	}
	if captured.NewValue != "operator=55" {
		t.Errorf("expected new_value 'operator=55', got %s", captured.NewValue)
	}
}

func TestHistoryHandler_PropagatesRepositoryError(t *testing.T) {
	repo := &mockRepository{
		addHistoryFunc: func(ctx context.Context, history domain.History) error {
			return errors.New("db unavailable")
		},
	}
	h := NewHistoryHandler(repo, testLogger())

	event := domainEvents.TicketCreated{BaseEvent: domainEvents.NewBaseEvent(), TicketID: 1, UserID: 100}
	if err := h.HandleTicketCreated(context.Background(), event); err == nil {
		t.Error("expected error to propagate from repository, got nil")
	}
}

func TestMetricsHandler_HandleDoesNotError(t *testing.T) {
	h := NewMetricsHandler(testLogger())

	events := []domainEvents.Event{
		domainEvents.TicketCreated{BaseEvent: domainEvents.NewBaseEvent()},
		domainEvents.TicketStatusChanged{BaseEvent: domainEvents.NewBaseEvent()},
		domainEvents.TicketCommentAdded{BaseEvent: domainEvents.NewBaseEvent()},
		domainEvents.TicketAssigned{BaseEvent: domainEvents.NewBaseEvent()},
	}

	for _, e := range events {
		if err := h.Handle(context.Background(), e); err != nil {
			t.Errorf("expected metrics handler not to error for %s, got: %v", e.EventName(), err)
		}
	}
}

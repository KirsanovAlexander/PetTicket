package notifications

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	domain "pet-ticket/internal/domain/notifications"

	"github.com/rs/zerolog"
)

type mockOutboxRepository struct {
	findPendingFunc func(ctx context.Context, limit int) ([]domain.OutboxEntry, error)
	updateFunc      func(ctx context.Context, entry domain.OutboxEntry) error
	updatedEntries  []domain.OutboxEntry
}

func (m *mockOutboxRepository) Create(ctx context.Context, entry domain.OutboxEntry) error {
	return errors.New("not implemented")
}

func (m *mockOutboxRepository) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	if m.findPendingFunc != nil {
		return m.findPendingFunc(ctx, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOutboxRepository) Update(ctx context.Context, entry domain.OutboxEntry) error {
	m.updatedEntries = append(m.updatedEntries, entry)
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entry)
	}
	return nil
}

type mockNotifier struct {
	notifyFunc func(ctx context.Context, n domain.Notification) error
	calls      []domain.Notification
}

func (m *mockNotifier) Notify(ctx context.Context, n domain.Notification) error {
	m.calls = append(m.calls, n)
	if m.notifyFunc != nil {
		return m.notifyFunc(ctx, n)
	}
	return nil
}

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestProcessBatch_Success(t *testing.T) {
	repo := &mockOutboxRepository{
		findPendingFunc: func(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
			return []domain.OutboxEntry{
				{ID: 1, UserID: 100, TicketID: 5, Type: domain.NotifStatusChanged, Attempts: 0, MaxAttempts: 5},
			}, nil
		},
	}
	notifier := &mockNotifier{}
	sender := NewSender(repo, notifier, testLogger(), time.Second)

	n, err := sender.ProcessBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 processed entry, got %d", n)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notify call, got %d", len(notifier.calls))
	}
	if notifier.calls[0].TicketID != 5 {
		t.Errorf("expected ticketID 5 in notification, got %d", notifier.calls[0].TicketID)
	}

	if len(repo.updatedEntries) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(repo.updatedEntries))
	}
	updated := repo.updatedEntries[0]
	if updated.Status != domain.OutboxStatusSent {
		t.Errorf("expected status sent, got %s", updated.Status)
	}
	if updated.SentAt == nil {
		t.Error("expected SentAt to be set")
	}
	if updated.Attempts != 1 {
		t.Errorf("expected attempts=1, got %d", updated.Attempts)
	}
}

func TestProcessBatch_FailureBelowMaxAttempts_StaysPending(t *testing.T) {
	repo := &mockOutboxRepository{
		findPendingFunc: func(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
			return []domain.OutboxEntry{
				{ID: 1, UserID: 100, TicketID: 5, Attempts: 1, MaxAttempts: 5},
			}, nil
		},
	}
	notifier := &mockNotifier{
		notifyFunc: func(ctx context.Context, n domain.Notification) error {
			return errors.New("connection refused")
		},
	}
	sender := NewSender(repo, notifier, testLogger(), time.Second)

	before := time.Now()
	_, err := sender.ProcessBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	updated := repo.updatedEntries[0]
	if updated.Status != domain.OutboxStatusPending {
		t.Errorf("expected status pending (attempts 2 < max 5), got %s", updated.Status)
	}
	if updated.Attempts != 2 {
		t.Errorf("expected attempts=2, got %d", updated.Attempts)
	}
	if updated.ErrorMessage != "connection refused" {
		t.Errorf("expected error message to be recorded, got %q", updated.ErrorMessage)
	}
	if !updated.NextRetryAt.After(before) {
		t.Error("expected NextRetryAt to be scheduled in the future")
	}
}

func TestProcessBatch_FailureAtMaxAttempts_MarksFailed(t *testing.T) {
	repo := &mockOutboxRepository{
		findPendingFunc: func(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
			return []domain.OutboxEntry{
				{ID: 1, UserID: 100, TicketID: 5, Attempts: 4, MaxAttempts: 5},
			}, nil
		},
	}
	notifier := &mockNotifier{
		notifyFunc: func(ctx context.Context, n domain.Notification) error {
			return errors.New("still down")
		},
	}
	sender := NewSender(repo, notifier, testLogger(), time.Second)

	_, err := sender.ProcessBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	updated := repo.updatedEntries[0]
	if updated.Status != domain.OutboxStatusFailed {
		t.Errorf("expected status failed (attempts 5 >= max 5), got %s", updated.Status)
	}
	if updated.Attempts != 5 {
		t.Errorf("expected attempts=5, got %d", updated.Attempts)
	}
}

func TestProcessBatch_NoPendingEntries(t *testing.T) {
	repo := &mockOutboxRepository{
		findPendingFunc: func(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
			return nil, nil
		},
	}
	notifier := &mockNotifier{}
	sender := NewSender(repo, notifier, testLogger(), time.Second)

	n, err := sender.ProcessBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 processed, got %d", n)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected notifier not to be called, got %d calls", len(notifier.calls))
	}
}

func TestProcessBatch_FindPendingError_Propagates(t *testing.T) {
	repo := &mockOutboxRepository{
		findPendingFunc: func(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
			return nil, errors.New("db down")
		},
	}
	sender := NewSender(repo, &mockNotifier{}, testLogger(), time.Second)

	_, err := sender.ProcessBatch(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBackoff_ExponentialGrowth(t *testing.T) {
	sender := NewSender(&mockOutboxRepository{}, &mockNotifier{}, testLogger(), time.Second)

	// base * 5^(attempt-1), допускаем ±20% джиттера
	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, time.Second},
		{2, 5 * time.Second},
		{3, 25 * time.Second},
	}

	for _, c := range cases {
		d := sender.backoff(c.attempt)
		tolerance := float64(c.expected) * 0.25 // чуть шире jitter, на всякий случай
		if math.Abs(float64(d-c.expected)) > tolerance {
			t.Errorf("attempt %d: expected ~%v (±20%%), got %v", c.attempt, c.expected, d)
		}
	}
}

func TestBackoff_CappedAtMaxBackoff(t *testing.T) {
	sender := NewSender(&mockOutboxRepository{}, &mockNotifier{}, testLogger(), time.Second)

	d := sender.backoff(50) // 5^49 астрономически больше maxBackoff
	upperBound := maxBackoff + time.Duration(float64(maxBackoff)*jitterFraction)
	if d > upperBound {
		t.Errorf("expected backoff capped at ~%v, got %v", maxBackoff, d)
	}
	if d <= 0 {
		t.Errorf("expected positive backoff, got %v", d)
	}
}

func TestBackoff_AttemptBelowOne_TreatedAsOne(t *testing.T) {
	sender := NewSender(&mockOutboxRepository{}, &mockNotifier{}, testLogger(), time.Second)

	d := sender.backoff(0)
	tolerance := float64(time.Second) * 0.25
	if math.Abs(float64(d-time.Second)) > tolerance {
		t.Errorf("expected backoff(0) to behave like backoff(1) (~1s), got %v", d)
	}
}

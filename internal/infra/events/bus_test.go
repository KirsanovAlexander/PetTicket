package events

import (
	"context"
	"errors"
	"testing"
	"time"

	domainEvents "pet-ticket/internal/domain/events"
)

// stubEvent — минимальная реализация domainEvents.Event для тестов шины,
// не завязанная на конкретные тикет-события.
type stubEvent struct {
	name string
}

func (e stubEvent) EventName() string     { return e.name }
func (e stubEvent) OccurredAt() time.Time { return time.Time{} }

func TestInMemoryBus_PublishCallsSubscribedHandler(t *testing.T) {
	bus := NewInMemoryBus()

	called := false
	var gotEvent domainEvents.Event
	bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
		called = true
		gotEvent = event
		return nil
	})

	event := stubEvent{name: "test.event"}
	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !called {
		t.Error("expected handler to be called")
	}
	if gotEvent.EventName() != "test.event" {
		t.Errorf("expected event name 'test.event', got %q", gotEvent.EventName())
	}
}

func TestInMemoryBus_PublishCallsMultipleHandlersInOrder(t *testing.T) {
	bus := NewInMemoryBus()

	var order []string
	bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
		order = append(order, "first")
		return nil
	})
	bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
		order = append(order, "second")
		return nil
	})

	if err := bus.Publish(context.Background(), stubEvent{name: "test.event"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("expected handlers called in subscription order, got %v", order)
	}
}

func TestInMemoryBus_PublishOnlyCallsHandlersForMatchingEventName(t *testing.T) {
	bus := NewInMemoryBus()

	otherCalled := false
	bus.Subscribe("other.event", func(ctx context.Context, event domainEvents.Event) error {
		otherCalled = true
		return nil
	})

	if err := bus.Publish(context.Background(), stubEvent{name: "test.event"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if otherCalled {
		t.Error("expected handler subscribed to a different event name NOT to be called")
	}
}

func TestInMemoryBus_PublishWithNoSubscribers(t *testing.T) {
	bus := NewInMemoryBus()

	if err := bus.Publish(context.Background(), stubEvent{name: "unheard.event"}); err != nil {
		t.Fatalf("expected no error when publishing with no subscribers, got: %v", err)
	}
}

func TestInMemoryBus_PublishAggregatesHandlerErrorsAndCallsAll(t *testing.T) {
	bus := NewInMemoryBus()

	errFirst := errors.New("first handler failed")
	errSecond := errors.New("second handler failed")

	secondCalled := false
	bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
		return errFirst
	})
	bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
		secondCalled = true
		return errSecond
	})

	err := bus.Publish(context.Background(), stubEvent{name: "test.event"})

	if !secondCalled {
		t.Error("expected second handler to be called even though the first one failed")
	}
	if !errors.Is(err, errFirst) {
		t.Errorf("expected aggregated error to include first handler's error, got: %v", err)
	}
	if !errors.Is(err, errSecond) {
		t.Errorf("expected aggregated error to include second handler's error, got: %v", err)
	}
}

func TestInMemoryBus_SubscribeIsConcurrencySafe(t *testing.T) {
	bus := NewInMemoryBus()

	const goroutines = 20
	done := make(chan struct{}, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
				return nil
			})
			done <- struct{}{}
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}

	callCount := 0
	bus.Subscribe("test.event", func(ctx context.Context, event domainEvents.Event) error {
		callCount++
		return nil
	})

	if err := bus.Publish(context.Background(), stubEvent{name: "test.event"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected the last subscribed handler to be called exactly once, got %d", callCount)
	}
}

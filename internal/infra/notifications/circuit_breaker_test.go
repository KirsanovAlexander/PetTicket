package notifications

import (
	"errors"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func TestCircuitBreaker_StartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Minute)
	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state Closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterFailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Minute)

	for i := 0; i < 3; i++ {
		err := cb.Call(func() error { return errBoom })
		if !errors.Is(err, errBoom) {
			t.Fatalf("expected errBoom, got: %v", err)
		}
	}

	if cb.State() != CircuitOpen {
		t.Fatalf("expected state Open after %d failures, got %s", 3, cb.State())
	}
}

func TestCircuitBreaker_OpenBlocksCallsWithoutInvokingFn(t *testing.T) {
	cb := NewCircuitBreaker(1, 2, time.Minute)

	if err := cb.Call(func() error { return errBoom }); !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom, got: %v", err)
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("expected Open after 1 failure with threshold 1, got %s", cb.State())
	}

	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
	if called {
		t.Error("expected fn NOT to be called while circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 1, 10*time.Millisecond)

	_ = cb.Call(func() error { return errBoom })
	if cb.State() != CircuitOpen {
		t.Fatalf("expected Open, got %s", cb.State())
	}

	time.Sleep(20 * time.Millisecond)

	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error for successful half-open probe, got: %v", err)
	}
	if !called {
		t.Error("expected fn to be called once timeout elapsed (half-open probe)")
	}
}

func TestCircuitBreaker_HalfOpenSuccessesCloseCircuit(t *testing.T) {
	cb := NewCircuitBreaker(1, 2, 10*time.Millisecond)

	_ = cb.Call(func() error { return errBoom })
	time.Sleep(20 * time.Millisecond)

	if err := cb.Call(func() error { return nil }); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("expected still HalfOpen after 1/2 successes, got %s", cb.State())
	}

	if err := cb.Call(func() error { return nil }); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cb.State() != CircuitClosed {
		t.Fatalf("expected Closed after successThreshold successes, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReturnsToOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 2, 10*time.Millisecond)

	_ = cb.Call(func() error { return errBoom })
	time.Sleep(20 * time.Millisecond)

	// Пробный вызов в HalfOpen проваливается
	err := cb.Call(func() error { return errBoom })
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom, got: %v", err)
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("expected Open after half-open probe failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_ClosedSuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Minute)

	_ = cb.Call(func() error { return errBoom })
	_ = cb.Call(func() error { return errBoom })
	if cb.State() != CircuitClosed {
		t.Fatalf("expected still Closed after 2/3 failures, got %s", cb.State())
	}

	// Успех должен сбросить счётчик неудач
	if err := cb.Call(func() error { return nil }); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Ещё 2 неудачи (после сброса) не должны разомкнуть breaker — нужно 3 подряд
	_ = cb.Call(func() error { return errBoom })
	_ = cb.Call(func() error { return errBoom })
	if cb.State() != CircuitClosed {
		t.Errorf("expected Closed (failure count was reset by success), got %s", cb.State())
	}
}

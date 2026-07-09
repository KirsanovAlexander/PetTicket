package notifications

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "pet-ticket/internal/domain/notifications"
)

func TestHTTPNotifier_Success(t *testing.T) {
	var receivedBody domain.Notification
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	breaker := NewCircuitBreaker(3, 2, time.Minute)
	notifier := NewHTTPNotifier(server.URL, 2*time.Second, breaker)

	notification := domain.Notification{
		UserID:   100,
		TicketID: 42,
		Type:     domain.NotifStatusChanged,
		Payload:  map[string]interface{}{"title": "test"},
	}

	if err := notifier.Notify(t.Context(), notification); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if receivedBody.UserID != 100 || receivedBody.TicketID != 42 {
		t.Errorf("unexpected received body: %+v", receivedBody)
	}
	if breaker.State() != CircuitClosed {
		t.Errorf("expected breaker to stay Closed after success, got %s", breaker.State())
	}
}

func TestHTTPNotifier_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	breaker := NewCircuitBreaker(3, 2, time.Minute)
	notifier := NewHTTPNotifier(server.URL, 2*time.Second, breaker)

	err := notifier.Notify(t.Context(), domain.Notification{UserID: 1, TicketID: 1})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestHTTPNotifier_ConnectionFailure(t *testing.T) {
	breaker := NewCircuitBreaker(3, 2, time.Minute)
	// Порт заведомо закрыт — соединение должно провалиться.
	notifier := NewHTTPNotifier("http://127.0.0.1:1", 500*time.Millisecond, breaker)

	err := notifier.Notify(t.Context(), domain.Notification{UserID: 1, TicketID: 1})
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
}

func TestHTTPNotifier_CircuitOpenSkipsRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	breaker := NewCircuitBreaker(1, 2, time.Minute)
	notifier := NewHTTPNotifier(server.URL, 2*time.Second, breaker)

	// Форсируем открытие breaker без обращения к серверу.
	_ = breaker.Call(func() error { return errors.New("boom") })
	if breaker.State() != CircuitOpen {
		t.Fatalf("expected breaker Open, got %s", breaker.State())
	}

	err := notifier.Notify(t.Context(), domain.Notification{UserID: 1, TicketID: 1})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
	if called {
		t.Error("expected server NOT to be called while circuit is open")
	}
}

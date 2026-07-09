package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	domain "pet-ticket/internal/domain/notifications"
)

// HTTPNotifier отправляет уведомления на внешний webhook по HTTP. Каждый
// вызов идёт через Circuit Breaker: при серии сбоев внешнего сервиса
// notifier перестаёт делать реальные HTTP-запросы (ErrCircuitOpen), пока
// breaker не даст пробный вызов — это защищает и приложение (не тратим
// время/горутины на заведомо падающие запросы), и сам внешний сервис (не
// заваливаем его retry-штормом, пока он восстанавливается).
type HTTPNotifier struct {
	client  *http.Client
	url     string
	breaker *CircuitBreaker
}

// NewHTTPNotifier создаёт HTTP notifier.
func NewHTTPNotifier(url string, timeout time.Duration, breaker *CircuitBreaker) *HTTPNotifier {
	return &HTTPNotifier{
		client:  &http.Client{Timeout: timeout},
		url:     url,
		breaker: breaker,
	}
}

// Notify отправляет уведомление на webhook через Circuit Breaker.
// Возвращает ErrCircuitOpen, если breaker разомкнут (запрос не делался).
func (n *HTTPNotifier) Notify(ctx context.Context, notification domain.Notification) error {
	return n.breaker.Call(func() error {
		return n.send(ctx, notification)
	})
}

func (n *HTTPNotifier) send(ctx context.Context, notification domain.Notification) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build notification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("notification request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 2xx — успех. Всё остальное (4xx/5xx) — неудача, которую засчитывает
	// Circuit Breaker и на которую sender спланирует retry.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notification endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

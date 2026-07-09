package events

import (
	"context"
	"errors"
	"sync"

	domainEvents "pet-ticket/internal/domain/events"
)

// Handler обрабатывает одно доменное событие.
type Handler func(ctx context.Context, event domainEvents.Event) error

// Bus — контракт шины доменных событий: подписка обработчика на имя события
// и публикация события всем подписчикам.
type Bus interface {
	Subscribe(eventName string, handler Handler)
	Publish(ctx context.Context, event domainEvents.Event) error
}

// InMemoryBus — синхронная in-memory реализация Bus. Publish вызывает все
// подписанные на данное имя события обработчики последовательно в
// вызывающей горутине — без очереди и без брокера сообщений. Для одного
// процесса этого достаточно; если понадобится персистентность/асинхронность,
// на замену придёт другая реализация Bus, а не переписывание вызывающего кода.
type InMemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewInMemoryBus создаёт пустую шину событий.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{handlers: make(map[string][]Handler)}
}

// Subscribe регистрирует обработчик на события с данным именем. Порядок
// вызова нескольких обработчиков одного события — порядок подписки.
func (b *InMemoryBus) Subscribe(eventName string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
}

// Publish вызывает все обработчики, подписанные на event.EventName().
// Ошибка одного обработчика не прерывает вызов остальных (например, сбой
// в metrics не должен помешать history записать историю) — все ошибки
// агрегируются через errors.Join.
func (b *InMemoryBus) Publish(ctx context.Context, event domainEvents.Event) error {
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers[event.EventName()]...)
	b.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

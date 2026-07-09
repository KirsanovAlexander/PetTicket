package notifications

import (
	"errors"
	"sync"
	"time"
)

// CircuitState состояние Circuit Breaker.
type CircuitState int

const (
	// CircuitClosed — нормальная работа, все вызовы проходят к fn.
	CircuitClosed CircuitState = iota
	// CircuitOpen — вызовы блокируются без обращения к fn (защищаемый
	// ресурс считается недоступным).
	CircuitOpen
	// CircuitHalfOpen — пробный режим после timeout: вызовы пропускаются,
	// но одна неудача сразу возвращает в Open.
	CircuitHalfOpen
)

// String возвращает строковое представление состояния (для логов/метрик).
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen возвращается Call, когда breaker разомкнут и fn не
// вызывается вовсе.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker защищает вызовы нестабильного внешнего ресурса: после
// failureThreshold подряд неудач в Closed переходит в Open и перестаёт
// пропускать вызовы на timeout. По истечении timeout пробует один вызов в
// HalfOpen — successThreshold подряд успехов закрывают breaker обратно,
// любая неудача в HalfOpen немедленно возвращает в Open.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	failures         int
	successes        int
	lastFailure      time.Time
}

// NewCircuitBreaker создаёт Circuit Breaker.
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Call выполняет fn через Circuit Breaker. Если breaker открыт и timeout ещё
// не истёк — fn не вызывается, возвращается ErrCircuitOpen. Если timeout
// истёк, breaker переходит в HalfOpen и пропускает один пробный вызов.
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.state == CircuitOpen && time.Since(cb.lastFailure) > cb.timeout {
		cb.state = CircuitHalfOpen
		cb.successes = 0
	}
	state := cb.state
	cb.mu.Unlock()

	if state == CircuitOpen {
		return ErrCircuitOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// recordResult обновляет состояние breaker по результату вызова.
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.successes = 0
		cb.lastFailure = time.Now()

		switch cb.state {
		case CircuitHalfOpen:
			// Пробный вызов в HalfOpen провалился — ресурс всё ещё плох.
			cb.state = CircuitOpen
		case CircuitClosed:
			if cb.failures >= cb.failureThreshold {
				cb.state = CircuitOpen
			}
		}
		return
	}

	switch cb.state {
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
		}
	case CircuitClosed:
		cb.failures = 0
	}
}

// State возвращает текущее состояние breaker (для метрик и тестов).
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

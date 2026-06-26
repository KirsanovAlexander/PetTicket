package tickets

import (
	"errors"
	"time"
)

// Ticket представляет доменную сущность тикета
type Ticket struct {
	ID                 int64
	UserID             int64
	TopicID            int64
	Status             Status
	Priority           Priority
	Amount             *float64
	Comment            string
	ResponseDeadline   time.Time
	ResolutionDeadline time.Time
	FirstResponseAt    *time.Time
	ResolvedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsFirstResponse возвращает true, если первый ответ саппорта уже зафиксирован
func (t Ticket) IsFirstResponse() bool {
	return t.FirstResponseAt != nil
}

// IsResolved возвращает true, если тикет решён
func (t Ticket) IsResolved() bool {
	return t.ResolvedAt != nil
}

// GetSLAStatus рассчитывает текущие SLA-метрики тикета
func (t Ticket) GetSLAStatus(now time.Time) SLAMetrics {
	return CalculateSLAStatus(
		t.CreatedAt,
		t.ResponseDeadline,
		t.ResolutionDeadline,
		t.FirstResponseAt,
		t.ResolvedAt,
		now,
	)
}

// Validate проверяет валидность тикета
func (t Ticket) Validate() error {
	if t.UserID <= 0 {
		return errors.New("invalid user_id")
	}
	if t.TopicID <= 0 {
		return errors.New("invalid topic_id")
	}
	if !t.Status.IsValid() {
		return errors.New("invalid status")
	}
	if !t.Priority.IsValid() {
		return errors.New("invalid priority")
	}
	if t.Amount != nil && *t.Amount < 0 {
		return errors.New("amount cannot be negative")
	}
	return nil
}

// IsNew проверяет, является ли тикет новым (не сохранённым)
func (t Ticket) IsNew() bool {
	return t.ID == 0
}

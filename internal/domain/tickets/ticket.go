package tickets

import (
	"errors"
	"time"
)

// Ticket представляет доменную сущность тикета
type Ticket struct {
	ID        int64
	UserID    int64
	TopicID   int64
	Status    Status
	Amount    *float64
	Comment   string
	CreatedAt time.Time
	UpdatedAt time.Time
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
	if t.Amount != nil && *t.Amount < 0 {
		return errors.New("amount cannot be negative")
	}
	return nil
}

// IsNew проверяет, является ли тикет новым (не сохранённым)
func (t Ticket) IsNew() bool {
	return t.ID == 0
}

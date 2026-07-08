package tickets

import (
	"fmt"
	"strconv"
)

// Priority представляет уровень приоритета тикета
type Priority int

const (
	PriorityLow      Priority = 1
	PriorityMedium   Priority = 2
	PriorityHigh     Priority = 3
	PriorityCritical Priority = 4
)

// String возвращает строковое представление приоритета
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// IsValid проверяет валидность приоритета
func (p Priority) IsValid() bool {
	return p >= PriorityLow && p <= PriorityCritical
}

// Escalate повышает приоритет на один уровень (максимум Critical)
func (p Priority) Escalate() Priority {
	if p >= PriorityCritical {
		return PriorityCritical
	}
	return p + 1
}

// ParsePriority парсит строковое представление приоритета
func ParsePriority(s string) (Priority, error) {
	switch s {
	case "low", "1":
		return PriorityLow, nil
	case "medium", "2":
		return PriorityMedium, nil
	case "high", "3":
		return PriorityHigh, nil
	case "critical", "4":
		return PriorityCritical, nil
	default:
		if id, err := strconv.Atoi(s); err == nil {
			p := Priority(id)
			if p.IsValid() {
				return p, nil
			}
		}
		return 0, fmt.Errorf("invalid priority: %s", s)
	}
}

package tickets

// Status представляет статус тикета в системе
type Status int

const (
	StatusNew        Status = 1
	StatusInProgress Status = 2
	StatusResolved   Status = 3
	StatusClosed     Status = 4
	StatusCancelled  Status = 5
)

// String возвращает строковое представление статуса
func (s Status) String() string {
	switch s {
	case StatusNew:
		return "new"
	case StatusInProgress:
		return "in_progress"
	case StatusResolved:
		return "resolved"
	case StatusClosed:
		return "closed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// IsValid проверяет валидность статуса
func (s Status) IsValid() bool {
	return s >= StatusNew && s <= StatusCancelled
}

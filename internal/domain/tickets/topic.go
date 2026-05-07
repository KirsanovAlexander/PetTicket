package tickets

// Topic представляет тему/категорию тикета
type Topic struct {
	ID          int64
	ExternalID  int64
	Title       string
	Description string
}

// IsValid проверяет валидность темы
func (t Topic) IsValid() bool {
	return t.ID > 0 && t.ExternalID > 0 && t.Title != ""
}

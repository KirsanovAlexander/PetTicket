package tickets

import "time"

// HistoryAction представляет тип действия в истории тикета
type HistoryAction string

const (
	ActionCreated         HistoryAction = "created"
	ActionStatusChanged   HistoryAction = "status_changed"
	ActionCommentAdded    HistoryAction = "comment_added"
	ActionUpdated         HistoryAction = "updated"
	ActionPriorityChanged HistoryAction = "priority_changed"
	ActionEscalated       HistoryAction = "escalated"
	ActionFirstResponse   HistoryAction = "first_response"
	ActionResolved        HistoryAction = "resolved"
)

// History представляет запись в истории изменений тикета
type History struct {
	ID        int64
	TicketID  int64
	UserID    int64
	Action    HistoryAction
	OldValue  string
	NewValue  string
	CreatedAt time.Time
}

// IsValid проверяет валидность записи истории
func (h History) IsValid() bool {
	return h.TicketID > 0 && h.UserID > 0 && h.Action != ""
}

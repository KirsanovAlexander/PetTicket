package tickets

import "time"

// TicketComment — комментарий к тикету, хранящийся в отдельной таблице
// ticket_comments. В отличие от Comment (см. ticket_full.go, который
// реконструируется из ticket_history для v2 API), это полноценная
// сущность с собственной историей изменений (UpdatedAt) и видимостью
// (IsInternal).
type TicketComment struct {
	ID         int64
	TicketID   int64
	UserID     int64
	Content    string
	IsInternal bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AddCommentInput входные данные для добавления комментария.
type AddCommentInput struct {
	TicketID   int64
	UserID     int64
	Content    string
	IsInternal bool
}

// UpdateCommentInput входные данные для редактирования комментария.
type UpdateCommentInput struct {
	ID      int64
	Content string
}

// ListCommentsFilter фильтр для выборки комментариев тикета.
type ListCommentsFilter struct {
	TicketID int64
	// IncludeInternal — включать ли internal-заметки (видны только
	// саппорту). false — только публичные комментарии.
	IncludeInternal bool
	Limit           int
	Offset          int
}

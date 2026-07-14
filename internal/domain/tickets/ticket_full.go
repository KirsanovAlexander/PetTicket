package tickets

import "time"

// User — минимальная ссылка на пользователя. Отдельного сервиса/таблицы
// профилей в системе нет: user_id приходит из внешней системы (см.
// AuthMiddleware, заголовок X-User-ID) и никаких данных кроме ID нам не
// известно — поэтому в отличие от подсказки задания (JOIN users u ON ...)
// здесь просто ID, без выдуманных Name/Email.
type User struct {
	ID int64
}

// TicketStatusInfo — статус тикета с человекочитаемым названием из
// справочника ticket_statuses (столбец description).
type TicketStatusInfo struct {
	ID          int
	Name        string
	DisplayName string
}

// Comment — комментарий к тикету, реконструированный из ticket_history
// (записи с action=comment_added). Отдельной таблицы комментариев в системе
// нет — Comment.Text = ticket_history.new_value.
type Comment struct {
	ID        int64
	UserID    int64
	Text      string
	CreatedAt time.Time
}

// SLAInfo — SLA-срезы тикета: дедлайны и статусы по фазам response/resolution.
type SLAInfo struct {
	ResponseDeadline   time.Time
	ResolutionDeadline time.Time
	ResponseStatus     SLAStatus
	ResolutionStatus   SLAStatus
	OverallStatus      SLAStatus
}

// TicketFull — тикет со всеми связями раскрытыми во вложенные объекты
// (v2 API). В отличие от Ticket (плоская модель с *_id полями), здесь
// User/Status/Topic/Assignee — полноценные вложенные структуры.
type TicketFull struct {
	ID        int64
	User      User
	Status    TicketStatusInfo
	Topic     Topic
	Assignee  *User
	Priority  Priority
	Comments  []Comment
	SLA       *SLAInfo
	Amount    *float64
	Comment   string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

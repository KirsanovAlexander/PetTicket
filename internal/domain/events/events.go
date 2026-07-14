package events

import "time"

// Названия событий — единственный источник истины для строк, на которые
// подписываются обработчики (infra/events.Bus.Subscribe принимает строку,
// а не тип), чтобы не разошлись копии литералов в разных пакетах.
const (
	EventTicketCreated       = "ticket.created"
	EventTicketStatusChanged = "ticket.status_changed"
	EventTicketCommentAdded  = "ticket.comment_added"
	EventTicketAssigned      = "ticket.assigned"
	EventTicketUnassigned    = "ticket.unassigned"
)

// Event — доменное событие: у любого события есть имя (для маршрутизации
// в шине) и момент возникновения.
type Event interface {
	EventName() string
	OccurredAt() time.Time
}

// BaseEvent — встраиваемая часть, общая для всех событий. occurredAt
// приватное: значение фиксируется один раз через NewBaseEvent и не может
// быть подменено после создания события.
type BaseEvent struct {
	occurredAt time.Time
}

// NewBaseEvent фиксирует текущее время как момент возникновения события.
func NewBaseEvent() BaseEvent {
	return BaseEvent{occurredAt: time.Now()}
}

// OccurredAt возвращает момент возникновения события.
func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

// TicketCreated — тикет создан.
type TicketCreated struct {
	BaseEvent
	TicketID int64
	UserID   int64
	TopicID  int64
	Status   string
}

// EventName возвращает имя события.
func (e TicketCreated) EventName() string { return EventTicketCreated }

// TicketStatusChanged — статус тикета изменился.
//
// Resolved — вспомогательный флаг: true, если это изменение статуса также
// является моментом решения тикета (см. SLACalculator.ShouldSetResolvedAt
// в app-слое). HistoryHandler по этому флагу пишет дополнительную запись
// с action=resolved — так сохраняется поведение, которое раньше было
// отдельным AddHistory-вызовом прямо в сервисе.
type TicketStatusChanged struct {
	BaseEvent
	TicketID  int64
	OldStatus string
	NewStatus string
	ChangedBy int64
	Resolved  bool
}

// EventName возвращает имя события.
func (e TicketStatusChanged) EventName() string { return EventTicketStatusChanged }

// TicketCommentAdded — комментарий тикета изменён.
type TicketCommentAdded struct {
	BaseEvent
	TicketID   int64
	UserID     int64
	OldComment string
	NewComment string
}

// EventName возвращает имя события.
func (e TicketCommentAdded) EventName() string { return EventTicketCommentAdded }

// TicketAssigned — саппорт взял тикет в работу (self-assign, см. Task 13).
// История назначения пишется напрямую в транзакции Service.AssignTicket
// (see AddHistory там же) — это событие только для метрик/наблюдаемости,
// HistoryHandler на него больше не подписан (иначе была бы задвоенная
// запись в ticket_history).
type TicketAssigned struct {
	BaseEvent
	TicketID   int64
	AssigneeID int64
}

// EventName возвращает имя события.
func (e TicketAssigned) EventName() string { return EventTicketAssigned }

// TicketUnassigned — саппорт снял с себя назначение тикета. Как и
// TicketAssigned, только для метрик — история пишется напрямую в транзакции.
type TicketUnassigned struct {
	BaseEvent
	TicketID   int64
	AssigneeID int64
}

// EventName возвращает имя события.
func (e TicketUnassigned) EventName() string { return EventTicketUnassigned }

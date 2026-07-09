package notifications

import "context"

// NotificationType определяет тип уведомления.
type NotificationType string

const (
	// NotifStatusChanged — уведомление о смене статуса тикета.
	NotifStatusChanged NotificationType = "status_changed"
)

// Channel определяет канал доставки уведомления.
type Channel string

const (
	// ChannelHTTP — доставка через HTTP webhook.
	ChannelHTTP Channel = "http"
)

// Notification представляет уведомление, готовое к отправке через Notifier.
// В отличие от OutboxEntry (запись в БД с метаданными доставки), это чистые
// данные для передачи во внешнюю систему.
type Notification struct {
	UserID   int64                  `json:"userId"`
	TicketID int64                  `json:"ticketId"`
	Type     NotificationType       `json:"type"`
	Payload  map[string]interface{} `json:"payload"`
}

// Notifier отправляет уведомление во внешний канал (HTTP webhook, email,
// push и т.п. — конкретная реализация в infra/notifications).
type Notifier interface {
	Notify(ctx context.Context, notification Notification) error
}

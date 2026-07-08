package analytics

import "time"

// StatusCount — количество тикетов в конкретном статусе.
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// SLASummary — сводка по соблюдению SLA.
//
// Это приближённая метрика: ticket.proto не передаёт response/resolution
// deadlines через gRPC (они существуют только в домене основного сервиса и
// не выставлены наружу), поэтому нарушением здесь считается тикет, который
// не находится в терминальном статусе (resolved/closed/cancelled) и живёт
// дольше resolutionSLA — см. константу в aggregator.go.
type SLASummary struct {
	ResolvedCount  int64   `json:"resolved_count"`
	ViolatedCount  int64   `json:"violated_count"`
	ComplianceRate float64 `json:"compliance_rate"`
}

// Overview — сводная статистика по всем тикетам (GET /api/v1/overview).
type Overview struct {
	TotalTickets         int64         `json:"total_tickets"`
	ByStatus             []StatusCount `json:"by_status"`
	AvgResolutionMinutes float64       `json:"avg_resolution_minutes"`
	SLA                  SLASummary    `json:"sla"`
	GeneratedAt          time.Time     `json:"generated_at"`
}

// HistoryEntry — запись истории тикета в разрезе аналитики пользователя.
type HistoryEntry struct {
	Action    string    `json:"action"`
	OldValue  string    `json:"old_value,omitempty"`
	NewValue  string    `json:"new_value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UserStats — статистика по тикетам конкретного пользователя (GET /api/v1/users/:id).
type UserStats struct {
	UserID               int64          `json:"user_id"`
	TotalTickets         int64          `json:"total_tickets"`
	ByStatus             []StatusCount  `json:"by_status"`
	AvgResolutionMinutes float64        `json:"avg_resolution_minutes"`
	LastTicketAt         *time.Time     `json:"last_ticket_at,omitempty"`
	RecentActivity       []HistoryEntry `json:"recent_activity,omitempty"`
	GeneratedAt          time.Time      `json:"generated_at"`
}

// TopicStat — статистика по одной теме.
type TopicStat struct {
	TopicID      int64         `json:"topic_id"`
	Title        string        `json:"title"`
	TotalTickets int64         `json:"total_tickets"`
	ByStatus     []StatusCount `json:"by_status"`
}

// TopicsOverview — статистика по всем темам (GET /api/v1/topics).
type TopicsOverview struct {
	Topics      []TopicStat `json:"topics"`
	GeneratedAt time.Time   `json:"generated_at"`
}

// TimelinePoint — точка таймлайна за один календарный день (UTC).
type TimelinePoint struct {
	Date     string `json:"date"`
	Created  int64  `json:"created"`
	Resolved int64  `json:"resolved"`
}

// Timeline — таймлайн создания/решения тикетов за период (GET /api/v1/timeline).
type Timeline struct {
	Period      string          `json:"period"`
	Points      []TimelinePoint `json:"points"`
	GeneratedAt time.Time       `json:"generated_at"`
}

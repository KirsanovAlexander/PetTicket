package dto

import (
	"time"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
)

// TicketCreateRequest представляет HTTP запрос на создание тикета
type TicketCreateRequest struct {
	UserID  int64    `json:"userId" validate:"required"`
	TopicID int64    `json:"topicId" validate:"required"`
	Amount  *float64 `json:"amount,omitempty"`
	Comment string   `json:"comment" validate:"required"`
}

// TicketUpdateRequest представляет HTTP запрос на обновление тикета
type TicketUpdateRequest struct {
	StatusID *int    `json:"statusId,omitempty"`
	Comment  *string `json:"comment,omitempty"`
}

// TicketResponse представляет HTTP ответ с данными тикета
type TicketResponse struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"userId"`
	TopicID    int64     `json:"topicId"`
	TopicName  string    `json:"topicName,omitempty"`
	StatusID     int       `json:"statusId"`
	StatusName   string    `json:"statusName,omitempty"`
	PriorityID   int       `json:"priorityId"`
	PriorityName string    `json:"priorityName,omitempty"`
	Amount       *float64  `json:"amount,omitempty"`
	Comment          string    `json:"comment"`
	ResponseDeadline time.Time `json:"responseDeadline,omitempty"`
	ResolutionDeadline time.Time `json:"resolutionDeadline,omitempty"`
	FirstResponseAt  *time.Time `json:"firstResponseAt,omitempty"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	SLAStatus        string    `json:"slaStatus,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// SLAMetricsResponse SLA-метрики для HTTP-ответа
type SLAMetricsResponse struct {
	ResponseStatus   string `json:"responseStatus"`
	ResolutionStatus string `json:"resolutionStatus"`
	OverallStatus    string `json:"overallStatus"`
}

// TicketWithSLAResponse расширенный ответ с SLA-полями
type TicketWithSLAResponse struct {
	TicketResponse
	SLAMetrics SLAMetricsResponse `json:"slaMetrics"`
}

// TicketHistoryResponse представляет HTTP ответ с записью истории тикета
type TicketHistoryResponse struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticketId"`
	UserID    int64     `json:"userId"`
	Action    string    `json:"action"`
	OldValue  string    `json:"oldValue,omitempty"`
	NewValue  string    `json:"newValue,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// StatusResponse представляет HTTP ответ со статусом
type StatusResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TopicResponse представляет HTTP ответ с темой
type TopicResponse struct {
	ID          int64  `json:"id"`
	ExternalID  int64  `json:"externalId"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ListResponse представляет HTTP ответ со списком данных
type ListResponse struct {
	Data  interface{} `json:"data"`
	Total int64       `json:"total"`
}

// ErrorResponse представляет HTTP ответ с ошибкой
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail содержит детали ошибки
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// ToTicketResponse конвертирует доменную модель в DTO
func ToTicketResponse(ticket domain.Ticket) TicketResponse {
	now := time.Now()
	sla := ticket.GetSLAStatus(now)

	return TicketResponse{
		ID:                 ticket.ID,
		UserID:             ticket.UserID,
		TopicID:            ticket.TopicID,
		StatusID:           int(ticket.Status),
		StatusName:         ticket.Status.String(),
		PriorityID:         int(ticket.Priority),
		PriorityName:       ticket.Priority.String(),
		Amount:             ticket.Amount,
		Comment:            ticket.Comment,
		ResponseDeadline:   ticket.ResponseDeadline,
		ResolutionDeadline: ticket.ResolutionDeadline,
		FirstResponseAt:    ticket.FirstResponseAt,
		ResolvedAt:         ticket.ResolvedAt,
		SLAStatus:          string(sla.OverallStatus),
		CreatedAt:          ticket.CreatedAt,
		UpdatedAt:          ticket.UpdatedAt,
	}
}

func toSLAMetricsResponse(metrics domain.SLAMetrics) SLAMetricsResponse {
	return SLAMetricsResponse{
		ResponseStatus:   string(metrics.ResponseStatus),
		ResolutionStatus: string(metrics.ResolutionStatus),
		OverallStatus:    string(metrics.OverallStatus),
	}
}

// ToTicketWithSLAResponse конвертирует тикет в ответ с SLA-метриками
func ToTicketWithSLAResponse(ticket domain.Ticket) TicketWithSLAResponse {
	return TicketWithSLAResponse{
		TicketResponse: ToTicketResponse(ticket),
		SLAMetrics:     toSLAMetricsResponse(ticket.GetSLAStatus(time.Now())),
	}
}

// ToTicketWithSLAResponseList конвертирует список тикетов
func ToTicketWithSLAResponseList(ticketList []domain.Ticket) []TicketWithSLAResponse {
	result := make([]TicketWithSLAResponse, len(ticketList))
	for i, ticket := range ticketList {
		result[i] = ToTicketWithSLAResponse(ticket)
	}
	return result
}

// ToTicketHistoryResponse конвертирует доменную модель истории в DTO
func ToTicketHistoryResponse(history domain.History) TicketHistoryResponse {
	return TicketHistoryResponse{
		ID:        history.ID,
		TicketID:  history.TicketID,
		UserID:    history.UserID,
		Action:    string(history.Action),
		OldValue:  history.OldValue,
		NewValue:  history.NewValue,
		CreatedAt: history.CreatedAt,
	}
}

// ToTicketResponseList конвертирует список доменных моделей в DTO
func ToTicketResponseList(ticketList []domain.Ticket) []TicketResponse {
	result := make([]TicketResponse, len(ticketList))
	for i, ticket := range ticketList {
		result[i] = ToTicketResponse(ticket)
	}
	return result
}

// ToTicketHistoryResponseList конвертирует список истории в DTO
func ToTicketHistoryResponseList(historyList []domain.History) []TicketHistoryResponse {
	result := make([]TicketHistoryResponse, len(historyList))
	for i, history := range historyList {
		result[i] = ToTicketHistoryResponse(history)
	}
	return result
}

// ToStatusResponse конвертирует StatusInfo в DTO
func ToStatusResponse(status tickets.StatusInfo) StatusResponse {
	return StatusResponse{
		ID:   status.ID,
		Name: status.Name,
	}
}

// ToStatusResponseList конвертирует список статусов в DTO
func ToStatusResponseList(statuses []tickets.StatusInfo) []StatusResponse {
	result := make([]StatusResponse, len(statuses))
	for i, status := range statuses {
		result[i] = ToStatusResponse(status)
	}
	return result
}

// ToTopicResponse конвертирует Topic в DTO
func ToTopicResponse(topic domain.Topic) TopicResponse {
	return TopicResponse{
		ID:          topic.ID,
		ExternalID:  topic.ExternalID,
		Title:       topic.Title,
		Description: topic.Description,
	}
}

// ToTopicResponseList конвертирует список тем в DTO
func ToTopicResponseList(topics []domain.Topic) []TopicResponse {
	result := make([]TopicResponse, len(topics))
	for i, topic := range topics {
		result[i] = ToTopicResponse(topic)
	}
	return result
}

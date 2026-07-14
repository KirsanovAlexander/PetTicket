// Package v2 содержит DTO и мапперы для v2 API: тикет отдаётся с
// раскрытыми вложенными объектами (user, status, topic, comments, sla)
// вместо плоских *_id полей v1.
package v2

import (
	"time"

	domain "pet-ticket/internal/domain/tickets"
)

// UserResponse — ссылка на пользователя. В системе нет отдельного сервиса
// профилей (см. domain.User), поэтому кроме id ничего нет.
type UserResponse struct {
	ID int64 `json:"id"`
}

// StatusResponse статус тикета с человекочитаемым названием.
type StatusResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// TopicResponse тема тикета.
type TopicResponse struct {
	ID          int64  `json:"id"`
	ExternalID  int64  `json:"externalId"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CommentResponse комментарий к тикету.
type CommentResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

// SLAResponse SLA-срез тикета.
type SLAResponse struct {
	ResponseDeadline   time.Time `json:"responseDeadline"`
	ResolutionDeadline time.Time `json:"resolutionDeadline"`
	ResponseStatus     string    `json:"responseStatus"`
	ResolutionStatus   string    `json:"resolutionStatus"`
	OverallStatus      string    `json:"overallStatus"`
}

// TicketResponse тикет со всеми связями, раскрытыми во вложенные объекты.
type TicketResponse struct {
	ID        int64             `json:"id"`
	User      UserResponse      `json:"user"`
	Status    StatusResponse    `json:"status"`
	Topic     TopicResponse     `json:"topic"`
	Assignee  *UserResponse     `json:"assignee,omitempty"`
	Priority  string            `json:"priority"`
	Amount    *float64          `json:"amount,omitempty"`
	Comment   string            `json:"comment"`
	Comments  []CommentResponse `json:"comments"`
	SLA       *SLAResponse      `json:"sla,omitempty"`
	Version   int               `json:"version"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// PaginationResponse offset-пагинация v2 (ListFull курсор не поддерживает,
// см. app/tickets/service.go — сознательный выбор объёма для Task 9).
type PaginationResponse struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// ListResponse список тикетов v2 с offset-пагинацией.
type ListResponse struct {
	Data       []TicketResponse   `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}

// MapTicketFullToResponse конвертирует доменную модель в v2 DTO.
func MapTicketFullToResponse(t domain.TicketFull) TicketResponse {
	resp := TicketResponse{
		ID:   t.ID,
		User: UserResponse{ID: t.User.ID},
		Status: StatusResponse{
			ID:          t.Status.ID,
			Name:        t.Status.Name,
			DisplayName: t.Status.DisplayName,
		},
		Topic: TopicResponse{
			ID:          t.Topic.ID,
			ExternalID:  t.Topic.ExternalID,
			Title:       t.Topic.Title,
			Description: t.Topic.Description,
		},
		Priority:  t.Priority.String(),
		Amount:    t.Amount,
		Comment:   t.Comment,
		Comments:  mapComments(t.Comments),
		Version:   t.Version,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	if t.Assignee != nil {
		resp.Assignee = &UserResponse{ID: t.Assignee.ID}
	}

	if t.SLA != nil {
		resp.SLA = &SLAResponse{
			ResponseDeadline:   t.SLA.ResponseDeadline,
			ResolutionDeadline: t.SLA.ResolutionDeadline,
			ResponseStatus:     string(t.SLA.ResponseStatus),
			ResolutionStatus:   string(t.SLA.ResolutionStatus),
			OverallStatus:      string(t.SLA.OverallStatus),
		}
	}

	return resp
}

// MapTicketFullListToResponse конвертирует список доменных моделей в v2 DTO.
func MapTicketFullListToResponse(list []domain.TicketFull) []TicketResponse {
	result := make([]TicketResponse, len(list))
	for i, t := range list {
		result[i] = MapTicketFullToResponse(t)
	}
	return result
}

func mapComments(comments []domain.Comment) []CommentResponse {
	result := make([]CommentResponse, len(comments))
	for i, c := range comments {
		result[i] = CommentResponse{
			ID:        c.ID,
			UserID:    c.UserID,
			Text:      c.Text,
			CreatedAt: c.CreatedAt,
		}
	}
	return result
}

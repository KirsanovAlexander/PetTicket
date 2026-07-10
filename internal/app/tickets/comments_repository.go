package tickets

import (
	"context"

	"pet-ticket/internal/domain/tickets"
)

// CommentsRepository — хранилище комментариев тикетов (таблица
// ticket_comments).
type CommentsRepository interface {
	Create(ctx context.Context, input tickets.AddCommentInput) (tickets.TicketComment, error)
	GetByTicketID(ctx context.Context, filter tickets.ListCommentsFilter) ([]tickets.TicketComment, error)
	GetLastByTicketID(ctx context.Context, ticketID int64) (*tickets.TicketComment, error)
	Update(ctx context.Context, input tickets.UpdateCommentInput) error
	Delete(ctx context.Context, id int64) error
}

package tickets

import (
	"context"
	"fmt"
	"time"

	"pet-ticket/internal/domain/tickets"

	"github.com/rs/zerolog"
)

// contextKey - приватный тип для ключей контекста (избегаем конфликтов)
type contextKey string

const (
	// txContextKey - ключ для хранения транзакции в контексте
	txContextKey contextKey = "tx"
)

// CreateTicketInput представляет входные данные для создания тикета
type CreateTicketInput struct {
	UserID   int64
	TopicID  int64
	Priority *tickets.Priority
	Amount   *float64
	Comment  string
}

// UpdateTicketInput представляет входные данные для обновления тикета
type UpdateTicketInput struct {
	ID      int64
	Status  *tickets.Status
	Comment *string
}

// ListTicketsInput представляет входные данные для получения списка тикетов
type ListTicketsInput struct {
	UserID   *int64
	TopicID  *int64
	Status   *tickets.Status
	Priority *tickets.Priority
	Limit    int
	Offset   int
	SortBy   string
	SortDesc bool
}

// AddCommentInput входные данные для добавления комментария
type AddCommentInput struct {
	TicketID         int64
	UserID           int64
	Comment          string
	IsSupportComment bool
}

// CloseTicketInput входные данные для закрытия тикета
type CloseTicketInput struct {
	TicketID int64
	UserID   int64
	Reason   string
}

// Service определяет бизнес-логику работы с тикетами
type Service interface {
	CreateTicket(ctx context.Context, input CreateTicketInput) (tickets.Ticket, error)
	GetTicket(ctx context.Context, id int64) (tickets.Ticket, error)
	UpdateTicket(ctx context.Context, input UpdateTicketInput) (tickets.Ticket, error)
	DeleteTicket(ctx context.Context, id int64) error
	ListTickets(ctx context.Context, input ListTicketsInput) ([]tickets.Ticket, error)
	GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int) ([]tickets.History, error)
	GetAllStatuses(ctx context.Context) ([]StatusInfo, error)
	GetAllTopics(ctx context.Context) ([]tickets.Topic, error)
	UpdatePriority(ctx context.Context, ticketID int64, priority tickets.Priority, userID int64) (tickets.Ticket, error)
	EscalateTicket(ctx context.Context, ticketID int64, userID int64) (tickets.Ticket, error)
	AddComment(ctx context.Context, input AddCommentInput) (tickets.Ticket, error)
	GetSLAViolations(ctx context.Context) ([]tickets.Ticket, error)
	CloseTicket(ctx context.Context, input CloseTicketInput) (tickets.Ticket, error)
}

// service реализует интерфейс Service
type service struct {
	repo    Repository
	db      TxBeginner
	logger  zerolog.Logger
	slaCalc *SLACalculator
}

// TxBeginner интерфейс для создания транзакций
type TxBeginner interface {
	BeginTx(context.Context) (TxCommitter, error)
}

// TxCommitter интерфейс для работы с транзакциями
type TxCommitter interface {
	Commit() error
	Rollback() error
}

// NewService создаёт новый экземпляр сервиса
func NewService(repo Repository, db TxBeginner, logger zerolog.Logger) Service {
	return &service{
		repo:    repo,
		db:      db,
		logger:  logger,
		slaCalc: NewSLACalculator(repo),
	}
}

// CreateTicket создаёт новый тикет
func (s *service) CreateTicket(ctx context.Context, input CreateTicketInput) (tickets.Ticket, error) {
	priority := tickets.PriorityMedium
	if input.Priority != nil {
		priority = *input.Priority
	}

	// Создаём доменную сущность
	ticket := tickets.Ticket{
		UserID:   input.UserID,
		TopicID:  input.TopicID,
		Status:   tickets.StatusNew,
		Priority: priority,
		Amount:   input.Amount,
		Comment:  input.Comment,
	}

	// Валидация доменной модели
	if err := ticket.Validate(); err != nil {
		s.logger.Warn().Err(err).Msg("invalid ticket data")
		return tickets.Ticket{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	now := time.Now()
	responseDeadline, resolutionDeadline, err := s.slaCalc.CalculateDeadlines(
		ctx, input.TopicID, int64(priority), now,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to calculate sla deadlines")
		return tickets.Ticket{}, fmt.Errorf("failed to calculate sla deadlines: %w", err)
	}
	ticket.ResponseDeadline = responseDeadline
	ticket.ResolutionDeadline = resolutionDeadline

	// Начинаем транзакцию
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to begin transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("failed to rollback transaction")
			}
		}
	}()

	// Контекст с транзакцией
	txCtx := context.WithValue(ctx, txContextKey, tx)

	// Сохранение в репозиторий
	created, err := s.repo.Create(txCtx, ticket)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to create ticket")
		return tickets.Ticket{}, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Добавление записи в историю
	history := tickets.History{
		TicketID: created.ID,
		UserID:   input.UserID,
		Action:   tickets.ActionCreated,
		NewValue: fmt.Sprintf("status=%s", created.Status.String()),
	}
	if err = s.repo.AddHistory(txCtx, history); err != nil {
		s.logger.Error().Err(err).Int64("ticket_id", created.ID).Msg("failed to add history")
		return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
	}

	// Коммит транзакции
	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("failed to commit transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Int64("ticket_id", created.ID).Msg("ticket created")
	return created, nil
}

// GetTicket возвращает тикет по ID
func (s *service) GetTicket(ctx context.Context, id int64) (tickets.Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", id).Msg("failed to get ticket")
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	return ticket, nil
}

// UpdateTicket обновляет существующий тикет
func (s *service) UpdateTicket(ctx context.Context, input UpdateTicketInput) (tickets.Ticket, error) {
	// Получаем существующий тикет
	existing, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", input.ID).Msg("failed to get ticket for update")
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	oldStatus := existing.Status
	oldComment := existing.Comment

	// Обновляем поля
	if input.Status != nil {
		if !input.Status.IsValid() {
			return tickets.Ticket{}, ErrInvalidStatus
		}
		existing.Status = *input.Status
	}

	if input.Comment != nil {
		existing.Comment = *input.Comment
	}

	now := time.Now()
	setResolved := false
	if input.Status != nil && s.slaCalc.ShouldSetResolvedAt(oldStatus, existing.Status) {
		existing.ResolvedAt = &now
		setResolved = true
	}

	// Валидация
	if err := existing.Validate(); err != nil {
		s.logger.Warn().Err(err).Msg("invalid ticket data after update")
		return tickets.Ticket{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	// Начинаем транзакцию
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to begin transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("failed to rollback transaction")
			}
		}
	}()

	// Контекст с транзакцией
	txCtx := context.WithValue(ctx, txContextKey, tx)

	// Сохранение
	updated, err := s.repo.Update(txCtx, existing)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", input.ID).Msg("failed to update ticket")
		return tickets.Ticket{}, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Добавление записи в историю при изменении статуса
	if input.Status != nil && oldStatus != *input.Status {
		history := tickets.History{
			TicketID: updated.ID,
			UserID:   updated.UserID,
			Action:   tickets.ActionStatusChanged,
			OldValue: oldStatus.String(),
			NewValue: input.Status.String(),
		}
		if err = s.repo.AddHistory(txCtx, history); err != nil {
			s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to add history")
			return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
		}
	}

	if setResolved {
		history := tickets.History{
			TicketID: updated.ID,
			UserID:   updated.UserID,
			Action:   tickets.ActionResolved,
			NewValue: updated.Status.String(),
		}
		if err = s.repo.AddHistory(txCtx, history); err != nil {
			s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to add resolved history")
			return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
		}
	}

	// Добавление записи в историю при изменении комментария
	if input.Comment != nil && oldComment != *input.Comment {
		history := tickets.History{
			TicketID: updated.ID,
			UserID:   updated.UserID,
			Action:   tickets.ActionCommentUpdated,
			OldValue: oldComment,
			NewValue: *input.Comment,
		}
		if err = s.repo.AddHistory(txCtx, history); err != nil {
			s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to add history for comment")
			return tickets.Ticket{}, fmt.Errorf("failed to add history for comment: %w", err)
		}
	}

	// Коммит транзакции
	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("failed to commit transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Int64("ticket_id", updated.ID).Msg("ticket updated")
	return updated, nil
}

// DeleteTicket удаляет тикет
func (s *service) DeleteTicket(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error().Err(err).Int64("id", id).Msg("failed to delete ticket")
		return fmt.Errorf("failed to delete ticket: %w", err)
	}

	s.logger.Info().Int64("ticket_id", id).Msg("ticket deleted")
	return nil
}

// ListTickets возвращает список тикетов
func (s *service) ListTickets(ctx context.Context, input ListTicketsInput) ([]tickets.Ticket, error) {
	filter := ListFilter(input)

	list, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list tickets")
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return list, nil
}

// GetTicketHistory возвращает историю изменений тикета
func (s *service) GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int) ([]tickets.History, error) {
	history, err := s.repo.GetHistory(ctx, ticketID, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Int64("ticket_id", ticketID).Msg("failed to get ticket history")
		return nil, fmt.Errorf("failed to get ticket history: %w", err)
	}

	return history, nil
}

// GetAllStatuses возвращает все статусы тикетов
func (s *service) GetAllStatuses(ctx context.Context) ([]StatusInfo, error) {
	statuses, err := s.repo.GetAllStatuses(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get statuses")
		return nil, fmt.Errorf("failed to get statuses: %w", err)
	}

	return statuses, nil
}

// GetAllTopics возвращает все темы тикетов
func (s *service) GetAllTopics(ctx context.Context) ([]tickets.Topic, error) {
	topics, err := s.repo.GetAllTopics(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get topics")
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}

	return topics, nil
}

// UpdatePriority изменяет приоритет тикета и записывает историю
func (s *service) UpdatePriority(ctx context.Context, ticketID int64, priority tickets.Priority, userID int64) (tickets.Ticket, error) {
	return s.changePriority(ctx, ticketID, priority, userID, tickets.ActionPriorityChanged)
}

// EscalateTicket повышает приоритет тикета на один уровень
func (s *service) EscalateTicket(ctx context.Context, ticketID int64, userID int64) (tickets.Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", ticketID).Msg("failed to get ticket for escalation")
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	newPriority := ticket.Priority.Escalate()
	if newPriority == ticket.Priority {
		return ticket, nil
	}

	return s.changePriority(ctx, ticketID, newPriority, userID, tickets.ActionEscalated)
}

func (s *service) changePriority(
	ctx context.Context,
	ticketID int64,
	priority tickets.Priority,
	userID int64,
	action tickets.HistoryAction,
) (tickets.Ticket, error) {
	if !priority.IsValid() {
		return tickets.Ticket{}, ErrInvalidPriority
	}

	existing, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", ticketID).Msg("failed to get ticket for priority update")
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	oldPriority := existing.Priority
	if oldPriority == priority {
		return existing, nil
	}

	existing.Priority = priority

	if err := existing.Validate(); err != nil {
		s.logger.Warn().Err(err).Msg("invalid ticket data after priority update")
		return tickets.Ticket{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to begin transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("failed to rollback transaction")
			}
		}
	}()

	txCtx := context.WithValue(ctx, txContextKey, tx)

	updated, err := s.repo.Update(txCtx, existing)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", ticketID).Msg("failed to update ticket priority")
		return tickets.Ticket{}, fmt.Errorf("failed to update ticket: %w", err)
	}

	history := tickets.History{
		TicketID: updated.ID,
		UserID:   userID,
		Action:   action,
		OldValue: oldPriority.String(),
		NewValue: priority.String(),
	}
	if err = s.repo.AddHistory(txCtx, history); err != nil {
		s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to add priority history")
		return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("failed to commit transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Int64("ticket_id", updated.ID).Str("priority", priority.String()).Msg("ticket priority updated")
	return updated, nil
}

// AddComment добавляет комментарий и при необходимости фиксирует first_response_at
func (s *service) AddComment(ctx context.Context, input AddCommentInput) (tickets.Ticket, error) {
	if input.Comment == "" {
		return tickets.Ticket{}, ErrInvalidInput
	}

	existing, err := s.repo.GetByID(ctx, input.TicketID)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", input.TicketID).Msg("failed to get ticket for comment")
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	setFirstResponse := s.slaCalc.ShouldSetFirstResponse(existing, input.IsSupportComment)
	now := time.Now()
	if setFirstResponse {
		existing.FirstResponseAt = &now
	}
	existing.Comment = input.Comment

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to begin transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("failed to rollback transaction")
			}
		}
	}()

	txCtx := context.WithValue(ctx, txContextKey, tx)

	if !input.IsSupportComment {
		if err := s.repo.UpdateLastUserActivity(txCtx, existing.ID); err != nil {
			s.logger.Warn().Err(err).Int64("ticket_id", existing.ID).Msg("failed to update user activity")
		}
	}

	updated, err := s.repo.Update(txCtx, existing)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", input.TicketID).Msg("failed to update ticket comment")
		return tickets.Ticket{}, fmt.Errorf("failed to update ticket: %w", err)
	}

	history := tickets.History{
		TicketID: updated.ID,
		UserID:   input.UserID,
		Action:   tickets.ActionCommentUpdated,
		NewValue: input.Comment,
	}
	if err = s.repo.AddHistory(txCtx, history); err != nil {
		s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to add comment history")
		return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
	}

	if setFirstResponse {
		firstResponseHistory := tickets.History{
			TicketID: updated.ID,
			UserID:   input.UserID,
			Action:   tickets.ActionFirstResponse,
			NewValue: now.Format(time.RFC3339),
		}
		if err = s.repo.AddHistory(txCtx, firstResponseHistory); err != nil {
			s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to add first response history")
			return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("failed to commit transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return updated, nil
}

// GetSLAViolations возвращает тикеты с нарушенным SLA
func (s *service) GetSLAViolations(ctx context.Context) ([]tickets.Ticket, error) {
	list, err := s.repo.FindSLAViolations(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to find sla violations")
		return nil, fmt.Errorf("failed to find sla violations: %w", err)
	}
	return list, nil
}

// CloseTicket закрывает resolved тикет и записывает историю
func (s *service) CloseTicket(ctx context.Context, input CloseTicketInput) (tickets.Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, input.TicketID)
	if err != nil {
		s.logger.Error().Err(err).Int64("ticket_id", input.TicketID).Msg("failed to get ticket")
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	if ticket.Status != tickets.StatusResolved {
		return tickets.Ticket{}, fmt.Errorf("can only close resolved tickets, current status: %s", ticket.Status)
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return tickets.Ticket{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("failed to rollback transaction")
			}
		}
	}()
	txCtx := context.WithValue(ctx, txContextKey, tx)

	oldStatus := ticket.Status
	ticket.Status = tickets.StatusClosed

	updated, err := s.repo.Update(txCtx, ticket)
	if err != nil {
		return tickets.Ticket{}, fmt.Errorf("failed to update ticket: %w", err)
	}

	history := tickets.History{
		TicketID: ticket.ID,
		UserID:   input.UserID,
		Action:   tickets.ActionAutoClosed,
		OldValue: oldStatus.String(),
		NewValue: tickets.StatusClosed.String(),
	}
	if err = s.repo.AddHistory(txCtx, history); err != nil {
		return tickets.Ticket{}, fmt.Errorf("failed to add history: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().
		Int64("ticket_id", ticket.ID).
		Str("reason", input.Reason).
		Int64("user_id", input.UserID).
		Msg("ticket closed")

	return updated, nil
}

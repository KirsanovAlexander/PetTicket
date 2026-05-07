package tickets

import (
	"context"
	"fmt"

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
	UserID  int64
	TopicID int64
	Amount  *float64
	Comment string
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
	Limit    int
	Offset   int
	SortBy   string
	SortDesc bool
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
}

// service реализует интерфейс Service
type service struct {
	repo   Repository
	db     TxBeginner
	logger zerolog.Logger
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
		repo:   repo,
		db:     db,
		logger: logger,
	}
}

// CreateTicket создаёт новый тикет
func (s *service) CreateTicket(ctx context.Context, input CreateTicketInput) (tickets.Ticket, error) {
	// Создаём доменную сущность
	ticket := tickets.Ticket{
		UserID:  input.UserID,
		TopicID: input.TopicID,
		Status:  tickets.StatusNew,
		Amount:  input.Amount,
		Comment: input.Comment,
	}

	// Валидация доменной модели
	if err := ticket.Validate(); err != nil {
		s.logger.Warn().Err(err).Msg("invalid ticket data")
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

	// Добавление записи в историю при изменении комментария
	if input.Comment != nil {
		history := tickets.History{
			TicketID: updated.ID,
			UserID:   updated.UserID,
			Action:   tickets.ActionCommentAdded,
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

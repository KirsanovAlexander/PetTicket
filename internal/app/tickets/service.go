package tickets

import (
	"context"
	"fmt"
	"time"

	domainEvents "pet-ticket/internal/domain/events"
	"pet-ticket/internal/domain/notifications"
	"pet-ticket/internal/domain/tickets"
	infraEvents "pet-ticket/internal/infra/events"

	"github.com/rs/zerolog"
)

// contextKey - приватный тип для ключей контекста (избегаем конфликтов)
type contextKey string

const (
	// TxContextKey - ключ для хранения транзакции в контексте. Экспортирован
	// (не просто txContextKey), потому что infra/postgres читает его же
	// значение из ctx.Value в getExecutor — если бы каждый пакет объявлял
	// свой собственный приватный contextKey с тем же именем "tx", это были
	// бы РАЗНЫЕ типы в терминах Go, ctx.Value() никогда бы не находил
	// сохранённую транзакцию, и все "транзакционные" операции сервиса
	// молча выполнялись бы через пул соединений в обход tx.
	TxContextKey contextKey = "tx"
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

// ListTicketsWithCursorInput входные данные для cursor-пагинации списка тикетов
type ListTicketsWithCursorInput struct {
	UserID    *int64
	TopicID   *int64
	Status    *tickets.Status
	Priority  *tickets.Priority
	Cursor    *string
	PageSize  int
	Direction string
}

// CursorPage представляет одну страницу cursor-пагинации
type CursorPage struct {
	Items      []tickets.Ticket
	NextCursor string
	HasMore    bool
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

// AssignTicketInput входные данные для назначения тикета оператору
type AssignTicketInput struct {
	TicketID   int64
	OperatorID int64
	AssignedBy int64
	Comment    string
}

// Service определяет бизнес-логику работы с тикетами
type Service interface {
	CreateTicket(ctx context.Context, input CreateTicketInput) (tickets.Ticket, error)
	GetTicket(ctx context.Context, id int64) (tickets.Ticket, error)
	UpdateTicket(ctx context.Context, input UpdateTicketInput) (tickets.Ticket, error)
	DeleteTicket(ctx context.Context, id int64) error
	ListTickets(ctx context.Context, input ListTicketsInput) ([]tickets.Ticket, error)
	ListTicketsWithCursor(ctx context.Context, input ListTicketsWithCursorInput) (CursorPage, error)
	GetTicketFull(ctx context.Context, id int64) (tickets.TicketFull, error)
	ListTicketsFull(ctx context.Context, input ListTicketsInput) ([]tickets.TicketFull, error)
	GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int) ([]tickets.History, error)
	GetAllStatuses(ctx context.Context) ([]StatusInfo, error)
	GetAllTopics(ctx context.Context) ([]tickets.Topic, error)
	UpdatePriority(ctx context.Context, ticketID int64, priority tickets.Priority, userID int64) (tickets.Ticket, error)
	EscalateTicket(ctx context.Context, ticketID int64, userID int64) (tickets.Ticket, error)
	AddComment(ctx context.Context, input AddCommentInput) (tickets.Ticket, error)
	GetSLAViolations(ctx context.Context) ([]tickets.Ticket, error)
	CloseTicket(ctx context.Context, input CloseTicketInput) (tickets.Ticket, error)
	AssignTicket(ctx context.Context, input AssignTicketInput) (tickets.Ticket, error)
}

// service реализует интерфейс Service
type service struct {
	repo       Repository
	db         TxBeginner
	logger     zerolog.Logger
	slaCalc    *SLACalculator
	eventBus   infraEvents.Bus
	outboxRepo notifications.OutboxRepository
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

// NewService создаёт новый экземпляр сервиса. eventBus и outboxRepo могут
// быть nil — сервис остаётся полностью рабочим, просто не публикует события
// и/или не создаёт outbox-записи (удобно для юнит-тестов и для флоу вроде
// auto-closer, которым уведомления не нужны).
func NewService(
	repo Repository, db TxBeginner, logger zerolog.Logger,
	eventBus infraEvents.Bus, outboxRepo notifications.OutboxRepository,
) Service {
	return &service{
		repo:       repo,
		db:         db,
		logger:     logger,
		slaCalc:    NewSLACalculator(repo),
		eventBus:   eventBus,
		outboxRepo: outboxRepo,
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
	txCtx := context.WithValue(ctx, TxContextKey, tx)

	// Сохранение в репозиторий
	created, err := s.repo.Create(txCtx, ticket)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to create ticket")
		return tickets.Ticket{}, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Коммит транзакции. История больше не пишется здесь напрямую — это
	// делает HistoryHandler, подписанный на ticket.created (событие
	// публикуется ниже, уже ПОСЛЕ коммита).
	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("failed to commit transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if s.eventBus != nil {
		_ = s.eventBus.Publish(ctx, domainEvents.TicketCreated{
			BaseEvent: domainEvents.NewBaseEvent(),
			TicketID:  created.ID,
			UserID:    created.UserID,
			TopicID:   created.TopicID,
			Status:    created.Status.String(),
		})
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
	statusChanged := input.Status != nil && oldStatus != *input.Status

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
	txCtx := context.WithValue(ctx, TxContextKey, tx)

	// Сохранение
	updated, err := s.repo.Update(txCtx, existing)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", input.ID).Msg("failed to update ticket")
		return tickets.Ticket{}, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Outbox-запись создаётся ВНУТРИ той же транзакции (тем же txCtx), что и
	// обновление тикета — transactional outbox: если тикет не закоммитился,
	// уведомление тоже не должно уйти в очередь на отправку, и наоборот.
	if statusChanged && s.outboxRepo != nil {
		outboxEntry := notifications.OutboxEntry{
			UserID:      updated.UserID,
			TicketID:    updated.ID,
			Type:        notifications.NotifStatusChanged,
			MaxAttempts: 5,
			NextRetryAt: time.Now(),
			Payload: map[string]interface{}{
				"title":   "Статус тикета изменен",
				"message": fmt.Sprintf("Ваш тикет #%d: %s -> %s", updated.ID, oldStatus, updated.Status),
			},
		}
		if err = s.outboxRepo.Create(txCtx, outboxEntry); err != nil {
			s.logger.Error().Err(err).Int64("ticket_id", updated.ID).Msg("failed to create outbox entry")
			return tickets.Ticket{}, fmt.Errorf("failed to create outbox entry: %w", err)
		}
	}

	// Коммит транзакции. История статуса/резолва/комментария больше не
	// пишется здесь напрямую — она уходит в HistoryHandler через события
	// ниже, публикуемые уже ПОСЛЕ коммита.
	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("failed to commit transaction")
		return tickets.Ticket{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if s.eventBus != nil {
		if input.Status != nil && oldStatus != *input.Status {
			_ = s.eventBus.Publish(ctx, domainEvents.TicketStatusChanged{
				BaseEvent: domainEvents.NewBaseEvent(),
				TicketID:  updated.ID,
				OldStatus: oldStatus.String(),
				NewStatus: input.Status.String(),
				ChangedBy: updated.UserID,
				Resolved:  setResolved,
			})
		}

		if input.Comment != nil {
			_ = s.eventBus.Publish(ctx, domainEvents.TicketCommentAdded{
				BaseEvent:  domainEvents.NewBaseEvent(),
				TicketID:   updated.ID,
				UserID:     updated.UserID,
				OldComment: oldComment,
				NewComment: *input.Comment,
			})
		}
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
	filter := ListFilter{
		UserID:   input.UserID,
		TopicID:  input.TopicID,
		Status:   input.Status,
		Priority: input.Priority,
		Limit:    input.Limit,
		Offset:   input.Offset,
		SortBy:   input.SortBy,
		SortDesc: input.SortDesc,
	}

	list, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list tickets")
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return list, nil
}

// ListTicketsWithCursor возвращает страницу тикетов через cursor-пагинацию.
// direction по умолчанию "next" (движение вглубь истории), pageSize по
// умолчанию DefaultCursorPageSize, ограничен MaxCursorPageSize.
func (s *service) ListTicketsWithCursor(ctx context.Context, input ListTicketsWithCursorInput) (CursorPage, error) {
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = DefaultCursorPageSize
	}
	if pageSize > MaxCursorPageSize {
		pageSize = MaxCursorPageSize
	}

	direction := input.Direction
	if direction != "prev" {
		direction = "next"
	}

	filter := ListFilter{
		UserID:    input.UserID,
		TopicID:   input.TopicID,
		Status:    input.Status,
		Priority:  input.Priority,
		Cursor:    input.Cursor,
		PageSize:  pageSize,
		Direction: direction,
	}

	items, hasMore, err := s.repo.ListWithCursor(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list tickets with cursor")
		return CursorPage{}, fmt.Errorf("failed to list tickets with cursor: %w", err)
	}

	var nextCursor string
	if hasMore && len(items) > 0 {
		// Для "next" продолжаем от последней (самой старой) записи страницы.
		// Для "prev" после разворота результата в дисплейный порядок (DESC)
		// самая новая запись — первая, от неё и продолжаем движение назад.
		anchor := items[len(items)-1]
		if direction == "prev" {
			anchor = items[0]
		}
		nextCursor = EncodeCursor(anchor.CreatedAt, anchor.ID)
	}

	return CursorPage{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// GetTicketFull возвращает тикет со всеми связями, раскрытыми во вложенные
// объекты (v2 API).
func (s *service) GetTicketFull(ctx context.Context, id int64) (tickets.TicketFull, error) {
	full, err := s.repo.GetFullByID(ctx, id)
	if err != nil {
		s.logger.Error().Err(err).Int64("id", id).Msg("failed to get full ticket")
		return tickets.TicketFull{}, fmt.Errorf("failed to get full ticket: %w", err)
	}

	return full, nil
}

// ListTicketsFull возвращает список тикетов с раскрытыми статусом/темой
// (v2 API). Использует те же фильтры и offset-пагинацию, что и ListTickets.
func (s *service) ListTicketsFull(ctx context.Context, input ListTicketsInput) ([]tickets.TicketFull, error) {
	filter := ListFilter{
		UserID:   input.UserID,
		TopicID:  input.TopicID,
		Status:   input.Status,
		Priority: input.Priority,
		Limit:    input.Limit,
		Offset:   input.Offset,
		SortBy:   input.SortBy,
		SortDesc: input.SortDesc,
	}

	list, err := s.repo.ListFull(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list full tickets")
		return nil, fmt.Errorf("failed to list full tickets: %w", err)
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

	txCtx := context.WithValue(ctx, TxContextKey, tx)

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

	txCtx := context.WithValue(ctx, TxContextKey, tx)

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
		Action:   tickets.ActionCommentAdded,
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
	txCtx := context.WithValue(ctx, TxContextKey, tx)

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

// AssignTicket назначает новый ("new") тикет на оператора: переводит его в
// статус in_progress через UpdateTicket (это уже публикует
// ticket.status_changed/ticket.comment_added по обычным правилам) и
// дополнительно публикует ticket.assigned с ID оператора — для истории
// назначения, которую события статуса сами по себе не несут.
func (s *service) AssignTicket(ctx context.Context, input AssignTicketInput) (tickets.Ticket, error) {
	existing, err := s.repo.GetByID(ctx, input.TicketID)
	if err != nil {
		return tickets.Ticket{}, fmt.Errorf("failed to get ticket: %w", err)
	}

	if existing.Status != tickets.StatusNew {
		return tickets.Ticket{}, fmt.Errorf("%w: ticket already assigned or in progress", ErrConflict)
	}

	comment := "Assigned to operator"
	if input.Comment != "" {
		comment = input.Comment
	}
	status := tickets.StatusInProgress

	updated, err := s.UpdateTicket(ctx, UpdateTicketInput{
		ID:      input.TicketID,
		Status:  &status,
		Comment: &comment,
	})
	if err != nil {
		return tickets.Ticket{}, err
	}

	if s.eventBus != nil {
		_ = s.eventBus.Publish(ctx, domainEvents.TicketAssigned{
			BaseEvent:  domainEvents.NewBaseEvent(),
			TicketID:   updated.ID,
			OperatorID: input.OperatorID,
			AssignedBy: input.AssignedBy,
		})
	}

	s.logger.Info().
		Int64("ticket_id", updated.ID).
		Int64("operator_id", input.OperatorID).
		Int64("assigned_by", input.AssignedBy).
		Msg("ticket assigned")

	return updated, nil
}

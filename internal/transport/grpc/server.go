package grpc

import (
	"context"

	"pet-ticket/api/gen/go/ticket/v1"
	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server реализует ticket.v1.TicketService
type Server struct {
	ticketv1.UnimplementedTicketServiceServer
	svc tickets.Service
}

// NewServer создаёт gRPC сервер для тикетов
func NewServer(svc tickets.Service) *Server {
	return &Server{svc: svc}
}

// CreateTicket создаёт тикет
func (s *Server) CreateTicket(ctx context.Context, req *ticketv1.CreateTicketRequest) (*ticketv1.CreateTicketResponse, error) {
	input := tickets.CreateTicketInput{
		UserID:  req.UserId,
		TopicID: req.TopicId,
		Amount:  req.Amount,
		Comment: req.Comment,
	}

	ticket, err := s.svc.CreateTicket(ctx, input)
	if err != nil {
		return nil, mapAppError(err)
	}

	return &ticketv1.CreateTicketResponse{
		Ticket: domainTicketToProto(ticket),
	}, nil
}

// GetTicket возвращает тикет по ID
func (s *Server) GetTicket(ctx context.Context, req *ticketv1.GetTicketRequest) (*ticketv1.GetTicketResponse, error) {
	ticket, err := s.svc.GetTicket(ctx, req.Id)
	if err != nil {
		return nil, mapAppError(err)
	}

	return &ticketv1.GetTicketResponse{
		Ticket: domainTicketToProto(ticket),
	}, nil
}

// UpdateTicket обновляет тикет
func (s *Server) UpdateTicket(ctx context.Context, req *ticketv1.UpdateTicketRequest) (*ticketv1.UpdateTicketResponse, error) {
	input := tickets.UpdateTicketInput{ID: req.Id}
	if req.StatusId != nil {
		st := domain.Status(*req.StatusId)
		input.Status = &st
	}
	input.Comment = req.Comment

	ticket, err := s.svc.UpdateTicket(ctx, input)
	if err != nil {
		return nil, mapAppError(err)
	}

	return &ticketv1.UpdateTicketResponse{
		Ticket: domainTicketToProto(ticket),
	}, nil
}

// DeleteTicket удаляет тикет
func (s *Server) DeleteTicket(ctx context.Context, req *ticketv1.DeleteTicketRequest) (*ticketv1.DeleteTicketResponse, error) {
	if err := s.svc.DeleteTicket(ctx, req.Id); err != nil {
		return nil, mapAppError(err)
	}
	return &ticketv1.DeleteTicketResponse{}, nil
}

// ListTickets возвращает список тикетов
func (s *Server) ListTickets(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
	var userID, topicID *int64
	var status *domain.Status

	if req.UserId != nil {
		userID = req.UserId
	}
	if req.TopicId != nil {
		topicID = req.TopicId
	}
	if req.StatusId != nil {
		st := domain.Status(*req.StatusId)
		status = &st
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	input := tickets.ListTicketsInput{
		UserID:   userID,
		TopicID:  topicID,
		Status:   status,
		Limit:    limit,
		Offset:   int(req.Offset),
		SortBy:   req.SortBy,
		SortDesc: req.SortDesc,
	}
	if input.SortBy == "" {
		input.SortBy = "created_at"
	}

	list, err := s.svc.ListTickets(ctx, input)
	if err != nil {
		return nil, mapAppError(err)
	}

	ticketsProto := make([]*ticketv1.Ticket, len(list))
	for i, t := range list {
		ticketsProto[i] = domainTicketToProto(t)
	}

	return &ticketv1.ListTicketsResponse{
		Tickets: ticketsProto,
		Total:   int64(len(list)),
	}, nil
}

// GetTicketHistory возвращает историю тикета
func (s *Server) GetTicketHistory(ctx context.Context, req *ticketv1.GetTicketHistoryRequest) (*ticketv1.GetTicketHistoryResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	records, err := s.svc.GetTicketHistory(ctx, req.TicketId, limit, int(req.Offset))
	if err != nil {
		return nil, mapAppError(err)
	}

	protoRecords := make([]*ticketv1.HistoryRecord, len(records))
	for i, r := range records {
		protoRecords[i] = &ticketv1.HistoryRecord{
			Id:        r.ID,
			TicketId:  r.TicketID,
			UserId:    r.UserID,
			Action:    string(r.Action),
			OldValue:  r.OldValue,
			NewValue:  r.NewValue,
			CreatedAt: timestamppb.New(r.CreatedAt),
		}
	}

	return &ticketv1.GetTicketHistoryResponse{
		Records: protoRecords,
	}, nil
}

// GetAllStatuses возвращает справочник статусов
func (s *Server) GetAllStatuses(ctx context.Context, _ *ticketv1.GetAllStatusesRequest) (*ticketv1.GetAllStatusesResponse, error) {
	statuses, err := s.svc.GetAllStatuses(ctx)
	if err != nil {
		return nil, mapAppError(err)
	}

	protoStatuses := make([]*ticketv1.Status, len(statuses))
	for i, st := range statuses {
		// Проверка на overflow при конвертации int -> int32
		if st.ID > 2147483647 || st.ID < -2147483648 {
			return nil, status.Error(codes.Internal, "status ID overflow")
		}
		protoStatuses[i] = &ticketv1.Status{
			Id:   int32(st.ID),
			Name: st.Name,
		}
	}

	return &ticketv1.GetAllStatusesResponse{
		Statuses: protoStatuses,
	}, nil
}

// GetAllTopics возвращает справочник тем
func (s *Server) GetAllTopics(ctx context.Context, _ *ticketv1.GetAllTopicsRequest) (*ticketv1.GetAllTopicsResponse, error) {
	topics, err := s.svc.GetAllTopics(ctx)
	if err != nil {
		return nil, mapAppError(err)
	}

	protoTopics := make([]*ticketv1.Topic, len(topics))
	for i, t := range topics {
		protoTopics[i] = &ticketv1.Topic{
			Id:          t.ID,
			ExternalId:  t.ExternalID,
			Title:       t.Title,
			Description: t.Description,
		}
	}

	return &ticketv1.GetAllTopicsResponse{
		Topics: protoTopics,
	}, nil
}

func domainTicketToProto(t domain.Ticket) *ticketv1.Ticket {
	// Проверка на overflow при конвертации int -> int32
	var statusID int32
	statusInt := int(t.Status)
	if statusInt > 2147483647 || statusInt < -2147483648 {
		statusID = 0 // Fallback значение при overflow
	} else {
		statusID = int32(statusInt) // #nosec G115 - проверено выше
	}

	ticket := &ticketv1.Ticket{
		Id:         t.ID,
		UserId:     t.UserID,
		TopicId:    t.TopicID,
		StatusId:   statusID,
		StatusName: t.Status.String(),
		Comment:    t.Comment,
		CreatedAt:  timestamppb.New(t.CreatedAt),
		UpdatedAt:  timestamppb.New(t.UpdatedAt),
	}
	if t.Amount != nil {
		ticket.Amount = t.Amount
	}
	return ticket
}

func mapAppError(err error) error {
	if err == nil {
		return nil
	}
	switch err {
	case tickets.ErrNotFound:
		return status.Error(codes.NotFound, "ticket not found")
	case tickets.ErrInvalidInput:
		return status.Error(codes.InvalidArgument, "invalid input")
	case tickets.ErrInvalidStatus:
		return status.Error(codes.InvalidArgument, "invalid status")
	case tickets.ErrUnauthorized:
		return status.Error(codes.PermissionDenied, "unauthorized")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

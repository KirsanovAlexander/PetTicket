package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"
	dto "pet-ticket/internal/transport/http/dto/v1"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// mockTicketsService — мок сервиса для тестов handlers
type mockTicketsService struct {
	createTicketFunc          func(ctx context.Context, input tickets.CreateTicketInput) (domain.Ticket, error)
	getTicketFunc             func(ctx context.Context, id int64) (domain.Ticket, error)
	updateTicketFunc          func(ctx context.Context, input tickets.UpdateTicketInput) (domain.Ticket, error)
	deleteTicketFunc          func(ctx context.Context, id int64) error
	listTicketsFunc           func(ctx context.Context, input tickets.ListTicketsInput) ([]domain.Ticket, error)
	listTicketsWithCursorFunc func(ctx context.Context, input tickets.ListTicketsWithCursorInput) (tickets.CursorPage, error)
	getTicketFullFunc         func(ctx context.Context, id int64) (domain.TicketFull, error)
	listTicketsFullFunc       func(ctx context.Context, input tickets.ListTicketsInput) ([]domain.TicketFull, error)
	getTicketHistoryFunc      func(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error)
	getAllStatusesFunc        func(ctx context.Context) ([]tickets.StatusInfo, error)
	getAllTopicsFunc          func(ctx context.Context) ([]domain.Topic, error)
	updatePriorityFunc        func(ctx context.Context, ticketID int64, priority domain.Priority, userID int64) (domain.Ticket, error)
	escalateTicketFunc        func(ctx context.Context, ticketID int64, userID int64) (domain.Ticket, error)
	addCommentFunc            func(ctx context.Context, input tickets.AddCommentInput) (domain.Ticket, error)
	getSLAViolationsFunc      func(ctx context.Context) ([]domain.Ticket, error)
	closeTicketFunc           func(ctx context.Context, input tickets.CloseTicketInput) (domain.Ticket, error)
	assignTicketFunc          func(ctx context.Context, input tickets.AssignTicketInput) (domain.Ticket, error)
}

func (m *mockTicketsService) CreateTicket(ctx context.Context, input tickets.CreateTicketInput) (domain.Ticket, error) {
	if m.createTicketFunc != nil {
		return m.createTicketFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) GetTicket(ctx context.Context, id int64) (domain.Ticket, error) {
	if m.getTicketFunc != nil {
		return m.getTicketFunc(ctx, id)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) UpdateTicket(ctx context.Context, input tickets.UpdateTicketInput) (domain.Ticket, error) {
	if m.updateTicketFunc != nil {
		return m.updateTicketFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) DeleteTicket(ctx context.Context, id int64) error {
	if m.deleteTicketFunc != nil {
		return m.deleteTicketFunc(ctx, id)
	}
	return errors.New("not implemented")
}

func (m *mockTicketsService) ListTickets(ctx context.Context, input tickets.ListTicketsInput) ([]domain.Ticket, error) {
	if m.listTicketsFunc != nil {
		return m.listTicketsFunc(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) ListTicketsWithCursor(ctx context.Context, input tickets.ListTicketsWithCursorInput) (tickets.CursorPage, error) {
	if m.listTicketsWithCursorFunc != nil {
		return m.listTicketsWithCursorFunc(ctx, input)
	}
	return tickets.CursorPage{}, errors.New("not implemented")
}

func (m *mockTicketsService) GetTicketFull(ctx context.Context, id int64) (domain.TicketFull, error) {
	if m.getTicketFullFunc != nil {
		return m.getTicketFullFunc(ctx, id)
	}
	return domain.TicketFull{}, errors.New("not implemented")
}

func (m *mockTicketsService) ListTicketsFull(ctx context.Context, input tickets.ListTicketsInput) ([]domain.TicketFull, error) {
	if m.listTicketsFullFunc != nil {
		return m.listTicketsFullFunc(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int) ([]domain.History, error) {
	if m.getTicketHistoryFunc != nil {
		return m.getTicketHistoryFunc(ctx, ticketID, limit, offset)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) GetAllStatuses(ctx context.Context) ([]tickets.StatusInfo, error) {
	if m.getAllStatusesFunc != nil {
		return m.getAllStatusesFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) GetAllTopics(ctx context.Context) ([]domain.Topic, error) {
	if m.getAllTopicsFunc != nil {
		return m.getAllTopicsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) UpdatePriority(ctx context.Context, ticketID int64, priority domain.Priority, userID int64) (domain.Ticket, error) {
	if m.updatePriorityFunc != nil {
		return m.updatePriorityFunc(ctx, ticketID, priority, userID)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) EscalateTicket(ctx context.Context, ticketID int64, userID int64) (domain.Ticket, error) {
	if m.escalateTicketFunc != nil {
		return m.escalateTicketFunc(ctx, ticketID, userID)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) AddComment(ctx context.Context, input tickets.AddCommentInput) (domain.Ticket, error) {
	if m.addCommentFunc != nil {
		return m.addCommentFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) GetSLAViolations(ctx context.Context) ([]domain.Ticket, error) {
	if m.getSLAViolationsFunc != nil {
		return m.getSLAViolationsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) CloseTicket(ctx context.Context, input tickets.CloseTicketInput) (domain.Ticket, error) {
	if m.closeTicketFunc != nil {
		return m.closeTicketFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) AssignTicket(ctx context.Context, input tickets.AssignTicketInput) (domain.Ticket, error) {
	if m.assignTicketFunc != nil {
		return m.assignTicketFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

// TestGetTicket_Success — успешное получение тикета через HTTP
func TestGetTicket_Success(t *testing.T) {
	// Arrange
	expectedTicket := domain.Ticket{
		ID:      1,
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Test ticket",
	}

	mockSvc := &mockTicketsService{
		getTicketFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			if id == 1 {
				return expectedTicket, nil
			}
			return domain.Ticket{}, tickets.ErrNotFound
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())

	app := fiber.New()
	app.Get("/tickets/:id", handler.getTicket)

	// Act
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets/1", nil)
	resp, err := app.Test(req)

	// Assert
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var ticketResp dto.TicketResponse
	if err := json.Unmarshal(body, &ticketResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if ticketResp.ID != 1 {
		t.Errorf("expected ticket ID 1, got %d", ticketResp.ID)
	}
	if ticketResp.UserID != 100 {
		t.Errorf("expected user ID 100, got %d", ticketResp.UserID)
	}
}

// TestGetTicket_NotFound — тикет не найден (404)
func TestGetTicket_NotFound(t *testing.T) {
	// Arrange
	mockSvc := &mockTicketsService{
		getTicketFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
			return domain.Ticket{}, tickets.ErrNotFound
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if errors.Is(err, tickets.ErrNotFound) {
				code = fiber.StatusNotFound
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})
	app.Get("/tickets/:id", handler.getTicket)

	// Act
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets/999", nil)
	resp, err := app.Test(req)

	// Assert
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

// TestCreateTicket_Success_WithValidation — создание тикета через handler с валидацией
func TestCreateTicket_Success_WithValidation(t *testing.T) {
	// Arrange
	createdTicket := domain.Ticket{
		ID:      42,
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Test ticket from handler",
	}

	mockSvc := &mockTicketsService{
		createTicketFunc: func(ctx context.Context, input tickets.CreateTicketInput) (domain.Ticket, error) {
			return createdTicket, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())

	app := fiber.New()
	// Без middleware валидации — тестируем только handler
	app.Post("/tickets", func(c *fiber.Ctx) error {
		// Имитируем валидированное body
		req := CreateTicketRequest{
			UserID:  100,
			TopicID: 1,
			Comment: "Test ticket from handler",
		}
		c.Locals("validatedBody", req)
		return handler.createTicket(c)
	})

	// Act
	body := []byte(`{"userId": 100, "topicId": 1, "comment": "Test ticket from handler"}`)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/tickets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	// Assert
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var ticketResp dto.TicketResponse
	if err := json.Unmarshal(bodyBytes, &ticketResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if ticketResp.ID != 42 {
		t.Errorf("expected ticket ID 42, got %d", ticketResp.ID)
	}
}

// TestListTickets_WithFilters — получение списка с фильтрами
func TestListTickets_WithFilters(t *testing.T) {
	// Arrange
	userID := int64(100)
	expectedTickets := []domain.Ticket{
		{ID: 1, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Ticket 1"},
		{ID: 2, UserID: 100, TopicID: 2, Status: domain.StatusInProgress, Comment: "Ticket 2"},
	}

	var capturedFilter tickets.ListTicketsInput

	mockSvc := &mockTicketsService{
		listTicketsFunc: func(ctx context.Context, input tickets.ListTicketsInput) ([]domain.Ticket, error) {
			capturedFilter = input
			return expectedTickets, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())

	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	// Act
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets?userId=100&limit=10&offset=5", nil)
	resp, err := app.Test(req)

	// Assert
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if capturedFilter.UserID == nil || *capturedFilter.UserID != userID {
		t.Errorf("expected userID filter 100, got %v", capturedFilter.UserID)
	}
	if capturedFilter.Limit != 10 {
		t.Errorf("expected limit 10, got %d", capturedFilter.Limit)
	}
	if capturedFilter.Offset != 5 {
		t.Errorf("expected offset 5, got %d", capturedFilter.Offset)
	}

	body, _ := io.ReadAll(resp.Body)
	var listResp dto.ListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if listResp.Total != 2 {
		t.Errorf("expected total 2, got %d", listResp.Total)
	}
}

// TestListTickets_CursorDispatch_UsesCursorPagination — наличие cursor
// в query должно переключать listTickets на ListTicketsWithCursor
func TestListTickets_CursorDispatch_UsesCursorPagination(t *testing.T) {
	expectedTickets := []domain.Ticket{
		{ID: 5, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "Ticket 5"},
	}

	var called bool
	mockSvc := &mockTicketsService{
		listTicketsFunc: func(ctx context.Context, input tickets.ListTicketsInput) ([]domain.Ticket, error) {
			t.Fatal("expected offset ListTickets NOT to be called when cursor is present")
			return nil, nil
		},
		listTicketsWithCursorFunc: func(ctx context.Context, input tickets.ListTicketsWithCursorInput) (tickets.CursorPage, error) {
			called = true
			return tickets.CursorPage{Items: expectedTickets, NextCursor: "next-token", HasMore: true}, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets?cursor=abc123", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if !called {
		t.Fatal("expected ListTicketsWithCursor to be called")
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var listResp dto.ListResponseWithPagination
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !listResp.Pagination.HasMore {
		t.Error("expected hasMore=true in response")
	}
	if listResp.Pagination.NextCursor != "next-token" {
		t.Errorf("expected nextCursor 'next-token', got %q", listResp.Pagination.NextCursor)
	}
}

// TestListTickets_CursorDispatch_PageSizeTriggersCursorMode — наличие
// page_size (без cursor, для первой страницы) тоже должно переключать режим
func TestListTickets_CursorDispatch_PageSizeTriggersCursorMode(t *testing.T) {
	var capturedInput tickets.ListTicketsWithCursorInput
	mockSvc := &mockTicketsService{
		listTicketsWithCursorFunc: func(ctx context.Context, input tickets.ListTicketsWithCursorInput) (tickets.CursorPage, error) {
			capturedInput = input
			return tickets.CursorPage{}, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets?page_size=5", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if capturedInput.PageSize != 5 {
		t.Errorf("expected page_size 5, got %d", capturedInput.PageSize)
	}
}

// TestListTickets_CursorDispatch_InvalidPageSize — page_size вне диапазона
func TestListTickets_CursorDispatch_InvalidPageSize(t *testing.T) {
	mockSvc := &mockTicketsService{}
	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets?page_size=0", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

// TestListTickets_CursorDispatch_InvalidDirection — direction вне {next, prev}
func TestListTickets_CursorDispatch_InvalidDirection(t *testing.T) {
	mockSvc := &mockTicketsService{}
	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets?page_size=10&direction=sideways", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

// TestListTickets_CursorDispatch_FiltersParsed — фильтры userId/topicId/statusId/priorityId
// корректно прокидываются в ListTicketsWithCursorInput
func TestListTickets_CursorDispatch_FiltersParsed(t *testing.T) {
	var capturedInput tickets.ListTicketsWithCursorInput
	mockSvc := &mockTicketsService{
		listTicketsWithCursorFunc: func(ctx context.Context, input tickets.ListTicketsWithCursorInput) (tickets.CursorPage, error) {
			capturedInput = input
			return tickets.CursorPage{}, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	req := httptest.NewRequestWithContext(context.Background(), "GET",
		"/tickets?page_size=10&userId=100&topicId=2&statusId=1&priorityId=3&direction=prev", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if capturedInput.UserID == nil || *capturedInput.UserID != 100 {
		t.Errorf("expected userID 100, got %v", capturedInput.UserID)
	}
	if capturedInput.TopicID == nil || *capturedInput.TopicID != 2 {
		t.Errorf("expected topicID 2, got %v", capturedInput.TopicID)
	}
	if capturedInput.Status == nil || *capturedInput.Status != domain.StatusNew {
		t.Errorf("expected status 'new', got %v", capturedInput.Status)
	}
	if capturedInput.Priority == nil || *capturedInput.Priority != domain.PriorityHigh {
		t.Errorf("expected priority 'high', got %v", capturedInput.Priority)
	}
	if capturedInput.Direction != "prev" {
		t.Errorf("expected direction 'prev', got %q", capturedInput.Direction)
	}
}

// testLogger возвращает no-op logger для тестов
func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

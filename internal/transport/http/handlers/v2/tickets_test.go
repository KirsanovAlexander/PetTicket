package v2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// mockTicketsService — мок сервиса для тестов handlers/v2. Реализует
// tickets.Service целиком (интерфейс общий для v1/v2), но v2-хендлеры
// реально используют только часть методов.
type mockTicketsService struct {
	createTicketFunc    func(ctx context.Context, input tickets.CreateTicketInput) (domain.Ticket, error)
	getTicketFullFunc   func(ctx context.Context, id int64) (domain.TicketFull, error)
	updateTicketFunc    func(ctx context.Context, input tickets.UpdateTicketInput) (domain.Ticket, error)
	deleteTicketFunc    func(ctx context.Context, id int64) error
	listTicketsFullFunc func(ctx context.Context, input tickets.ListTicketsInput) ([]domain.TicketFull, error)
}

func (m *mockTicketsService) CreateTicket(ctx context.Context, input tickets.CreateTicketInput) (domain.Ticket, error) {
	if m.createTicketFunc != nil {
		return m.createTicketFunc(ctx, input)
	}
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) GetTicket(ctx context.Context, id int64) (domain.Ticket, error) {
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
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) ListTicketsWithCursor(ctx context.Context, input tickets.ListTicketsWithCursorInput) (tickets.CursorPage, error) {
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
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) GetAllStatuses(ctx context.Context) ([]tickets.StatusInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) GetAllTopics(ctx context.Context) ([]domain.Topic, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) UpdatePriority(ctx context.Context, ticketID int64, priority domain.Priority, userID int64) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) EscalateTicket(ctx context.Context, ticketID int64, userID int64) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) AddComment(ctx context.Context, input domain.AddCommentInput) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) GetComments(ctx context.Context, filter domain.ListCommentsFilter) ([]domain.TicketComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) GetLastComment(ctx context.Context, ticketID int64) (*domain.TicketComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) UpdateComment(ctx context.Context, input domain.UpdateCommentInput) error {
	return errors.New("not implemented")
}

func (m *mockTicketsService) DeleteComment(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (m *mockTicketsService) GetSLAViolations(ctx context.Context) ([]domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTicketsService) CloseTicket(ctx context.Context, input tickets.CloseTicketInput) (domain.Ticket, error) {
	return domain.Ticket{}, errors.New("not implemented")
}

func (m *mockTicketsService) AssignTicket(ctx context.Context, ticketID, assigneeID int64) error {
	return errors.New("not implemented")
}

func (m *mockTicketsService) UnassignTicket(ctx context.Context, ticketID, assigneeID int64) error {
	return errors.New("not implemented")
}

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func sampleTicketFull() domain.TicketFull {
	return domain.TicketFull{
		ID:   1,
		User: domain.User{ID: 100},
		Status: domain.TicketStatusInfo{
			ID: 1, Name: "new", DisplayName: "Новый тикет",
		},
		Topic:    domain.Topic{ID: 1, ExternalID: 1, Title: "Депозит", Description: "Проблемы с депозитом"},
		Assignee: &domain.User{ID: 200},
		Priority: domain.PriorityMedium,
		Comments: []domain.Comment{
			{ID: 1, UserID: 100, Text: "Первый комментарий", CreatedAt: time.Now()},
		},
		SLA: &domain.SLAInfo{
			ResponseStatus:   domain.SLAStatusOK,
			ResolutionStatus: domain.SLAStatusOK,
			OverallStatus:    domain.SLAStatusOK,
		},
		Comment:   "Первый комментарий",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// TestGetTicket_ReturnsNestedObjects — v2 GET /tickets/:id должен вернуть
// user/status/topic/assignee/comments/sla как вложенные объекты.
func TestGetTicket_ReturnsNestedObjects(t *testing.T) {
	full := sampleTicketFull()

	mockSvc := &mockTicketsService{
		getTicketFullFunc: func(ctx context.Context, id int64) (domain.TicketFull, error) {
			if id == 1 {
				return full, nil
			}
			return domain.TicketFull{}, tickets.ErrNotFound
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets/:id", handler.getTicket)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets/1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	for _, field := range []string{"user", "status", "topic", "assignee", "comments", "sla"} {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected nested field %q in response, got: %s", field, body)
		}
	}

	userObj, ok := parsed["user"].(map[string]any)
	if !ok || userObj["id"] != float64(100) {
		t.Errorf("expected user.id=100, got: %v", parsed["user"])
	}

	statusObj, ok := parsed["status"].(map[string]any)
	if !ok || statusObj["displayName"] != "Новый тикет" {
		t.Errorf("expected status.displayName='Новый тикет', got: %v", parsed["status"])
	}
}

// TestGetTicket_NotFound — 404 маппится корректно и для v2.
func TestGetTicket_NotFound(t *testing.T) {
	mockSvc := &mockTicketsService{
		getTicketFullFunc: func(ctx context.Context, id int64) (domain.TicketFull, error) {
			return domain.TicketFull{}, tickets.ErrNotFound
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

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets/999", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

// TestCreateTicket_RefetchesFullAfterCreate — после CreateTicket handler
// обязан дочитать GetTicketFull и вернуть именно его, а не голый Ticket.
func TestCreateTicket_RefetchesFullAfterCreate(t *testing.T) {
	created := domain.Ticket{ID: 42, UserID: 100, TopicID: 1, Status: domain.StatusNew, Comment: "New ticket via v2 API"}
	full := sampleTicketFull()
	full.ID = 42

	var getFullCalledWith int64
	mockSvc := &mockTicketsService{
		createTicketFunc: func(ctx context.Context, input tickets.CreateTicketInput) (domain.Ticket, error) {
			return created, nil
		},
		getTicketFullFunc: func(ctx context.Context, id int64) (domain.TicketFull, error) {
			getFullCalledWith = id
			return full, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Post("/tickets", func(c *fiber.Ctx) error {
		c.Locals("validatedBody", CreateTicketRequest{
			UserID: 100, TopicID: 1, Comment: "New ticket via v2 API",
		})
		return handler.createTicket(c)
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/tickets", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	if getFullCalledWith != 42 {
		t.Errorf("expected GetTicketFull to be called with id=42, got %d", getFullCalledWith)
	}

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := parsed["user"]; !ok {
		t.Errorf("expected nested 'user' field in create response, got: %s", body)
	}
}

// TestListTickets_ReturnsFullList — v2 список тикетов использует
// ListTicketsFull и оборачивает в dtov2.ListResponse с pagination.
func TestListTickets_ReturnsFullList(t *testing.T) {
	list := []domain.TicketFull{sampleTicketFull()}

	var capturedInput tickets.ListTicketsInput
	mockSvc := &mockTicketsService{
		listTicketsFullFunc: func(ctx context.Context, input tickets.ListTicketsInput) ([]domain.TicketFull, error) {
			capturedInput = input
			return list, nil
		},
	}

	handler := NewTicketsHandler(mockSvc, testLogger())
	app := fiber.New()
	app.Get("/tickets", handler.listTickets)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets?limit=10&offset=0&userId=100", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if capturedInput.UserID == nil || *capturedInput.UserID != 100 {
		t.Errorf("expected userID filter 100, got %v", capturedInput.UserID)
	}

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data, ok := parsed["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("expected data array with 1 item, got: %s", body)
	}
}

package analytics

import (
	"context"
	"testing"
	"time"

	ticketv1 "pet-ticket/api/gen/go/ticket/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockTicketClient — мок TicketClientInterface для юнит-тестов агрегатора.
type mockTicketClient struct {
	listTicketsFunc      func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error)
	getTicketHistoryFunc func(ctx context.Context, ticketID int64, limit, offset int32) (*ticketv1.GetTicketHistoryResponse, error)
	getAllStatusesFunc   func(ctx context.Context) (*ticketv1.GetAllStatusesResponse, error)
	getAllTopicsFunc     func(ctx context.Context) (*ticketv1.GetAllTopicsResponse, error)
}

func (m *mockTicketClient) ListTickets(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
	if m.listTicketsFunc != nil {
		return m.listTicketsFunc(ctx, req)
	}
	return &ticketv1.ListTicketsResponse{}, nil
}

func (m *mockTicketClient) GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int32) (*ticketv1.GetTicketHistoryResponse, error) {
	if m.getTicketHistoryFunc != nil {
		return m.getTicketHistoryFunc(ctx, ticketID, limit, offset)
	}
	return &ticketv1.GetTicketHistoryResponse{}, nil
}

func (m *mockTicketClient) GetAllStatuses(ctx context.Context) (*ticketv1.GetAllStatusesResponse, error) {
	if m.getAllStatusesFunc != nil {
		return m.getAllStatusesFunc(ctx)
	}
	return &ticketv1.GetAllStatusesResponse{}, nil
}

func (m *mockTicketClient) GetAllTopics(ctx context.Context) (*ticketv1.GetAllTopicsResponse, error) {
	if m.getAllTopicsFunc != nil {
		return m.getAllTopicsFunc(ctx)
	}
	return &ticketv1.GetAllTopicsResponse{}, nil
}

func ts(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func TestAggregator_Overview(t *testing.T) {
	now := time.Now()
	created1 := now.Add(-48 * time.Hour)
	updated1 := now.Add(-24 * time.Hour)

	client := &mockTicketClient{
		getAllStatusesFunc: func(ctx context.Context) (*ticketv1.GetAllStatusesResponse, error) {
			return &ticketv1.GetAllStatusesResponse{
				Statuses: []*ticketv1.Status{
					{Id: 1, Name: "new"},
					{Id: 2, Name: "resolved"},
					{Id: 3, Name: "cancelled"}, // ни одного тикета в этом статусе
				},
			}, nil
		},
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{
					{Id: 1, StatusName: "resolved", CreatedAt: ts(created1), UpdatedAt: ts(updated1)},
					{Id: 2, StatusName: "new", CreatedAt: ts(now), UpdatedAt: ts(now)},
				},
			}, nil
		},
	}

	agg := NewAggregator(client)
	overview, err := agg.Overview(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if overview.TotalTickets != 2 {
		t.Errorf("expected 2 total tickets, got %d", overview.TotalTickets)
	}
	if overview.AvgResolutionMinutes != 24*60 {
		t.Errorf("expected avg resolution 1440 minutes, got %f", overview.AvgResolutionMinutes)
	}
	// Должны быть представлены все 3 статуса из справочника, включая "cancelled" с 0
	if len(overview.ByStatus) != 3 {
		t.Fatalf("expected 3 status groups (including zero-count ones), got %d", len(overview.ByStatus))
	}
	for _, sc := range overview.ByStatus {
		if sc.Status == "cancelled" && sc.Count != 0 {
			t.Errorf("expected 0 cancelled tickets, got %d", sc.Count)
		}
	}
	if overview.SLA.ResolvedCount != 1 {
		t.Errorf("expected 1 resolved ticket in SLA summary, got %d", overview.SLA.ResolvedCount)
	}
}

func TestAggregator_Overview_Pagination(t *testing.T) {
	// fetchAllTickets должен пройти несколько страниц, а не останавливаться
	// после первой — иначе total_tickets будет занижен на больших объёмах.
	callCount := 0
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			callCount++
			if req.Offset == 0 {
				tickets := make([]*ticketv1.Ticket, fetchPageSize)
				for i := range tickets {
					tickets[i] = &ticketv1.Ticket{Id: int64(i + 1), StatusName: "new", CreatedAt: ts(time.Now()), UpdatedAt: ts(time.Now())}
				}
				return &ticketv1.ListTicketsResponse{Tickets: tickets}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{
					{Id: 999, StatusName: "new", CreatedAt: ts(time.Now()), UpdatedAt: ts(time.Now())},
				},
			}, nil
		},
	}

	agg := NewAggregator(client)
	overview, err := agg.Overview(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if overview.TotalTickets != int64(fetchPageSize+1) {
		t.Errorf("expected %d total tickets, got %d", fetchPageSize+1, overview.TotalTickets)
	}
	if callCount != 2 {
		t.Errorf("expected 2 pages fetched, got %d", callCount)
	}
}

func TestAggregator_UserStats_FetchesRecentActivity(t *testing.T) {
	now := time.Now()
	historyCalledWithTicketID := int64(0)

	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			if req.UserId == nil || *req.UserId != 42 {
				t.Errorf("expected filter by user_id=42, got %+v", req.UserId)
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{
					{Id: 10, UserId: 42, StatusName: "new", CreatedAt: ts(now.Add(-time.Hour)), UpdatedAt: ts(now.Add(-time.Hour))},
					{Id: 11, UserId: 42, StatusName: "in_progress", CreatedAt: ts(now), UpdatedAt: ts(now)},
				},
			}, nil
		},
		getTicketHistoryFunc: func(ctx context.Context, ticketID int64, limit, offset int32) (*ticketv1.GetTicketHistoryResponse, error) {
			historyCalledWithTicketID = ticketID
			return &ticketv1.GetTicketHistoryResponse{
				Records: []*ticketv1.HistoryRecord{
					{Action: "created", CreatedAt: ts(now)},
				},
			}, nil
		},
	}

	agg := NewAggregator(client)
	stats, err := agg.UserStats(context.Background(), 42)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if stats.TotalTickets != 2 {
		t.Errorf("expected 2 tickets, got %d", stats.TotalTickets)
	}
	if historyCalledWithTicketID != 11 {
		t.Errorf("expected history to be fetched for the most recently updated ticket (11), got %d", historyCalledWithTicketID)
	}
	if len(stats.RecentActivity) != 1 {
		t.Errorf("expected 1 recent activity entry, got %d", len(stats.RecentActivity))
	}
}

func TestAggregator_TopicStats_IncludesTopicsWithZeroTickets(t *testing.T) {
	client := &mockTicketClient{
		getAllTopicsFunc: func(ctx context.Context) (*ticketv1.GetAllTopicsResponse, error) {
			return &ticketv1.GetAllTopicsResponse{
				Topics: []*ticketv1.Topic{
					{Id: 1, Title: "Billing"},
					{Id: 2, Title: "Technical"},
				},
			}, nil
		},
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{
					{Id: 1, TopicId: 1, StatusName: "new", CreatedAt: ts(time.Now()), UpdatedAt: ts(time.Now())},
				},
			}, nil
		},
	}

	agg := NewAggregator(client)
	topics, err := agg.TopicStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(topics.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics.Topics))
	}
	if topics.Topics[0].TotalTickets != 1 {
		t.Errorf("expected topic 1 to have 1 ticket, got %d", topics.Topics[0].TotalTickets)
	}
	if topics.Topics[1].TotalTickets != 0 {
		t.Errorf("expected topic 2 to have 0 tickets, got %d", topics.Topics[1].TotalTickets)
	}
}

func TestAggregator_Timeline_InvalidPeriod(t *testing.T) {
	agg := NewAggregator(&mockTicketClient{})
	_, err := agg.Timeline(context.Background(), "1y")
	if err == nil {
		t.Error("expected error for unsupported period, got nil")
	}
}

func TestAggregator_Timeline_BucketsByDay(t *testing.T) {
	now := time.Now().UTC()
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{
					{Id: 1, StatusName: "new", CreatedAt: ts(now), UpdatedAt: ts(now)},
					{Id: 2, StatusName: "resolved", CreatedAt: ts(now.AddDate(0, 0, -1)), UpdatedAt: ts(now)},
				},
			}, nil
		},
	}

	agg := NewAggregator(client)
	timeline, err := agg.Timeline(context.Background(), Period7Days)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(timeline.Points) != 7 {
		t.Fatalf("expected 7 points, got %d", len(timeline.Points))
	}

	last := timeline.Points[len(timeline.Points)-1]
	if last.Date != now.Format("2006-01-02") {
		t.Errorf("expected last point to be today (%s), got %s", now.Format("2006-01-02"), last.Date)
	}
	if last.Created != 1 {
		t.Errorf("expected 1 ticket created today, got %d", last.Created)
	}
	if last.Resolved != 1 {
		t.Errorf("expected 1 ticket resolved today, got %d", last.Resolved)
	}
}

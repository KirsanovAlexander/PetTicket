package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	ticketv1 "pet-ticket/api/gen/go/ticket/v1"

	"github.com/rs/zerolog"
)

// mockCache — мок Cache, хранит данные в памяти как JSON, чтобы поведение
// (де)сериализации было максимально близко к реальному RedisCache.
type mockCache struct {
	store   map[string][]byte
	getErr  error
	setErr  error
	getHits int
	setHits int
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string][]byte)}
}

func (m *mockCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	m.getHits++
	if m.getErr != nil {
		return false, m.getErr
	}
	data, ok := m.store[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}) error {
	m.setHits++
	if m.setErr != nil {
		return m.setErr
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.store[key] = data
	return nil
}

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestService_GetOverview_CacheMiss_FallsBackAndPopulatesCache(t *testing.T) {
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{{Id: 1, StatusName: "new"}},
			}, nil
		},
	}
	cache := newMockCache()
	svc := NewService(NewAggregator(client), cache, testLogger())

	overview, err := svc.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if overview.TotalTickets != 1 {
		t.Errorf("expected 1 ticket, got %d", overview.TotalTickets)
	}
	if _, ok := cache.store[keyOverview]; !ok {
		t.Error("expected overview to be written to cache after live aggregation")
	}
}

func TestService_GetOverview_CacheHit_DoesNotCallAggregator(t *testing.T) {
	aggregatorCalled := false
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			aggregatorCalled = true
			return &ticketv1.ListTicketsResponse{}, nil
		},
	}
	cache := newMockCache()
	cached := Overview{TotalTickets: 42}
	data, _ := json.Marshal(cached)
	cache.store[keyOverview] = data

	svc := NewService(NewAggregator(client), cache, testLogger())

	overview, err := svc.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if overview.TotalTickets != 42 {
		t.Errorf("expected cached value 42, got %d", overview.TotalTickets)
	}
	if aggregatorCalled {
		t.Error("expected aggregator NOT to be called on cache hit")
	}
}

func TestService_GetOverview_CacheReadError_GracefullyFallsBack(t *testing.T) {
	// Ключевой сценарий из AC: недоступность Redis не должна ронять API.
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{{Id: 1, StatusName: "new"}},
			}, nil
		},
	}
	cache := newMockCache()
	cache.getErr = errors.New("redis: connection refused")

	svc := NewService(NewAggregator(client), cache, testLogger())

	overview, err := svc.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("expected no error (graceful fallback), got: %v", err)
	}
	if overview.TotalTickets != 1 {
		t.Errorf("expected live-aggregated value 1, got %d", overview.TotalTickets)
	}
}

func TestService_GetOverview_CacheWriteError_StillReturnsResult(t *testing.T) {
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			if req.Offset > 0 {
				return &ticketv1.ListTicketsResponse{}, nil
			}
			return &ticketv1.ListTicketsResponse{
				Tickets: []*ticketv1.Ticket{{Id: 1, StatusName: "new"}},
			}, nil
		},
	}
	cache := newMockCache()
	cache.setErr = errors.New("redis: connection refused")

	svc := NewService(NewAggregator(client), cache, testLogger())

	overview, err := svc.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("expected no error even if cache write fails, got: %v", err)
	}
	if overview.TotalTickets != 1 {
		t.Errorf("expected 1 ticket, got %d", overview.TotalTickets)
	}
}

func TestService_GetUserStats_CachesPerUser(t *testing.T) {
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			return &ticketv1.ListTicketsResponse{}, nil
		},
	}
	cache := newMockCache()
	svc := NewService(NewAggregator(client), cache, testLogger())

	if _, err := svc.GetUserStats(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, err := svc.GetUserStats(context.Background(), 8); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, ok := cache.store[keyUserStatsPrefix+"7"]; !ok {
		t.Error("expected user 7 stats to be cached under its own key")
	}
	if _, ok := cache.store[keyUserStatsPrefix+"8"]; !ok {
		t.Error("expected user 8 stats to be cached under its own key")
	}
}

func TestService_GetTimeline_CachesPerPeriod(t *testing.T) {
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			return &ticketv1.ListTicketsResponse{}, nil
		},
	}
	cache := newMockCache()
	svc := NewService(NewAggregator(client), cache, testLogger())

	if _, err := svc.GetTimeline(context.Background(), Period7Days); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, err := svc.GetTimeline(context.Background(), Period30Days); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, ok := cache.store[keyTimelinePrefix+Period7Days]; !ok {
		t.Error("expected 7d timeline to be cached under its own key")
	}
	if _, ok := cache.store[keyTimelinePrefix+Period30Days]; !ok {
		t.Error("expected 30d timeline to be cached under its own key")
	}
}

func TestService_GetTimeline_InvalidPeriod_ReturnsError(t *testing.T) {
	client := &mockTicketClient{}
	cache := newMockCache()
	svc := NewService(NewAggregator(client), cache, testLogger())

	if _, err := svc.GetTimeline(context.Background(), "1y"); err == nil {
		t.Error("expected error for unsupported period, got nil")
	}
}

func TestService_RefreshBackground_PopulatesOverviewTopicsAndTimelines(t *testing.T) {
	client := &mockTicketClient{
		listTicketsFunc: func(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
			return &ticketv1.ListTicketsResponse{}, nil
		},
		getAllTopicsFunc: func(ctx context.Context) (*ticketv1.GetAllTopicsResponse, error) {
			return &ticketv1.GetAllTopicsResponse{}, nil
		},
	}
	cache := newMockCache()
	svc := NewService(NewAggregator(client), cache, testLogger())

	svc.RefreshBackground(context.Background())

	for _, key := range []string{keyOverview, keyTopics, keyTimelinePrefix + Period7Days, keyTimelinePrefix + Period30Days} {
		if _, ok := cache.store[key]; !ok {
			t.Errorf("expected background refresh to populate cache key %q", key)
		}
	}

	// UserStats намеренно не прогревается фоном (см. комментарий в RefreshBackground)
	for key := range cache.store {
		if len(key) >= len(keyUserStatsPrefix) && key[:len(keyUserStatsPrefix)] == keyUserStatsPrefix {
			t.Errorf("did not expect background refresh to populate per-user key %q", key)
		}
	}
}

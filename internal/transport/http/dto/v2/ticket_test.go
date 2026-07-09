package v2

import (
	"testing"
	"time"

	domain "pet-ticket/internal/domain/tickets"
)

func TestMapTicketFullToResponse_WithAssigneeAndSLA(t *testing.T) {
	now := time.Now()
	full := domain.TicketFull{
		ID:   1,
		User: domain.User{ID: 100},
		Status: domain.TicketStatusInfo{
			ID: 2, Name: "in_progress", DisplayName: "В работе",
		},
		Topic:    domain.Topic{ID: 1, ExternalID: 1, Title: "Депозит", Description: "desc"},
		Assignee: &domain.User{ID: 200},
		Priority: domain.PriorityHigh,
		Comments: []domain.Comment{
			{ID: 1, UserID: 100, Text: "first", CreatedAt: now},
			{ID: 2, UserID: 200, Text: "second", CreatedAt: now},
		},
		SLA: &domain.SLAInfo{
			ResponseStatus:   domain.SLAStatusWarning,
			ResolutionStatus: domain.SLAStatusOK,
			OverallStatus:    domain.SLAStatusWarning,
		},
		Comment:   "second",
		CreatedAt: now,
		UpdatedAt: now,
	}

	resp := MapTicketFullToResponse(full)

	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}
	if resp.User.ID != 100 {
		t.Errorf("expected user.id 100, got %d", resp.User.ID)
	}
	if resp.Status.DisplayName != "В работе" {
		t.Errorf("expected status.displayName 'В работе', got %q", resp.Status.DisplayName)
	}
	if resp.Priority != "high" {
		t.Errorf("expected priority 'high', got %q", resp.Priority)
	}
	if resp.Assignee == nil || resp.Assignee.ID != 200 {
		t.Errorf("expected assignee.id 200, got %v", resp.Assignee)
	}
	if len(resp.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(resp.Comments))
	}
	if resp.Comments[0].Text != "first" {
		t.Errorf("expected first comment text 'first', got %q", resp.Comments[0].Text)
	}
	if resp.SLA == nil || resp.SLA.OverallStatus != "warning" {
		t.Errorf("expected sla.overallStatus 'warning', got %v", resp.SLA)
	}
}

func TestMapTicketFullToResponse_NoAssigneeNoSLA(t *testing.T) {
	full := domain.TicketFull{
		ID:       1,
		User:     domain.User{ID: 100},
		Status:   domain.TicketStatusInfo{ID: 1, Name: "new", DisplayName: "Новый тикет"},
		Topic:    domain.Topic{ID: 1, ExternalID: 1, Title: "Депозит"},
		Priority: domain.PriorityLow,
		Comments: nil,
	}

	resp := MapTicketFullToResponse(full)

	if resp.Assignee != nil {
		t.Errorf("expected nil assignee, got %v", resp.Assignee)
	}
	if resp.SLA != nil {
		t.Errorf("expected nil sla, got %v", resp.SLA)
	}
	if len(resp.Comments) != 0 {
		t.Errorf("expected empty comments, got %d", len(resp.Comments))
	}
}

func TestMapTicketFullListToResponse(t *testing.T) {
	list := []domain.TicketFull{
		{ID: 1, User: domain.User{ID: 100}},
		{ID: 2, User: domain.User{ID: 101}},
	}

	result := MapTicketFullListToResponse(list)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != 1 || result[1].ID != 2 {
		t.Errorf("unexpected ids: %d, %d", result[0].ID, result[1].ID)
	}
}

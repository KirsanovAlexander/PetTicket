package tickets

import (
	"testing"
	"time"
)

func TestTicket_IsInactiveResolved(t *testing.T) {
	tests := []struct {
		name         string
		status       Status
		lastActivity time.Time
		inactiveDays int
		want         bool
	}{
		{
			name:         "resolved and inactive",
			status:       StatusResolved,
			lastActivity: time.Now().AddDate(0, 0, -10),
			inactiveDays: 7,
			want:         true,
		},
		{
			name:         "resolved but active",
			status:       StatusResolved,
			lastActivity: time.Now().AddDate(0, 0, -1),
			inactiveDays: 7,
			want:         false,
		},
		{
			name:         "not resolved",
			status:       StatusInProgress,
			lastActivity: time.Now().AddDate(0, 0, -10),
			inactiveDays: 7,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &Ticket{
				Status:             tt.status,
				LastUserActivityAt: tt.lastActivity,
			}
			if got := ticket.IsInactiveResolved(tt.inactiveDays); got != tt.want {
				t.Errorf("IsInactiveResolved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTicket_UpdateUserActivity(t *testing.T) {
	ticket := &Ticket{}
	before := time.Now()
	ticket.UpdateUserActivity()
	after := time.Now()

	if ticket.LastUserActivityAt.Before(before) || ticket.LastUserActivityAt.After(after) {
		t.Errorf("LastUserActivityAt %v not between %v and %v", ticket.LastUserActivityAt, before, after)
	}
}

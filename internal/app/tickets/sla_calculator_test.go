package tickets

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "pet-ticket/internal/domain/tickets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSLARuleRepo struct {
	mock.Mock
}

func (m *mockSLARuleRepo) GetSLARule(ctx context.Context, topicID, priorityID int64) (*domain.SLARule, error) {
	args := m.Called(ctx, topicID, priorityID)
	rule, _ := args.Get(0).(*domain.SLARule)
	return rule, args.Error(1)
}

func TestSLACalculator_CalculateDeadlines_WithRule(t *testing.T) {
	repo := new(mockSLARuleRepo)
	calc := NewSLACalculator(repo)

	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	rule := &domain.SLARule{ResponseTimeMinutes: 30, ResolutionTimeMinutes: 120}

	repo.On("GetSLARule", mock.Anything, int64(1), int64(3)).Return(rule, nil)

	response, resolution, err := calc.CalculateDeadlines(context.Background(), 1, 3, created)
	assert.NoError(t, err)
	assert.Equal(t, created.Add(30*time.Minute), response)
	assert.Equal(t, created.Add(120*time.Minute), resolution)
}

func TestSLACalculator_CalculateDeadlines_DefaultRule(t *testing.T) {
	repo := new(mockSLARuleRepo)
	calc := NewSLACalculator(repo)

	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	repo.On("GetSLARule", mock.Anything, int64(99), int64(99)).Return((*domain.SLARule)(nil), nil)

	response, resolution, err := calc.CalculateDeadlines(context.Background(), 99, 99, created)
	assert.NoError(t, err)
	assert.Equal(t, created.Add(120*time.Minute), response)
	assert.Equal(t, created.Add(1440*time.Minute), resolution)
}

func TestSLACalculator_CalculateDeadlines_RepoError(t *testing.T) {
	repo := new(mockSLARuleRepo)
	calc := NewSLACalculator(repo)

	repo.On("GetSLARule", mock.Anything, int64(1), int64(1)).Return((*domain.SLARule)(nil), errors.New("db error"))

	_, _, err := calc.CalculateDeadlines(context.Background(), 1, 1, time.Now())
	assert.Error(t, err)
}

func TestSLACalculator_ShouldSetFirstResponse(t *testing.T) {
	calc := NewSLACalculator(nil)

	ticket := domain.Ticket{}
	assert.True(t, calc.ShouldSetFirstResponse(ticket, true))
	assert.False(t, calc.ShouldSetFirstResponse(ticket, false))

	now := time.Now()
	ticket.FirstResponseAt = &now
	assert.False(t, calc.ShouldSetFirstResponse(ticket, true))
}

func TestSLACalculator_ShouldSetResolvedAt(t *testing.T) {
	calc := NewSLACalculator(nil)

	assert.True(t, calc.ShouldSetResolvedAt(domain.StatusInProgress, domain.StatusResolved))
	assert.True(t, calc.ShouldSetResolvedAt(domain.StatusNew, domain.StatusClosed))
	assert.False(t, calc.ShouldSetResolvedAt(domain.StatusResolved, domain.StatusClosed))
	assert.False(t, calc.ShouldSetResolvedAt(domain.StatusNew, domain.StatusInProgress))
}

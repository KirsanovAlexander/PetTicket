package tickets

import (
	"testing"
	"time"
)

func TestCalculateSLAStatus_OK(t *testing.T) {
	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	responseDeadline := created.Add(30 * time.Minute)
	resolutionDeadline := created.Add(2 * time.Hour)
	now := created.Add(10 * time.Minute)

	metrics := CalculateSLAStatus(created, responseDeadline, resolutionDeadline, nil, nil, now)

	if metrics.ResponseStatus != SLAStatusOK {
		t.Errorf("expected response ok, got %s", metrics.ResponseStatus)
	}
	if metrics.OverallStatus != SLAStatusOK {
		t.Errorf("expected overall ok, got %s", metrics.OverallStatus)
	}
}

func TestCalculateSLAStatus_Warning(t *testing.T) {
	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	responseDeadline := created.Add(30 * time.Minute)
	resolutionDeadline := created.Add(2 * time.Hour)
	now := created.Add(25 * time.Minute)

	metrics := CalculateSLAStatus(created, responseDeadline, resolutionDeadline, nil, nil, now)

	if metrics.ResponseStatus != SLAStatusWarning {
		t.Errorf("expected response warning, got %s", metrics.ResponseStatus)
	}
}

func TestCalculateSLAStatus_ViolatedNoResponse(t *testing.T) {
	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	responseDeadline := created.Add(30 * time.Minute)
	resolutionDeadline := created.Add(2 * time.Hour)
	now := created.Add(31 * time.Minute)

	metrics := CalculateSLAStatus(created, responseDeadline, resolutionDeadline, nil, nil, now)

	if metrics.ResponseStatus != SLAStatusViolated {
		t.Errorf("expected response violated, got %s", metrics.ResponseStatus)
	}
}

func TestCalculateSLAStatus_ViolatedLateResponse(t *testing.T) {
	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	responseDeadline := created.Add(30 * time.Minute)
	resolutionDeadline := created.Add(2 * time.Hour)
	firstResponse := created.Add(45 * time.Minute)
	now := created.Add(50 * time.Minute)

	metrics := CalculateSLAStatus(created, responseDeadline, resolutionDeadline, &firstResponse, nil, now)

	if metrics.ResponseStatus != SLAStatusViolated {
		t.Errorf("expected response violated, got %s", metrics.ResponseStatus)
	}
}

func TestSLARule_CalculateDeadlines(t *testing.T) {
	rule := SLARule{ResponseTimeMinutes: 30, ResolutionTimeMinutes: 120}
	created := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)

	response, resolution := rule.CalculateDeadlines(created)

	if !response.Equal(created.Add(30 * time.Minute)) {
		t.Errorf("unexpected response deadline: %v", response)
	}
	if !resolution.Equal(created.Add(120 * time.Minute)) {
		t.Errorf("unexpected resolution deadline: %v", resolution)
	}
}

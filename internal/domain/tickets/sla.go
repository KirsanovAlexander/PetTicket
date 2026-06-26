package tickets

import "time"

// SLAStatus описывает состояние SLA по фазе
type SLAStatus string

const (
	SLAStatusOK       SLAStatus = "ok"
	SLAStatusWarning  SLAStatus = "warning"
	SLAStatusViolated SLAStatus = "violated"
)

// SLARule описывает правило SLA для пары topic + priority
type SLARule struct {
	ID                    int64
	TopicID               int64
	PriorityID            int64
	ResponseTimeMinutes   int
	ResolutionTimeMinutes int
}

// CalculateDeadlines рассчитывает дедлайны ответа и решения
func (r SLARule) CalculateDeadlines(createdAt time.Time) (responseDeadline, resolutionDeadline time.Time) {
	responseDeadline = createdAt.Add(time.Duration(r.ResponseTimeMinutes) * time.Minute)
	resolutionDeadline = createdAt.Add(time.Duration(r.ResolutionTimeMinutes) * time.Minute)
	return responseDeadline, resolutionDeadline
}

// SLAMetrics содержит статусы SLA по фазам
type SLAMetrics struct {
	ResponseStatus   SLAStatus
	ResolutionStatus SLAStatus
	OverallStatus    SLAStatus
}

// CalculateSLAStatus рассчитывает SLA-статусы на момент now
func CalculateSLAStatus(
	createdAt, responseDeadline, resolutionDeadline time.Time,
	firstResponseAt, resolvedAt *time.Time,
	now time.Time,
) SLAMetrics {
	responseStatus := calculatePhaseStatus(createdAt, responseDeadline, firstResponseAt, now)
	resolutionStatus := calculatePhaseStatus(createdAt, resolutionDeadline, resolvedAt, now)

	return SLAMetrics{
		ResponseStatus:   responseStatus,
		ResolutionStatus: resolutionStatus,
		OverallStatus:    worstSLAStatus(responseStatus, resolutionStatus),
	}
}

func calculatePhaseStatus(windowStart, deadline time.Time, completedAt *time.Time, now time.Time) SLAStatus {
	if deadline.IsZero() || !deadline.After(windowStart) {
		return SLAStatusOK
	}

	if completedAt != nil {
		if completedAt.After(deadline) {
			return SLAStatusViolated
		}
		return SLAStatusOK
	}

	if now.After(deadline) {
		return SLAStatusViolated
	}

	total := deadline.Sub(windowStart)
	remaining := deadline.Sub(now)
	if remaining < time.Duration(float64(total)*0.2) {
		return SLAStatusWarning
	}

	return SLAStatusOK
}

func worstSLAStatus(a, b SLAStatus) SLAStatus {
	if a == SLAStatusViolated || b == SLAStatusViolated {
		return SLAStatusViolated
	}
	if a == SLAStatusWarning || b == SLAStatusWarning {
		return SLAStatusWarning
	}
	return SLAStatusOK
}

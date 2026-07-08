package analytics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	ticketv1 "pet-ticket/api/gen/go/ticket/v1"
)

// ErrUnsupportedPeriod возвращается Timeline при периоде, отличном от "7d"/"30d".
// Отдельная переменная-ошибка (а не просто fmt.Errorf) нужна, чтобы вызывающий
// HTTP-слой мог через errors.Is отличить невалидный запрос клиента (400) от
// сбоя при обращении к основному сервису (502).
var ErrUnsupportedPeriod = errors.New("unsupported period")

const (
	// fetchPageSize — размер страницы при постраничном вычитывании тикетов.
	// Совпадает с максимальным limit, который принудительно применяет gRPC
	// сервер основного сервиса (internal/transport/grpc/server.go зажимает
	// limit до 100) — если запросить больше, ответ молча обрежется, и
	// инкремент offset разъедется с реальным количеством записей.
	fetchPageSize = 100

	// maxFetchPages — защита от неограниченного цикла на случай неожиданно
	// большого количества тикетов (100 * 200 = 20000 тикетов максимум).
	maxFetchPages = 200

	// resolutionSLA — порог, начиная с которого незавершённый тикет считается
	// нарушившим SLA. ticket.proto не передаёт response/resolution deadlines
	// через gRPC, поэтому вместо реальных дедлайнов используется фиксированное
	// приближение.
	resolutionSLA = 72 * time.Hour

	// recentActivityLimit — сколько последних записей истории подтягивать
	// для UserStats.RecentActivity.
	recentActivityLimit = 5

	// Периоды таймлайна, поддерживаемые API.
	Period7Days  = "7d"
	Period30Days = "30d"
)

// periodDays переводит поддерживаемый период в количество дней.
var periodDays = map[string]int{
	Period7Days:  7,
	Period30Days: 30,
}

// terminalStatuses — статусы, которые считаются "завершёнными" для расчёта
// среднего времени решения и SLA.
var terminalStatuses = map[string]bool{
	"resolved":  true,
	"closed":    true,
	"cancelled": true,
}

// TicketClientInterface — то, что агрегатору нужно от gRPC-клиента к
// основному сервису pet-ticket. Агрегатор работает через интерфейс, а не
// напрямую через *grpc.TicketClient: это позволяет юнит-тестировать всю
// агрегацию с мок-клиентом, без реального сетевого соединения и без Docker.
type TicketClientInterface interface {
	ListTickets(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error)
	GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int32) (*ticketv1.GetTicketHistoryResponse, error)
	GetAllStatuses(ctx context.Context) (*ticketv1.GetAllStatusesResponse, error)
	GetAllTopics(ctx context.Context) (*ticketv1.GetAllTopicsResponse, error)
}

// Aggregator считает метрики поверх данных, полученных через gRPC от
// основного сервиса. Сам по себе он не хранит состояния (кеша) — этим
// занимается Service, который оборачивает Aggregator кешем.
type Aggregator struct {
	client TicketClientInterface
}

// NewAggregator создаёт агрегатор поверх переданного gRPC-клиента.
func NewAggregator(client TicketClientInterface) *Aggregator {
	return &Aggregator{client: client}
}

// fetchAllTickets постранично вычитывает тикеты через ListTickets и
// склеивает их в один срез. userID/topicID/statusID — опциональные фильтры,
// nil означает "без фильтра по этому полю".
func (a *Aggregator) fetchAllTickets(ctx context.Context, userID, topicID *int64, statusID *int32) ([]*ticketv1.Ticket, error) {
	var all []*ticketv1.Ticket

	for page := 0; page < maxFetchPages; page++ {
		req := &ticketv1.ListTicketsRequest{
			UserId:   userID,
			TopicId:  topicID,
			StatusId: statusID,
			Limit:    fetchPageSize,
			Offset:   int32(page) * fetchPageSize,
			SortBy:   "created_at",
		}

		resp, err := a.client.ListTickets(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list tickets (offset %d): %w", req.Offset, err)
		}

		all = append(all, resp.GetTickets()...)

		if len(resp.GetTickets()) < fetchPageSize {
			break
		}
	}

	return all, nil
}

// statusCounts группирует тикеты по status_name. knownStatuses (если
// непустой) гарантирует, что в результате будут представлены все статусы из
// справочника, даже если по какому-то из них сейчас 0 тикетов — для
// дашборда это полезнее, чем "тихое" отсутствие статуса в ответе.
func statusCounts(ticketsList []*ticketv1.Ticket, knownStatuses []string) []StatusCount {
	counts := make(map[string]int64, len(knownStatuses))
	for _, name := range knownStatuses {
		counts[name] = 0
	}
	for _, t := range ticketsList {
		counts[t.GetStatusName()]++
	}

	result := make([]StatusCount, 0, len(counts))
	for name, count := range counts {
		result = append(result, StatusCount{Status: name, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Status < result[j].Status })

	return result
}

// avgResolutionMinutes считает среднее время между created_at и updated_at
// для "завершённых" тикетов. updated_at здесь — приближение к моменту
// решения: proto не передаёт отдельного resolved_at, но для
// resolved/closed/cancelled тикетов updated_at обычно и есть момент
// последнего изменения статуса.
func avgResolutionMinutes(ticketsList []*ticketv1.Ticket) float64 {
	var total float64
	var count int64

	for _, t := range ticketsList {
		if !terminalStatuses[t.GetStatusName()] {
			continue
		}
		created := t.GetCreatedAt().AsTime()
		updated := t.GetUpdatedAt().AsTime()
		if updated.Before(created) {
			continue
		}
		total += updated.Sub(created).Minutes()
		count++
	}

	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// slaSummary считает нарушения SLA приближённо (см. комментарий к resolutionSLA).
func slaSummary(ticketsList []*ticketv1.Ticket, now time.Time) SLASummary {
	var resolved, violated int64

	for _, t := range ticketsList {
		if terminalStatuses[t.GetStatusName()] {
			resolved++
			continue
		}
		if now.Sub(t.GetCreatedAt().AsTime()) > resolutionSLA {
			violated++
		}
	}

	rate := 1.0
	if total := len(ticketsList); total > 0 {
		rate = 1 - float64(violated)/float64(total)
	}

	return SLASummary{
		ResolvedCount:  resolved,
		ViolatedCount:  violated,
		ComplianceRate: rate,
	}
}

// Overview агрегирует общую сводку по всем тикетам.
func (a *Aggregator) Overview(ctx context.Context) (Overview, error) {
	statusesResp, err := a.client.GetAllStatuses(ctx)
	if err != nil {
		return Overview{}, fmt.Errorf("failed to get statuses: %w", err)
	}
	knownStatuses := make([]string, 0, len(statusesResp.GetStatuses()))
	for _, st := range statusesResp.GetStatuses() {
		knownStatuses = append(knownStatuses, st.GetName())
	}

	ticketsList, err := a.fetchAllTickets(ctx, nil, nil, nil)
	if err != nil {
		return Overview{}, err
	}

	now := time.Now()
	return Overview{
		TotalTickets:         int64(len(ticketsList)),
		ByStatus:             statusCounts(ticketsList, knownStatuses),
		AvgResolutionMinutes: avgResolutionMinutes(ticketsList),
		SLA:                  slaSummary(ticketsList, now),
		GeneratedAt:          now,
	}, nil
}

// UserStats агрегирует статистику по тикетам конкретного пользователя и
// подтягивает историю его самого недавно изменённого тикета (recent
// activity) — единственное место, где агрегатору нужен GetTicketHistory.
func (a *Aggregator) UserStats(ctx context.Context, userID int64) (UserStats, error) {
	ticketsList, err := a.fetchAllTickets(ctx, &userID, nil, nil)
	if err != nil {
		return UserStats{}, err
	}

	var lastTicketAt *time.Time
	var mostRecentTicketID int64
	var mostRecentUpdatedAt time.Time

	for _, t := range ticketsList {
		createdAt := t.GetCreatedAt().AsTime()
		if lastTicketAt == nil || createdAt.After(*lastTicketAt) {
			ct := createdAt
			lastTicketAt = &ct
		}

		updatedAt := t.GetUpdatedAt().AsTime()
		if updatedAt.After(mostRecentUpdatedAt) {
			mostRecentUpdatedAt = updatedAt
			mostRecentTicketID = t.GetId()
		}
	}

	var recentActivity []HistoryEntry
	if mostRecentTicketID != 0 {
		historyResp, err := a.client.GetTicketHistory(ctx, mostRecentTicketID, recentActivityLimit, 0)
		if err != nil {
			return UserStats{}, fmt.Errorf("failed to get ticket history for ticket %d: %w", mostRecentTicketID, err)
		}
		for _, rec := range historyResp.GetRecords() {
			recentActivity = append(recentActivity, HistoryEntry{
				Action:    rec.GetAction(),
				OldValue:  rec.GetOldValue(),
				NewValue:  rec.GetNewValue(),
				CreatedAt: rec.GetCreatedAt().AsTime(),
			})
		}
	}

	return UserStats{
		UserID:               userID,
		TotalTickets:         int64(len(ticketsList)),
		ByStatus:             statusCounts(ticketsList, nil),
		AvgResolutionMinutes: avgResolutionMinutes(ticketsList),
		LastTicketAt:         lastTicketAt,
		RecentActivity:       recentActivity,
		GeneratedAt:          time.Now(),
	}, nil
}

// TopicStats агрегирует статистику по каждой теме из справочника.
// Справочник тем берётся отдельным вызовом GetAllTopics, а не выводится из
// встречавшихся в тикетах topic_id — так в отчёте видны и темы без единого
// тикета (с нулевым счётчиком), что полезнее для дашборда.
func (a *Aggregator) TopicStats(ctx context.Context) (TopicsOverview, error) {
	topicsResp, err := a.client.GetAllTopics(ctx)
	if err != nil {
		return TopicsOverview{}, fmt.Errorf("failed to get topics: %w", err)
	}

	ticketsList, err := a.fetchAllTickets(ctx, nil, nil, nil)
	if err != nil {
		return TopicsOverview{}, err
	}

	byTopic := make(map[int64][]*ticketv1.Ticket)
	for _, t := range ticketsList {
		byTopic[t.GetTopicId()] = append(byTopic[t.GetTopicId()], t)
	}

	result := make([]TopicStat, 0, len(topicsResp.GetTopics()))
	for _, topic := range topicsResp.GetTopics() {
		topicTickets := byTopic[topic.GetId()]
		result = append(result, TopicStat{
			TopicID:      topic.GetId(),
			Title:        topic.GetTitle(),
			TotalTickets: int64(len(topicTickets)),
			ByStatus:     statusCounts(topicTickets, nil),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].TopicID < result[j].TopicID })

	return TopicsOverview{
		Topics:      result,
		GeneratedAt: time.Now(),
	}, nil
}

// Timeline строит посуточную статистику создания/решения тикетов за период
// ("7d" или "30d"). Proto не поддерживает фильтр по дате создания, поэтому
// приходится вычитывать все тикеты и группировать их на своей стороне.
func (a *Aggregator) Timeline(ctx context.Context, period string) (Timeline, error) {
	days, ok := periodDays[period]
	if !ok {
		return Timeline{}, fmt.Errorf("%w: %q (expected %q or %q)", ErrUnsupportedPeriod, period, Period7Days, Period30Days)
	}

	ticketsList, err := a.fetchAllTickets(ctx, nil, nil, nil)
	if err != nil {
		return Timeline{}, err
	}

	now := time.Now().UTC()
	startDate := now.AddDate(0, 0, -days+1)

	buckets := make(map[string]*TimelinePoint, days)
	order := make([]string, 0, days)
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i).Format("2006-01-02")
		buckets[date] = &TimelinePoint{Date: date}
		order = append(order, date)
	}

	for _, t := range ticketsList {
		createdDate := t.GetCreatedAt().AsTime().UTC().Format("2006-01-02")
		if point, ok := buckets[createdDate]; ok {
			point.Created++
		}

		if terminalStatuses[t.GetStatusName()] {
			resolvedDate := t.GetUpdatedAt().AsTime().UTC().Format("2006-01-02")
			if point, ok := buckets[resolvedDate]; ok {
				point.Resolved++
			}
		}
	}

	points := make([]TimelinePoint, 0, days)
	for _, date := range order {
		points = append(points, *buckets[date])
	}

	return Timeline{
		Period:      period,
		Points:      points,
		GeneratedAt: now,
	}, nil
}

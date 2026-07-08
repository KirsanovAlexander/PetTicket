package analytics

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"
)

const (
	keyOverview        = "analytics:overview"
	keyTopics          = "analytics:topics"
	keyUserStatsPrefix = "analytics:user:"
	keyTimelinePrefix  = "analytics:timeline:"
)

// Cache — то, что сервису нужно от кеша: положить/забрать значение по ключу.
// TTL — деталь конкретной реализации (задаётся один раз при создании кеша,
// не на каждый Set), поэтому в интерфейсе его нет. Узкий интерфейс вместо
// полного API redis.Client упрощает мокирование в тестах и не привязывает
// бизнес-логику к конкретному клиенту.
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) (bool, error)
	Set(ctx context.Context, key string, value interface{}) error
}

// Service — бизнес-слой аналитики: оборачивает Aggregator кешем.
// Правило одинаковое для всех методов: сначала пробуем кеш; при промахе или
// ошибке чтения кеша считаем метрику через агрегатор напрямую (недоступность
// Redis не должна ронять API — это и есть graceful fallback из AC) и
// best-effort кладём результат обратно в кеш.
type Service struct {
	aggregator *Aggregator
	cache      Cache
	logger     zerolog.Logger
}

// NewService создаёт сервис аналитики.
func NewService(aggregator *Aggregator, cache Cache, logger zerolog.Logger) *Service {
	return &Service{
		aggregator: aggregator,
		cache:      cache,
		logger:     logger.With().Str("module", "analytics_service").Logger(),
	}
}

// GetOverview возвращает сводную статистику, пробуя сначала кеш.
func (s *Service) GetOverview(ctx context.Context) (Overview, error) {
	var overview Overview
	if hit, err := s.cache.Get(ctx, keyOverview, &overview); err != nil {
		s.logger.Warn().Err(err).Str("key", keyOverview).Msg("cache read failed, falling back to live aggregation")
	} else if hit {
		return overview, nil
	}

	overview, err := s.aggregator.Overview(ctx)
	if err != nil {
		return Overview{}, fmt.Errorf("failed to aggregate overview: %w", err)
	}

	s.setCache(ctx, keyOverview, overview)
	return overview, nil
}

// GetUserStats возвращает статистику по пользователю, пробуя сначала кеш.
func (s *Service) GetUserStats(ctx context.Context, userID int64) (UserStats, error) {
	key := keyUserStatsPrefix + strconv.FormatInt(userID, 10)

	var stats UserStats
	if hit, err := s.cache.Get(ctx, key, &stats); err != nil {
		s.logger.Warn().Err(err).Str("key", key).Msg("cache read failed, falling back to live aggregation")
	} else if hit {
		return stats, nil
	}

	stats, err := s.aggregator.UserStats(ctx, userID)
	if err != nil {
		return UserStats{}, fmt.Errorf("failed to aggregate user stats: %w", err)
	}

	s.setCache(ctx, key, stats)
	return stats, nil
}

// GetTopicStats возвращает статистику по темам, пробуя сначала кеш.
func (s *Service) GetTopicStats(ctx context.Context) (TopicsOverview, error) {
	var topics TopicsOverview
	if hit, err := s.cache.Get(ctx, keyTopics, &topics); err != nil {
		s.logger.Warn().Err(err).Str("key", keyTopics).Msg("cache read failed, falling back to live aggregation")
	} else if hit {
		return topics, nil
	}

	topics, err := s.aggregator.TopicStats(ctx)
	if err != nil {
		return TopicsOverview{}, fmt.Errorf("failed to aggregate topic stats: %w", err)
	}

	s.setCache(ctx, keyTopics, topics)
	return topics, nil
}

// GetTimeline возвращает таймлайн за период ("7d" или "30d"), пробуя сначала кеш.
func (s *Service) GetTimeline(ctx context.Context, period string) (Timeline, error) {
	key := keyTimelinePrefix + period

	var timeline Timeline
	if hit, err := s.cache.Get(ctx, key, &timeline); err != nil {
		s.logger.Warn().Err(err).Str("key", key).Msg("cache read failed, falling back to live aggregation")
	} else if hit {
		return timeline, nil
	}

	timeline, err := s.aggregator.Timeline(ctx, period)
	if err != nil {
		return Timeline{}, fmt.Errorf("failed to aggregate timeline: %w", err)
	}

	s.setCache(ctx, key, timeline)
	return timeline, nil
}

// setCache — best-effort запись в кеш: ошибка записи не должна ломать ответ,
// который мы уже успешно посчитали, поэтому только логируем предупреждение.
func (s *Service) setCache(ctx context.Context, key string, value interface{}) {
	if err := s.cache.Set(ctx, key, value); err != nil {
		s.logger.Warn().Err(err).Str("key", key).Msg("failed to write cache")
	}
}

// RefreshBackground пересчитывает overview/topics/timeline(7d,30d) напрямую
// через агрегатор и кладёт результаты в кеш, минуя чтение. Вызывается
// фоновым тикером — так большинство HTTP-запросов попадают в уже тёплый кеш,
// а не ждут агрегации по 100+ тикетов на лету.
//
// UserStats сюда намеренно не входит: пользователей может быть сколько
// угодно, прогревать статистику по каждому не масштабируется — для неё
// кеширование остаётся ленивым, по факту запроса (см. GetUserStats).
func (s *Service) RefreshBackground(ctx context.Context) {
	if overview, err := s.aggregator.Overview(ctx); err != nil {
		s.logger.Error().Err(err).Msg("background refresh: overview failed")
	} else {
		s.setCache(ctx, keyOverview, overview)
	}

	if topics, err := s.aggregator.TopicStats(ctx); err != nil {
		s.logger.Error().Err(err).Msg("background refresh: topics failed")
	} else {
		s.setCache(ctx, keyTopics, topics)
	}

	for _, period := range []string{Period7Days, Period30Days} {
		timeline, err := s.aggregator.Timeline(ctx, period)
		if err != nil {
			s.logger.Error().Err(err).Str("period", period).Msg("background refresh: timeline failed")
			continue
		}
		s.setCache(ctx, keyTimelinePrefix+period, timeline)
	}

	s.logger.Info().Msg("background refresh completed")
}

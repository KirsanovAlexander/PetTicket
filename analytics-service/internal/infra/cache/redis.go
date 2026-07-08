package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache — обёртка над go-redis, реализует analytics.Cache.
// Хранит значения как JSON: это чуть менее компактно, чем бинарный формат,
// зато данные остаются человекочитаемыми при отладке через redis-cli
// (GET analytics:overview сразу показывает осмысленный JSON).
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCache создаёт клиент Redis. ttl применяется ко всем ключам,
// записанным через Set — единый TTL для всего сервиса проще конфигурировать
// и достаточен для этой задачи (в отличие от TTL на каждый вызов Set).
func NewRedisCache(addr, password string, db int, ttl time.Duration) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisCache{client: client, ttl: ttl}
}

// Get пытается прочитать значение по ключу и распаковать его в dest.
// Промах кеша (ключ не найден) — это НЕ ошибка: возвращается (false, nil),
// и вызывающий код должен просто посчитать значение заново.
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get cache key %q: %w", key, err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		// false, а не true: раз распаковать не удалось, dest нельзя считать
		// валидным результатом, даже если часть полей успела заполниться.
		return false, fmt.Errorf("failed to unmarshal cache value for key %q: %w", key, err)
	}

	return true, nil
}

// Set сериализует value в JSON и сохраняет с TTL, заданным при создании кеша.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value for key %q: %w", key, err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache key %q: %w", key, err)
	}

	return nil
}

// Ping проверяет доступность Redis (используется при старте сервиса, чтобы
// сразу видеть в логах проблему с подключением, а не только на первом запросе).
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close закрывает соединение с Redis.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

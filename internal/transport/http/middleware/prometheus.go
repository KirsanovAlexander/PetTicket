package middleware

import (
	"runtime"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served",
		},
	)

	apiVersionRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_version_requests_total",
			Help: "Total number of HTTP requests per API version (v1/v2)",
		},
		[]string{"version", "method", "path", "status"},
	)

	// Системные метрики
	memoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)

	memoryAlloc = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memory_alloc_bytes",
			Help: "Allocated memory in bytes",
		},
	)

	goroutinesCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "goroutines_count",
			Help: "Current number of goroutines",
		},
	)
)

func init() {
	// Запускаем сбор системных метрик
	go collectSystemMetrics()
}

func collectSystemMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		memoryUsage.Set(float64(m.Sys))
		memoryAlloc.Set(float64(m.Alloc))
		goroutinesCount.Set(float64(runtime.NumGoroutine()))
	}
}

// PrometheusMiddleware создаёт middleware для сбора метрик
func PrometheusMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Пропускаем метрики endpoint чтобы не создавать рекурсию
		if c.Path() == "/metrics" {
			return c.Next()
		}

		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		err := c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)

		return err
	}
}

// APIVersionMetricsMiddleware считает запросы в разрезе версии API (v1/v2)
// — отдельно от общих http_requests_total, чтобы видеть скорость перехода
// клиентов с v1 на v2 (например, для решения "когда можно выключать v1").
func APIVersionMetricsMiddleware(version string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		status := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}

		apiVersionRequestsTotal.WithLabelValues(version, method, path, status).Inc()

		return err
	}
}

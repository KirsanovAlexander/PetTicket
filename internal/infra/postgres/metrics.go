package postgres

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Number of open database connections",
		},
	)

	dbConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	dbConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	dbConnectionsWaitCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_wait_count",
			Help: "Total number of connections waited for",
		},
	)

	dbConnectionsWaitDuration = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_wait_duration_seconds",
			Help: "Total time blocked waiting for new connections",
		},
	)

	dbConnectionsMaxIdleClosed = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_max_idle_closed",
			Help: "Total number of connections closed due to SetMaxIdleConns",
		},
	)

	dbConnectionsMaxLifetimeClosed = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_max_lifetime_closed",
			Help: "Total number of connections closed due to SetConnMaxLifetime",
		},
	)
)

// StartMetricsCollector запускает горутину для сбора метрик БД
func (db *DB) StartMetricsCollector() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			db.collectDBMetrics()
		}
	}()
}

func (db *DB) collectDBMetrics() {
	stats := db.conn.Stats()

	dbConnectionsOpen.Set(float64(stats.OpenConnections))
	dbConnectionsInUse.Set(float64(stats.InUse))
	dbConnectionsIdle.Set(float64(stats.Idle))
	dbConnectionsWaitCount.Set(float64(stats.WaitCount))
	dbConnectionsWaitDuration.Set(stats.WaitDuration.Seconds())
	dbConnectionsMaxIdleClosed.Set(float64(stats.MaxIdleClosed))
	dbConnectionsMaxLifetimeClosed.Set(float64(stats.MaxLifetimeClosed))
}

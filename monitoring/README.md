# Мониторинг с Prometheus и Grafana

Этот каталог содержит конфигурацию для мониторинга приложения через Prometheus и Grafana.

## Структура

```
monitoring/
├── prometheus.yml                    # Конфигурация Prometheus
└── grafana/
    └── provisioning/
        └── datasources/
            └── datasource.yml        # Автоматическая настройка datasource
```

## Конфигурация Prometheus

`prometheus.yml` настраивает scrape targets:
- `localhost:9090` - сам Prometheus
- `app:9000` - метрики pet-ticket приложения

## Конфигурация Grafana

`grafana/provisioning/datasources/datasource.yml` автоматически добавляет Prometheus как datasource при старте контейнера.

### Доступ к Grafana

1. Откройте браузер и перейдите на `http://localhost:3000`
2. Войдите с учетными данными:
   - Username: `admin`
   - Password: `admin`
3. **Dashboard автоматически доступен!**
   - Перейдите на главную страницу или в **Dashboards → Browse**
   - Найдите "Pet Ticket - Application Overview"
   - Или прямая ссылка: http://localhost:3000/d/pet-ticket-overview

### Подключение Prometheus к Grafana

Prometheus уже автоматически настроен через provisioning. Datasource создается при старте Grafana со следующими параметрами:
- **Name**: Prometheus
- **Type**: Prometheus
- **URL**: `http://prometheus:9090` (внутренний Docker DNS)
- **Access**: Server (proxy)

**Примечание**: При нажатии кнопки "Save & Test" в UI Grafana может показать ошибку `connection refused`, так как браузер пытается подключиться к `localhost:9090`. Это нормально - datasource работает корректно внутри Docker-сети. Для проверки создайте dashboard с запросом к Prometheus (например, `up`).

## Встроенный Dashboard

При старте Grafana автоматически загружается dashboard **"Pet Ticket - Application Overview"** с **17 панелями мониторинга**:

### HTTP метрики
- **HTTP Requests Rate** - частота запросов (req/s) с разбивкой по методам, путям и статусам
- **HTTP Requests In Flight** - текущие запросы в обработке
- **HTTP Request Duration (p95)** - 95-й перцентиль времени ответа
- **HTTP Request Duration Percentiles** - p50, p90, p95, p99 для детального анализа
- **Total HTTP Requests** (stat) - общее количество запросов

### Метрики базы данных
- **Database Connections** - открытые, используемые и простаивающие соединения
- **DB Connection Wait Time** - время ожидания свободного соединения
- **DB Connection Wait Count** - количество ожиданий
- **DB Connections Closed** - соединения закрытые по idle и lifetime лимитам
- **Open DB Connections** (stat) - текущее количество открытых соединений

### Системные метрики
- **Memory Usage** - использование памяти (System Memory и Allocated Memory)
- **Memory Usage (MB)** (stat) - текущее использование в мегабайтах
- **Go Memory Stats** - детальная статистика кучи (Heap Alloc, In Use, Idle)
- **Goroutines** - количество горутин (custom metric + go runtime)
- **Current Goroutines** (stat) - текущее количество горутин
- **Go Threads** - количество OS потоков
- **GC Duration** - время выполнения garbage collection

Dashboard доступен сразу после запуска по адресу: http://localhost:3000/d/pet-ticket-overview

**Совет:** Dashboard автоматически обновляется каждые 30 секунд. Вы можете изменить панели через UI - изменения сохраняются благодаря `allowUiUpdates: true`.

## Создание своих Dashboard в Grafana

После подключения к Grafana вы можете создать dashboard с метриками приложения:

1. Перейдите в **Dashboards → New → New Dashboard**
2. Добавьте панели с запросами (примеры):

### Примеры запросов для панелей:

**Memory Usage:**
```promql
memory_usage_bytes
```

**DB Connections:**
```promql
db_connections_open
db_connections_in_use
db_connections_idle
```

**HTTP Requests Rate:**
```promql
rate(http_requests_total[5m])
```

**Goroutines:**
```promql
goroutines_count
```

**HTTP Request Duration (p95):**
```promql
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

## Добавление метрик в приложение

Для добавления метрик в pet-ticket используйте библиотеку `github.com/prometheus/client_golang/prometheus`:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
)
```

Метрики будут доступны на `http://localhost:9000/metrics`

## Доступные метрики

Приложение автоматически экспортирует следующие метрики:

### HTTP метрики
- `http_requests_total` - Количество HTTP запросов (labels: method, path, status)
- `http_request_duration_seconds` - Длительность HTTP запросов в секундах
- `http_requests_in_flight` - Количество запросов в обработке

### Метрики базы данных
- `db_connections_open` - Количество открытых соединений с БД
- `db_connections_in_use` - Количество используемых соединений
- `db_connections_idle` - Количество простаивающих соединений
- `db_connections_wait_count` - Количество ожиданий соединений
- `db_connections_wait_duration_seconds` - Время ожидания соединений
- `db_connections_max_idle_closed` - Соединений закрыто по MaxIdleConns
- `db_connections_max_lifetime_closed` - Соединений закрыто по ConnMaxLifetime

### Системные метрики
- `memory_usage_bytes` - Использование памяти (Sys)
- `memory_alloc_bytes` - Выделенная память (Alloc)
- `goroutines_count` - Количество горутин

### Go runtime метрики
Стандартные метрики Go (автоматически от Prometheus client):
- `go_goroutines` - Количество горутин
- `go_memstats_*` - Статистика памяти
- `go_gc_duration_seconds` - Время garbage collection

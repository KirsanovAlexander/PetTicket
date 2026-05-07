# Docker — развёртывание

## Быстрый старт

### Docker Compose (рекомендуется)

Запустите все сервисы одной командой:

```bash
docker-compose up -d
```

После запуска доступны:
- **API сервер**: http://localhost:9000
- **gRPC сервер**: localhost:9001
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000

**Логины для Grafana:**
- Username: `admin`
- Password: `admin`

### Остановка сервисов

```bash
docker-compose down
```

### Логи

```bash
# Все сервисы
docker-compose logs -f

# Только приложение
docker-compose logs -f app

# Только Grafana
docker-compose logs -f grafana
```

---

## Мониторинг с Grafana

### Первый вход в Grafana

1. Откройте http://localhost:3000
2. Войдите с credentials:
   - **Username**: `admin`
   - **Password**: `admin`
3. Grafana предложит сменить пароль (можно пропустить)

### Prometheus уже настроен

Datasource Prometheus автоматически настроен при старте через:
- `monitoring/grafana/provisioning/datasources/datasource.yml`

Проверить можно: **Configuration → Data Sources → Prometheus**

### Создание дашборда в Grafana

1. Перейдите в **Dashboards → New → New Dashboard**
2. Добавьте панель → выберите Prometheus datasource
3. Примеры запросов:

**HTTP request duration (p95):**
```promql
histogram_quantile(0.95, 
  rate(http_request_duration_seconds_bucket[5m])
)
```

**Request rate по endpoint:**
```promql
rate(http_requests_total[5m])
```

**Error rate:**
```promql
rate(http_requests_total{status=~"5.."}[5m])
```

**Goroutines count:**
```promql
go_goroutines
```

### Импорт готовых дашбордов из Grafana Labs

1. Перейдите в **Dashboards → Import**
2. Введите ID дашборда (например, для Go приложений):
   - **Go Metrics**: `10826`
   - **HTTP Stats**: `3662`
   - **PostgreSQL**: `9628`

---

## Сборка образа

Для текущей архитектуры (определяется автоматически):
```bash
docker build -t pet-ticket:latest .
```

Мультиплатформенная сборка (amd64 + arm64):
```bash
docker buildx build --platform linux/amd64,linux/arm64 -t pet-ticket:latest .
```

Для конкретной архитектуры:
```bash
# amd64 (Intel/AMD)
docker buildx build --platform linux/amd64 -t pet-ticket:latest .

# arm64 (Apple Silicon, ARM серверы)
docker buildx build --platform linux/arm64 -t pet-ticket:latest .
```

### Запуск контейнера

```bash
docker run -d \
  --name pet-ticket \
  -p 9000:9000 \
  -e POSTGRES_HOST=host.docker.internal \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_DATABASE=pet_ticket \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e ENV=production \
  pet-ticket:latest
```

### Через Docker Compose

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: pet_ticket
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  app:
    build: .
    ports:
      - "9000:9000"
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_PORT: 5432
      POSTGRES_DATABASE: pet_ticket
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      ENV: production
      POSTGRES_ENABLED_MIGRATION: true
    depends_on:
      - postgres

volumes:
  postgres_data:
```

Запуск:
```bash
docker-compose up -d
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|-------------|----------|
| `ENV` | `local` | Окружение (local/production) |
| `LISTEN` | `:9000` | Адрес HTTP сервера |
| `POSTGRES_HOST` | `localhost` | Хост PostgreSQL |
| `POSTGRES_PORT` | `5432` | Порт PostgreSQL |
| `POSTGRES_DATABASE` | `pet_ticket` | Имя базы данных |
| `POSTGRES_USER` | `postgres` | Пользователь БД |
| `POSTGRES_PASSWORD` | - | Пароль БД |
| `POSTGRES_ENABLED_MIGRATION` | `true` | Запускать миграции при старте |
| `LOGGER_LEVEL` | `info` | Уровень логирования (debug/info/warn/error) |
| `LOGGER_FORMAT` | `json` | Формат логов (console/json) |

## Многоступенчатая сборка

Dockerfile использует multi-stage build с кроссплатформенной поддержкой:

1. **Builder** (`golang:1.24-alpine`)
   - Скачивает зависимости
   - Компилирует Go-бинарник с оптимизациями
   - Поддерживает amd64 и arm64 через `ARG TARGETARCH`
   - Размер бинарника: ~10-15 МБ (с `-ldflags="-w -s"`)

2. **Runtime** (`alpine:latest`)
   - Минимальный образ (~5 МБ)
   - Только рантайм-зависимости (ca-certificates, tzdata)
   - Итоговый образ: ~20-25 МБ

### Поддержка архитектур

Dockerfile автоматически определяет целевую архитектуру через `ARG TARGETARCH`:
- **amd64** (Intel/AMD x86_64)
- **arm64** (Apple Silicon M1/M2/M3, AWS Graviton)

Build-аргументы устанавливаются автоматически через Docker buildx:
```dockerfile
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build ...
```

## Безопасность

Реализовано:
- Без sensitive-данных в образе
- Минимальная поверхность атаки (alpine base)
- Статический бинарник (без рантайм-зависимостей)

Опционально:
```dockerfile
# Add non-root user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

USER app

# Запуск без root
CMD ["./pet-ticket"]
```

## Health Check

Добавить в Dockerfile:
```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9000/api/v1/healthcheck || exit 1
```

Или в docker-compose:
```yaml
healthcheck:
  test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:9000/api/v1/healthcheck"]
  interval: 30s
  timeout: 3s
  retries: 3
  start_period: 5s
```

## Развёртывание в продакшене

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pet-ticket
spec:
  replicas: 3
  selector:
    matchLabels:
      app: pet-ticket
  template:
    metadata:
      labels:
        app: pet-ticket
    spec:
      containers:
      - name: pet-ticket
        image: pet-ticket:latest
        ports:
        - containerPort: 9000
        env:
        - name: POSTGRES_HOST
          value: "postgres-service"
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
        livenessProbe:
          httpGet:
            path: /api/v1/healthcheck
            port: 9000
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/healthcheck
            port: 9000
          initialDelaySeconds: 3
          periodSeconds: 5
```

### Docker Swarm

```bash
docker service create \
  --name pet-ticket \
  --replicas 3 \
  --publish 9000:9000 \
  --env POSTGRES_HOST=postgres \
  --env POSTGRES_PASSWORD=secret \
  pet-ticket:latest
```

## Решение проблем

### Сборка падает на Apple Silicon (M1/M2/M3)

**Ошибка:**
```
exec format error
```

**Причина:** сборка под неправильную архитектуру (amd64 на arm64)

**Решение:** убрать явную архитектуру или использовать buildx:
```bash
# Вариант 1: buildx для текущей платформы
docker buildx build --platform linux/arm64 -t pet-ticket:latest .

# Вариант 2: обычная сборка (автоопределение)
docker build -t pet-ticket:latest .
```

### Ошибка версии Go при сборке образа

**Ошибка:**
```
go: go.mod requires go >= 1.24.0 (running go 1.23.12)
```

**Решение:** обновить первую строку Dockerfile:
```dockerfile
FROM golang:1.24-alpine AS builder
```

### Контейнер сразу завершается

Проверьте логи:
```bash
docker logs pet-ticket
```

Частые причины:
- Не заданы переменные окружения
- Не удалось подключиться к БД
- Порт уже занят

### Ошибка подключения к БД

Убедитесь, что PostgreSQL доступен:
```bash
# С хоста в контейнер
docker run --rm -it postgres:16-alpine psql -h host.docker.internal -U postgres

# Между контейнерами (по имени сервиса)
docker-compose exec app ping postgres
```

## Оптимизация производительности

### Оптимизация сборки

```dockerfile
# Кэширование слоя зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Параллельная компиляция
RUN go build -j $(nproc) ...
```

### Оптимизация рантайма

Настройка пула соединений PostgreSQL:
```bash
-e POSTGRES_MAX_OPEN_CONN=100 \
-e POSTGRES_MAX_IDLE_CONN=10 \
-e POSTGRES_CONN_MAX_LIFETIME=3600
```

## Сравнение размеров образов

| Стадия | Размер | Описание |
|--------|--------|----------|
| Builder | ~300 МБ | golang:1.24-alpine + зависимости |
| Runtime | ~20 МБ | alpine + бинарник + ca-certificates |
| **Итого** | **~20 МБ** | Оптимизирован для продакшена |

Для сравнения с полным образом Go:
- `golang:1.24` (не alpine): ~800 МБ
- Оптимизированный образ: ~20 МБ (на 97% меньше)

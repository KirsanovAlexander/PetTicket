# Pet Ticket - Система управления тикетами

Упрощенная версия ticket сервиса, созданная как пет-проект для демонстрации архитектуры и подходов.

## Архитектура

Проект построен по принципу Clean Architecture с разделением на слои:

- **internal/domain** - доменные модели (чистые, без тегов)
- **internal/app** - бизнес-логика (use cases) и интерфейсы портов
- **internal/infra** - инфраструктура (config, postgres, migrations)
- **internal/transport/http** - HTTP транспорт (Fiber) и DTO
- **internal/transport/grpc** - gRPC транспорт
- **cmd/api-server** - точка входа приложения (`main.go`)

## Технологии

- **Go 1.24** - язык программирования
- **Fiber v2** - веб-фреймворк
- **database/sql** - стандартная библиотека для работы с БД (без ORM)
- **PostgreSQL 16** - база данных
- **Zerolog** - структурированное логирование
- **gRPC** - RPC фреймворк
- **Testcontainers** - интеграционные тесты с Docker

## Установка и запуск

### Требования

- Go 1.24+
- PostgreSQL 16+
- Docker (для интеграционных тестов)

### Настройка IDE для тестов

#### GoLand/IntelliJ IDEA

Файл уже настроен, но если нужно вручную:

1. **Settings → Go → Build Tags & Vendoring**
2. Добавьте в "Custom tags": `integration`
3. Или в "OS and Arch": `-tags=integration`

#### VS Code

Создан файл `.vscode/settings.json`:
```json
{
  "go.buildTags": "integration",
  "gopls": {
    "build.buildFlags": ["-tags=integration"]
  }
}
```

После изменений перезапустите Go Language Server:
- **Cmd+Shift+P** → "Go: Restart Language Server"

### Настройка

1. Установите PostgreSQL:

```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# Создайте базу данных
createdb pet_ticket
```

2. Скопируйте `.env.example` в `.env` и настройте параметры:

```bash
cp .env.example .env
```

3. Установите зависимости:

```bash
go mod download
```

4. Запустите приложение:

```bash
go run cmd/api-server/main.go
```

Сервер запустится на порту 9000 (HTTP) и 9001 (gRPC) по умолчанию.

## API Endpoints

### Healthcheck

```
GET /api/v1/healthcheck
```

### Тикеты

**Создать тикет**
```
POST /api/v1/tickets
Content-Type: application/json

{
  "userId": 1,
  "topicId": 1,
  "amount": 100.50,
  "comment": "Описание проблемы"
}
```

**Получить тикет**
```
GET /api/v1/tickets/:id
```

**Обновить тикет**
```
PUT /api/v1/tickets/:id
Content-Type: application/json

{
  "statusId": 2,
  "comment": "Обновленный комментарий"
}
```

**Удалить тикет**
```
DELETE /api/v1/tickets/:id
```

**Список тикетов**
```
GET /api/v1/tickets?userId=1&status=new&limit=20&offset=0
```

**История тикета**
```
GET /api/v1/tickets/:id/history?limit=10&offset=0
```

**Назначить тикет**
```
POST /api/v1/tickets/:id/assign
Content-Type: application/json

{
  "userId": 2
}
```

**Эскалировать тикет**
```
POST /api/v1/tickets/:id/escalate
```

### Справочники

**Получить все статусы**
```
GET /api/v1/statuses
```

**Получить все топики**
```
GET /api/v1/topics
```

## Postman

В репозитории есть готовая коллекция `postman_collection.json` с примерами всех запросов.

### Как использовать

1. Откройте Postman и нажмите **Import**
2. Выберите файл `postman_collection.json` из корня проекта
3. Перейдите в **Environments** и создайте переменную `{{baseUrl}}` = `http://localhost:9000`

Коллекция содержит запросы для всех эндпоинтов: создание, получение, обновление, удаление тикетов, фильтрация, история, справочники.

## Структура проекта

```
.
├── cmd/
│   ├── api-server/          # HTTP + gRPC сервер
│   └── migrate/             # Утилита для миграций
├── internal/
│   ├── domain/              # Доменные модели
│   │   └── tickets/
│   ├── app/                 # Бизнес-логика
│   │   └── tickets/
│   ├── infra/               # Инфраструктура
│   │   ├── config/
│   │   ├── postgres/
│   │   └── migration/
│   └── transport/           # Транспортный слой
│       ├── http/            # HTTP handlers
│       └── grpc/            # gRPC handlers
├── api/
│   ├── proto/               # .proto файлы
│   └── gen/                 # Сгенерированный код
└── pkg/                     # Переиспользуемые пакеты
    └── logger/
```

## Разработка

### Запуск локально

```bash
make run
```

### Сборка

```bash
make build
```

### Линтер

```bash
# Проверка кода
make lint

# Автоматическое исправление
make lint-fix
```

Подробнее см. [LINTER.md](LINTER.md)

### Тестирование

```bash
# Unit-тесты
make test

# Интеграционные тесты (требуется Docker)
make test-integration

# Все тесты
make test-all

# Coverage
make cover
make cover-integration

# Benchmarks
make bench
make bench-integration
```

Подробнее см.:
- [TESTING.md](TESTING.md) — unit-тесты с моками
- [INTEGRATION_TESTS.md](INTEGRATION_TESTS.md) — интеграционные тесты с testcontainers

### Создание новой миграции

Создайте файлы в `internal/infra/migration/migrations/`:
- `XXX_name.up.sql` - применение миграции
- `XXX_name.down.sql` - откат миграции

### Добавление новых endpoints

1. Определите модель в `internal/domain/tickets/`
2. Добавьте бизнес-логику в `internal/app/tickets/service.go`
3. Создайте handler в `internal/transport/http/handlers/`
4. Зарегистрируйте роут в `internal/transport/http/handlers/tickets.go`

## Docker

### Запуск через Docker Compose

```bash
# Запустить все сервисы (app + postgres + prometheus + grafana)
make docker-up

# Остановить
make docker-down

# Пересобрать
make docker-build

# Логи
make docker-logs
```

После запуска доступны:
- **API сервер**: http://localhost:9000
- **gRPC сервер**: localhost:9001
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (логин: `admin`, пароль: `admin`)

### Grafana

После запуска `docker-compose up -d`:

1. Откройте http://localhost:3000
2. Войдите с credentials:
   - **Username**: `admin`
   - **Password**: `admin`
3. Prometheus уже настроен как datasource
4. Создайте дашборд или импортируйте готовые шаблоны

**Рекомендуемые метрики для мониторинга:**
- HTTP request duration
- Request rate по endpoint'ам
- Error rate
- Database connection pool
- Go runtime метрики (goroutines, memory)

Подробнее см. [DOCKER.md](DOCKER.md)

## Миграции

Миграции применяются автоматически при запуске приложения.

Ручное управление:

```bash
# Применить миграции
make migrate-up

# Откатить миграции
make migrate-down
```

## gRPC API

Сервер поддерживает gRPC на порту 9001.

```bash
# Генерация кода из .proto
make proto

# Тестирование через grpcurl
grpcurl -plaintext localhost:9001 list
grpcurl -plaintext localhost:9001 ticket.v1.TicketService/GetTicket
```

Подробнее см. [GRPC.md](GRPC.md)

## Переменные окружения

Основные переменные (см. `.env.example`):

```env
# HTTP сервер
LISTEN=:9000

# gRPC сервер
GRPC_LISTEN=:9001

# База данных
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DATABASE=pet_ticket
POSTGRES_SSLMODE=disable

# Логирование
LOGGER_LEVEL=info
LOGGER_FORMAT=console
```

## Troubleshooting

### База данных не найдена
```bash
createdb pet_ticket
```

### Ошибка подключения
Проверьте, что PostgreSQL запущен:
```bash
brew services list
```

Если не запущен:
```bash
brew services start postgresql@16
```

### Ошибка миграций
Пересоздайте БД:
```bash
dropdb pet_ticket
createdb pet_ticket
go run cmd/api-server/main.go
```

### Интеграционные тесты не запускаются

Проверьте Docker:
```bash
docker info
```

Если не запущен:
```bash
open -a Docker  # macOS
```

### IDE не видит integration тесты

**GoLand:** Settings → Go → Build Tags → добавьте `integration`

**VS Code:** Перезапустите Go Language Server (Cmd+Shift+P → "Go: Restart Language Server")

## Дополнительные материалы

- [QUICKSTART.md](./QUICKSTART.md) — быстрый старт
- [DOCKER.md](./DOCKER.md) — работа с Docker
- [LINTER.md](./LINTER.md) — настройка линтера
- [GRPC.md](./GRPC.md) — gRPC API
- [TESTING.md](./TESTING.md) — unit-тесты с моками
- [INTEGRATION_TESTS.md](./INTEGRATION_TESTS.md) — интеграционные тесты с testcontainers
- [monitoring/README.md](./monitoring/README.md) — мониторинг (Prometheus + Grafana)

## Лицензия

MIT

# Быстрый старт Pet Ticket

## За 3 минуты

```bash
# 1. Установить PostgreSQL (если еще не установлен)
brew install postgresql@16
brew services start postgresql@16

# 2. Создать базу данных
createdb pet_ticket

# 3. Запустить приложение
cp .env.example .env
go run cmd/api-server/main.go
```

Готово! Приложение работает на http://localhost:9000

## PostgreSQL через Docker Compose

Если PostgreSQL не установлен локально, можно поднять через Docker Compose:

```bash
# Поднять только PostgreSQL
docker compose up -d postgres

# Проверить что работает
docker compose ps
docker compose logs postgres
```

После этого в `.env` укажите:
```env
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
```

И запустите приложение как обычно:
```bash
go run cmd/api-server/main.go
```

## Проверка

```bash
# Healthcheck
curl http://localhost:9000/api/v1/healthcheck

# Создать тикет
curl -X POST http://localhost:9000/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"userId":1,"topicId":1,"amount":100.50,"comment":"Тест проблемы с пополнением"}'

# Получить список тикетов (с фильтрами)
curl "http://localhost:9000/api/v1/tickets?limit=20&offset=0"

# Получить список тикетов с фильтром по userId
curl "http://localhost:9000/api/v1/tickets?userId=1&statusId=1"

# Обновить тикет (ID=1, в dev-режиме заголовок X-User-ID необязателен)
curl -X PUT http://localhost:9000/api/v1/tickets/1 \
  -H "Content-Type: application/json" \
  -d '{"statusId":2,"comment":"Взято в работу"}'

# Получить историю тикета
curl http://localhost:9000/api/v1/tickets/1/history
```

## Основные команды

```bash
# Разработка
go run cmd/api-server/main.go   # Запустить приложение
go build ./...                   # Собрать все пакеты
go test ./...                    # Запустить все тесты

# Зависимости
go mod download      # Скачать зависимости
go mod tidy          # Очистить зависимости

# PostgreSQL
createdb pet_ticket       # Создать БД
dropdb pet_ticket         # Удалить БД
psql -d pet_ticket       # Подключиться к БД
psql -d pet_ticket -c "\dt"  # Список таблиц

# Docker (альтернатива локальному PostgreSQL)
docker-compose up -d      # Запустить PostgreSQL в Docker
docker-compose down       # Остановить PostgreSQL
docker-compose down -v    # Остановить и удалить данные
```

## API Endpoints

### Основные
- `GET /api/v1/healthcheck` - проверка здоровья

### Тикеты
- `POST /api/v1/tickets` - создать тикет
- `GET /api/v1/tickets/:id` - получить тикет
- `PUT /api/v1/tickets/:id` - обновить тикет (требует `X-User-ID` в production)
- `GET /api/v1/tickets` - список с фильтрами (query параметры: `limit`, `offset`, `userId`, `topicId`, `statusId`, `sortBy`, `sortDesc`)
- `GET /api/v1/tickets/:id/history` - история тикета
- `POST /api/v1/tickets/:id/assign` - назначить тикет на оператора
- `POST /api/v1/tickets/:id/escalate` - эскалировать тикет

> **Примечание:** В проекте используется справочники статусов (1-5: new, in_progress, resolved, closed, cancelled) и тем (1-4: пополнение, вывод, ошибка пополнения, ошибка вывода), которые хранятся в БД и заполняются при миграциях.

## Примеры запросов

Смотри файл `examples.http` для готовых примеров в IntelliJ IDEA / VSCode REST Client.

## Структура проекта

```
pet-ticket/
├── cmd/
│   └── api-server/
│       └── main.go          # Точка входа
├── internal/
│   ├── domain/
│   │   └── tickets/         # Доменные модели (Ticket, Status, Topic, History)
│   ├── app/
│   │   └── tickets/         # Бизнес-логика (use cases)
│   │       ├── service.go
│   │       ├── repository.go (интерфейс)
│   │       └── errors.go
│   ├── infra/
│   │   ├── config/          # Конфигурация
│   │   ├── postgres/        # Реализация репозитория (SQL запросы)
│   │   │   ├── db.go
│   │   │   └── tickets_repo.go
│   │   └── migration/       # Миграции (PostgreSQL)
│   │       ├── migration.go
│   │       └── migrations/
│   │           ├── 001_init.up.sql
│   │           └── 001_init.down.sql
│   └── transport/
│       └── http/            # HTTP транспорт (Fiber)
│           ├── transport.go
│           ├── handlers.go
│           ├── middleware.go
│           └── dto/
│               └── ticket.go
├── .env.example             # Пример конфигурации
├── .env                     # Ваша конфигурация (не в git)
└── README.md                # Документация
```

## Конфигурация (.env)

```env
# Окружение (local = dev-режим, без обязательного X-User-ID)
ENV=local

# Логирование
LOGGER_LEVEL=info
LOGGER_FORMAT=console
LOG_REPORT_CALLER=true

# HTTP сервер
LISTEN=:9000

# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DATABASE=pet_ticket
POSTGRES_USER=vicf              # Ваш username
POSTGRES_PASSWORD=              # Пустой пароль
POSTGRES_MAX_OPEN_CONN=100
POSTGRES_MAX_IDLE_CONN=10
POSTGRES_CONN_MAX_LIFETIME=3600
POSTGRES_ENABLED_MIGRATION=true # Автоматические миграции при запуске
POSTGRES_SSLMODE=disable
```

## Подключение через DataGrip

```
Host: localhost
Port: 5432
Database: pet_ticket
User: vicf (или ваш username)
Password: (оставьте пустым)
```

## Таблицы в БД

После первого запуска создаются 5 таблиц:

1. **ticket_statuses** - статусы тикетов
   - new (1)
   - in_progress (2)
   - resolved (3)
   - closed (4)
   - cancelled (5)

2. **ticket_topics** - темы тикетов
   - Не дошло пополнение (1)
   - Не дошел вывод (2)
   - Ошибка при пополнении (3)
   - Ошибка при выводе (4)

3. **tickets** - основная таблица тикетов
   - id, user_id, topic_id, status_id, amount, comment
   - created_at, updated_at (с автоматическим триггером)

4. **ticket_history** - история изменений тикетов
   - id, ticket_id, user_id, action, old_value, new_value, created_at

5. **schema_migrations** - служебная таблица миграций

## Troubleshooting

### PostgreSQL не установлен
```bash
brew install postgresql@16
brew services start postgresql@16
```

### База данных не найдена
```bash
createdb pet_ticket
```

### Ошибка подключения "pq: database vicf does not exist"
PostgreSQL пытается подключиться к БД с вашим username. Проверьте переменную `POSTGRES_USER` в `.env`:
```bash
whoami  # Узнайте свой username
# Укажите его в .env или оставьте пустым POSTGRES_USER=
```

### Порт 9000 занят
Измените в `.env`:
```env
LISTEN=:9001
```

### Ошибка миграций
```bash
# Пересоздать БД
dropdb pet_ticket
createdb pet_ticket
go run cmd/api-server/main.go
```

### PostgreSQL не запускается
```bash
# Проверить статус
brew services list

# Перезапустить
brew services restart postgresql@16

# Посмотреть логи
tail -f /opt/homebrew/var/log/postgresql@16.log
```

### Docker вместо локального PostgreSQL
```bash
# Запустить PostgreSQL в Docker
docker-compose up -d

# В .env укажите:
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
```

## Особенности реализации

### Без ORM
Проект использует нативный `database/sql` вместо GORM:
- ✅ Полный контроль над SQL
- ✅ Лучшая производительность
- ✅ Явные ошибки
- ✅ Проще понять что происходит

### Кастомные ошибки
```go
// internal/app/tickets/errors.go
var (
    ErrNotFound      = errors.New("ticket not found")
    ErrInvalidInput  = errors.New("invalid input")
    ErrInvalidStatus = errors.New("invalid status")
)

// Проверка через errors.Is
if errors.Is(err, tickets.ErrNotFound) {
    // Обработка 404
}
```

### Dev/Production режимы
В dev-режиме (`ENV=local`) заголовок `X-User-ID` необязателен для обновления тикета, используется значение по умолчанию (1).

В production режиме заголовок обязателен, иначе возвращается 401.

### Тесты с mock storage
```go
// internal/transport/http/handlers_test.go (если есть)
type mockTicketsService struct{}

func (m *mockTicketsService) GetTicket(ctx context.Context, id int64) (*domain.Ticket, error) {
    return nil, tickets.ErrNotFound
}
```

## Документация

- `README.md` - полная документация
- `QUICKSTART.md` - быстрый старт (этот файл)
- `LINTER.md` - настройка линтера
- `examples.http` - примеры API запросов

## Поддержка

Проект создан как пет-проект на основе реального ticket сервиса.
Использует только публичные библиотеки с GitHub.

**Стек:**
- Go 1.24
- PostgreSQL 16
- Fiber v2
- database/sql (без ORM)
- Zerolog
- golang-migrate
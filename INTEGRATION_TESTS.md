# Интеграционные тесты с Testcontainers

Этот документ описывает интеграционные тесты в проекте pet-ticket с использованием [testcontainers-go](https://golang.testcontainers.org/).

## Что такое Testcontainers?

Testcontainers — это библиотека для запуска Docker контейнеров в тестах. Она позволяет:

- ✅ Тестировать с реальной PostgreSQL вместо моков
- ✅ Изолировать каждый тест (свежая БД для каждого теста)
- ✅ Автоматически очищать ресурсы после тестов
- ✅ Запускать тесты в CI/CD (GitHub Actions, GitLab CI и т.д.)
- ✅ Избежать проблем "works on my machine"

## Требования

### Локальная разработка

1. **Docker** — должен быть запущен
   ```bash
   docker --version
   # Docker version 24.0.0 или выше
   ```

2. **Go 1.24+**
   ```bash
   go version
   # go version go1.24.0 darwin/arm64
   ```

3. **Зависимости**
   ```bash
   go mod download
   ```

### CI/CD

В GitHub Actions Docker уже предустановлен. См. `.github/workflows/integration-tests.yml`.

## Запуск тестов

### Все интеграционные тесты

```bash
make test-integration
```

Или напрямую:

```bash
go test ./... -tags=integration -v
```

### Конкретный пакет

```bash
go test ./internal/infra/postgres/... -tags=integration -v
```

### С таймаутом

Контейнеры могут долго стартовать при первом запуске (скачивание образа):

```bash
go test ./... -tags=integration -timeout 5m
```

### Только unit-тесты (без интеграционных)

```bash
make test
```

### Все тесты (unit + integration)

```bash
make test-all
```

## Структура интеграционных тестов

### Файл с тестами

```
internal/infra/postgres/tickets_repo_integration_test.go
```

Все интеграционные тесты помечены build tag:

```go
//go:build integration

package postgres
```

### Setup функция

Каждый тест использует `setupTestDB(t)` для создания изолированного окружения:

```go
func setupTestDB(t *testing.T) *testDB {
    ctx := context.Background()

    // 1. Создаём PostgreSQL контейнер
    postgresContainer, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("testuser"),
        postgres.WithPassword("testpass"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(60*time.Second),
        ),
    )

    // 2. Получаем connection string
    connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")

    // 3. Подключаемся к БД
    sqlDB, err := sql.Open("postgres", connStr)

    // 4. Применяем миграции из проекта (не дублируем SQL!)
    applyMigrations(sqlDB)

    // 5. Cleanup — контейнер удалится после теста
    t.Cleanup(func() {
        sqlDB.Close()
        postgresContainer.Terminate(ctx)
    })

    return &testDB{db: &DB{db: sqlDB}, container: postgresContainer}
}

// applyMigrations использует реальную миграцию из проекта
func applyMigrations(db *sql.DB) error {
    migrationPath := filepath.Join("..", "..", "..", "internal", "infra", "migration", "migrations", "001_init.up.sql")
    migrationSQL, err := os.ReadFile(migrationPath)
    if err != nil {
        return err
    }
    _, err = db.Exec(string(migrationSQL))
    return err
}
```

## Примеры тестов

### 1. Тест создания тикета

```go
func TestTicketsRepository_Create_Integration(t *testing.T) {
    testDB := setupTestDB(t)
    repo := NewTicketsRepository(testDB.db)

    ctx := context.Background()

    newTicket := domain.Ticket{
        UserID:  100,
        TopicID: 1,
        Status:  domain.StatusNew,
        Comment: "Integration test ticket",
    }

    created, err := repo.Create(ctx, newTicket)

    if err != nil {
        t.Fatalf("failed to create ticket: %v", err)
    }
    if created.ID == 0 {
        t.Error("expected ticket ID to be set")
    }
    if created.CreatedAt.IsZero() {
        t.Error("expected created_at to be set by database")
    }
}
```

**Что проверяется:**
- ✅ Тикет создаётся в реальной БД
- ✅ ID автоматически генерируется (SERIAL)
- ✅ Timestamps проставляются базой данных

### 2. Тест фильтрации и пагинации

```go
func TestTicketsRepository_List_Integration(t *testing.T) {
    testDB := setupTestDB(t)
    repo := NewTicketsRepository(testDB.db)

    ctx := context.Background()

    // Создаём тестовые данные
    userID := int64(500)
    for i := 0; i < 5; i++ {
        repo.Create(ctx, domain.Ticket{
            UserID:  userID,
            TopicID: 1,
            Status:  domain.StatusNew,
            Comment: fmt.Sprintf("Ticket %d", i),
        })
    }

    // Тест фильтра по userID
    t.Run("filter by userID", func(t *testing.T) {
        filter := tickets.ListFilter{
            UserID: &userID,
            Limit:  10,
            Offset: 0,
        }

        list, err := repo.List(ctx, filter)
        if err != nil {
            t.Fatalf("failed to list tickets: %v", err)
        }
        if len(list) != 5 {
            t.Errorf("expected 5 tickets, got %d", len(list))
        }
    })

    // Тест пагинации
    t.Run("pagination", func(t *testing.T) {
        filter := tickets.ListFilter{
            UserID: &userID,
            Limit:  2,
            Offset: 0,
        }

        page1, err := repo.List(ctx, filter)
        if len(page1) != 2 {
            t.Errorf("expected 2 tickets on page 1, got %d", len(page1))
        }

        filter.Offset = 2
        page2, err := repo.List(ctx, filter)
        if len(page2) != 1 {
            t.Errorf("expected 1 ticket on page 2, got %d", len(page2))
        }
    })
}
```

**Что проверяется:**
- ✅ WHERE условия работают корректно
- ✅ LIMIT и OFFSET работают
- ✅ Индексы используются (можно проверить через EXPLAIN)

### 3. Тест транзакций

```go
func TestTicketsRepository_Transaction_Integration(t *testing.T) {
    testDB := setupTestDB(t)
    repo := NewTicketsRepository(testDB.db)

    ctx := context.Background()

    // Начинаем транзакцию
    tx, err := testDB.db.BeginTx(ctx)
    if err != nil {
        t.Fatalf("failed to begin transaction: %v", err)
    }

    txCtx := context.WithValue(ctx, txKey, tx)

    // Создаём тикет в транзакции
    ticket, err := repo.Create(txCtx, domain.Ticket{
        UserID:  700,
        TopicID: 1,
        Status:  domain.StatusNew,
        Comment: "Ticket in transaction",
    })
    if err != nil {
        tx.Rollback()
        t.Fatalf("failed to create ticket: %v", err)
    }

    // Добавляем историю в транзакции
    err = repo.AddHistory(txCtx, domain.History{
        TicketID: ticket.ID,
        Action:   domain.ActionCreated,
        NewValue: "new",
    })
    if err != nil {
        tx.Rollback()
        t.Fatalf("failed to add history: %v", err)
    }

    // Коммитим
    if err := tx.Commit(); err != nil {
        t.Fatalf("failed to commit: %v", err)
    }

    // Проверяем, что данные сохранились
    found, err := repo.GetByID(ctx, ticket.ID)
    if err != nil {
        t.Fatalf("failed to get ticket after commit: %v", err)
    }
    if found.ID != ticket.ID {
        t.Errorf("expected ticket ID %d, got %d", ticket.ID, found.ID)
    }
}
```

**Что проверяется:**
- ✅ BEGIN/COMMIT работают
- ✅ Данные видны после commit
- ✅ Транзакция атомарна (тикет + история создаются вместе)

### 4. Тест отката транзакции

```go
func TestTicketsRepository_TransactionRollback_Integration(t *testing.T) {
    testDB := setupTestDB(t)
    repo := NewTicketsRepository(testDB.db)

    ctx := context.Background()

    tx, err := testDB.db.BeginTx(ctx)
    if err != nil {
        t.Fatalf("failed to begin transaction: %v", err)
    }

    txCtx := context.WithValue(ctx, txKey, tx)

    // Создаём тикет
    ticket, err := repo.Create(txCtx, domain.Ticket{
        UserID:  800,
        TopicID: 1,
        Status:  domain.StatusNew,
        Comment: "Ticket to rollback",
    })

    // Откатываем
    if err := tx.Rollback(); err != nil {
        t.Fatalf("failed to rollback: %v", err)
    }

    // Проверяем, что тикет НЕ сохранился
    _, err = repo.GetByID(ctx, ticket.ID)
    if err != tickets.ErrNotFound {
        t.Errorf("expected ErrNotFound after rollback, got: %v", err)
    }
}
```

**Что проверяется:**
- ✅ ROLLBACK отменяет изменения
- ✅ Данные не видны после rollback

## Покрытие кода

### Генерация отчёта

```bash
make cover-integration
```

Откроется HTML отчёт `coverage-integration.html` с визуализацией покрытия.

### Просмотр в терминале

```bash
go test ./internal/infra/postgres/... -tags=integration -cover

# Вывод:
# PASS
# coverage: 87.3% of statements
```

## Benchmarks

Интеграционные тесты включают benchmarks для измерения производительности:

```bash
make bench-integration
```

Или:

```bash
go test ./internal/infra/postgres/... -tags=integration -bench=. -benchmem
```

**Пример вывода:**

```
BenchmarkTicketsRepository_Create-8        1000    1234567 ns/op    1024 B/op    10 allocs/op
BenchmarkTicketsRepository_GetByID-8       5000     234567 ns/op     512 B/op     5 allocs/op
```

**Интерпретация:**
- `1000` — количество итераций
- `1234567 ns/op` — время на операцию (наносекунды)
- `1024 B/op` — байт выделено на операцию
- `10 allocs/op` — количество аллокаций памяти

## CI/CD

### GitHub Actions

Файл: `.github/workflows/integration-tests.yml`

```yaml
name: Integration Tests

on:
  pull_request:
    branches: [ main ]
  push:
    branches: [ main ]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make test-integration
```

Docker уже предустановлен в GitHub Actions runners.

## Troubleshooting

### 1. Docker не запущен

**Ошибка:**
```
Error: Cannot connect to the Docker daemon at unix:///var/run/docker.sock
```

**Решение:**
```bash
# macOS
open -a Docker

# Linux
sudo systemctl start docker
```

### 2. Порт занят

**Ошибка:**
```
Error: port is already allocated
```

**Решение:**

Testcontainers автоматически выбирает свободные порты. Если ошибка повторяется:

```bash
docker ps
docker stop $(docker ps -q)
```

### 3. Медленный старт

При первом запуске Docker скачивает образ PostgreSQL (~80MB).

**Решение:**

Предварительно скачать образ:

```bash
docker pull postgres:16-alpine
```

Последующие запуски будут быстрее.

### 4. Timeout при старте контейнера

**Ошибка:**
```
Error: context deadline exceeded
```

**Решение:**

Увеличить timeout:

```bash
go test ./... -tags=integration -timeout 10m
```

### 5. Контейнеры не удаляются

Testcontainers автоматически удаляет контейнеры через `t.Cleanup()`.

Если контейнеры остались:

```bash
# Показать все контейнеры
docker ps -a

# Удалить testcontainers
docker rm -f $(docker ps -a -q --filter "label=org.testcontainers=true")
```

### 6. Ошибка "too many open files"

**macOS/Linux:**

```bash
ulimit -n 4096
```

## Best Practices

### 1. Изоляция тестов

Каждый тест должен создавать свою БД через `setupTestDB(t)`:

```go
func TestSomething(t *testing.T) {
    testDB := setupTestDB(t)  // Свежая БД для этого теста
    repo := NewTicketsRepository(testDB.db)
    // ...
}
```

### 2. Cleanup

Используйте `t.Cleanup()` для гарантированной очистки:

```go
t.Cleanup(func() {
    sqlDB.Close()
    postgresContainer.Terminate(ctx)
})
```

### 3. Параллельные тесты

Testcontainers поддерживает параллельное выполнение:

```go
func TestSomething(t *testing.T) {
    t.Parallel()  // Этот тест может выполняться параллельно
    testDB := setupTestDB(t)
    // ...
}
```

### 4. Таймауты

Всегда используйте контексты с таймаутами:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

### 5. Миграции

Применяйте миграции в `setupTestDB()` для консистентности:

```go
func applyMigrations(db *sql.DB) error {
    // Применить SQL миграции
    _, err := db.Exec(migrationSQL)
    return err
}
```

## Сравнение: Моки vs Testcontainers

| Аспект | Моки | Testcontainers |
|--------|------|----------------|
| Скорость | ⚡ Очень быстро | 🐢 Медленнее (запуск контейнера) |
| Реалистичность | ⚠️ Имитация | ✅ Реальная БД |
| SQL особенности | ❌ Не проверяются | ✅ Проверяются |
| Транзакции | ⚠️ Имитация | ✅ Реальные |
| Индексы | ❌ Не проверяются | ✅ Проверяются |
| Триггеры | ❌ Не работают | ✅ Работают |
| CI/CD | ✅ Легко | ✅ Требует Docker |
| Изоляция | ✅ Полная | ✅ Полная |

**Рекомендация:** Используйте оба подхода:
- **Моки** — для unit-тестов сервисного слоя (быстрые, изолированные)
- **Testcontainers** — для интеграционных тестов репозитория (реалистичные)

## Дополнительные ресурсы

- [Testcontainers Go Documentation](https://golang.testcontainers.org/)
- [Testcontainers PostgreSQL Module](https://golang.testcontainers.org/modules/postgres/)
- [Docker Documentation](https://docs.docker.com/)
- [Go Testing Package](https://pkg.go.dev/testing)

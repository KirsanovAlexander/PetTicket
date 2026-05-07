# Тестирование

Этот документ описывает подход к тестированию в проекте pet-ticket, включая примеры unit-тестов с моками.

## Структура тестов

Тесты организованы рядом с тестируемым кодом:

```
internal/
├── app/
│   └── tickets/
│       ├── service.go
│       └── service_test.go          # Unit-тесты сервисного слоя
├── transport/
│   └── http/
│       └── handlers/
│           ├── tickets.go
│           └── tickets_test.go      # Unit-тесты HTTP handlers
```

## Запуск тестов

### Все тесты

```bash
make test
```

### Тесты конкретного пакета

```bash
go test ./internal/app/tickets/... -v
go test ./internal/transport/http/handlers/... -v
```

### С покрытием

```bash
go test ./... -cover
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Unit-тесты с моками

### Принципы

1. **Изоляция**: каждый тест проверяет один компонент изолированно
2. **Моки**: зависимости заменяются на моки с контролируемым поведением
3. **AAA паттерн**: Arrange (подготовка) → Act (действие) → Assert (проверка)
4. **Именование**: `Test<Function>_<Scenario>` (например, `TestGetTicket_NotFound`)

### Примеры моков

#### Мок репозитория (для тестов сервиса)

```go
type mockRepository struct {
    getByIDFunc func(ctx context.Context, id int64) (domain.Ticket, error)
    // ... другие методы
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (domain.Ticket, error) {
    if m.getByIDFunc != nil {
        return m.getByIDFunc(ctx, id)
    }
    return domain.Ticket{}, errors.New("not implemented")
}
```

**Использование:**

```go
repo := &mockRepository{
    getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
        if id == 1 {
            return expectedTicket, nil
        }
        return domain.Ticket{}, tickets.ErrNotFound
    },
}
```

#### Мок транзакций

```go
type mockDB struct {
    beginTxFunc func(ctx context.Context) (TxCommitter, error)
}

type mockTx struct {
    commitFunc   func() error
    rollbackFunc func() error
}
```

**Использование для проверки commit:**

```go
committed := false
db := &mockDB{
    beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
        return &mockTx{
            commitFunc: func() error {
                committed = true
                return nil
            },
        }, nil
    },
}

// ... выполнение операции

if !committed {
    t.Error("expected transaction to be committed")
}
```

#### Мок сервиса (для тестов handlers)

```go
type mockTicketsService struct {
    getTicketFunc func(ctx context.Context, id int64) (domain.Ticket, error)
    // ... другие методы
}

func (m *mockTicketsService) GetTicket(ctx context.Context, id int64) (domain.Ticket, error) {
    if m.getTicketFunc != nil {
        return m.getTicketFunc(ctx, id)
    }
    return domain.Ticket{}, errors.New("not implemented")
}
```

## Примеры тестов

### Тест сервисного слоя: успешное получение

```go
func TestGetTicket_Success(t *testing.T) {
    // Arrange
    expectedTicket := domain.Ticket{
        ID:      1,
        UserID:  100,
        Status:  domain.StatusNew,
    }

    repo := &mockRepository{
        getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
            return expectedTicket, nil
        },
    }

    svc := NewService(repo, &mockDB{}, testLogger())

    // Act
    ticket, err := svc.GetTicket(context.Background(), 1)

    // Assert
    if err != nil {
        t.Fatalf("expected no error, got: %v", err)
    }
    if ticket.ID != expectedTicket.ID {
        t.Errorf("expected ticket ID %d, got %d", expectedTicket.ID, ticket.ID)
    }
}
```

### Тест сервисного слоя: ошибка Not Found

```go
func TestGetTicket_NotFound(t *testing.T) {
    // Arrange
    repo := &mockRepository{
        getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
            return domain.Ticket{}, tickets.ErrNotFound
        },
    }

    svc := NewService(repo, &mockDB{}, testLogger())

    // Act
    _, err := svc.GetTicket(context.Background(), 999)

    // Assert
    if !errors.Is(err, tickets.ErrNotFound) {
        t.Errorf("expected ErrNotFound, got: %v", err)
    }
}
```

### Тест транзакций: проверка commit

```go
func TestCreateTicket_Success(t *testing.T) {
    // Arrange
    committed := false

    repo := &mockRepository{
        createFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
            ticket.ID = 42
            return ticket, nil
        },
        addHistoryFunc: func(ctx context.Context, history domain.History) error {
            return nil
        },
    }

    db := &mockDB{
        beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
            return &mockTx{
                commitFunc: func() error {
                    committed = true
                    return nil
                },
            }, nil
        },
    }

    svc := NewService(repo, db, testLogger())

    // Act
    _, err := svc.CreateTicket(context.Background(), CreateTicketInput{
        UserID:  100,
        TopicID: 1,
        Comment: "Test",
    })

    // Assert
    if err != nil {
        t.Fatalf("expected no error, got: %v", err)
    }
    if !committed {
        t.Error("expected transaction to be committed")
    }
}
```

### Тест транзакций: проверка rollback

```go
func TestUpdateTicket_TransactionRollback(t *testing.T) {
    // Arrange
    rolledBack := false

    repo := &mockRepository{
        getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
            return domain.Ticket{ID: 1, Status: domain.StatusNew}, nil
        },
        updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
            return ticket, nil
        },
        addHistoryFunc: func(ctx context.Context, history domain.History) error {
            return errors.New("database error")
        },
    }

    db := &mockDB{
        beginTxFunc: func(ctx context.Context) (TxCommitter, error) {
            return &mockTx{
                rollbackFunc: func() error {
                    rolledBack = true
                    return nil
                },
            }, nil
        },
    }

    svc := NewService(repo, db, testLogger())

    // Act
    newStatus := domain.StatusInProgress
    _, err := svc.UpdateTicket(context.Background(), UpdateTicketInput{
        ID:     1,
        Status: &newStatus,
    })

    // Assert
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    if !rolledBack {
        t.Error("expected transaction to be rolled back")
    }
}
```

### Тест HTTP handler

```go
func TestGetTicket_Success(t *testing.T) {
    // Arrange
    expectedTicket := domain.Ticket{
        ID:      1,
        UserID:  100,
        Status:  domain.StatusNew,
    }

    mockSvc := &mockTicketsService{
        getTicketFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
            return expectedTicket, nil
        },
    }

    handler := NewTicketsHandler(mockSvc, testLogger())

    app := fiber.New()
    app.Get("/tickets/:id", handler.getTicket)

    // Act
    req := httptest.NewRequest("GET", "/tickets/1", nil)
    resp, err := app.Test(req)

    // Assert
    if err != nil {
        t.Fatalf("request failed: %v", err)
    }
    if resp.StatusCode != fiber.StatusOK {
        t.Errorf("expected status 200, got %d", resp.StatusCode)
    }

    body, _ := io.ReadAll(resp.Body)
    var ticketResp dto.TicketResponse
    json.Unmarshal(body, &ticketResp)

    if ticketResp.ID != 1 {
        t.Errorf("expected ticket ID 1, got %d", ticketResp.ID)
    }
}
```

### Тест с захватом аргументов

```go
func TestUpdateTicket_StatusChange(t *testing.T) {
    // Arrange
    var capturedHistory domain.History

    repo := &mockRepository{
        getByIDFunc: func(ctx context.Context, id int64) (domain.Ticket, error) {
            return domain.Ticket{ID: 1, Status: domain.StatusNew}, nil
        },
        updateFunc: func(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
            return ticket, nil
        },
        addHistoryFunc: func(ctx context.Context, history domain.History) error {
            capturedHistory = history  // Захватываем аргумент
            return nil
        },
    }

    // ... создание сервиса и выполнение операции

    // Assert
    if capturedHistory.Action != domain.ActionStatusChanged {
        t.Errorf("expected action 'status_changed', got %s", capturedHistory.Action)
    }
    if capturedHistory.OldValue != "new" {
        t.Errorf("expected old value 'new', got %s", capturedHistory.OldValue)
    }
}
```

## Table-Driven тесты

Для тестирования множества сценариев используйте table-driven подход:

```go
func TestCreateTicket_InvalidInput(t *testing.T) {
    repo := &mockRepository{}
    svc := NewService(repo, &mockDB{}, testLogger())

    tests := []struct {
        name  string
        input CreateTicketInput
    }{
        {
            name: "invalid user_id",
            input: CreateTicketInput{
                UserID:  0,
                TopicID: 1,
                Comment: "Test",
            },
        },
        {
            name: "invalid topic_id",
            input: CreateTicketInput{
                UserID:  1,
                TopicID: 0,
                Comment: "Test",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := svc.CreateTicket(context.Background(), tt.input)
            if !errors.Is(err, tickets.ErrInvalidInput) {
                t.Errorf("expected ErrInvalidInput, got: %v", err)
            }
        })
    }
}
```

## Best Practices

### 1. Используйте testLogger для тестов

```go
func testLogger() zerolog.Logger {
    return zerolog.New(io.Discard)
}
```

### 2. Проверяйте не только успешные сценарии

- Успешное выполнение
- Ошибки валидации
- Ошибки базы данных
- Not Found сценарии
- Rollback транзакций

### 3. Тестируйте границы

```go
tests := []struct {
    name    string
    limit   int
    wantErr bool
}{
    {"valid limit", 10, false},
    {"zero limit", 0, false},
    {"negative limit", -1, true},
    {"max limit", 1000, false},
}
```

### 4. Избегайте дублирования

Выносите общую логику в helper-функции:

```go
func setupTestService(repo Repository) Service {
    return NewService(repo, &mockDB{}, testLogger())
}
```

### 5. Очищайте ресурсы

```go
func TestSomething(t *testing.T) {
    // Setup
    cleanup := setupTestData()
    defer cleanup()

    // Test logic
}
```

## Интеграционные тесты

Подробнее об интеграционных тестах см. [INTEGRATION_TESTS.md](./INTEGRATION_TESTS.md).

Краткий запуск:

```bash
# Все интеграционные тесты
make test-integration

# Конкретный пакет
go test ./internal/infra/postgres/... -tags=integration -v

# С benchmarks
go test ./internal/infra/postgres/... -tags=integration -bench=. -benchmem
```

## CI/CD

В CI выполняются:

```bash
make ci  # fmt, vet, lint, test
```

См. `.github/workflows/lint.yml` для деталей.

## Дополнительные ресурсы

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Testify](https://github.com/stretchr/testify) - популярная библиотека для assertions (опционально)

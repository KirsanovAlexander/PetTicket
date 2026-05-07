//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	apptickets "pet-ticket/internal/app/tickets"
	domain "pet-ticket/internal/domain/tickets"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testDB содержит тестовую БД и контейнер
type testDB struct {
	db        *DB
	container *postgres.PostgresContainer
	connStr   string
}

// setupTestDB создаёт PostgreSQL контейнер и применяет миграции
func setupTestDB(t *testing.T) *testDB {
	ctx := context.Background()

	// Создаём PostgreSQL контейнер
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
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// Получаем connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Подключаемся к БД
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		postgresContainer.Terminate(ctx)
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Применяем миграции
	if err := applyMigrations(sqlDB); err != nil {
		sqlDB.Close()
		postgresContainer.Terminate(ctx)
		t.Fatalf("failed to apply migrations: %v", err)
	}

	db := &DB{conn: sqlDB}

	// Cleanup функция
	t.Cleanup(func() {
		sqlDB.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	return &testDB{
		db:        db,
		container: postgresContainer,
		connStr:   connStr,
	}
}

// applyMigrations применяет SQL миграции из файлов проекта
// Использует реальную миграцию из internal/infra/migration/migrations/001_init.up.sql
// вместо дублирования SQL кода в тестах
func applyMigrations(db *sql.DB) error {
	// Путь к миграции относительно текущего пакета (internal/infra/postgres)
	// Поднимаемся на 3 уровня вверх и спускаемся к миграциям
	migrationPath := filepath.Join("..", "..", "..", "internal", "infra", "migration", "migrations", "001_init.up.sql")

	// Читаем файл миграции
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		return err
	}

	// Применяем миграцию к тестовой БД
	_, err = db.Exec(string(migrationSQL))
	return err
}

// TestTicketsRepository_Create_Integration — интеграционный тест создания тикета
func TestTicketsRepository_Create_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Arrange
	newTicket := domain.Ticket{
		UserID:  100,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Integration test ticket",
	}

	// Act
	created, err := repo.Create(ctx, newTicket)

	// Assert
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	if created.ID == 0 {
		t.Error("expected ticket ID to be set")
	}
	if created.UserID != newTicket.UserID {
		t.Errorf("expected user_id %d, got %d", newTicket.UserID, created.UserID)
	}
	if created.TopicID != newTicket.TopicID {
		t.Errorf("expected topic_id %d, got %d", newTicket.TopicID, created.TopicID)
	}
	if created.Status != newTicket.Status {
		t.Errorf("expected status %s, got %s", newTicket.Status, created.Status)
	}
	if created.Comment != newTicket.Comment {
		t.Errorf("expected comment %s, got %s", newTicket.Comment, created.Comment)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set by database")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set by database")
	}
}

// TestTicketsRepository_GetByID_Integration — интеграционный тест получения тикета
func TestTicketsRepository_GetByID_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Создаём тикет
	created, err := repo.Create(ctx, domain.Ticket{
		UserID:  200,
		TopicID: 2,
		Status:  domain.StatusNew,
		Comment: "Test ticket for GetByID",
	})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Act
	found, err := repo.GetByID(ctx, created.ID)

	// Assert
	if err != nil {
		t.Fatalf("failed to get ticket: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, found.ID)
	}
	if found.UserID != created.UserID {
		t.Errorf("expected user_id %d, got %d", created.UserID, found.UserID)
	}
	if found.Comment != created.Comment {
		t.Errorf("expected comment %s, got %s", created.Comment, found.Comment)
	}
}

// TestTicketsRepository_GetByID_NotFound_Integration — тикет не найден
func TestTicketsRepository_GetByID_NotFound_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Act
	_, err := repo.GetByID(ctx, 99999)

	// Assert
	if err != apptickets.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestTicketsRepository_Update_Integration — обновление тикета
func TestTicketsRepository_Update_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Создаём тикет
	created, err := repo.Create(ctx, domain.Ticket{
		UserID:  300,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Original comment",
	})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Ждём немного, чтобы updated_at изменился
	time.Sleep(10 * time.Millisecond)

	// Act - обновляем статус и комментарий
	created.Status = domain.StatusInProgress
	created.Comment = "Updated comment"
	updated, err := repo.Update(ctx, created)

	// Assert
	if err != nil {
		t.Fatalf("failed to update ticket: %v", err)
	}

	if updated.Status != domain.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", updated.Status)
	}
	if updated.Comment != "Updated comment" {
		t.Errorf("expected updated comment, got %s", updated.Comment)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Error("expected updated_at to be later than created_at")
	}

	// Проверяем, что изменения сохранились в БД
	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get updated ticket: %v", err)
	}
	if found.Status != domain.StatusInProgress {
		t.Errorf("expected status in_progress in DB, got %s", found.Status)
	}
}

// TestTicketsRepository_Delete_Integration — удаление тикета
func TestTicketsRepository_Delete_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Создаём тикет
	created, err := repo.Create(ctx, domain.Ticket{
		UserID:  400,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Ticket to delete",
	})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Act - удаляем
	err = repo.Delete(ctx, created.ID)

	// Assert
	if err != nil {
		t.Fatalf("failed to delete ticket: %v", err)
	}

	// Проверяем, что тикет действительно удалён
	_, err = repo.GetByID(ctx, created.ID)
	if err != apptickets.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got: %v", err)
	}
}

// TestTicketsRepository_List_Integration — получение списка с фильтрами
func TestTicketsRepository_List_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Создаём несколько тикетов
	userID := int64(500)
	tickets := []domain.Ticket{
		{UserID: userID, TopicID: 1, Status: domain.StatusNew, Comment: "Ticket 1"},
		{UserID: userID, TopicID: 2, Status: domain.StatusInProgress, Comment: "Ticket 2"},
		{UserID: userID, TopicID: 1, Status: domain.StatusNew, Comment: "Ticket 3"},
		{UserID: 999, TopicID: 1, Status: domain.StatusNew, Comment: "Other user ticket"},
	}

	for _, ticket := range tickets {
		_, err := repo.Create(ctx, ticket)
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
	}

	// Test 1: фильтр по userID
	t.Run("filter by userID", func(t *testing.T) {
		filter := apptickets.ListFilter{
			UserID: &userID,
			Limit:  10,
			Offset: 0,
		}

		list, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}

		if len(list) != 3 {
			t.Errorf("expected 3 tickets for user %d, got %d", userID, len(list))
		}

		for _, ticket := range list {
			if ticket.UserID != userID {
				t.Errorf("expected user_id %d, got %d", userID, ticket.UserID)
			}
		}
	})

	// Test 2: фильтр по статусу
	t.Run("filter by status", func(t *testing.T) {
		statusNew := domain.StatusNew
		filter := apptickets.ListFilter{
			UserID: &userID,
			Status: &statusNew,
			Limit:  10,
			Offset: 0,
		}

		list, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}

		if len(list) != 2 {
			t.Errorf("expected 2 tickets with status 'new', got %d", len(list))
		}

		for _, ticket := range list {
			if ticket.Status != domain.StatusNew {
				t.Errorf("expected status 'new', got %s", ticket.Status)
			}
		}
	})

	// Test 3: фильтр по topicID
	t.Run("filter by topicID", func(t *testing.T) {
		topicID := int64(1)
		filter := apptickets.ListFilter{
			UserID:  &userID,
			TopicID: &topicID,
			Limit:   10,
			Offset:  0,
		}

		list, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}

		if len(list) != 2 {
			t.Errorf("expected 2 tickets with topic_id 1, got %d", len(list))
		}

		for _, ticket := range list {
			if ticket.TopicID != 1 {
				t.Errorf("expected topic_id 1, got %d", ticket.TopicID)
			}
		}
	})

	// Test 4: пагинация
	t.Run("pagination", func(t *testing.T) {
		filter := apptickets.ListFilter{
			UserID: &userID,
			Limit:  2,
			Offset: 0,
		}

		page1, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}

		if len(page1) != 2 {
			t.Errorf("expected 2 tickets on page 1, got %d", len(page1))
		}

		filter.Offset = 2
		page2, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}

		if len(page2) != 1 {
			t.Errorf("expected 1 ticket on page 2, got %d", len(page2))
		}

		// Проверяем, что тикеты разные
		if len(page1) > 0 && len(page2) > 0 && page1[0].ID == page2[0].ID {
			t.Error("expected different tickets on different pages")
		}
	})
}

// TestTicketsRepository_History_Integration — работа с историей
func TestTicketsRepository_History_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Создаём тикет
	ticket, err := repo.Create(ctx, domain.Ticket{
		UserID:  600,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Ticket with history",
	})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Добавляем записи в историю
	histories := []domain.History{
		{
			TicketID: ticket.ID,
			Action:   domain.ActionCreated,
			NewValue: "new",
		},
		{
			TicketID: ticket.ID,
			Action:   domain.ActionStatusChanged,
			OldValue: "new",
			NewValue: "in_progress",
		},
		{
			TicketID: ticket.ID,
			Action:   domain.ActionCommentAdded,
			NewValue: "Additional comment",
		},
	}

	for _, h := range histories {
		if err := repo.AddHistory(ctx, h); err != nil {
			t.Fatalf("failed to add history: %v", err)
		}
		time.Sleep(5 * time.Millisecond) // Чтобы created_at различались
	}

	// Act - получаем историю
	history, err := repo.GetHistory(ctx, ticket.ID, 10, 0)

	// Assert
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}

	// Проверяем порядок (должны быть отсортированы по created_at DESC)
	if history[0].Action != domain.ActionCommentAdded {
		t.Errorf("expected first entry to be 'comment_added', got %s", history[0].Action)
	}

	// Test пагинация истории
	t.Run("history pagination", func(t *testing.T) {
		page1, err := repo.GetHistory(ctx, ticket.ID, 2, 0)
		if err != nil {
			t.Fatalf("failed to get history page 1: %v", err)
		}
		if len(page1) != 2 {
			t.Errorf("expected 2 entries on page 1, got %d", len(page1))
		}

		page2, err := repo.GetHistory(ctx, ticket.ID, 2, 2)
		if err != nil {
			t.Fatalf("failed to get history page 2: %v", err)
		}
		if len(page2) != 1 {
			t.Errorf("expected 1 entry on page 2, got %d", len(page2))
		}
	})
}

// TestTicketsRepository_GetAllStatuses_Integration — получение всех статусов
func TestTicketsRepository_GetAllStatuses_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Act
	statuses, err := repo.GetAllStatuses(ctx)

	// Assert
	if err != nil {
		t.Fatalf("failed to get statuses: %v", err)
	}

	if len(statuses) < 4 {
		t.Errorf("expected at least 4 statuses, got %d", len(statuses))
	}

	// Проверяем наличие основных статусов
	statusNames := make(map[string]bool)
	for _, s := range statuses {
		statusNames[s.Name] = true
	}

	expectedStatuses := []string{"new", "in_progress", "resolved", "closed"}
	for _, expected := range expectedStatuses {
		if !statusNames[expected] {
			t.Errorf("expected status '%s' not found", expected)
		}
	}
}

// TestTicketsRepository_GetAllTopics_Integration — получение всех топиков
func TestTicketsRepository_GetAllTopics_Integration(t *testing.T) {
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Act
	topics, err := repo.GetAllTopics(ctx)

	// Assert
	if err != nil {
		t.Fatalf("failed to get topics: %v", err)
	}

	if len(topics) < 3 {
		t.Errorf("expected at least 3 topics, got %d", len(topics))
	}

	// Проверяем что топики загрузились
	if len(topics) == 0 {
		t.Error("no topics found in database")
	}
}

// TestTicketsRepository_Transaction_Integration — тест транзакций
func TestTicketsRepository_Transaction_Integration(t *testing.T) {
	testDB := setupTestDB(t)

	ctx := context.Background()

	// Начинаем транзакцию
	tx, err := testDB.db.BeginTx(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Сохраняем транзакцию в контекст
	txCtx := context.WithValue(ctx, txContextKey, tx)

	repo := NewTicketsRepository(testDB.db)

	// Создаём тикет в транзакции
	ticket, err := repo.Create(txCtx, domain.Ticket{
		UserID:  700,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Ticket in transaction",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to create ticket in transaction: %v", err)
	}

	// Добавляем историю в транзакции
	err = repo.AddHistory(txCtx, domain.History{
		TicketID: ticket.ID,
		Action:   domain.ActionCreated,
		NewValue: "new",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to add history in transaction: %v", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Проверяем, что данные сохранились
	found, err := repo.GetByID(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("failed to get ticket after commit: %v", err)
	}
	if found.ID != ticket.ID {
		t.Errorf("expected ticket ID %d, got %d", ticket.ID, found.ID)
	}

	history, err := repo.GetHistory(ctx, ticket.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get history after commit: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

// TestTicketsRepository_TransactionRollback_Integration — тест отката транзакции
func TestTicketsRepository_TransactionRollback_Integration(t *testing.T) {
	testDB := setupTestDB(t)

	ctx := context.Background()

	// Начинаем транзакцию
	tx, err := testDB.db.BeginTx(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	txCtx := context.WithValue(ctx, txContextKey, tx)

	repo := NewTicketsRepository(testDB.db)

	// Создаём тикет в транзакции
	ticket, err := repo.Create(txCtx, domain.Ticket{
		UserID:  800,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Ticket to rollback",
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to create ticket in transaction: %v", err)
	}

	// Откатываем транзакцию
	if err := tx.Rollback(); err != nil {
		t.Fatalf("failed to rollback transaction: %v", err)
	}

	// Проверяем, что тикет НЕ сохранился
	_, err = repo.GetByID(ctx, ticket.ID)
	if err != apptickets.ErrNotFound {
		t.Errorf("expected ErrNotFound after rollback, got: %v", err)
	}
}

// Benchmark тесты

func BenchmarkTicketsRepository_Create(b *testing.B) {
	// Используем testing.B вместо testing.T
	t := &testing.T{}
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	ticket := domain.Ticket{
		UserID:  1000,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Benchmark ticket",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.Create(ctx, ticket)
		if err != nil {
			b.Fatalf("failed to create ticket: %v", err)
		}
	}
}

func BenchmarkTicketsRepository_GetByID(b *testing.B) {
	t := &testing.T{}
	testDB := setupTestDB(t)
	repo := NewTicketsRepository(testDB.db)

	ctx := context.Background()

	// Создаём тикет для бенчмарка
	created, err := repo.Create(ctx, domain.Ticket{
		UserID:  1001,
		TopicID: 1,
		Status:  domain.StatusNew,
		Comment: "Benchmark ticket",
	})
	if err != nil {
		b.Fatalf("failed to create ticket: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			b.Fatalf("failed to get ticket: %v", err)
		}
	}
}

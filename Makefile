.PHONY: run build clean test test-integration test-all cover cover-integration bench bench-integration deps lint lint-fix fmt vet ci proto docker-up docker-down docker-reset docker-build docker-logs docker-logs-app docker-logs-postgres docker-restart docker-full-reset migrate-up migrate-down help

run:
	go run cmd/api-server/main.go

build:
	go build -o bin/pet-ticket cmd/api-server/main.go

clean:
	rm -rf bin/

# Тестирование
# Unit-тесты: быстрые тесты с моками, проверяют бизнес-логику изолированно
test:
	go test -v -race -timeout 30s ./...

# Интеграционные тесты: используют testcontainers для запуска реальной PostgreSQL
# Требуют запущенный Docker. Проверяют работу с реальной БД, SQL запросы, транзакции
test-integration:
	@echo "🐳 Running integration tests (requires Docker)..."
	go test -v -tags=integration -timeout 5m ./...

# Запуск всех тестов: сначала unit, затем integration
test-all:
	@echo "🧪 Running all tests..."
	go test -v -race -timeout 30s ./...
	go test -v -tags=integration -timeout 5m ./...

# Генерация HTML отчёта о покрытии кода unit-тестами
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

# Генерация HTML отчёта о покрытии кода интеграционными тестами
cover-integration:
	go test -tags=integration -coverprofile=coverage-integration.out ./...
	go tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "📊 Integration coverage report: coverage-integration.html"

# Бенчмарки для unit-тестов: измерение производительности с моками
bench:
	go test -bench=. -benchmem ./...

# Бенчмарки для интеграционных тестов: измерение производительности с реальной БД
bench-integration:
	@echo "⚡ Running integration benchmarks..."
	go test -tags=integration -bench=. -benchmem ./internal/infra/postgres/...

deps:
	go mod download
	go mod tidy

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

fmt:
	go fmt ./...

vet:
	go vet ./...

ci: fmt vet lint test
	@echo "✅ All checks passed!"

# Генерация Go кода из .proto (нужны: protoc, protoc-gen-go, protoc-gen-go-grpc)
proto:
	@which protoc >/dev/null || (echo "install protoc: brew install protobuf" && exit 1)
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@mkdir -p api/gen/go
	protoc --go_out=api/gen/go --go_opt=paths=source_relative \
		--go-grpc_out=api/gen/go --go-grpc_opt=paths=source_relative \
		-I api/proto -I $$(brew --prefix protobuf 2>/dev/null)/include \
		api/proto/ticket/v1/ticket.proto
	@echo "✅ Proto generated in api/gen/go"

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-reset:
	docker-compose down -v
	docker-compose up -d
	@echo "✅ PostgreSQL reset complete"

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f

docker-logs-app:
	docker-compose logs -f app

docker-logs-postgres:
	docker-compose logs -f postgres

docker-restart:
	docker-compose restart app

docker-full-reset:
	docker-compose down -v
	docker-compose build --no-cache
	docker-compose up -d
	@echo "✅ Full Docker reset complete"

migrate-up:
	go run cmd/migrate/main.go -action=up

migrate-down:
	go run cmd/migrate/main.go -action=down

help:
	@echo "Available targets:"
	@echo "  run                - Run application locally"
	@echo "  build              - Build binary to bin/pet-ticket"
	@echo "  clean              - Remove built binaries"
	@echo "  test               - Run all tests"
	@echo "  deps               - Download and tidy dependencies"
	@echo "  lint               - Run linter"
	@echo "  lint-fix           - Run linter with auto-fix"
	@echo "  fmt                - Format code"
	@echo "  vet                - Run go vet"
	@echo "  ci                 - Run fmt+vet+lint+test (CI pipeline)"
	@echo "  proto              - Generate Go code from api/proto (protoc required)"
	@echo "  docker-up          - Start all services (PostgreSQL + App)"
	@echo "  docker-down        - Stop all services"
	@echo "  docker-reset       - Reset PostgreSQL (delete all data)"
	@echo "  docker-build       - Build Docker images"
	@echo "  docker-logs        - Show logs for all services"
	@echo "  docker-logs-app    - Show logs for app only"
	@echo "  docker-logs-postgres - Show logs for PostgreSQL only"
	@echo "  docker-restart     - Restart app container"
	@echo "  docker-full-reset  - Full reset (rebuild + reset data)"
	@echo "  migrate-up         - Apply database migrations"
	@echo "  migrate-down       - Rollback database migrations"
	@echo "  help               - Show this help message"


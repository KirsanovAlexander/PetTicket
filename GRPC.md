# gRPC API

Сервис поднимает gRPC сервер на порту **9001** (по умолчанию). Используется тот же домен и сервисный слой, что и для HTTP.

## Конфигурация

```env
GRPC_LISTEN=:9001   # адрес gRPC сервера; пусто = gRPC не запускается
```

## Proto

- **Файл:** `api/proto/ticket/v1/ticket.proto`
- **Сгенерированный Go код:** `api/gen/go/ticket/v1/`

Перегенерация после изменения proto:

```bash
make proto
```

Требуется установленный `protoc` (например, `brew install protobuf`).

## Методы сервиса

| RPC | Описание |
|-----|----------|
| CreateTicket | Создать тикет |
| GetTicket | Получить тикет по ID |
| UpdateTicket | Обновить тикет |
| DeleteTicket | Удалить тикет |
| ListTickets | Список тикетов с фильтрами |
| GetTicketHistory | История тикета |
| GetAllStatuses | Справочник статусов |
| GetAllTopics | Справочник тем |

## Проверка через grpcurl

[grpcurl](https://github.com/fullstorydev/grpcurl) — CLI для вызова gRPC.

```bash
# Установка
brew install grpcurl

# Список сервисов
grpcurl -plaintext localhost:9001 list

# Список методов TicketService
grpcurl -plaintext localhost:9001 list ticket.v1.TicketService

# Описание метода
grpcurl -plaintext localhost:9001 describe ticket.v1.TicketService.GetTicket

# Вызов GetTicket (id=1)
grpcurl -plaintext -d '{"id": 1}' localhost:9001 ticket.v1.TicketService/GetTicket

# GetAllStatuses
grpcurl -plaintext localhost:9001 ticket.v1.TicketService/GetAllStatuses

# CreateTicket
grpcurl -plaintext -d '{"user_id": 1, "topic_id": 1, "comment": "Test ticket from gRPC"}' \
  localhost:9001 ticket.v1.TicketService/CreateTicket
```

## Reflection

Включена gRPC reflection — `grpcurl` и клиенты (evans, bloomrpc и т.п.) могут получать описание сервисов без отдельного .proto файла.

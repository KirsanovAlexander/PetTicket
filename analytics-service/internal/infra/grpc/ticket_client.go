package grpc

import (
	"context"
	"fmt"
	"time"

	ticketv1 "pet-ticket/api/gen/go/ticket/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// TicketClient — тонкая обёртка над сгенерированным gRPC-клиентом
// TicketService. Реализует analytics.TicketClientInterface: агрегатор не
// знает про эту структуру, только про интерфейс, который она реализует.
// Каждый метод прозрачно ретраится через withRetry — агрегатору не нужно
// ничего знать про сетевую нестабильность между сервисами.
type TicketClient struct {
	conn   *grpc.ClientConn
	client ticketv1.TicketServiceClient
}

// NewTicketClient устанавливает соединение с основным сервисом pet-ticket по
// gRPC. grpc.NewClient не блокирует и не проверяет доступность немедленно —
// соединение "ленивое", реальные ошибки сети проявятся при первом вызове RPC.
// Это нормально: main.go не должен падать при старте только потому, что
// основной сервис ещё не поднялся (типичная ситуация в docker-compose, где
// порядок запуска контейнеров не гарантирует готовность зависимости).
func NewTicketClient(target string) (*TicketClient, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client for %q: %w", target, err)
	}

	return &TicketClient{
		conn:   conn,
		client: ticketv1.NewTicketServiceClient(conn),
	}, nil
}

// ListTickets возвращает страницу тикетов по фильтру.
func (c *TicketClient) ListTickets(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
	return withRetry(ctx, func() (*ticketv1.ListTicketsResponse, error) {
		return c.client.ListTickets(ctx, req)
	})
}

// GetTicketHistory возвращает историю изменений одного тикета.
func (c *TicketClient) GetTicketHistory(ctx context.Context, ticketID int64, limit, offset int32) (*ticketv1.GetTicketHistoryResponse, error) {
	return withRetry(ctx, func() (*ticketv1.GetTicketHistoryResponse, error) {
		return c.client.GetTicketHistory(ctx, &ticketv1.GetTicketHistoryRequest{
			TicketId: ticketID,
			Limit:    limit,
			Offset:   offset,
		})
	})
}

// GetAllStatuses возвращает справочник статусов.
func (c *TicketClient) GetAllStatuses(ctx context.Context) (*ticketv1.GetAllStatusesResponse, error) {
	return withRetry(ctx, func() (*ticketv1.GetAllStatusesResponse, error) {
		return c.client.GetAllStatuses(ctx, &ticketv1.GetAllStatusesRequest{})
	})
}

// GetAllTopics возвращает справочник тем.
func (c *TicketClient) GetAllTopics(ctx context.Context) (*ticketv1.GetAllTopicsResponse, error) {
	return withRetry(ctx, func() (*ticketv1.GetAllTopicsResponse, error) {
		return c.client.GetAllTopics(ctx, &ticketv1.GetAllTopicsRequest{})
	})
}

// Close закрывает соединение с основным сервисом.
func (c *TicketClient) Close() error {
	return c.conn.Close()
}

// withRetry выполняет fn с экспоненциальным backoff (1s, 2s, 4s, ... до
// maxBackoff), но только если ошибка похожа на временную (см. isRetryable).
// Дженерик вместо копирования одного и того же цикла в 4 метода — тип
// ответа у каждого RPC свой, а логика ретраев одна.
func withRetry[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if !isRetryable(err) {
			var zero T
			return zero, err
		}
		if attempt == maxRetries {
			break
		}

		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	var zero T
	return zero, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

// isRetryable решает, стоит ли повторять RPC после этой ошибки.
// Unavailable/DeadlineExceeded/ResourceExhausted обычно означают временную
// проблему (сервис перезапускается, сеть моргнула, перегрузка) — есть смысл
// подождать и попробовать снова. Остальные коды (InvalidArgument, NotFound,
// PermissionDenied и т.п.) — это ошибки самого запроса, повтор их не
// исправит, а только зря добавит задержку к ответу API.
func isRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		// Не gRPC-статус (например, обрыв соединения на транспортном уровне) —
		// тоже стоит попробовать ещё раз.
		return true
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

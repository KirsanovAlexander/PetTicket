package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWithRetry_SucceedsOnFirstTry(t *testing.T) {
	calls := 0
	result, err := withRetry(context.Background(), func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_RetriesRetryableErrorThenSucceeds(t *testing.T) {
	calls := 0
	result, err := withRetry(context.Background(), func() (string, error) {
		calls++
		if calls < 3 {
			return "", status.Error(codes.Unavailable, "service temporarily unavailable")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected no error after eventual success, got: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", calls)
	}
}

func TestWithRetry_DoesNotRetryNonRetryableError(t *testing.T) {
	calls := 0
	_, err := withRetry(context.Background(), func() (string, error) {
		calls++
		return "", status.Error(codes.InvalidArgument, "bad request")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call for a non-retryable error, got %d", calls)
	}
}

func TestWithRetry_GivesUpAfterMaxRetries(t *testing.T) {
	calls := 0
	start := time.Now()
	_, err := withRetry(context.Background(), func() (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "still down")
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != maxRetries {
		t.Errorf("expected %d calls, got %d", maxRetries, calls)
	}
	// Между попытками должны быть паузы 1s + 2s = минимум ~3s для maxRetries=3.
	if elapsed < initialBackoff+2*initialBackoff {
		t.Errorf("expected backoff delays to elapse, got %v", elapsed)
	}
}

func TestWithRetry_StopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := withRetry(ctx, func() (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "down")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if calls >= maxRetries {
		t.Errorf("expected cancellation to cut retries short, got %d calls", calls)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unavailable", status.Error(codes.Unavailable, "x"), true},
		{"deadline exceeded", status.Error(codes.DeadlineExceeded, "x"), true},
		{"resource exhausted", status.Error(codes.ResourceExhausted, "x"), true},
		{"invalid argument", status.Error(codes.InvalidArgument, "x"), false},
		{"not found", status.Error(codes.NotFound, "x"), false},
		{"permission denied", status.Error(codes.PermissionDenied, "x"), false},
		{"non-grpc error", errors.New("plain error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

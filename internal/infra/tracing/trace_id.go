package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// TraceIDFromCtx возвращает hex trace_id активного спана в ctx, либо пустую
// строку, если спана нет (например, запрос ещё не прошёл TracingMiddleware).
func TraceIDFromCtx(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.HasTraceID() {
		return ""
	}
	return spanCtx.TraceID().String()
}

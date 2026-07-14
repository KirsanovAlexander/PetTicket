package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitTracer инициализирует глобальный OpenTelemetry TracerProvider с
// экспортом спанов по OTLP/gRPC в endpoint (например, Jaeger). Возвращает
// shutdown-функцию, которую нужно вызвать при остановке приложения, чтобы
// докинуть недоотправленные спаны.
func InitTracer(serviceName, endpoint string) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	shutdown := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = tp.Shutdown(shutdownCtx)
	}

	return shutdown, nil
}

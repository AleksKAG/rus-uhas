package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	TracerName = "rus-uhas-control-plane"
)

// InitTracer инициализирует OpenTelemetry tracer
func InitTracer(ctx context.Context, serviceName, endpoint string) (func(), error) {
	// Создаем exporter (OTLP over HTTP)
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // Для локальной разработки
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer: %w", err)
	}

	// Создаем resource с информацией о сервисе
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", "production"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Создаем tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Устанавливаем как глобальный
	otel.SetTracerProvider(tp)

	// Возвращаем функцию для graceful shutdown
	cleanup := func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			fmt.Printf("Error shutting down tracer: %v\n", err)
		}
	}

	return cleanup, nil
}

// GetTracer возвращает tracer для текущего пакета
func GetTracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// StartSpan создает новый span
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := GetTracer()
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

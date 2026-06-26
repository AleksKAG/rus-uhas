package telemetry

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	operationIDKey   contextKey = "operation_id"
	surgeonIDKey     contextKey = "surgeon_id"
)

// Logger обертка над slog с дополнительными методами
type Logger struct {
	*slog.Logger
}

// NewLogger создает новый logger с JSON форматом
func NewLogger(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	
	return &Logger{Logger: logger}
}

// WithCorrelationID добавляет correlation ID в контекст
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// WithOperationID добавляет operation ID в контекст
func WithOperationID(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, operationIDKey, operationID)
}

// WithSurgeonID добавляет surgeon ID в контекст
func WithSurgeonID(ctx context.Context, surgeonID string) context.Context {
	return context.WithValue(ctx, surgeonIDKey, surgeonID)
}

// GetCorrelationID извлекает correlation ID из контекста
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// GetOperationID извлекает operation ID из контекста
func GetOperationID(ctx context.Context) string {
	if id, ok := ctx.Value(operationIDKey).(string); ok {
		return id
	}
	return ""
}

// GetSurgeonID извлекает surgeon ID из контекста
func GetSurgeonID(ctx context.Context) string {
	if id, ok := ctx.Value(surgeonIDKey).(string); ok {
		return id
	}
	return ""
}

// WithContext возвращает logger с атрибутами из контекста
func (l *Logger) WithContext(ctx context.Context) *slog.Logger {
	logger := l.Logger
	
	if corrID := GetCorrelationID(ctx); corrID != "" {
		logger = logger.With("correlation_id", corrID)
	}
	if opID := GetOperationID(ctx); opID != "" {
		logger = logger.With("operation_id", opID)
	}
	if surgID := GetSurgeonID(ctx); surgID != "" {
		logger = logger.With("surgeon_id", surgID)
	}
	
	return logger
}

// GenerateCorrelationID генерирует новый correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// GenerateOperationID генерирует новый operation ID
func GenerateOperationID() string {
	return "op_" + uuid.New().String()[:8]
}

package ai

import (
	"context"
	"log/slog"
	"rus-uhas/internal/hal"
	"sync/atomic"
)

// FallbackClassifier оборачивает основной классификатор и fallback
// Если основной падает - автоматически переключается на эвристику
type FallbackClassifier struct {
	primary    hal.TissueSensor
	fallback   hal.TissueSensor
	logger     *slog.Logger
	
	// Счетчики для мониторинга
	primaryCalls   atomic.Int64
	fallbackCalls  atomic.Int64
	primaryErrors  atomic.Int64
}

func NewFallbackClassifier(primary, fallback hal.TissueSensor, logger *slog.Logger) *FallbackClassifier {
	return &FallbackClassifier{
		primary:  primary,
		fallback: fallback,
		logger:   logger,
	}
}

// Classify пытается использовать primary, при ошибке - fallback
func (c *FallbackClassifier) Classify(ctx context.Context, state hal.GeneratorState) (map[hal.TissueType]float64, error) {
	// Пробуем основной классификатор
	probs, err := c.primary.Classify(ctx, state)
	if err == nil {
		c.primaryCalls.Add(1)
		return probs, nil
	}
	
	// Логируем ошибку
	c.primaryErrors.Add(1)
	c.logger.Warn("Primary classifier failed, using fallback",
		"error", err,
		"primary_errors_total", c.primaryErrors.Load())
	
	// Используем fallback
	c.fallbackCalls.Add(1)
	return c.fallback.Classify(ctx, state)
}

// Stats возвращает статистику использования
func (c *FallbackClassifier) Stats() (primary, fallback, errors int64) {
	return c.primaryCalls.Load(), c.fallbackCalls.Load(), c.primaryErrors.Load()
}

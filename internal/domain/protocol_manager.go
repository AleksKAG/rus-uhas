package domain

import (
	"context"
	"fmt"
	"log/slog"
	"rus-uhas/internal/hal"
	"sync"
	"time"
	"rus-uhas/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// ProtocolType тип протокола
type ProtocolType string

const (
	ProtocolNeuro  ProtocolType = "neuro"
	ProtocolHepatic ProtocolType = "hepatic"
	ProtocolWound  ProtocolType = "wound"
)

// ProtocolManager управляет выбором и применением протоколов
type ProtocolManager struct {
	protocols map[ProtocolType]Protocol
	current   Protocol
	limits    SafetyLimits
	generator hal.Generator
	sensor    hal.TissueSensor
	logger    *telemetry.Logger
	metrics   *telemetry.Metrics
	
	mu sync.RWMutex
}

// NewProtocolManager создает новый менеджер протоколов
func NewProtocolManager(
	generator hal.Generator,
	sensor hal.TissueSensor,
	limits SafetyLimits,
	logger *telemetry.Logger,
	metrics *telemetry.Metrics,
) *ProtocolManager {
	pm := &ProtocolManager{
		protocols: make(map[ProtocolType]Protocol),
		limits:    limits,
		generator: generator,
		sensor:    sensor,
		logger:    logger,
		metrics:   metrics,
	}
	
	pm.RegisterProtocol(ProtocolNeuro, NewNeuroProtocol(limits))
	pm.RegisterProtocol(ProtocolHepatic, NewHepaticProtocol(limits))
	pm.RegisterProtocol(ProtocolWound, NewWoundProtocol(limits))
	
	return pm
	}

//  controlLoop с телеметрией
func (pm *ProtocolManager) controlLoop(ctx context.Context) error {
	startTime := time.Now()
	
	// Создаем span для tracing
	ctx, span := telemetry.StartSpan(ctx, "control_loop")
	defer span.End()
	
	defer func() {
		duration := time.Since(startTime).Seconds()
		pm.metrics.ControlLoopDuration.Observe(duration)
		pm.metrics.OperationCyclesTotal.Inc()
	}()
	
	// 1. Получаем состояние
	state, err := pm.generator.GetState(ctx)
	if err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("get_state").Inc()
		return fmt.Errorf("ошибка получения состояния: %w", err)
	}
	
	//  метрики генератора
	pm.metrics.UpdateFromState(
		state.PowerWatts,
		state.FrequencyHz,
		state.TipTempC,
		state.ImpedanceOhms,
		state.AspirationBar,
		state.IrrigationMl,
		state.IsFiring,
	)
	
	// 2. Проверяем safety
	if pm.current.ShouldStop(state) {
		pm.metrics.ProtocolSafetyStops.Inc()
		pm.logger.WithContext(ctx).Warn("Экстренная остановка по условиям безопасности",
			"temp", state.TipTempC,
			"impedance", state.ImpedanceOhms)
		return pm.generator.Stop(ctx)
	}
	
	if !state.IsFiring {
		return nil
	}
	
	// 3. Классифицируем ткань с замером времени
	aiStart := time.Now()
	tissueProbs, err := pm.sensor.Classify(ctx, state)
	aiDuration := time.Since(aiStart).Seconds()
	pm.metrics.AIClassificationDuration.Observe(aiDuration)
	
	if err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("ai_classification").Inc()
		pm.logger.WithContext(ctx).Warn("Ошибка классификации ткани", "error", err)
		tissueProbs = map[hal.TissueType]float64{hal.TissueSoft: 1.0}
	}
	
	// Определяем доминантную ткань
	dominantTissue := hal.TissueUnknown
	maxProb := 0.0
	for tissue, prob := range tissueProbs {
		if prob > maxProb {
			maxProb = prob
			dominantTissue = tissue
		}
	}
	
	// Логируем классификацию
	pm.metrics.AIClassificationsTotal.WithLabelValues(fmt.Sprintf("%d", dominantTissue)).Inc()
	span.SetAttributes(
		attribute.Int("tissue_type", int(dominantTissue)),
		attribute.Float64("confidence", maxProb),
	)
	
	// 4. Получаем параметры
	params, err := pm.current.GetParameters(dominantTissue, state)
	if err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("get_parameters").Inc()
		return fmt.Errorf("ошибка получения параметров: %w", err)
	}
	
	// 5. Корректируем
	params = pm.current.AdjustOnTheFly(params, state)
	
	// 6. Проверяем safety limits
	if err := params.Validate(pm.limits); err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("safety_violation").Inc()
		pm.logger.WithContext(ctx).Error("Параметры превышают safety limits", "error", err)
		return pm.generator.Stop(ctx)
	}
	
	// 7. Применяем параметры
	if err := pm.generator.Start(ctx, params.FrequencyHz, params.PowerWatts); err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("generator_start").Inc()
		return fmt.Errorf("ошибка запуска генератора: %w", err)
	}
	
	if err := pm.generator.SetAspiration(ctx, params.Aspiration); err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("set_aspiration").Inc()
		return fmt.Errorf("ошибка установки аспирации: %w", err)
	}
	
	if err := pm.generator.SetIrrigation(ctx, params.IrrigationMl); err != nil {
		pm.metrics.OperationErrorsTotal.WithLabelValues("set_irrigation").Inc()
		return fmt.Errorf("ошибка установки ирригации: %w", err)
	}
	
	// Structured logging
	pm.logger.WithContext(ctx).Debug("Цикл управления",
		"tissue", dominantTissue,
		"confidence", maxProb,
		"power", params.PowerWatts,
		"freq", params.FrequencyHz,
		"temp", state.TipTempC,
		"impedance", state.ImpedanceOhms,
		"loop_duration_ms", time.Since(startTime).Milliseconds())
	
	return nil
}

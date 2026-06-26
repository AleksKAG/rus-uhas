package domain

import (
	"context"
	"fmt"
	"log/slog"
	"rus-uhas/internal/hal"
	"sync"
	"time"
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
	logger    *slog.Logger
	
	mu sync.RWMutex
}

// NewProtocolManager создает новый менеджер протоколов
func NewProtocolManager(
	generator hal.Generator,
	sensor hal.TissueSensor,
	limits SafetyLimits,
	logger *slog.Logger,
) *ProtocolManager {
	pm := &ProtocolManager{
		protocols: make(map[ProtocolType]Protocol),
		limits:    limits,
		generator: generator,
		sensor:    sensor,
		logger:    logger,
	}
	
	// Регистрируем стандартные протоколы
	pm.RegisterProtocol(ProtocolNeuro, NewNeuroProtocol(limits))
	pm.RegisterProtocol(ProtocolHepatic, NewHepaticProtocol(limits))
	pm.RegisterProtocol(ProtocolWound, NewWoundProtocol(limits))
	
	return pm
}

// RegisterProtocol регистрирует новый протокол
func (pm *ProtocolManager) RegisterProtocol(ptype ProtocolType, protocol Protocol) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.protocols[ptype] = protocol
}

// SetProtocol устанавливает текущий протокол
func (pm *ProtocolManager) SetProtocol(ptype ProtocolType) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	protocol, ok := pm.protocols[ptype]
	if !ok {
		return fmt.Errorf("%w: %s", ErrProtocolNotFound, ptype)
	}
	
	pm.current = protocol
	pm.logger.Info("Установлен протокол", "protocol", protocol.Name())
	return nil
}

// GetCurrentProtocol возвращает текущий протокол
func (pm *ProtocolManager) GetCurrentProtocol() Protocol {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.current
}

// Run запускает цикл управления генератором на основе протокола
// Это основной рабочий цикл системы
func (pm *ProtocolManager) Run(ctx context.Context, interval time.Duration) error {
	if pm.current == nil {
		return fmt.Errorf("протокол не установлен")
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	pm.logger.Info("Запуск цикла управления", "interval", interval)
	
	for {
		select {
		case <-ctx.Done():
			pm.logger.Info("Остановка цикла управления")
			// Экстренная остановка генератора
			if err := pm.generator.Stop(context.Background()); err != nil {
				pm.logger.Error("Ошибка остановки генератора", "error", err)
			}
			return ctx.Err()
			
		case <-ticker.C:
			if err := pm.controlLoop(ctx); err != nil {
				pm.logger.Error("Ошибка в цикле управления", "error", err)
				// При ошибке останавливаем генератор
				if err := pm.generator.Stop(context.Background()); err != nil {
					pm.logger.Error("Ошибка остановки генератора", "error", err)
				}
				return err
			}
		}
	}
}

// controlLoop выполняет один цикл управления
func (pm *ProtocolManager) controlLoop(ctx context.Context) error {
	// 1. Получаем текущее состояние генератора
	state, err := pm.generator.GetState(ctx)
	if err != nil {
		return fmt.Errorf("ошибка получения состояния: %w", err)
	}
	
	// 2. Проверяем safety conditions
	if pm.current.ShouldStop(state) {
		pm.logger.Warn("Экстренная остановка по условиям безопасности",
			"temp", state.TipTempC,
			"impedance", state.ImpedanceOhms)
		return pm.generator.Stop(ctx)
	}
	
	// 3. Если генератор не активен - ничего не делаем
	if !state.IsFiring {
		return nil
	}
	
	// 4. Классифицируем ткань через AI-сенсор
	tissueProbs, err := pm.sensor.Classify(ctx, state)
	if err != nil {
		pm.logger.Warn("Ошибка классификации ткани", "error", err)
		// Используем безопасные параметры по умолчанию
		tissueProbs = map[hal.TissueType]float64{
			hal.TissueSoft: 1.0,
		}
	}
	
	// 5. Определяем наиболее вероятный тип ткани
	dominantTissue := hal.TissueUnknown
	maxProb := 0.0
	for tissue, prob := range tissueProbs {
		if prob > maxProb {
			maxProb = prob
			dominantTissue = tissue
		}
	}
	
	// 6. Получаем параметры протокола для этой ткани
	params, err := pm.current.GetParameters(dominantTissue, state)
	if err != nil {
		return fmt.Errorf("ошибка получения параметров: %w", err)
	}
	
	// 7. Корректируем параметры в реальном времени
	params = pm.current.AdjustOnTheFly(params, state)
	
	// 8. Финальная проверка safety limits
	if err := params.Validate(pm.limits); err != nil {
		pm.logger.Error("Параметры превышают safety limits", "error", err)
		return pm.generator.Stop(ctx)
	}
	
	// 9. Применяем параметры к генератору
	if err := pm.generator.Start(ctx, params.FrequencyHz, params.PowerWatts); err != nil {
		return fmt.Errorf("ошибка запуска генератора: %w", err)
	}
	
	if err := pm.generator.SetAspiration(ctx, params.Aspiration); err != nil {
		return fmt.Errorf("ошибка установки аспирации: %w", err)
	}
	
	if err := pm.generator.SetIrrigation(ctx, params.IrrigationMl); err != nil {
		return fmt.Errorf("ошибка установки ирригации: %w", err)
	}
	
	pm.logger.Debug("Цикл управления",
		"tissue", dominantTissue,
		"power", params.PowerWatts,
		"freq", params.FrequencyHz,
		"temp", state.TipTempC)
	
	return nil
}

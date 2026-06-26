package domain

import (
	"errors"
	"rus-uhas/internal/hal"
)

// Errors
var (
	ErrProtocolNotFound     = errors.New("протокол не найден")
	ErrSafetyLimitExceeded  = errors.New("превышен безопасный лимит")
	ErrInvalidParameter     = errors.New("недопустимый параметр")
)

// SafetyLimits определяет жесткие ограничения для защиты пациента
// Эти значения НЕ могут быть изменены AI или пользователем
type SafetyLimits struct {
	MaxPowerWatts      float64 // Максимальная мощность (Вт)
	MaxFrequencyHz     float64 // Максимальная частота (Гц)
	MaxTipTempC        float64 // Максимальная температура наконечника (°C)
	MaxAspirationBar   float64 // Максимальный вакуум (бар)
	MaxIrrigationMlMin float64 // Максимальный поток ирригации (мл/мин)
	MaxImpedanceOhms   float64 // Максимальный импеданс (Ом) - защита от короткого замыкания
}

// DefaultSafetyLimits - стандартные ограничения для человека
func DefaultSafetyLimits() SafetyLimits {
	return SafetyLimits{
		MaxPowerWatts:      50.0,
		MaxFrequencyHz:     60000.0,
		MaxTipTempC:        80.0, // Выше 80°C - риск термического повреждения
		MaxAspirationBar:   0.9,
		MaxIrrigationMlMin: 200.0,
		MaxImpedanceOhms:   200.0,
	}
}

// ProtocolParameters - параметры протокола для конкретного типа ткани
type ProtocolParameters struct {
	FrequencyHz    float64
	PowerWatts     float64
	Aspiration     float64 // 0.0 - 1.0 (относительно максимума)
	IrrigationMl   float64
	Mode           string  // "continuous", "pulsed", "burst"
	PulseFrequency float64 // Для импульсного режима (Гц)
}

// Validate проверяет параметры на соответствие safety limits
func (p ProtocolParameters) Validate(limits SafetyLimits) error {
	if p.PowerWatts > limits.MaxPowerWatts {
		return ErrSafetyLimitExceeded
	}
	if p.FrequencyHz > limits.MaxFrequencyHz {
		return ErrSafetyLimitExceeded
	}
	if p.Aspiration > limits.MaxAspirationBar {
		return ErrSafetyLimitExceeded
	}
	if p.IrrigationMl > limits.MaxIrrigationMlMin {
		return ErrSafetyLimitExceeded
	}
	return nil
}

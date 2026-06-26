package domain

import (
	"context"
	"rus-uhas/internal/hal"
)

// Protocol определяет контракт для хирургического протокола
type Protocol interface {
	// Name возвращает имя протокола
	Name() string
	
	// GetParameters возвращает оптимальные параметры для данного типа ткани
	GetParameters(tissue hal.TissueType, state hal.GeneratorState) (ProtocolParameters, error)
	
	// ShouldStop возвращает true, если нужно экстренно остановить генерацию
	ShouldStop(state hal.GeneratorState) bool
	
	// AdjustOnTheFly корректирует параметры в реальном времени (например, при изменении импеданса)
	AdjustOnTheFly(params ProtocolParameters, state hal.GeneratorState) ProtocolParameters
}

// NeuroProtocol - протокол для нейрохирургии (высокая точность, низкая мощность)
type NeuroProtocol struct {
	limits SafetyLimits
}

func NewNeuroProtocol(limits SafetyLimits) *NeuroProtocol {
	return &NeuroProtocol{limits: limits}
}

func (p *NeuroProtocol) Name() string {
	return "Нейрохирургия"
}

func (p *NeuroProtocol) GetParameters(tissue hal.TissueType, state hal.GeneratorState) (ProtocolParameters, error) {
	params := ProtocolParameters{
		Mode: "continuous",
	}
	
	switch tissue {
	case hal.TissueSoft:
		params.FrequencyHz = 25000 // 25 кГц - для мягкой ткани мозга
		params.PowerWatts = 15.0   // Низкая мощность
		params.Aspiration = 0.3
		params.IrrigationMl = 50.0
	case hal.TissueVessel:
		params.FrequencyHz = 25000
		params.PowerWatts = 10.0 // Еще ниже - защита сосудов
		params.Aspiration = 0.2
		params.IrrigationMl = 80.0 // Увеличенная ирригация для охлаждения
	case hal.TissueNerve:
		params.FrequencyHz = 25000
		params.PowerWatts = 5.0 // Минимальная мощность
		params.Aspiration = 0.1
		params.IrrigationMl = 100.0
	case hal.TissueTumor:
		params.FrequencyHz = 25000
		params.PowerWatts = 20.0 // Чуть выше для опухоли
		params.Aspiration = 0.5
		params.IrrigationMl = 60.0
	default:
		params.FrequencyHz = 25000
		params.PowerWatts = 10.0
		params.Aspiration = 0.2
		params.IrrigationMl = 50.0
	}
	
	if err := params.Validate(p.limits); err != nil {
		return ProtocolParameters{}, err
	}
	
	return params, nil
}

func (p *NeuroProtocol) ShouldStop(state hal.GeneratorState) bool {
	// Экстренная остановка при перегреве или высоком импедансе
	return state.TipTempC > 70.0 || state.ImpedanceOhms > 150.0
}

func (p *NeuroProtocol) AdjustOnTheFly(params ProtocolParameters, state hal.GeneratorState) ProtocolParameters {
	// Если импеданс растет - снижаем мощность (ткань становится плотнее)
	if state.ImpedanceOhms > 100.0 {
		params.PowerWatts *= 0.8
	}
	// Если температура растет - увеличиваем ирригацию
	if state.TipTempC > 60.0 {
		params.IrrigationMl *= 1.2
	}
	return params
}

// HepaticProtocol - протокол для гепатологии (резекция печени)
type HepaticProtocol struct {
	limits SafetyLimits
}

func NewHepaticProtocol(limits SafetyLimits) *HepaticProtocol {
	return &HepaticProtocol{limits: limits}
}

func (p *HepaticProtocol) Name() string {
	return "Гепатология"
}

func (p *HepaticProtocol) GetParameters(tissue hal.TissueType, state hal.GeneratorState) (ProtocolParameters, error) {
	params := ProtocolParameters{
		Mode: "continuous",
	}
	
	switch tissue {
	case hal.TissueSoft:
		params.FrequencyHz = 25000
		params.PowerWatts = 30.0 // Высокая мощность для быстрой диссекции
		params.Aspiration = 0.6
		params.IrrigationMl = 100.0
	case hal.TissueVessel:
		params.FrequencyHz = 25000
		params.PowerWatts = 25.0
		params.Aspiration = 0.4
		params.IrrigationMl = 120.0
	default:
		params.FrequencyHz = 25000
		params.PowerWatts = 25.0
		params.Aspiration = 0.5
		params.IrrigationMl = 100.0
	}
	
	if err := params.Validate(p.limits); err != nil {
		return ProtocolParameters{}, err
	}
	
	return params, nil
}

func (p *HepaticProtocol) ShouldStop(state hal.GeneratorState) bool {
	return state.TipTempC > 75.0 || state.ImpedanceOhms > 180.0
}

func (p *HepaticProtocol) AdjustOnTheFly(params ProtocolParameters, state hal.GeneratorState) ProtocolParameters {
	if state.ImpedanceOhms > 120.0 {
		params.PowerWatts *= 0.85
	}
	if state.TipTempC > 65.0 {
		params.IrrigationMl *= 1.3
	}
	return params
}

// WoundProtocol - протокол для хирургии ран (импульсный режим)
type WoundProtocol struct {
	limits SafetyLimits
}

func NewWoundProtocol(limits SafetyLimits) *WoundProtocol {
	return &WoundProtocol{limits: limits}
}

func (p *WoundProtocol) Name() string {
	return "Хирургия ран"
}

func (p *WoundProtocol) GetParameters(tissue hal.TissueType, state hal.GeneratorState) (ProtocolParameters, error) {
	// Для ран используем импульсный режим для деликатной очистки
	params := ProtocolParameters{
		Mode:           "pulsed",
		FrequencyHz:    25000,
		PowerWatts:     20.0,
		Aspiration:     0.7, // Высокая аспирация для удаления некротических тканей
		IrrigationMl:   150.0,
		PulseFrequency: 10.0, // 10 Гц пульсация
	}
	
	if err := params.Validate(p.limits); err != nil {
		return ProtocolParameters{}, err
	}
	
	return params, nil
}

func (p *WoundProtocol) ShouldStop(state hal.GeneratorState) bool {
	return state.TipTempC > 70.0 || state.ImpedanceOhms > 160.0
}

func (p *WoundProtocol) AdjustOnTheFly(params ProtocolParameters, state hal.GeneratorState) ProtocolParameters {
	if state.TipTempC > 60.0 {
		params.PulseFrequency *= 0.8 // Снижаем частоту пульсации
		params.IrrigationMl *= 1.2
	}
	return params
}

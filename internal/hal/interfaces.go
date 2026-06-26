package hal

import (
	"context"
	"time"
)

// TissueType представляет тип ткани, определяемый датчиками
type TissueType int

const (
	TissueUnknown TissueType = iota
	TissueSoft
	TissueVessel
	TissueNerve
	TissueBone
	TissueTumor
)

// GeneratorState текущее состояние ультразвукового генератора
type GeneratorState struct {
	IsFiring      bool
	FrequencyHz   float64
	PowerWatts    float64
	TipTempC      float64
	ImpedanceOhms float64
	AspirationBar float64
	IrrigationMl  float64
	Timestamp     time.Time
}

// Generator определяет контракт для управления ультразвуковым генератором.
// В реальности это будет gRPC или UART клиент к embedded-части.
type Generator interface {
	// Start генерацию ультразвука с заданными параметрами
	Start(ctx context.Context, freqHz, powerWatts float64) error
	
	// Stop генерацию
	Stop(ctx context.Context) error
	
	// GetState возвращает текущее состояние датчиков и генератора
	GetState(ctx context.Context) (GeneratorState, error)
	
	// SetAspiration устанавливает уровень вакуума (0.0 - 1.0)
	SetAspiration(ctx context.Context, level float64) error
	
	// SetIrrigation устанавливает поток промывания (мл/мин)
	SetIrrigation(ctx context.Context, flowMlMin float64) error
}

// TissueSensor контракт для AI-датчика/анализатора тканей
type TissueSensor interface {
	// Classify возвращает вероятностное распределение типов тканей
	// на основе импеданса, температуры и акустического отклика
	Classify(ctx context.Context, state GeneratorState) (map[TissueType]float64, error)
}

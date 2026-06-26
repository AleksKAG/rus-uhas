package mock

import (
	"context"
	"math/rand"
	"time"

	"rus-uhas/internal/hal"
)

type MockGenerator struct {
	state hal.GeneratorState
}

func NewMockGenerator() *MockGenerator {
	return &MockGenerator{
		state: hal.GeneratorState{
			FrequencyHz: 25000, // 25 кГц по умолчанию
		},
	}
}

func (m *MockGenerator) Start(ctx context.Context, freqHz, powerWatts float64) error {
	m.state.IsFiring = true
	m.state.FrequencyHz = freqHz
	m.state.PowerWatts = powerWatts
	return nil
}

func (m *MockGenerator) Stop(ctx context.Context) error {
	m.state.IsFiring = false
	m.state.PowerWatts = 0
	return nil
}

func (m *MockGenerator) GetState(ctx context.Context) (hal.GeneratorState, error) {
	// Имитируем "шум" и изменение данных с датчиков в реальном времени
	if m.state.IsFiring {
		m.state.TipTempC += rand.Float64() * 0.5 // Нагрев
		m.state.ImpedanceOhms = 50 + rand.Float64() * 20 // Импеданс ткани
	}
	m.state.Timestamp = time.Now()
	return m.state, nil
}

func (m *MockGenerator) SetAspiration(ctx context.Context, level float64) error {
	m.state.AspirationBar = level * 0.9 // Макс 0.9 бар
	return nil
}

func (m *MockGenerator) SetIrrigation(ctx context.Context, flowMlMin float64) error {
	m.state.IrrigationMl = flowMlMin
	return nil
}

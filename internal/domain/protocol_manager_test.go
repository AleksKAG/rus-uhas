package domain

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"rus-uhas/internal/hal"
	"rus-uhas/internal/hal/mock"
)

// MockTissueSensor для тестов
type MockTissueSensor struct {
	tissue hal.TissueType
}

// Classify возвращает вероятностное распределение типов тканей
func (m *MockTissueSensor) Classify(ctx context.Context, state hal.GeneratorState) (map[hal.TissueType]float64, error) {
	return map[hal.TissueType]float64{
		m.tissue: 1.0,
	}, nil
}

// TestProtocolManager_SetProtocol тестирует установку протокола
func TestProtocolManager_SetProtocol(t *testing.T) {
	gen := mock.NewMockGenerator()
	sensor := &MockTissueSensor{tissue: hal.TissueSoft}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pm := NewProtocolManager(gen, sensor, DefaultSafetyLimits(), logger)

	err := pm.SetProtocol(ProtocolNeuro)
	if err != nil {
		t.Fatalf("ошибка установки протокола: %v", err)
	}

	if pm.GetCurrentProtocol().Name() != "Нейрохирургия" {
		t.Errorf("неверный протокол: %s", pm.GetCurrentProtocol().Name())
	}
}

// TestProtocolManager_InvalidProtocol тестирует установку несуществующего протокола
func TestProtocolManager_InvalidProtocol(t *testing.T) {
	gen := mock.NewMockGenerator()
	sensor := &MockTissueSensor{tissue: hal.TissueSoft}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pm := NewProtocolManager(gen, sensor, DefaultSafetyLimits(), logger)

	err := pm.SetProtocol("invalid_protocol")
	if err == nil {
		t.Error("должна быть ошибка при установке несуществующего протокола")
	}
}

// TestNeuroProtocol_Parameters тестирует параметры нейрохирургического протокола
func TestNeuroProtocol_Parameters(t *testing.T) {
	protocol := NewNeuroProtocol(DefaultSafetyLimits())
	state := hal.GeneratorState{}

	// Тест для мягкой ткани
	params, err := protocol.GetParameters(hal.TissueSoft, state)
	if err != nil {
		t.Fatalf("ошибка получения параметров для мягкой ткани: %v", err)
	}
	if params.PowerWatts > 20.0 {
		t.Errorf("мощность для мягкой ткани слишком высокая: %f", params.PowerWatts)
	}

	// Тест для сосудов
	params, err = protocol.GetParameters(hal.TissueVessel, state)
	if err != nil {
		t.Fatalf("ошибка получения параметров для сосудов: %v", err)
	}
	if params.PowerWatts > 15.0 {
		t.Errorf("мощность для сосудов слишком высокая: %f", params.PowerWatts)
	}

	// Тест для нервов
	params, err = protocol.GetParameters(hal.TissueNerve, state)
	if err != nil {
		t.Fatalf("ошибка получения параметров для нервов: %v", err)
	}
	if params.PowerWatts > 10.0 {
		t.Errorf("мощность для нервов слишком высокая: %f", params.PowerWatts)
	}
}

// TestHepaticProtocol_Parameters тестирует параметры гепатологического протокола
func TestHepaticProtocol_Parameters(t *testing.T) {
	protocol := NewHepaticProtocol(DefaultSafetyLimits())
	state := hal.GeneratorState{}

	params, err := protocol.GetParameters(hal.TissueSoft, state)
	if err != nil {
		t.Fatalf("ошибка получения параметров: %v", err)
	}
	if params.PowerWatts < 25.0 {
		t.Errorf("мощность для печени слишком низкая: %f", params.PowerWatts)
	}
}

// TestWoundProtocol_Parameters тестирует параметры протокола для хирургии ран
func TestWoundProtocol_Parameters(t *testing.T) {
	protocol := NewWoundProtocol(DefaultSafetyLimits())
	state := hal.GeneratorState{}

	params, err := protocol.GetParameters(hal.TissueSoft, state)
	if err != nil {
		t.Fatalf("ошибка получения параметров: %v", err)
	}
	if params.Mode != "pulsed" {
		t.Errorf("режим должен быть pulsed, получен: %s", params.Mode)
	}
}

// TestProtocolManager_ShouldStop тестирует условия экстренной остановки
func TestProtocolManager_ShouldStop(t *testing.T) {
	protocol := NewNeuroProtocol(DefaultSafetyLimits())

	// Тест на перегрев
	state := hal.GeneratorState{
		TipTempC: 75.0,
	}
	if !protocol.ShouldStop(state) {
		t.Error("протокол должен остановить генератор при перегреве (75°C)")
	}

	// Тест на высокий импеданс
	state = hal.GeneratorState{
		ImpedanceOhms: 160.0,
	}
	if !protocol.ShouldStop(state) {
		t.Error("протокол должен остановить генератор при высоком импедансе (160 Ом)")
	}

	// Тест на нормальные условия
	state = hal.GeneratorState{
		TipTempC:      50.0,
		ImpedanceOhms: 80.0,
	}
	if protocol.ShouldStop(state) {
		t.Error("протокол не должен останавливать генератор при нормальных условиях")
	}

	// Тест на граничные значения
	state = hal.GeneratorState{
		TipTempC:      70.0, // Граница
		ImpedanceOhms: 150.0, // Граница
	}
	if !protocol.ShouldStop(state) {
		t.Error("протокол должен остановить генератор на граничных значениях")
	}
}

// TestProtocolManager_AdjustOnTheFly тестирует адаптивную подстройку параметров
func TestProtocolManager_AdjustOnTheFly(t *testing.T) {
	protocol := NewNeuroProtocol(DefaultSafetyLimits())
	state := hal.GeneratorState{}

	params, _ := protocol.GetParameters(hal.TissueSoft, state)
	initialPower := params.PowerWatts
	initialIrrigation := params.IrrigationMl

	// Тест при высоком импедансе
	state = hal.GeneratorState{
		ImpedanceOhms: 120.0,
	}
	adjustedParams := protocol.AdjustOnTheFly(params, state)
	if adjustedParams.PowerWatts >= initialPower {
		t.Error("мощность должна снизиться при высоком импедансе")
	}

	// Тест при высокой температуре
	state = hal.GeneratorState{
		TipTempC: 65.0,
	}
	adjustedParams = protocol.AdjustOnTheFly(params, state)
	if adjustedParams.IrrigationMl <= initialIrrigation {
		t.Error("ирригация должна увеличиться при высокой температуре")
	}
}

// TestSafetyLimits_Validate тестирует валидацию параметров
func TestSafetyLimits_Validate(t *testing.T) {
	limits := DefaultSafetyLimits()

	// Нормальные параметры
	params := ProtocolParameters{
		PowerWatts:   30.0,
		FrequencyHz:  25000.0,
		Aspiration:   0.5,
		IrrigationMl: 100.0,
	}
	if err := params.Validate(limits); err != nil {
		t.Errorf("нормальные параметры не должны вызывать ошибку: %v", err)
	}

	// Превышение мощности
	params.PowerWatts = 60.0
	if err := params.Validate(limits); err == nil {
		t.Error("превышение мощности должно вызывать ошибку")
	}

	// Превышение частоты
	params.PowerWatts = 30.0
	params.FrequencyHz = 70000.0
	if err := params.Validate(limits); err == nil {
		t.Error("превышение частоты должно вызывать ошибку")
	}
}

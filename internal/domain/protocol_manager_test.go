package domain

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"rus-uhas/internal/hal"
	"rus-uhas/internal/hal/mock"
)

// MockTissueSensor для тестов
type MockTissueSensor struct {
	tissue hal.TissueType
}

func (m *MockTissueSensor) Classify(ctx context.Context, state hal.GeneratorState) (map[hal.TissueType]float64, error) {
	return map[hal.TissueType]float64{
		m.tissue: 1.0,
	}, nil
}

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

func TestProtocolManager_SafetyStop(t *testing.T) {
	gen := mock.NewMockGenerator()
	sensor := &MockTissueSensor{tissue: hal.TissueSoft}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	pm := NewProtocolManager(gen, sensor, DefaultSafetyLimits(), logger)
	pm.SetProtocol(ProtocolNeuro)
	
	// Запускаем генератор
	ctx := context.Background()
	gen.Start(ctx, 25000, 15.0)
	
	// Имитируем перегрев
	gen.(*mock.MockGenerator).SetTipTemp(75.0)
	
	// Запускаем цикл управления на короткое время
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	
	go pm.Run(ctx, 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	
	// Проверяем, что генератор остановлен
	state, _ := gen.GetState(context.Background())
	if state.IsFiring {
		t.Error("генератор должен был остановиться из-за перегрева")
	}
}

func TestNeuroProtocol_Parameters(t *testing.T) {
	protocol := NewNeuroProtocol(DefaultSafetyLimits())
	state := hal.GeneratorState{}
	
	params, err := protocol.GetParameters(hal.TissueVessel, state)
	if err != nil {
		t.Fatalf("ошибка получения параметров: %v", err)
	}
	
	if params.PowerWatts > 15.0 {
		t.Errorf("мощность для сосудов слишком высокая: %f", params.PowerWatts)
	}
}

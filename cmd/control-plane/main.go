package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"rus-uhas/internal/domain"
	"rus-uhas/internal/hal/mock"
	"rus-uhas/internal/telemetry"
)

func main() {
	// Инициализация logger
	logger := telemetry.NewLogger(slog.LevelInfo)
	logger.Info("Запуск РУС-УХАС Control Plane")

	// Инициализация метрик
	metrics := telemetry.NewMetrics()

	// Инициализация tracing (опционально, если есть OTLP endpoint)
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		cleanup, err := telemetry.InitTracer(context.Background(), "rus-uhas-control-plane", endpoint)
		if err != nil {
			logger.Error("Ошибка инициализации tracing", "error", err)
		} else {
			defer cleanup()
			logger.Info("Tracing инициализирован", "endpoint", endpoint)
		}
	}

	// Создание mock генератора и сенсора
	generator := mock.NewMockGenerator()
	sensor := &MockTissueSensor{tissue: 1} // TissueSoft

	// Создание ProtocolManager
	pm := domain.NewProtocolManager(
		generator,
		sensor,
		domain.DefaultSafetyLimits(),
		logger,
		metrics,
	)

	// Установка протокола
	if err := pm.SetProtocol(domain.ProtocolNeuro); err != nil {
		logger.Error("Ошибка установки протокола", "error", err)
		os.Exit(1)
	}

	// HTTP сервер для метрик Prometheus
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	go func() {
		logger.Info("Запуск HTTP сервера для метрик", "port", 8080)
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Error("Ошибка HTTP сервера", "error", err)
		}
	}()

	// Запуск основного цикла управления
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Генерируем correlation ID для сессии
	correlationID := telemetry.GenerateCorrelationID()
	operationID := telemetry.GenerateOperationID()
	ctx = telemetry.WithCorrelationID(ctx, correlationID)
	ctx = telemetry.WithOperationID(ctx, operationID)

	logger.Info("Запуск цикла управления",
		"correlation_id", correlationID,
		"operation_id", operationID)

	// Запуск в горутине
	go func() {
		if err := pm.Run(ctx, 100*time.Millisecond); err != nil && err != context.Canceled {
			logger.Error("Ошибка в цикле управления", "error", err)
			cancel()
		}
	}()

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Получен сигнал завершения, останавливаем систему")
	cancel()

	// Даем время на graceful shutdown
	time.Sleep(1 * time.Second)
	logger.Info("Система остановлена")
}

// MockTissueSensor для демонстрации
type MockTissueSensor struct {
	tissue int
}

func (m *MockTissueSensor) Classify(ctx context.Context, state interface{}) (map[int]float64, error) {
	return map[int]float64{m.tissue: 1.0}, nil
}

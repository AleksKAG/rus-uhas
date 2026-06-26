package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"rus-uhas/internal/ai"
	"rus-uhas/internal/domain"
	"rus-uhas/internal/hal"
	"rus-uhas/internal/hal/mock"
	"rus-uhas/internal/telemetry"
)

// Config - конфигурация приложения
type Config struct {
	// HTTP сервер
	HTTPPort int
	MetricsPath string
	
	// Control loop
	ControlLoopInterval time.Duration
	
	// Tracing
	OTELEndpoint  string
	ServiceName   string
	
	// AI модель
	ONNXModelPath string
	UseMockHAL    bool
	
	// Протокол по умолчанию
	DefaultProtocol string
	
	// Логирование
	LogLevel string
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() Config {
	cfg := Config{
		HTTPPort:            getEnvInt("HTTP_PORT", 8080),
		MetricsPath:         getEnv("METRICS_PATH", "/metrics"),
		ControlLoopInterval: time.Duration(getEnvInt("CONTROL_LOOP_INTERVAL_MS", 100)) * time.Millisecond,
		OTELEndpoint:        getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		ServiceName:         getEnv("SERVICE_NAME", "rus-uhas-control-plane"),
		ONNXModelPath:       getEnv("ONNX_MODEL_PATH", "./models/tissue_classifier.onnx"),
		UseMockHAL:          getEnvBool("USE_MOCK_HAL", true),
		DefaultProtocol:     getEnv("DEFAULT_PROTOCOL", "neuro"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
	}
	return cfg
}

func main() {
	// Загружаем конфигурацию
	cfg := LoadConfig()
	
	// 1. Инициализация Logger
	logLevel := parseLogLevel(cfg.LogLevel)
	logger := telemetry.NewLogger(logLevel)
	logger.Info("Запуск РУС-УХАС Control Plane",
		"version", "1.0.0",
		"config", cfg)
	
	// 2. Инициализация метрик
	metrics := telemetry.NewMetrics()
	logger.Info("Метрики инициализированы")
	
	// 3. Инициализация Tracing (опционально)
	if cfg.OTELEndpoint != "" {
		cleanup, err := telemetry.InitTracer(context.Background(), cfg.ServiceName, cfg.OTELEndpoint)
		if err != nil {
			logger.Error("Ошибка инициализации tracing", "error", err)
			// Не критично, продолжаем без tracing
		} else {
			defer cleanup()
			logger.Info("Tracing инициализирован", "endpoint", cfg.OTELEndpoint)
		}
	} else {
		logger.Info("Tracing отключен (OTEL_EXPORTER_OTLP_ENDPOINT не задан)")
	}
	
	// 4. Создание HAL (Hardware Abstraction Layer)
	var generator hal.Generator
	if cfg.UseMockHAL {
		logger.Info("Используется Mock HAL (для разработки)")
		generator = mock.NewMockGenerator()
	} else {
		// TODO: Реализовать real HAL через gRPC/UART к embedded
		logger.Warn("Real HAL не реализован, переключаемся на Mock")
		generator = mock.NewMockGenerator()
	}
	
	// 5. Создание AI классификатора тканей
	tissueSensor := initTissueSensor(cfg, logger)
	
	// 6. Создание ProtocolManager
	limits := domain.DefaultSafetyLimits()
	pm := domain.NewProtocolManager(
		generator,
		tissueSensor,
		limits,
		logger,
		metrics,
	)
	
	// Устанавливаем протокол по умолчанию
	protocolType := domain.ProtocolType(cfg.DefaultProtocol)
	if err := pm.SetProtocol(protocolType); err != nil {
		logger.Error("Ошибка установки протокола по умолчанию",
			"protocol", cfg.DefaultProtocol,
			"error", err)
		os.Exit(1)
	}
	logger.Info("Протокол установлен", "protocol", pm.GetCurrentProtocol().Name())
	
	// 7. Запуск HTTP сервера для метрик и health check
	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	mux := http.NewServeMux()
	
	// Prometheus metrics endpoint
	mux.Handle(cfg.MetricsPath, promhttp.Handler())
	
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"rus-uhas-control-plane"}`))
	})
	
	// Readiness check (проверяет, что система готова к работе)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Проверить, что генератор и сенсор инициализированы
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})
	
	// Info endpoint (версия, конфигурация)
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"version":"1.0.0","protocol":"%s","mock_hal":%t}`,
			pm.GetCurrentProtocol().Name(), cfg.UseMockHAL)
	})
	
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	go func() {
		logger.Info("Запуск HTTP сервера", "addr", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Ошибка HTTP сервера", "error", err)
		}
	}()
	
	// 8. Запуск основного цикла управления
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Генерируем correlation ID для сессии
	correlationID := telemetry.GenerateCorrelationID()
	operationID := telemetry.GenerateOperationID()
	ctx = telemetry.WithCorrelationID(ctx, correlationID)
	ctx = telemetry.WithOperationID(ctx, operationID)
	
	logger.Info("Запуск цикла управления",
		"correlation_id", correlationID,
		"operation_id", operationID,
		"interval", cfg.ControlLoopInterval)
	
	// Запуск control loop в горутине
	controlLoopDone := make(chan error, 1)
	go func() {
		err := pm.Run(ctx, cfg.ControlLoopInterval)
		controlLoopDone <- err
	}()
	
	// 9. Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Ждем либо сигнала, либо ошибки в control loop
	select {
	case sig := <-sigChan:
		logger.Info("Получен сигнал завершения", "signal", sig)
	case err := <-controlLoopDone:
		if err != nil && err != context.Canceled {
			logger.Error("Control loop завершился с ошибкой", "error", err)
		}
	}
	
	// 10. Graceful shutdown
	logger.Info("Начинаем graceful shutdown")
	
	// Отменяем контекст (останавливает control loop)
	cancel()
	
	// Даем время на завершение control loop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	// Останавливаем HTTP сервер
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Ошибка при остановке HTTP сервера", "error", err)
	} else {
		logger.Info("HTTP сервер остановлен")
	}
	
	// Ждем завершения control loop
	select {
	case <-controlLoopDone:
		logger.Info("Control loop завершен")
	case <-time.After(3 * time.Second):
		logger.Warn("Control loop не завершился вовремя, принудительная остановка")
	}
	
	// Финальная остановка генератора (safety)
	if err := generator.Stop(context.Background()); err != nil {
		logger.Error("Ошибка остановки генератора", "error", err)
	} else {
		logger.Info("Генератор остановлен")
	}
	
	logger.Info("Система полностью остановлена",
		"correlation_id", correlationID,
		"operation_id", operationID)
}

// initTissueSensor инициализирует AI классификатор тканей
func initTissueSensor(cfg Config, logger *telemetry.Logger) hal.TissueSensor {
	// Пробуем загрузить ONNX модель
	onnxConfig := ai.DefaultONNXConfig(cfg.ONNXModelPath)
	onnxClassifier, err := ai.NewONNXClassifier(onnxConfig)
	if err != nil {
		logger.Warn("Не удалось загрузить ONNX модель, используем эвристику",
			"model_path", cfg.ONNXModelPath,
			"error", err)
		
		// Fallback на эвристику
		heuristic := ai.NewHeuristicClassifier(ai.DefaultThresholds())
		logger.Info("Используется эвристический классификатор")
		return heuristic
	}
	
	logger.Info("ONNX модель загружена успешно",
		"model_path", cfg.ONNXModelPath)
	
	// Оборачиваем в fallback-классификатор
	heuristic := ai.NewHeuristicClassifier(ai.DefaultThresholds())
	fallbackClassifier := ai.NewFallbackClassifier(onnxClassifier, heuristic, logger.Logger)
	
	logger.Info("Используется ONNX классификатор с fallback на эвристику")
	return fallbackClassifier
}

// parseLogLevel парсит строковый уровень логирования
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// getEnv возвращает значение переменной окружения или default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt возвращает целочисленное значение переменной окружения или default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool возвращает булево значение переменной окружения или default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

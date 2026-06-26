package ai

import (
	"context"
	"fmt"
	"rus-uhas/internal/hal"
	"sync"

	"github.com/yalue/onnxruntime_go"
)

// ONNXClassifier - классификатор на основе ONNX модели
type ONNXClassifier struct {
	session   *onnxruntime_go.AdvancedSession
	inputTensor *onnxruntime_go.Tensor[float32]
	outputTensor *onnxruntime_go.Tensor[float32]
	
	// Нормализация входных данных (mean/std из обучения)
	impedanceMean float32
	impedanceStd  float32
	tempMean      float32
	tempStd       float32
	
	mu sync.RWMutex
}

// ONNXConfig - конфигурация ONNX классификатора
type ONNXConfig struct {
	ModelPath      string
	ImpedanceMean  float32
	ImpedanceStd   float32
	TempMean       float32
	TempStd        float32
}

// DefaultONNXConfig - стандартная конфигурация
func DefaultONNXConfig(modelPath string) ONNXConfig {
	return ONNXConfig{
		ModelPath:     modelPath,
		ImpedanceMean: 80.0,  // Средний импеданс из обучающей выборки
		ImpedanceStd:  30.0,
		TempMean:      40.0,
		TempStd:       5.0,
	}
}

// NewONNXClassifier создает новый ONNX классификатор
func NewONNXClassifier(config ONNXConfig) (*ONNXClassifier, error) {
	// Инициализируем ONNX Runtime (один раз на процесс)
	// В реальности путь к библиотеке должен быть в конфиге
	err := onnxruntime_go.InitializeEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX runtime: %w", err)
	}
	
	// Входные данные: [impedance, temperature, power, aspiration, irrigation]
	// Batch size = 1
	inputShape := onnxruntime_go.NewShape(1, 5)
	inputTensor, err := onnxruntime_go.NewTensor(inputShape, []float32{0, 0, 0, 0, 0})
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	
	// Выходные данные: вероятности для 5 типов тканей
	outputShape := onnxruntime_go.NewShape(1, 5)
	outputTensor, err := onnxruntime_go.NewTensor(outputShape, make([]float32, 5))
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	
	// Создаем сессию
	session, err := onnxruntime_go.NewAdvancedSession(
		config.ModelPath,
		[]string{"input"},
		[]string{"output"},
		[]onnxruntime_go.ArbitraryTensor{inputTensor},
		[]onnxruntime_go.ArbitraryTensor{outputTensor},
		nil, // Опции (можно настроить thread pool)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}
	
	return &ONNXClassifier{
		session:       session,
		inputTensor:   inputTensor,
		outputTensor:  outputTensor,
		impedanceMean: config.ImpedanceMean,
		impedanceStd:  config.ImpedanceStd,
		tempMean:      config.TempMean,
		tempStd:       config.TempStd,
	}, nil
}

// Classify выполняет инференс ONNX модели
func (c *ONNXClassifier) Classify(ctx context.Context, state hal.GeneratorState) (map[hal.TissueType]float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Нормализуем входные данные (z-score normalization)
	impedanceNorm := (float32(state.ImpedanceOhms) - c.impedanceMean) / c.impedanceStd
	tempNorm := (float32(state.TipTempC) - c.tempMean) / c.tempStd
	powerNorm := float32(state.PowerWatts) / 50.0  // Нормализуем к [0, 1]
	aspirationNorm := float32(state.AspirationBar)
	irrigationNorm := float32(state.IrrigationMl) / 200.0
	
	// Заполняем входной тензор
	inputData := c.inputTensor.GetData()
	inputData[0] = impedanceNorm
	inputData[1] = tempNorm
	inputData[2] = powerNorm
	inputData[3] = aspirationNorm
	inputData[4] = irrigationNorm
	
	// Запускаем инференс
	if err := c.session.Run(); err != nil {
		return nil, fmt.Errorf("ONNX inference failed: %w", err)
	}
	
	// Получаем выходные данные (softmax probabilities)
	outputData := c.outputTensor.GetData()
	
	// Маппим на типы тканей
	// Порядок должен совпадать с тем, как модель обучалась!
	probs := map[hal.TissueType]float64{
		hal.TissueSoft:   float64(outputData[0]),
		hal.TissueVessel: float64(outputData[1]),
		hal.TissueNerve:  float64(outputData[2]),
		hal.TissueBone:   float64(outputData[3]),
		hal.TissueTumor:  float64(outputData[4]),
	}
	
	return probs, nil
}

// Close освобождает ресурсы ONNX
func (c *ONNXClassifier) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.session != nil {
		if err := c.session.Destroy(); err != nil {
			return err
		}
	}
	if c.inputTensor != nil {
		if err := c.inputTensor.Destroy(); err != nil {
			return err
		}
	}
	if c.outputTensor != nil {
		if err := c.outputTensor.Destroy(); err != nil {
			return err
		}
	}
	
	onnxruntime_go.DestroyEnvironment()
	return nil
}

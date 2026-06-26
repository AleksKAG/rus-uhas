package ai

import (
	"context"
	"rus-uhas/internal/hal"
)

// HeuristicClassifier - простой классификатор на основе правил
// Используется как fallback, если ONNX модель недоступна
type HeuristicClassifier struct {
	// Пороговые значения для типов тканей (на основе клинических данных)
	thresholds TissueThresholds
}

// TissueThresholds - пороговые значения импеданса и температуры для тканей
type TissueThresholds struct {
	Soft  ImpedanceRange
	Vessel ImpedanceRange
	Nerve  ImpedanceRange
	Bone   ImpedanceRange
	Tumor  ImpedanceRange
}

// ImpedanceRange - диапазон импеданса для типа ткани
type ImpedanceRange struct {
	MinOhms float64
	MaxOhms float64
	TempMinC float64
	TempMaxC float64
}

// DefaultThresholds - стандартные пороговые значения
// Основаны на литературных данных по биоимпедансу тканей
func DefaultThresholds() TissueThresholds {
	return TissueThresholds{
		Soft: ImpedanceRange{
			MinOhms: 30, MaxOhms: 80,
			TempMinC: 35, TempMaxC: 45,
		},
		Vessel: ImpedanceRange{
			MinOhms: 50, MaxOhms: 120,
			TempMinC: 36, TempMaxC: 42,
		},
		Nerve: ImpedanceRange{
			MinOhms: 80, MaxOhms: 150,
			TempMinC: 36, TempMaxC: 40,
		},
		Bone: ImpedanceRange{
			MinOhms: 150, MaxOhms: 500,
			TempMinC: 35, TempMaxC: 50,
		},
		Tumor: ImpedanceRange{
			MinOhms: 20, MaxOhms: 60, // Опухоли часто имеют пониженный импеданс
			TempMinC: 37, TempMaxC: 48, // Часто повышена температура
		},
	}
}

func NewHeuristicClassifier(thresholds TissueThresholds) *HeuristicClassifier {
	return &HeuristicClassifier{thresholds: thresholds}
}

// Classify классифицирует ткань на основе импеданса и температуры
// Возвращает вероятностное распределение (сумма = 1.0)
func (c *HeuristicClassifier) Classify(ctx context.Context, state hal.GeneratorState) (map[hal.TissueType]float64, error) {
	impedance := state.ImpedanceOhms
	temp := state.TipTempC
	
	// Если данные невалидны - возвращаем равномерное распределение
	if impedance <= 0 || temp <= 0 {
		return map[hal.TissueType]float64{
			hal.TissueSoft: 0.2,
			hal.TissueVessel: 0.2,
			hal.TissueNerve: 0.2,
			hal.TissueBone: 0.2,
			hal.TissueTumor: 0.2,
		}, nil
	}
	
	// Вычисляем "score" для каждого типа ткани (чем ближе к центру диапазона, тем выше)
	scores := map[hal.TissueType]float64{
		hal.TissueSoft:   c.calculateScore(impedance, temp, c.thresholds.Soft),
		hal.TissueVessel: c.calculateScore(impedance, temp, c.thresholds.Vessel),
		hal.TissueNerve:  c.calculateScore(impedance, temp, c.thresholds.Nerve),
		hal.TissueBone:   c.calculateScore(impedance, temp, c.thresholds.Bone),
		hal.TissueTumor:  c.calculateScore(impedance, temp, c.thresholds.Tumor),
	}
	
	// Нормализуем в вероятности (сумма = 1.0)
	total := 0.0
	for _, score := range scores {
		total += score
	}
	
	probs := make(map[hal.TissueType]float64)
	if total > 0 {
		for tissue, score := range scores {
			probs[tissue] = score / total
		}
	} else {
		// Fallback: равномерное распределение
		for tissue := range scores {
			probs[tissue] = 0.2
		}
	}
	
	return probs, nil
}

// calculateScore вычисляет "близость" к центру диапазона (0.0 - 1.0)
func (c *HeuristicClassifier) calculateScore(impedance, temp float64, r ImpedanceRange) float64 {
	// Импеданс: гауссово распределение вокруг центра
	impCenter := (r.MinOhms + r.MaxOhms) / 2
	impWidth := (r.MaxOhms - r.MinOhms) / 2
	impScore := gaussian(impedance, impCenter, impWidth)
	
	// Температура: гауссово распределение вокруг центра
	tempCenter := (r.TempMinC + r.TempMaxC) / 2
	tempWidth := (r.TempMaxC - r.TempMinC) / 2
	tempScore := gaussian(temp, tempCenter, tempWidth)
	
	// Комбинируем (взвешенная сумма)
	return 0.7*impScore + 0.3*tempScore
}

// gaussian - функция Гаусса
func gaussian(x, mean, sigma float64) float64 {
	if sigma <= 0 {
		sigma = 1
	}
	diff := (x - mean) / sigma
	return exp(-0.5 * diff * diff)
}

// exp - быстрая экспонента (Taylor approximation для малых значений)
func exp(x float64) float64 {
	if x < -10 {
		return 0
	}
	if x > 10 {
		return 22026.46579
	}
	// Taylor series: e^x ≈ 1 + x + x²/2 + x³/6 + x⁴/24
	result := 1.0 + x
	xn := x
	for i := 2; i <= 10; i++ {
		xn *= x / float64(i)
		result += xn
	}
	return result
}

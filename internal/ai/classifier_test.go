package ai

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"rus-uhas/internal/hal"
)

func TestHeuristicClassifier_SoftTissue(t *testing.T) {
	classifier := NewHeuristicClassifier(DefaultThresholds())
	
	state := hal.GeneratorState{
		ImpedanceOhms: 55,  // В диапазоне soft tissue
		TipTempC:      38,
	}
	
	probs, err := classifier.Classify(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Проверяем, что soft tissue имеет наибольшую вероятность
	maxProb := 0.0
	var maxTissue hal.TissueType
	for tissue, prob := range probs {
		if prob > maxProb {
			maxProb = prob
			maxTissue = tissue
		}
	}
	
	if maxTissue != hal.TissueSoft {
		t.Errorf("expected soft tissue, got %v (prob: %.2f)", maxTissue, maxProb)
	}
	
	if maxProb < 0.5 {
		t.Errorf("confidence too low: %.2f", maxProb)
	}
}

func TestHeuristicClassifier_BoneTissue(t *testing.T) {
	classifier := NewHeuristicClassifier(DefaultThresholds())
	
	state := hal.GeneratorState{
		ImpedanceOhms: 280, // Высокий импеданс - кость
		TipTempC:      42,
	}
	
	probs, err := classifier.Classify(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	maxProb := 0.0
	var maxTissue hal.TissueType
	for tissue, prob := range probs {
		if prob > maxProb {
			maxProb = prob
			maxTissue = tissue
		}
	}
	
	if maxTissue != hal.TissueBone {
		t.Errorf("expected bone tissue, got %v", maxTissue)
	}
}

func TestFallbackClassifier_PrimarySuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	// Mock primary, который всегда работает
	primary := &mockClassifier{
		probs: map[hal.TissueType]float64{hal.TissueSoft: 1.0},
	}
	fallback := NewHeuristicClassifier(DefaultThresholds())
	
	classifier := NewFallbackClassifier(primary, fallback, logger)
	
	state := hal.GeneratorState{ImpedanceOhms: 50, TipTempC: 38}
	probs, err := classifier.Classify(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if probs[hal.TissueSoft] != 1.0 {
		t.Errorf("expected primary result, got %v", probs)
	}
	
	primaryCalls, fallbackCalls, _ := classifier.Stats()
	if primaryCalls != 1 || fallbackCalls != 0 {
		t.Errorf("unexpected stats: primary=%d, fallback=%d", primaryCalls, fallbackCalls)
	}
}

func TestFallbackClassifier_FallbackOnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	// Mock primary, который всегда падает
	primary := &mockClassifier{err: context.DeadlineExceeded}
	fallback := NewHeuristicClassifier(DefaultThresholds())
	
	classifier := NewFallbackClassifier(primary, fallback, logger)
	
	state := hal.GeneratorState{ImpedanceOhms: 50, TipTempC: 38}
	probs, err := classifier.Classify(context.Background(), state)
	if err != nil {
		t.Fatalf("fallback should not return error: %v", err)
	}
	
	// Должны получить результат от fallback (эвристика)
	if len(probs) == 0 {
		t.Error("expected non-empty probabilities from fallback")
	}
	
	_, fallbackCalls, errors := classifier.Stats()
	if fallbackCalls != 1 || errors != 1 {
		t.Errorf("unexpected stats: fallback=%d, errors=%d", fallbackCalls, errors)
	}
}

// mockClassifier для тестов
type mockClassifier struct {
	probs map[hal.TissueType]float64
	err   error
}

func (m *mockClassifier) Classify(ctx context.Context, state hal.GeneratorState) (map[hal.TissueType]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.probs, nil
}

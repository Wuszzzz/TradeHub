package quant

import (
	"context"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct {
	indicators []model.IndicatorDefinition
	patterns   []model.PatternDefinition
	values     []map[string]any
	hits       []map[string]any
}

func (r *fakeRepository) ListIndicatorDefinitions(category string, enabledOnly bool) ([]model.IndicatorDefinition, error) {
	return r.indicators, nil
}

func (r *fakeRepository) UpsertIndicatorDefinition(definition model.IndicatorDefinition) error {
	r.indicators = append(r.indicators, definition)
	return nil
}

func (r *fakeRepository) ListPatternDefinitions(category string, enabledOnly bool) ([]model.PatternDefinition, error) {
	return r.patterns, nil
}

func (r *fakeRepository) UpsertPatternDefinition(definition model.PatternDefinition) error {
	r.patterns = append(r.patterns, definition)
	return nil
}

func (r *fakeRepository) QueryIndicatorValues(symbol, period, indicatorCode string, limit int) ([]map[string]any, error) {
	return r.values, nil
}

func (r *fakeRepository) QueryPatternHits(symbol, period, patternCode string, limit int) ([]map[string]any, error) {
	return r.hits, nil
}

func TestCreateIndicatorValidatesRequiredFields(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.CreateIndicator(context.Background(), IndicatorInput{IndicatorCode: "MACD"}); err == nil {
		t.Fatalf("expected required field error")
	}
}

func TestCreateIndicatorDefaultsDefinitionFields(t *testing.T) {
	service := NewService(&fakeRepository{})
	definition, err := service.CreateIndicator(context.Background(), IndicatorInput{
		IndicatorCode: "macd",
		Name:          "MACD",
	})
	if err != nil {
		t.Fatalf("create indicator failed: %v", err)
	}
	if definition.IndicatorCode != "MACD" || definition.Category != "technical" || !definition.Enabled {
		t.Fatalf("unexpected indicator defaults: %+v", definition)
	}
	if definition.ParamsSchema != "{}" || definition.OutputFields != "[]" {
		t.Fatalf("unexpected json defaults: %+v", definition)
	}
}

func TestCreatePatternKeepsExistingPatternCodeCase(t *testing.T) {
	service := NewService(&fakeRepository{})
	definition, err := service.CreatePattern(context.Background(), PatternInput{
		PatternCode:   "doji_Star",
		Name:          "十字星",
		TALibFunction: "cdldojistar",
	})
	if err != nil {
		t.Fatalf("create pattern failed: %v", err)
	}
	if definition.PatternCode != "doji_Star" || definition.TALibFunction != "CDLDOJISTAR" {
		t.Fatalf("unexpected pattern normalization: %+v", definition)
	}
	if definition.Category != "candlestick" || definition.Direction != "both" {
		t.Fatalf("unexpected pattern defaults: %+v", definition)
	}
}

func TestListIndicatorValuesValidatesRequiredFields(t *testing.T) {
	service := NewService(&fakeRepository{}, &fakeRepository{})
	if _, err := service.ListIndicatorValues(context.Background(), "600519", "1d", "", 100); err == nil {
		t.Fatalf("expected indicator_code validation error")
	}
}

func TestListPatternHitsDefaultsPeriodAndLimit(t *testing.T) {
	repo := &fakeRepository{hits: []map[string]any{{"pattern_code": "doji"}}}
	service := NewService(repo, repo)
	items, err := service.ListPatternHits(context.Background(), "600519", "", "", 0)
	if err != nil {
		t.Fatalf("list pattern hits failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one hit, got %d", len(items))
	}
}

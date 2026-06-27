package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stock-etf-monitor/backend/internal/application/quant"
	"stock-etf-monitor/backend/model"
)

type fakeQuantRepository struct {
	indicators []model.IndicatorDefinition
	patterns   []model.PatternDefinition
	values     []map[string]any
	hits       []map[string]any
}

func (r *fakeQuantRepository) ListIndicatorDefinitions(category string, enabledOnly bool) ([]model.IndicatorDefinition, error) {
	return r.indicators, nil
}

func (r *fakeQuantRepository) UpsertIndicatorDefinition(definition model.IndicatorDefinition) error {
	r.indicators = append(r.indicators, definition)
	return nil
}

func (r *fakeQuantRepository) ListPatternDefinitions(category string, enabledOnly bool) ([]model.PatternDefinition, error) {
	return r.patterns, nil
}

func (r *fakeQuantRepository) UpsertPatternDefinition(definition model.PatternDefinition) error {
	r.patterns = append(r.patterns, definition)
	return nil
}

func (r *fakeQuantRepository) QueryIndicatorValues(symbol, period, indicatorCode string, limit int) ([]map[string]any, error) {
	return r.values, nil
}

func (r *fakeQuantRepository) QueryPatternHits(symbol, period, patternCode string, limit int) ([]map[string]any, error) {
	return r.hits, nil
}

func TestQuantIndicatorsPostReturnsUnifiedResponse(t *testing.T) {
	handler := NewQuantHandler(quant.NewService(&fakeQuantRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/quant/indicators", strings.NewReader(`{"indicator_code":"MACD","name":"MACD"}`))
	rec := httptest.NewRecorder()

	handler.Indicators(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestQuantPatternsPostRejectsMissingFunction(t *testing.T) {
	handler := NewQuantHandler(quant.NewService(&fakeQuantRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/quant/patterns", strings.NewReader(`{"pattern_code":"doji","name":"十字"}`))
	rec := httptest.NewRecorder()

	handler.Patterns(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestQuantPatternsGetReturnsUnifiedResponse(t *testing.T) {
	repo := &fakeQuantRepository{patterns: []model.PatternDefinition{{PatternCode: "doji", Name: "十字", Enabled: true}}}
	handler := NewQuantHandler(quant.NewService(repo))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/quant/patterns", nil)
	rec := httptest.NewRecorder()

	handler.Patterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestQuantIndicatorValuesGetReturnsUnifiedResponse(t *testing.T) {
	repo := &fakeQuantRepository{values: []map[string]any{{"indicator_code": "MACD", "value": 1.2}}}
	handler := NewQuantHandler(quant.NewService(repo, repo))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/quant/indicator-values?symbol=600519&indicator_code=MACD", nil)
	rec := httptest.NewRecorder()

	handler.IndicatorValues(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestQuantPatternHitsRejectsEmptySymbol(t *testing.T) {
	repo := &fakeQuantRepository{}
	handler := NewQuantHandler(quant.NewService(repo, repo))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/quant/pattern-hits", nil)
	rec := httptest.NewRecorder()

	handler.PatternHits(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

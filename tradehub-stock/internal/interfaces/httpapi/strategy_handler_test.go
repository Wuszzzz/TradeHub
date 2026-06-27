package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	strategyapp "stock-etf-monitor/backend/internal/application/strategy"
	"stock-etf-monitor/backend/model"
)

type fakeStrategyRepository struct {
	templates []model.StrategyTemplate
}

func (r *fakeStrategyRepository) ListStrategyTemplates(enabledOnly bool) ([]model.StrategyTemplate, error) {
	return r.templates, nil
}

func (r *fakeStrategyRepository) UpsertStrategyTemplate(template model.StrategyTemplate) error {
	r.templates = append(r.templates, template)
	return nil
}

func (r *fakeStrategyRepository) DeleteStrategyTemplate(strategyID string) error {
	return nil
}

func (r *fakeStrategyRepository) ListStrategyRuns(strategyID, taskID, status string, limit int) ([]model.StrategyRun, error) {
	return []model.StrategyRun{{StrategyID: strategyID, TaskID: taskID, Status: status}}, nil
}

func TestStrategyTemplatePostReturnsCreated(t *testing.T) {
	repo := &fakeStrategyRepository{}
	handler := NewStrategyHandler(strategyapp.NewService(repo))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/strategies/templates", bytes.NewReader([]byte(`{"name":"MACD 策略","backtest_params":{"hold_bars":20}}`)))
	rec := httptest.NewRecorder()

	handler.Templates(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Code != "OK" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(repo.templates) != 1 {
		t.Fatalf("expected one saved template, got %d", len(repo.templates))
	}
}

func TestStrategyTemplateDeleteRequiresStrategyID(t *testing.T) {
	handler := NewStrategyHandler(strategyapp.NewService(&fakeStrategyRepository{}))
	req := httptest.NewRequest(http.MethodDelete, "/api/stock/v1/strategies/templates", nil)
	rec := httptest.NewRecorder()

	handler.Templates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStrategyRunsReturnsUnifiedResponse(t *testing.T) {
	handler := NewStrategyHandler(strategyapp.NewService(&fakeStrategyRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/strategies/runs?strategy_id=strategy_1&status=succeeded", nil)
	rec := httptest.NewRecorder()

	handler.Runs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Code != "OK" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

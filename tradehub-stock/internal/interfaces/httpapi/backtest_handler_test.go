package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	backtestapp "stock-etf-monitor/backend/internal/application/backtest"
	"stock-etf-monitor/backend/model"
)

type fakeBacktestRepository struct{}

func (r *fakeBacktestRepository) ListBacktestResults(taskID, symbol string, limit int) ([]model.BacktestResult, error) {
	return []model.BacktestResult{{TaskID: taskID, Symbol: symbol}}, nil
}

func (r *fakeBacktestRepository) ListBacktestSummaries(taskID string, limit int) ([]model.BacktestSummary, error) {
	return []model.BacktestSummary{{TaskID: taskID, Metrics: model.BacktestMetrics{TotalTrades: 3}}}, nil
}

func TestBacktestResultsReturnsUnifiedResponse(t *testing.T) {
	handler := NewBacktestHandler(backtestapp.NewService(&fakeBacktestRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/backtest/results?task_id=task_1&symbol=600519", nil)
	rec := httptest.NewRecorder()

	handler.Results(rec, req)

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

func TestBacktestResultsRejectsBadLimit(t *testing.T) {
	handler := NewBacktestHandler(backtestapp.NewService(&fakeBacktestRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/backtest/results?limit=bad", nil)
	rec := httptest.NewRecorder()

	handler.Results(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestBacktestSummariesReturnsUnifiedResponse(t *testing.T) {
	handler := NewBacktestHandler(backtestapp.NewService(&fakeBacktestRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/backtest/summaries?task_id=task_1", nil)
	rec := httptest.NewRecorder()

	handler.Summaries(rec, req)

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

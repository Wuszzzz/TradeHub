package backtest

import (
	"context"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct{}

func (r *fakeRepository) ListBacktestResults(taskID, symbol string, limit int) ([]model.BacktestResult, error) {
	return []model.BacktestResult{{TaskID: taskID, Symbol: symbol}}, nil
}

func (r *fakeRepository) ListBacktestSummaries(taskID string, limit int) ([]model.BacktestSummary, error) {
	return []model.BacktestSummary{{TaskID: taskID, Metrics: model.BacktestMetrics{TotalTrades: 3}}}, nil
}

func TestListResultsNormalizesLimit(t *testing.T) {
	service := NewService(&fakeRepository{})
	items, err := service.ListResults(context.Background(), " task_1 ", " 600519 ", 0)
	if err != nil {
		t.Fatalf("list results failed: %v", err)
	}
	if len(items) != 1 || items[0].TaskID != "task_1" || items[0].Symbol != "600519" {
		t.Fatalf("unexpected results: %+v", items)
	}
}

func TestListSummariesNormalizesLimit(t *testing.T) {
	service := NewService(&fakeRepository{})
	items, err := service.ListSummaries(context.Background(), " task_1 ", 0)
	if err != nil {
		t.Fatalf("list summaries failed: %v", err)
	}
	if len(items) != 1 || items[0].TaskID != "task_1" || items[0].Metrics.TotalTrades != 3 {
		t.Fatalf("unexpected summaries: %+v", items)
	}
}

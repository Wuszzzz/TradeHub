package strategy

import (
	"context"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct {
	templates []model.StrategyTemplate
}

func (r *fakeRepository) ListStrategyTemplates(enabledOnly bool) ([]model.StrategyTemplate, error) {
	return r.templates, nil
}

func (r *fakeRepository) UpsertStrategyTemplate(template model.StrategyTemplate) error {
	r.templates = append(r.templates, template)
	return nil
}

func (r *fakeRepository) DeleteStrategyTemplate(strategyID string) error {
	return nil
}

func (r *fakeRepository) ListStrategyRuns(strategyID, taskID, status string, limit int) ([]model.StrategyRun, error) {
	return []model.StrategyRun{{StrategyID: strategyID, TaskID: taskID, Status: status}}, nil
}

func TestSaveTemplateDefaultsIDAndJSON(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	template, err := service.SaveTemplate(context.Background(), TemplateInput{
		Name:           "MACD 吞噬回测",
		BacktestParams: map[string]any{"hold_bars": 20},
	})
	if err != nil {
		t.Fatalf("save template failed: %v", err)
	}
	if template.StrategyID == "" || !template.Enabled || template.CreatedBy != "system" {
		t.Fatalf("unexpected defaults: %+v", template)
	}
	if template.ConditionsJSON == "" || template.BacktestParamsJSON == "" || template.RiskParamsJSON == "" {
		t.Fatalf("expected json payloads: %+v", template)
	}
}

func TestDeleteTemplateRequiresStrategyID(t *testing.T) {
	service := NewService(&fakeRepository{})
	if err := service.DeleteTemplate(context.Background(), " "); err == nil {
		t.Fatalf("expected strategy_id validation error")
	}
}

func TestListRunsNormalizesLimit(t *testing.T) {
	service := NewService(&fakeRepository{})
	items, err := service.ListRuns(context.Background(), " strategy_1 ", " task_1 ", " succeeded ", 0)
	if err != nil {
		t.Fatalf("list runs failed: %v", err)
	}
	if len(items) != 1 || items[0].StrategyID != "strategy_1" || items[0].TaskID != "task_1" || items[0].Status != "succeeded" {
		t.Fatalf("unexpected runs: %+v", items)
	}
}

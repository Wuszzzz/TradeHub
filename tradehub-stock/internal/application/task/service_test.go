package task

import (
	"context"
	"strings"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeIngestionRepository struct {
	tasks      []model.IngestionTask
	stockTasks []model.StockTask
	logs       []model.StockTaskLog
	strategy   *model.StrategyTemplate
	runs       []model.StrategyRun
}

func (r *fakeIngestionRepository) ListTasks() ([]model.IngestionTask, error) {
	return r.tasks, nil
}

func (r *fakeIngestionRepository) UpsertTask(task model.IngestionTask) error {
	r.tasks = append(r.tasks, task)
	return nil
}

func (r *fakeIngestionRepository) DeleteTask(taskID string) error {
	return nil
}

func (r *fakeIngestionRepository) UpsertStockTask(task model.StockTask) error {
	r.stockTasks = append(r.stockTasks, task)
	return nil
}

func (r *fakeIngestionRepository) ListStockTasks(taskType, status string, limit int) ([]model.StockTask, error) {
	return r.stockTasks, nil
}

func (r *fakeIngestionRepository) GetStockTask(taskID string) (*model.StockTask, error) {
	for _, task := range r.stockTasks {
		if task.TaskID == taskID {
			return &task, nil
		}
	}
	return nil, nil
}

func (r *fakeIngestionRepository) UpdateStockTaskStatus(taskID, status, resultRef, lastError string, progress int) error {
	return nil
}

func (r *fakeIngestionRepository) AppendStockTaskLog(log model.StockTaskLog) error {
	r.logs = append(r.logs, log)
	return nil
}

func (r *fakeIngestionRepository) ListStockTaskLogs(taskID string, limit int) ([]model.StockTaskLog, error) {
	return r.logs, nil
}

func (r *fakeIngestionRepository) GetStrategyTemplate(strategyID string) (*model.StrategyTemplate, error) {
	return r.strategy, nil
}

func (r *fakeIngestionRepository) UpsertStrategyRun(run model.StrategyRun) error {
	r.runs = append(r.runs, run)
	return nil
}

func TestCreateIngestionValidatesRequiredFields(t *testing.T) {
	service := NewService(&fakeIngestionRepository{})
	if _, err := service.CreateIngestion(context.Background(), IngestionInput{Symbol: "600519"}); err == nil {
		t.Fatalf("expected interval validation error")
	}
}

func TestCreateIngestionDefaultsMarketFieldsAndStatus(t *testing.T) {
	repo := &fakeIngestionRepository{}
	service := NewService(repo)
	task, err := service.CreateIngestion(context.Background(), IngestionInput{Symbol: "600519", Interval: "1m"})
	if err != nil {
		t.Fatalf("create ingestion failed: %v", err)
	}
	if task.Market != "CN-A" || task.Status != "pending" || task.Source != "akshare" {
		t.Fatalf("unexpected task defaults: %+v", task)
	}
	if len(task.Fields) == 0 {
		t.Fatalf("expected default fields")
	}
}

func TestDeleteIngestionValidatesTaskID(t *testing.T) {
	service := NewService(&fakeIngestionRepository{})
	if err := service.DeleteIngestion(context.Background(), " "); err == nil {
		t.Fatalf("expected task_id validation error")
	}
}

func TestCreateStockTaskValidatesType(t *testing.T) {
	service := NewService(&fakeIngestionRepository{})
	if _, err := service.CreateStockTask(context.Background(), StockTaskInput{TaskType: "bad"}); err == nil {
		t.Fatalf("expected task_type validation error")
	}
}

func TestCreateStockTaskDefaultsStatusAndCreatedBy(t *testing.T) {
	repo := &fakeIngestionRepository{}
	service := NewService(repo)
	task, err := service.CreateStockTask(context.Background(), StockTaskInput{TaskType: "pattern_scan"})
	if err != nil {
		t.Fatalf("create stock task failed: %v", err)
	}
	if task.Status != "pending" || task.CreatedBy != "system" {
		t.Fatalf("unexpected task defaults: %+v", task)
	}
	if len(repo.logs) == 0 {
		t.Fatalf("expected creation log")
	}
}

func TestCreateStockTaskAttachesStrategySnapshot(t *testing.T) {
	repo := &fakeIngestionRepository{strategy: &model.StrategyTemplate{
		StrategyID:          "strategy_1",
		Name:                "模板策略",
		ScreeningTemplateID: "screen_1",
		ConditionsJSON:      `{"logic":"and"}`,
		BacktestParamsJSON:  `{"hold_bars":20}`,
		RiskParamsJSON:      `{"stop_loss":0.08}`,
	}}
	service := NewService(repo)
	task, err := service.CreateStockTask(context.Background(), StockTaskInput{
		TaskType: "backtest",
		Params:   map[string]any{"strategy_id": "strategy_1"},
	})
	if err != nil {
		t.Fatalf("create stock task failed: %v", err)
	}
	if task.Params == "" {
		t.Fatalf("expected params payload")
	}
	if !strings.Contains(task.Params, "strategy_snapshot") {
		t.Fatalf("expected strategy_snapshot in params: %s", task.Params)
	}
	if len(repo.runs) != 1 || repo.runs[0].StrategyID != "strategy_1" {
		t.Fatalf("expected strategy run created, got %+v", repo.runs)
	}
}

func TestUpdateStatusValidatesStatus(t *testing.T) {
	service := NewService(&fakeIngestionRepository{})
	if err := service.UpdateStatus(context.Background(), "task_1", StatusInput{Status: "bad"}); err == nil {
		t.Fatalf("expected status validation error")
	}
}

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	taskapp "stock-etf-monitor/backend/internal/application/task"
	"stock-etf-monitor/backend/model"
)

type fakeTaskRepository struct {
	tasks      []model.IngestionTask
	stockTasks []model.StockTask
	logs       []model.StockTaskLog
	strategy   *model.StrategyTemplate
	runs       []model.StrategyRun
}

func (r *fakeTaskRepository) ListTasks() ([]model.IngestionTask, error) {
	return r.tasks, nil
}

func (r *fakeTaskRepository) UpsertTask(task model.IngestionTask) error {
	r.tasks = append(r.tasks, task)
	return nil
}

func (r *fakeTaskRepository) DeleteTask(taskID string) error {
	return nil
}

func (r *fakeTaskRepository) UpsertStockTask(task model.StockTask) error {
	r.stockTasks = append(r.stockTasks, task)
	return nil
}

func (r *fakeTaskRepository) ListStockTasks(taskType, status string, limit int) ([]model.StockTask, error) {
	return r.stockTasks, nil
}

func (r *fakeTaskRepository) GetStockTask(taskID string) (*model.StockTask, error) {
	for _, task := range r.stockTasks {
		if task.TaskID == taskID {
			return &task, nil
		}
	}
	return nil, nil
}

func (r *fakeTaskRepository) UpdateStockTaskStatus(taskID, status, resultRef, lastError string, progress int) error {
	return nil
}

func (r *fakeTaskRepository) AppendStockTaskLog(log model.StockTaskLog) error {
	r.logs = append(r.logs, log)
	return nil
}

func (r *fakeTaskRepository) ListStockTaskLogs(taskID string, limit int) ([]model.StockTaskLog, error) {
	return r.logs, nil
}

func (r *fakeTaskRepository) GetStrategyTemplate(strategyID string) (*model.StrategyTemplate, error) {
	return r.strategy, nil
}

func (r *fakeTaskRepository) UpsertStrategyRun(run model.StrategyRun) error {
	r.runs = append(r.runs, run)
	return nil
}

func TestTaskIngestionPostReturnsUnifiedResponse(t *testing.T) {
	handler := NewTaskHandler(taskapp.NewService(&fakeTaskRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/tasks/ingestion", strings.NewReader(`{"symbol":"600519","interval":"1m"}`))
	rec := httptest.NewRecorder()

	handler.Ingestion(rec, req)

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

func TestTaskIngestionDeleteRejectsMissingTaskID(t *testing.T) {
	handler := NewTaskHandler(taskapp.NewService(&fakeTaskRepository{}))
	req := httptest.NewRequest(http.MethodDelete, "/api/stock/v1/tasks/ingestion", nil)
	rec := httptest.NewRecorder()

	handler.Ingestion(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTasksPostReturnsUnifiedResponse(t *testing.T) {
	handler := NewTaskHandler(taskapp.NewService(&fakeTaskRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/tasks", strings.NewReader(`{"task_type":"indicator_compute","params":{"symbol":"600519"}}`))
	rec := httptest.NewRecorder()

	handler.Tasks(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestTaskStatusRejectsInvalidStatus(t *testing.T) {
	handler := NewTaskHandler(taskapp.NewService(&fakeTaskRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/tasks/status?task_id=task_1", strings.NewReader(`{"status":"bad"}`))
	rec := httptest.NewRecorder()

	handler.Status(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

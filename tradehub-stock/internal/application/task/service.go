package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type IngestionRepository interface {
	ListTasks() ([]model.IngestionTask, error)
	UpsertTask(model.IngestionTask) error
	DeleteTask(taskID string) error
	UpsertStockTask(model.StockTask) error
	ListStockTasks(taskType, status string, limit int) ([]model.StockTask, error)
	GetStockTask(taskID string) (*model.StockTask, error)
	UpdateStockTaskStatus(taskID, status, resultRef, lastError string, progress int) error
	AppendStockTaskLog(model.StockTaskLog) error
	ListStockTaskLogs(taskID string, limit int) ([]model.StockTaskLog, error)
	GetStrategyTemplate(strategyID string) (*model.StrategyTemplate, error)
	UpsertStrategyRun(model.StrategyRun) error
}

type Service struct {
	repo IngestionRepository
}

type IngestionInput struct {
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name"`
	Market   string   `json:"market"`
	Interval string   `json:"interval"`
	Fields   []string `json:"fields"`
}

type StockTaskInput struct {
	TaskType  string         `json:"task_type"`
	Params    map[string]any `json:"params"`
	CreatedBy string         `json:"created_by"`
}

type StatusInput struct {
	Status    string `json:"status"`
	ResultRef string `json:"result_ref"`
	LastError string `json:"last_error"`
	Progress  int    `json:"progress"`
}

type LogInput struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Context map[string]any `json:"context"`
}

var validTaskTypes = map[string]bool{
	"indicator_compute": true,
	"pattern_scan":      true,
	"screening":         true,
	"backtest":          true,
	"ai_report":         true,
	"data_ingest":       true,
}

var validStatuses = map[string]bool{
	"pending":   true,
	"running":   true,
	"succeeded": true,
	"failed":    true,
	"retrying":  true,
	"cancelled": true,
}

func NewService(repo IngestionRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListIngestion(_ context.Context) ([]model.IngestionTask, error) {
	return s.repo.ListTasks()
}

func (s *Service) CreateIngestion(_ context.Context, input IngestionInput) (model.IngestionTask, error) {
	symbol := strings.TrimSpace(input.Symbol)
	interval := strings.TrimSpace(input.Interval)
	if symbol == "" || interval == "" {
		return model.IngestionTask{}, fmt.Errorf("symbol and interval are required")
	}
	now := time.Now().UTC()
	name := strings.TrimSpace(input.Name)
	market := strings.TrimSpace(input.Market)
	if market == "" {
		market = "CN-A"
	}
	fields := normalizeFields(input.Fields)
	task := model.IngestionTask{
		TaskID:      fmt.Sprintf("task_%d", now.UnixNano()),
		Symbol:      symbol,
		Name:        name,
		Market:      market,
		Interval:    interval,
		Fields:      fields,
		Enabled:     true,
		Status:      "pending",
		Source:      "akshare",
		CreatedAt:   now,
		UpdatedAt:   now,
		LastMessage: "created",
	}
	return task, s.repo.UpsertTask(task)
}

func (s *Service) DeleteIngestion(_ context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	return s.repo.DeleteTask(taskID)
}

func (s *Service) CreateStockTask(_ context.Context, input StockTaskInput) (model.StockTask, error) {
	taskType := strings.TrimSpace(input.TaskType)
	if !validTaskTypes[taskType] {
		return model.StockTask{}, fmt.Errorf("task_type is invalid")
	}
	params := input.Params
	if params == nil {
		params = map[string]any{}
	}
	if err := s.attachStrategySnapshot(params); err != nil {
		return model.StockTask{}, err
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		return model.StockTask{}, err
	}
	now := time.Now().UTC()
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "system"
	}
	task := model.StockTask{
		TaskID:    fmt.Sprintf("stock_task_%d", now.UnixNano()),
		TaskType:  taskType,
		Status:    "pending",
		Params:    string(rawParams),
		Progress:  0,
		Attempts:  0,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.UpsertStockTask(task); err != nil {
		return model.StockTask{}, err
	}
	if run, ok := strategyRunFromParams(task); ok {
		if err := s.repo.UpsertStrategyRun(run); err != nil {
			return model.StockTask{}, err
		}
	}
	_ = s.AppendLog(context.Background(), task.TaskID, LogInput{
		Level:   "info",
		Message: "task created",
		Context: map[string]any{"task_type": taskType},
	})
	return task, nil
}

func strategyRunFromParams(task model.StockTask) (model.StrategyRun, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(task.Params), &payload); err != nil {
		return model.StrategyRun{}, false
	}
	rawSnapshot, ok := payload["strategy_snapshot"]
	if !ok {
		return model.StrategyRun{}, false
	}
	raw, err := json.Marshal(rawSnapshot)
	if err != nil {
		return model.StrategyRun{}, false
	}
	var snapshot model.StrategySnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return model.StrategyRun{}, false
	}
	if snapshot.StrategyID == "" {
		return model.StrategyRun{}, false
	}
	return model.StrategyRun{
		RunID:        fmt.Sprintf("strategy_run_%s", task.TaskID),
		TaskID:       task.TaskID,
		TaskType:     task.TaskType,
		StrategyID:   snapshot.StrategyID,
		SnapshotID:   snapshot.SnapshotID,
		StrategyName: snapshot.Name,
		Status:       task.Status,
		CreatedBy:    task.CreatedBy,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}, true
}

func (s *Service) attachStrategySnapshot(params map[string]any) error {
	rawStrategyID, ok := params["strategy_id"]
	if !ok {
		return nil
	}
	strategyID := strings.TrimSpace(fmt.Sprintf("%v", rawStrategyID))
	if strategyID == "" || strategyID == "<nil>" {
		return nil
	}
	template, err := s.repo.GetStrategyTemplate(strategyID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	params["strategy_snapshot"] = model.StrategySnapshot{
		SnapshotID:          fmt.Sprintf("strategy_snapshot_%d", now.UnixNano()),
		StrategyID:          template.StrategyID,
		Name:                template.Name,
		Description:         template.Description,
		ScreeningTemplateID: template.ScreeningTemplateID,
		ConditionsJSON:      template.ConditionsJSON,
		BacktestParamsJSON:  template.BacktestParamsJSON,
		RiskParamsJSON:      template.RiskParamsJSON,
		SnapshotAt:          now,
	}
	return nil
}

func (s *Service) ListStockTasks(_ context.Context, taskType, status string, limit int) ([]model.StockTask, error) {
	return s.repo.ListStockTasks(strings.TrimSpace(taskType), strings.TrimSpace(status), limit)
}

func (s *Service) GetStockTask(_ context.Context, taskID string) (*model.StockTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	return s.repo.GetStockTask(taskID)
}

func (s *Service) UpdateStatus(_ context.Context, taskID string, input StatusInput) error {
	taskID = strings.TrimSpace(taskID)
	status := strings.TrimSpace(input.Status)
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if !validStatuses[status] {
		return fmt.Errorf("status is invalid")
	}
	if err := s.repo.UpdateStockTaskStatus(taskID, status, input.ResultRef, input.LastError, input.Progress); err != nil {
		return err
	}
	level := "info"
	if status == "failed" {
		level = "error"
	}
	return s.AppendLog(context.Background(), taskID, LogInput{
		Level:   level,
		Message: "status changed to " + status,
		Context: map[string]any{"progress": input.Progress, "result_ref": input.ResultRef},
	})
}

func (s *Service) AppendLog(_ context.Context, taskID string, input LogInput) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	level := strings.TrimSpace(input.Level)
	if level == "" {
		level = "info"
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return fmt.Errorf("message is required")
	}
	context := input.Context
	if context == nil {
		context = map[string]any{}
	}
	rawContext, err := json.Marshal(context)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.repo.AppendStockTaskLog(model.StockTaskLog{
		LogID:     fmt.Sprintf("task_log_%d", now.UnixNano()),
		TaskID:    taskID,
		Level:     level,
		Message:   message,
		Context:   string(rawContext),
		CreatedAt: now,
	})
}

func (s *Service) ListLogs(_ context.Context, taskID string, limit int) ([]model.StockTaskLog, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	return s.repo.ListStockTaskLogs(taskID, limit)
}

func normalizeFields(fields []string) []string {
	defaults := []string{
		"price",
		"volume",
		"amount",
		"turnover_rate",
		"turnover_amount",
		"volume_ratio",
		"premium_ratio",
		"buy_sell_5",
		"order_flow",
	}
	if len(fields) == 0 {
		return defaults
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	if len(out) == 0 {
		return defaults
	}
	return out
}

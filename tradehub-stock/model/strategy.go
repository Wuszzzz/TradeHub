package model

import "time"

type StrategyTemplate struct {
	StrategyID          string    `json:"strategy_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ScreeningTemplateID string    `json:"screening_template_id"`
	ConditionsJSON      string    `json:"conditions_json"`
	BacktestParamsJSON  string    `json:"backtest_params_json"`
	RiskParamsJSON      string    `json:"risk_params_json"`
	Enabled             bool      `json:"enabled"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type StrategySnapshot struct {
	SnapshotID          string    `json:"snapshot_id"`
	StrategyID          string    `json:"strategy_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ScreeningTemplateID string    `json:"screening_template_id"`
	ConditionsJSON      string    `json:"conditions_json"`
	BacktestParamsJSON  string    `json:"backtest_params_json"`
	RiskParamsJSON      string    `json:"risk_params_json"`
	SnapshotAt          time.Time `json:"snapshot_at"`
}

type StrategyRun struct {
	RunID        string    `json:"run_id"`
	TaskID       string    `json:"task_id"`
	TaskType     string    `json:"task_type"`
	StrategyID   string    `json:"strategy_id"`
	SnapshotID   string    `json:"snapshot_id"`
	StrategyName string    `json:"strategy_name"`
	Status       string    `json:"status"`
	ResultRef    string    `json:"result_ref"`
	SummaryRef   string    `json:"summary_ref"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

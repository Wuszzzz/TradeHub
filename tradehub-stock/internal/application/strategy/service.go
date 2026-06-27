package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type Repository interface {
	ListStrategyTemplates(enabledOnly bool) ([]model.StrategyTemplate, error)
	UpsertStrategyTemplate(model.StrategyTemplate) error
	DeleteStrategyTemplate(strategyID string) error
	ListStrategyRuns(strategyID, taskID, status string, limit int) ([]model.StrategyRun, error)
}

type Service struct {
	repo Repository
}

type TemplateInput struct {
	StrategyID          string         `json:"strategy_id"`
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	ScreeningTemplateID string         `json:"screening_template_id"`
	Conditions          map[string]any `json:"conditions"`
	BacktestParams      map[string]any `json:"backtest_params"`
	RiskParams          map[string]any `json:"risk_params"`
	Enabled             *bool          `json:"enabled"`
	CreatedBy           string         `json:"created_by"`
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListTemplates(_ context.Context, enabledOnly bool) ([]model.StrategyTemplate, error) {
	return s.repo.ListStrategyTemplates(enabledOnly)
}

func (s *Service) SaveTemplate(_ context.Context, input TemplateInput) (model.StrategyTemplate, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.StrategyTemplate{}, fmt.Errorf("name is required")
	}
	conditions := input.Conditions
	if conditions == nil {
		conditions = map[string]any{}
	}
	backtestParams := input.BacktestParams
	if backtestParams == nil {
		backtestParams = map[string]any{}
	}
	riskParams := input.RiskParams
	if riskParams == nil {
		riskParams = map[string]any{}
	}
	rawConditions, err := json.Marshal(conditions)
	if err != nil {
		return model.StrategyTemplate{}, err
	}
	rawBacktestParams, err := json.Marshal(backtestParams)
	if err != nil {
		return model.StrategyTemplate{}, err
	}
	rawRiskParams, err := json.Marshal(riskParams)
	if err != nil {
		return model.StrategyTemplate{}, err
	}
	now := time.Now().UTC()
	strategyID := strings.TrimSpace(input.StrategyID)
	if strategyID == "" {
		strategyID = fmt.Sprintf("strategy_tpl_%d", now.UnixNano())
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "system"
	}
	template := model.StrategyTemplate{
		StrategyID:          strategyID,
		Name:                name,
		Description:         strings.TrimSpace(input.Description),
		ScreeningTemplateID: strings.TrimSpace(input.ScreeningTemplateID),
		ConditionsJSON:      string(rawConditions),
		BacktestParamsJSON:  string(rawBacktestParams),
		RiskParamsJSON:      string(rawRiskParams),
		Enabled:             enabled,
		CreatedBy:           createdBy,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return template, s.repo.UpsertStrategyTemplate(template)
}

func (s *Service) DeleteTemplate(_ context.Context, strategyID string) error {
	strategyID = strings.TrimSpace(strategyID)
	if strategyID == "" {
		return fmt.Errorf("strategy_id is required")
	}
	return s.repo.DeleteStrategyTemplate(strategyID)
}

func (s *Service) ListRuns(_ context.Context, strategyID, taskID, status string, limit int) ([]model.StrategyRun, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.ListStrategyRuns(strings.TrimSpace(strategyID), strings.TrimSpace(taskID), strings.TrimSpace(status), limit)
}

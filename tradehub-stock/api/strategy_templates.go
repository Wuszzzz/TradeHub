package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"stock-etf-monitor/backend/model"
)

func (r *PostgresRepository) ListStrategyTemplates(enabledOnly bool) ([]model.StrategyTemplate, error) {
	sql := `
	select row_to_json(t)
	from (
	  select strategy_id, name, description, screening_template_id,
	         conditions::text as conditions_json,
	         backtest_params::text as backtest_params_json,
	         risk_params::text as risk_params_json,
	         enabled, created_by, created_at, updated_at
	  from stock_strategy_templates
	  where (not $1 or enabled)
	  order by updated_at desc
	) t;`
	lines, err := r.queryLines(sql, enabledOnly)
	if err != nil {
		return nil, err
	}
	templates := make([]model.StrategyTemplate, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var template model.StrategyTemplate
		if err := json.Unmarshal([]byte(line), &template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (r *PostgresRepository) UpsertStrategyTemplate(template model.StrategyTemplate) error {
	return r.exec(`
	insert into stock_strategy_templates
	  (strategy_id, name, description, screening_template_id, conditions, backtest_params, risk_params, enabled, created_by, created_at, updated_at)
	values
	  ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9, $10, $11)
	on conflict (strategy_id) do update set
	  name = excluded.name,
	  description = excluded.description,
	  screening_template_id = excluded.screening_template_id,
	  conditions = excluded.conditions,
	  backtest_params = excluded.backtest_params,
	  risk_params = excluded.risk_params,
	  enabled = excluded.enabled,
	  updated_at = excluded.updated_at;`,
		template.StrategyID,
		template.Name,
		template.Description,
		template.ScreeningTemplateID,
		defaultJSONText(template.ConditionsJSON, "{}"),
		defaultJSONText(template.BacktestParamsJSON, "{}"),
		defaultJSONText(template.RiskParamsJSON, "{}"),
		template.Enabled,
		template.CreatedBy,
		template.CreatedAt,
		template.UpdatedAt,
	)
}

func (r *PostgresRepository) DeleteStrategyTemplate(strategyID string) error {
	return r.exec(`delete from stock_strategy_templates where strategy_id = $1;`, strategyID)
}

func (r *PostgresRepository) GetStrategyTemplate(strategyID string) (*model.StrategyTemplate, error) {
	sql := `
	select row_to_json(t)
	from (
	  select strategy_id, name, description, screening_template_id,
	         conditions::text as conditions_json,
	         backtest_params::text as backtest_params_json,
	         risk_params::text as risk_params_json,
	         enabled, created_by, created_at, updated_at
	  from stock_strategy_templates
	  where strategy_id = $1 and enabled = true
	  limit 1
	) t;`
	lines, err := r.queryLines(sql, strategyID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("strategy template not found or disabled: %s", strategyID)
	}
	var template model.StrategyTemplate
	if err := json.Unmarshal([]byte(lines[0]), &template); err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *PostgresRepository) UpsertStrategyRun(run model.StrategyRun) error {
	return r.exec(`
	insert into stock_strategy_runs
	  (run_id, task_id, task_type, strategy_id, snapshot_id, strategy_name, status, result_ref, summary_ref, created_by, created_at, updated_at)
	values
	  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	on conflict (run_id) do update set
	  task_type = excluded.task_type,
	  strategy_id = excluded.strategy_id,
	  snapshot_id = excluded.snapshot_id,
	  strategy_name = excluded.strategy_name,
	  status = excluded.status,
	  result_ref = excluded.result_ref,
	  summary_ref = excluded.summary_ref,
	  updated_at = excluded.updated_at;`,
		run.RunID,
		run.TaskID,
		run.TaskType,
		run.StrategyID,
		run.SnapshotID,
		run.StrategyName,
		run.Status,
		run.ResultRef,
		run.SummaryRef,
		run.CreatedBy,
		run.CreatedAt,
		run.UpdatedAt,
	)
}

func (r *PostgresRepository) ListStrategyRuns(strategyID, taskID, status string, limit int) ([]model.StrategyRun, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t)
	from (
	  select run_id, task_id, task_type, strategy_id, snapshot_id, strategy_name, status,
	         result_ref, summary_ref, created_by, created_at, updated_at
	  from stock_strategy_runs
	  where ($1 = '' or strategy_id = $1)
	    and ($2 = '' or task_id = $2)
	    and ($3 = '' or status = $3)
	  order by created_at desc
	  limit $4
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(strategyID), strings.TrimSpace(taskID), strings.TrimSpace(status), limit)
	if err != nil {
		return nil, err
	}
	runs := make([]model.StrategyRun, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var run model.StrategyRun
		if err := json.Unmarshal([]byte(line), &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func defaultJSONText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

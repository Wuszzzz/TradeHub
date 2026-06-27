package main

import (
	"encoding/json"
	"strings"

	"stock-etf-monitor/backend/model"
)

func (r *PostgresRepository) ListScreeningTemplates(enabledOnly bool) ([]model.ScreeningTemplate, error) {
	sql := `
	select row_to_json(t)
	from (
	  select template_id, name, description, conditions::text as conditions_json,
	         enabled, created_by, created_at, updated_at
	  from stock_screening_templates
	  where (not $1 or enabled)
	  order by updated_at desc
	) t;`
	lines, err := r.queryLines(sql, enabledOnly)
	if err != nil {
		return nil, err
	}
	templates := make([]model.ScreeningTemplate, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var template model.ScreeningTemplate
		if err := json.Unmarshal([]byte(line), &template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (r *PostgresRepository) UpsertScreeningTemplate(template model.ScreeningTemplate) error {
	conditions := strings.TrimSpace(template.ConditionsJSON)
	if conditions == "" {
		conditions = "{}"
	}
	return r.exec(`
	insert into stock_screening_templates
	  (template_id, name, description, conditions, enabled, created_by, created_at, updated_at)
	values
	  ($1, $2, $3, $4::jsonb, $5, $6, $7, $8)
	on conflict (template_id) do update set
	  name = excluded.name,
	  description = excluded.description,
	  conditions = excluded.conditions,
	  enabled = excluded.enabled,
	  updated_at = excluded.updated_at;`,
		template.TemplateID,
		template.Name,
		template.Description,
		conditions,
		template.Enabled,
		template.CreatedBy,
		template.CreatedAt,
		template.UpdatedAt,
	)
}

func (r *PostgresRepository) DeleteScreeningTemplate(templateID string) error {
	return r.exec(`delete from stock_screening_templates where template_id = $1;`, templateID)
}

func (r *PostgresRepository) ListScreeningResults(taskID, templateID string, limit int) ([]model.ScreeningResult, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t)
	from (
	  select result_id, task_id, template_id, symbol, score,
	         matched_conditions::text as matched_conditions_json,
	         snapshot::text as snapshot_json,
	         created_at
	  from stock_screening_results
	  where ($1 = '' or task_id = $1)
	    and ($2 = '' or template_id = $2)
	  order by score desc, created_at desc
	  limit $3
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(taskID), strings.TrimSpace(templateID), limit)
	if err != nil {
		return nil, err
	}
	results := make([]model.ScreeningResult, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var result model.ScreeningResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

package main

import (
	"encoding/json"
	"strings"

	"stock-etf-monitor/backend/model"
)

func (r *PostgresRepository) UpsertIndicatorDefinition(definition model.IndicatorDefinition) error {
	paramsSchema := strings.TrimSpace(definition.ParamsSchema)
	if paramsSchema == "" {
		paramsSchema = "{}"
	}
	outputFields := strings.TrimSpace(definition.OutputFields)
	if outputFields == "" {
		outputFields = "[]"
	}
	return r.exec(`
	insert into stock_indicator_definitions
	  (indicator_code, name, category, description, params_schema, output_fields, enabled, created_at, updated_at)
	values
	  ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9)
	on conflict (indicator_code) do update set
	  name = excluded.name,
	  category = excluded.category,
	  description = excluded.description,
	  params_schema = excluded.params_schema,
	  output_fields = excluded.output_fields,
	  enabled = excluded.enabled,
	  updated_at = excluded.updated_at;`,
		definition.IndicatorCode,
		definition.Name,
		definition.Category,
		definition.Description,
		paramsSchema,
		outputFields,
		definition.Enabled,
		definition.CreatedAt,
		definition.UpdatedAt,
	)
}

func (r *PostgresRepository) ListIndicatorDefinitions(category string, enabledOnly bool) ([]model.IndicatorDefinition, error) {
	sql := `
	select row_to_json(t)
	from (
	  select indicator_code, name, category, description,
	         params_schema::text as params_schema,
	         output_fields::text as output_fields,
	         enabled, created_at, updated_at
	  from stock_indicator_definitions
	  where ($1 = '' or category = $1)
	    and (not $2 or enabled)
	  order by category asc, indicator_code asc
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(category), enabledOnly)
	if err != nil {
		return nil, err
	}
	definitions := make([]model.IndicatorDefinition, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var definition model.IndicatorDefinition
		if err := json.Unmarshal([]byte(line), &definition); err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func (r *PostgresRepository) UpsertPatternDefinition(definition model.PatternDefinition) error {
	paramsSchema := strings.TrimSpace(definition.ParamsSchema)
	if paramsSchema == "" {
		paramsSchema = "{}"
	}
	return r.exec(`
	insert into stock_pattern_definitions
	  (pattern_code, name, category, talib_function, direction, description, params_schema, enabled, created_at, updated_at)
	values
	  ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10)
	on conflict (pattern_code) do update set
	  name = excluded.name,
	  category = excluded.category,
	  talib_function = excluded.talib_function,
	  direction = excluded.direction,
	  description = excluded.description,
	  params_schema = excluded.params_schema,
	  enabled = excluded.enabled,
	  updated_at = excluded.updated_at;`,
		definition.PatternCode,
		definition.Name,
		definition.Category,
		definition.TALibFunction,
		definition.Direction,
		definition.Description,
		paramsSchema,
		definition.Enabled,
		definition.CreatedAt,
		definition.UpdatedAt,
	)
}

func (r *PostgresRepository) ListPatternDefinitions(category string, enabledOnly bool) ([]model.PatternDefinition, error) {
	sql := `
	select row_to_json(t)
	from (
	  select pattern_code, name, category, talib_function, direction, description,
	         params_schema::text as params_schema,
	         enabled, created_at, updated_at
	  from stock_pattern_definitions
	  where ($1 = '' or category = $1)
	    and (not $2 or enabled)
	  order by pattern_code asc
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(category), enabledOnly)
	if err != nil {
		return nil, err
	}
	definitions := make([]model.PatternDefinition, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var definition model.PatternDefinition
		if err := json.Unmarshal([]byte(line), &definition); err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

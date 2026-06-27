package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

// =============== Alert Repository (worker 端) ===============

func (r *PostgresRepository) ListEnabledAlertRules() ([]model.AlertRule, error) {
	sql := `
	select row_to_json(t) from (
	  select rule_id, symbol, name, market, metric, op, threshold,
	         cooldown_seconds, enabled,
	         coalesce(last_triggered_at, 'epoch'::timestamptz) as last_triggered_at,
	         last_value, last_message, created_at, updated_at
	  from alert_rules
	  where enabled = true
	  order by created_at asc
	) t;`
	lines, err := r.queryLines(sql)
	if err != nil {
		return nil, err
	}
	rules := make([]model.AlertRule, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rule model.AlertRule
		if err := json.Unmarshal([]byte(line), &rule); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *PostgresRepository) RecordAlertEvent(ev model.AlertEvent) error {
	insertStmt := sqlStmt{
		query: `
	insert into alert_events
	  (event_id, rule_id, symbol, name, metric, op, threshold, value, status, message, triggered_at)
	values
	  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`,
		args: []any{
			ev.EventID, ev.RuleID, ev.Symbol, ev.Name,
			ev.Metric, ev.Op, ev.Threshold, ev.Value, ev.Status,
			ev.Message, ev.TriggeredAt,
		},
	}
	updateStmt := sqlStmt{
		query: `
	update alert_rules
	set last_triggered_at = $1,
	    last_value = $2,
	    last_message = $3,
	    updated_at = now()
	where rule_id = $4;`,
		args: []any{ev.TriggeredAt, ev.Value, ev.Message, ev.RuleID},
	}
	return r.execTx([]sqlStmt{insertStmt, updateStmt})
}

// RunAlertScan 扫描所有启用的告警规则，检查是否触发。
func (r *TaskRunner) RunAlertScan() error {
	rules, err := r.repo.ListEnabledAlertRules()
	if err != nil {
		return fmt.Errorf("ListEnabledAlertRules: %w", err)
	}
	if len(rules) == 0 {
		return nil
	}
	bySymbol := map[string][]model.AlertRule{}
	for _, rule := range rules {
		bySymbol[rule.Symbol] = append(bySymbol[rule.Symbol], rule)
	}
	for symbol, group := range bySymbol {
		snapshot, err := r.adapter.Snapshot(symbol)
		if err != nil {
			log.Printf("[backend-worker][alert] snapshot %s failed: %v", symbol, err)
			continue
		}
		now := time.Now().UTC()
		for _, rule := range group {
			value, ok := extractMetric(snapshot, rule.Metric)
			if !ok {
				continue
			}
			if !rule.LastTriggeredAt.IsZero() {
				elapsed := now.Sub(rule.LastTriggeredAt)
				if elapsed < time.Duration(rule.CooldownSeconds)*time.Second {
					continue
				}
			}
			triggered := false
			switch rule.Op {
			case "gt":
				triggered = value > rule.Threshold
			case "lt":
				triggered = value < rule.Threshold
			case "gte":
				triggered = value >= rule.Threshold
			case "lte":
				triggered = value <= rule.Threshold
			case "eq":
				triggered = value == rule.Threshold
			}
			if !triggered {
				continue
			}
			message := fmt.Sprintf("[%s] %s %.4f %s %.4f (now %.4f)",
				rule.Symbol, rule.Metric, value, rule.Op, rule.Threshold, value)
			ev := model.AlertEvent{
				EventID:      fmt.Sprintf("ev_%d", now.UnixNano()),
				RuleID:       rule.RuleID,
				Symbol:       rule.Symbol,
				Name:         rule.Name,
				Metric:       rule.Metric,
				Op:           rule.Op,
				Threshold:    rule.Threshold,
				Value:        value,
				Status:       "triggered",
				Message:      message,
				TriggeredAt:  now,
			}
			if err := r.repo.RecordAlertEvent(ev); err != nil {
				log.Printf("[backend-worker][alert] RecordAlertEvent failed: %v", err)
			} else {
				log.Printf("[backend-worker][alert] triggered: %s", message)
			}
		}
	}
	return nil
}

func extractMetric(snapshot map[string]any, metric string) (float64, bool) {
	switch metric {
	case "price":
		if v, ok := snapshot["price"].(float64); ok {
			return v, true
		}
	case "pct_change":
		if v, ok := snapshot["pct_change"].(float64); ok {
			return v, true
		}
	case "volume_ratio":
		if v, ok := snapshot["volume_ratio"].(float64); ok {
			return v, true
		}
	case "premium_ratio":
		if v, ok := snapshot["premium_ratio"].(float64); ok {
			return v, true
		}
	case "iopv":
		if v, ok := snapshot["iopv"].(float64); ok {
			return v, true
		}
	case "turnover_rate":
		if v, ok := snapshot["turnover_rate"].(float64); ok {
			return v, true
		}
	}
	return 0, false
}


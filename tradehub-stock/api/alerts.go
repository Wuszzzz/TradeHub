package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

// =============== Alert Repository ===============

func (r *PostgresRepository) ListAlertRules(onlyEnabled bool) ([]model.AlertRule, error) {
	sql := `
	select row_to_json(t) from (
	  select rule_id, symbol, name, market, metric, op, threshold,
	         cooldown_seconds, enabled,
	         coalesce(last_triggered_at, 'epoch'::timestamptz) as last_triggered_at,
	         last_value, last_message, created_at, updated_at
	  from alert_rules
	  where (not $1 or enabled)
	  order by created_at desc
	) t;`
	lines, err := r.queryLines(sql, onlyEnabled)
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

func (r *PostgresRepository) UpsertAlertRule(rule model.AlertRule) error {
	return r.exec(`
	insert into alert_rules
	  (rule_id, symbol, name, market, metric, op, threshold, cooldown_seconds, enabled, created_at, updated_at)
	values
	  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	on conflict (rule_id) do update set
	  symbol = excluded.symbol,
	  name = excluded.name,
	  market = excluded.market,
	  metric = excluded.metric,
	  op = excluded.op,
	  threshold = excluded.threshold,
	  cooldown_seconds = excluded.cooldown_seconds,
	  enabled = excluded.enabled,
	  updated_at = excluded.updated_at;`,
		rule.RuleID, rule.Symbol, rule.Name, rule.Market,
		rule.Metric, rule.Op, rule.Threshold, rule.CooldownSeconds, rule.Enabled,
		rule.CreatedAt, rule.UpdatedAt)
}

func (r *PostgresRepository) DeleteAlertRule(ruleID string) error {
	return r.exec(`delete from alert_rules where rule_id = $1;`, ruleID)
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

func (r *PostgresRepository) ListAlertEvents(status string, limit int) ([]model.AlertEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t) from (
	  select event_id, rule_id, symbol, name, metric, op, threshold, value,
	         status, message, triggered_at,
	         coalesce(ack_at, 'epoch'::timestamptz) as ack_at
	  from alert_events
	  where ($1 = '' or status = $1)
	  order by triggered_at desc
	  limit $2
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(status), limit)
	if err != nil {
		return nil, err
	}
	events := make([]model.AlertEvent, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev model.AlertEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

func (r *PostgresRepository) AckAlertEvent(eventID string) error {
	return r.exec(`update alert_events set status = 'ack', ack_at = now() where event_id = $1;`, eventID)
}

// =============== Alert Handlers ===============

type alertRuleReq struct {
	RuleID          string  `json:"rule_id"`
	Symbol          string  `json:"symbol"`
	Name            string  `json:"name"`
	Market          string  `json:"market"`
	Metric          string  `json:"metric"`
	Op              string  `json:"op"`
	Threshold       float64 `json:"threshold"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Enabled         *bool   `json:"enabled,omitempty"`
}

var validMetrics = map[string]bool{
	"price": true, "pct_change": true, "volume_ratio": true,
	"premium_ratio": true, "iopv": true, "turnover_rate": true,
}
var validOps = map[string]bool{
	"gt": true, "lt": true, "gte": true, "lte": true, "eq": true,
}

func (s *Server) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := s.taskRepo.ListAlertRules(false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": rules})
	case http.MethodPost:
		var req alertRuleReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
			return
		}
		req.Symbol = strings.TrimSpace(req.Symbol)
		req.Metric = strings.TrimSpace(req.Metric)
		req.Op = strings.TrimSpace(req.Op)
		if req.Symbol == "" || !validMetrics[req.Metric] || !validOps[req.Op] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol/metric/op invalid"})
			return
		}
		if req.CooldownSeconds <= 0 {
			req.CooldownSeconds = 300
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		now := time.Now().UTC()
		ruleID := req.RuleID
		if ruleID == "" {
			ruleID = fmt.Sprintf("rule_%d", now.UnixNano())
		}
		rule := model.AlertRule{
			RuleID:          ruleID,
			Symbol:          req.Symbol,
			Name:            fallback(req.Name, req.Symbol),
			Market:          fallback(req.Market, "CN-A"),
			Metric:          req.Metric,
			Op:              req.Op,
			Threshold:       req.Threshold,
			CooldownSeconds: req.CooldownSeconds,
			Enabled:         enabled,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.taskRepo.UpsertAlertRule(rule); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": rule})
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("rule_id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "rule_id is required"})
			return
		}
		if err := s.taskRepo.DeleteAlertRule(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAlertEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &limit)
	}
	events, err := s.taskRepo.ListAlertEvents(status, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) handleAlertEventAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("event_id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "event_id is required"})
		return
	}
	if err := s.taskRepo.AckAlertEvent(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acked": id})
}

// =============== ETF Risk ===============

func (s *Server) handleETFRisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol is required"})
		return
	}
	limit := 120
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &limit)
	}
	cacheKey := fmt.Sprintf("%s|%d", symbol, limit)
	if cached, ok := s.etfRiskCache.Get(cacheKey); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	data, err := s.adapter.ETFRisk(symbol, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	s.etfRiskCache.Set(cacheKey, data)
	writeJSON(w, http.StatusOK, data)
}

// =============== Broker (THS stub) ===============

func (s *Server) handleBrokerStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"broker":    "noop",
		"connected": false,
		"note":      "同花顺/券商对接尚未启用，当前仅模拟交易（paper trading）",
	})
}

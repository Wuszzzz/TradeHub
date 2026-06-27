package alert

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type Repository interface {
	ListAlertRules(onlyEnabled bool) ([]model.AlertRule, error)
	UpsertAlertRule(model.AlertRule) error
	DeleteAlertRule(ruleID string) error
	ListAlertEvents(status string, limit int) ([]model.AlertEvent, error)
	AckAlertEvent(eventID string) error
}

type Service struct {
	repo Repository
}

type RuleInput struct {
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

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListRules(_ context.Context) ([]model.AlertRule, error) {
	return s.repo.ListAlertRules(false)
}

func (s *Service) CreateRule(_ context.Context, input RuleInput) (model.AlertRule, error) {
	symbol := strings.TrimSpace(input.Symbol)
	metric := strings.TrimSpace(input.Metric)
	op := strings.TrimSpace(input.Op)
	if symbol == "" || !validMetrics[metric] || !validOps[op] {
		return model.AlertRule{}, fmt.Errorf("symbol/metric/op invalid")
	}
	cooldown := input.CooldownSeconds
	if cooldown <= 0 {
		cooldown = 300
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	now := time.Now().UTC()
	ruleID := strings.TrimSpace(input.RuleID)
	if ruleID == "" {
		ruleID = fmt.Sprintf("rule_%d", now.UnixNano())
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = symbol
	}
	market := strings.TrimSpace(input.Market)
	if market == "" {
		market = "CN-A"
	}
	rule := model.AlertRule{
		RuleID:          ruleID,
		Symbol:          symbol,
		Name:            name,
		Market:          market,
		Metric:          metric,
		Op:              op,
		Threshold:       input.Threshold,
		CooldownSeconds: cooldown,
		Enabled:         enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return rule, s.repo.UpsertAlertRule(rule)
}

func (s *Service) DeleteRule(_ context.Context, ruleID string) error {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return fmt.Errorf("rule_id is required")
	}
	return s.repo.DeleteAlertRule(ruleID)
}

func (s *Service) ListEvents(_ context.Context, status string, limit int) ([]model.AlertEvent, error) {
	return s.repo.ListAlertEvents(strings.TrimSpace(status), limit)
}

func (s *Service) AckEvent(_ context.Context, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return fmt.Errorf("event_id is required")
	}
	return s.repo.AckAlertEvent(eventID)
}

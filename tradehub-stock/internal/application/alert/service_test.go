package alert

import (
	"context"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct {
	rules  []model.AlertRule
	events []model.AlertEvent
}

func (r *fakeRepository) ListAlertRules(onlyEnabled bool) ([]model.AlertRule, error) {
	return r.rules, nil
}

func (r *fakeRepository) UpsertAlertRule(rule model.AlertRule) error {
	r.rules = append(r.rules, rule)
	return nil
}

func (r *fakeRepository) DeleteAlertRule(ruleID string) error {
	return nil
}

func (r *fakeRepository) ListAlertEvents(status string, limit int) ([]model.AlertEvent, error) {
	return r.events, nil
}

func (r *fakeRepository) AckAlertEvent(eventID string) error {
	return nil
}

func TestCreateRuleValidatesSymbolMetricAndOp(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.CreateRule(context.Background(), RuleInput{Symbol: "600519", Metric: "bad", Op: "gt"}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestCreateRuleDefaultsNameMarketCooldownAndEnabled(t *testing.T) {
	service := NewService(&fakeRepository{})
	rule, err := service.CreateRule(context.Background(), RuleInput{
		Symbol:    "600519",
		Metric:    "price",
		Op:        "gt",
		Threshold: 100,
	})
	if err != nil {
		t.Fatalf("create rule failed: %v", err)
	}
	if rule.Name != "600519" || rule.Market != "CN-A" || rule.CooldownSeconds != 300 || !rule.Enabled {
		t.Fatalf("unexpected rule defaults: %+v", rule)
	}
}

func TestAckEventValidatesEventID(t *testing.T) {
	service := NewService(&fakeRepository{})
	if err := service.AckEvent(context.Background(), " "); err == nil {
		t.Fatalf("expected event_id validation error")
	}
}

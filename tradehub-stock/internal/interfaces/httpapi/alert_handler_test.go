package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	alertapp "stock-etf-monitor/backend/internal/application/alert"
	"stock-etf-monitor/backend/model"
)

type fakeAlertRepository struct {
	rules  []model.AlertRule
	events []model.AlertEvent
}

func (r *fakeAlertRepository) ListAlertRules(onlyEnabled bool) ([]model.AlertRule, error) {
	return r.rules, nil
}

func (r *fakeAlertRepository) UpsertAlertRule(rule model.AlertRule) error {
	r.rules = append(r.rules, rule)
	return nil
}

func (r *fakeAlertRepository) DeleteAlertRule(ruleID string) error {
	return nil
}

func (r *fakeAlertRepository) ListAlertEvents(status string, limit int) ([]model.AlertEvent, error) {
	return r.events, nil
}

func (r *fakeAlertRepository) AckAlertEvent(eventID string) error {
	return nil
}

func TestAlertRulesPostReturnsUnifiedResponse(t *testing.T) {
	handler := NewAlertHandler(alertapp.NewService(&fakeAlertRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/alerts/rules", strings.NewReader(`{"symbol":"600519","metric":"price","op":"gt","threshold":100}`))
	rec := httptest.NewRecorder()

	handler.Rules(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestAlertAckRejectsMissingEventID(t *testing.T) {
	handler := NewAlertHandler(alertapp.NewService(&fakeAlertRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/alerts/events/ack", nil)
	rec := httptest.NewRecorder()

	handler.AckEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

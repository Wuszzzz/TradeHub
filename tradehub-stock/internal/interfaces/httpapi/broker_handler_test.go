package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stock-etf-monitor/backend/internal/application/broker"
)

func TestBrokerStatusReturnsUnifiedResponse(t *testing.T) {
	handler := NewBrokerHandler(broker.NewService())
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/broker/status", nil)
	rec := httptest.NewRecorder()

	handler.Status(rec, req)

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

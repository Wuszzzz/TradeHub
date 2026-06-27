package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stock-etf-monitor/backend/internal/application/instrument"
)

type fakeInstrumentProvider struct{}

func (p fakeInstrumentProvider) Search(keyword string) ([]map[string]any, error) {
	return []map[string]any{{"symbol": "600519", "keyword": keyword}}, nil
}

func (p fakeInstrumentProvider) Profile(symbol string) (map[string]any, error) {
	return map[string]any{"symbol": symbol, "name": "贵州茅台"}, nil
}

func TestInstrumentSearchReturnsUnifiedResponse(t *testing.T) {
	handler := NewInstrumentHandler(instrument.NewService(fakeInstrumentProvider{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/instruments/search?keyword=茅台", nil)
	rec := httptest.NewRecorder()

	handler.Search(rec, req)

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestInstrumentSearchRejectsEmptyKeyword(t *testing.T) {
	handler := NewInstrumentHandler(instrument.NewService(fakeInstrumentProvider{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/instruments/search", nil)
	rec := httptest.NewRecorder()

	handler.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

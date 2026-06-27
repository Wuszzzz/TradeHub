package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stock-etf-monitor/backend/internal/application/market"
)

type fakeMarketSnapshotProvider struct{}

func (p fakeMarketSnapshotProvider) Snapshot(symbol string) (map[string]any, error) {
	return map[string]any{"symbol": symbol, "price": 100.0}, nil
}

type fakeMarketBarsProvider struct{}

func (p fakeMarketBarsProvider) QueryBars(symbol, period string, limit int) ([]map[string]any, error) {
	return []map[string]any{{"symbol": symbol, "period": period, "close": 100.0}}, nil
}

type fakeMarketETFRiskProvider struct{}

func (p fakeMarketETFRiskProvider) ETFRisk(symbol string, limit int) (map[string]any, error) {
	return map[string]any{"symbol": symbol, "limit": limit}, nil
}

func TestMarketSnapshotReturnsUnifiedResponse(t *testing.T) {
	handler := NewMarketHandler(market.NewService(fakeMarketSnapshotProvider{}, fakeMarketBarsProvider{}, fakeMarketETFRiskProvider{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/market/snapshot?symbol=600519", nil)
	rec := httptest.NewRecorder()

	handler.Snapshot(rec, req)

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestMarketKlineRejectsInvalidLimit(t *testing.T) {
	handler := NewMarketHandler(market.NewService(fakeMarketSnapshotProvider{}, fakeMarketBarsProvider{}, fakeMarketETFRiskProvider{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/market/kline?symbol=600519&limit=bad", nil)
	rec := httptest.NewRecorder()

	handler.Kline(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMarketETFRiskReturnsUnifiedResponse(t *testing.T) {
	handler := NewMarketHandler(market.NewService(fakeMarketSnapshotProvider{}, fakeMarketBarsProvider{}, fakeMarketETFRiskProvider{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/etf/risk?symbol=513310", nil)
	rec := httptest.NewRecorder()

	handler.ETFRisk(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

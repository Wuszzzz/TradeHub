package market

import (
	"context"
	"testing"
)

type fakeSnapshotProvider struct{}

func (p fakeSnapshotProvider) Snapshot(symbol string) (map[string]any, error) {
	return map[string]any{"symbol": symbol, "price": 100.0}, nil
}

type fakeBarsProvider struct{}

func (p fakeBarsProvider) QueryBars(symbol, period string, limit int) ([]map[string]any, error) {
	return []map[string]any{{"symbol": symbol, "period": period, "close": 100.0}}, nil
}

type fakeETFRiskProvider struct{}

func (p fakeETFRiskProvider) ETFRisk(symbol string, limit int) (map[string]any, error) {
	return map[string]any{"symbol": symbol, "limit": limit}, nil
}

func TestServiceSnapshotValidatesSymbol(t *testing.T) {
	service := NewService(fakeSnapshotProvider{}, fakeBarsProvider{}, fakeETFRiskProvider{})
	if _, err := service.Snapshot(context.Background(), " "); err == nil {
		t.Fatalf("expected symbol validation error")
	}
}

func TestServiceKlineDefaultsPeriod(t *testing.T) {
	service := NewService(fakeSnapshotProvider{}, fakeBarsProvider{}, fakeETFRiskProvider{})
	data, err := service.Kline(context.Background(), "600519", "", 120)
	if err != nil {
		t.Fatalf("kline failed: %v", err)
	}
	if data["period"] != "1m" {
		t.Fatalf("expected default period 1m, got %v", data["period"])
	}
}

func TestServiceKlineValidatesLimit(t *testing.T) {
	service := NewService(fakeSnapshotProvider{}, fakeBarsProvider{}, fakeETFRiskProvider{})
	if _, err := service.Kline(context.Background(), "600519", "1m", 0); err == nil {
		t.Fatalf("expected limit validation error")
	}
}

func TestServiceETFRiskValidatesSymbol(t *testing.T) {
	service := NewService(fakeSnapshotProvider{}, fakeBarsProvider{}, fakeETFRiskProvider{})
	if _, err := service.ETFRisk(context.Background(), " ", 120); err == nil {
		t.Fatalf("expected symbol validation error")
	}
}

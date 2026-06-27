package paper

import (
	"context"
	"testing"
	"time"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct {
	account   *model.PaperAccountRow
	orders    []model.PaperOrder
	positions []model.PaperPositionAgg
}

func (r *fakeRepository) ListPaperOrders(symbol string, limit int) ([]model.PaperOrder, error) {
	return r.orders, nil
}

func (r *fakeRepository) GetPaperAccountRow() (*model.PaperAccountRow, error) {
	if r.account != nil {
		return r.account, nil
	}
	return &model.PaperAccountRow{Cash: 1000000, Initial: 1000000, UpdatedAt: time.Now()}, nil
}

func (r *fakeRepository) PlaceOrder(order model.PaperOrder) error {
	r.orders = append(r.orders, order)
	return nil
}

func (r *fakeRepository) ComputePosition(symbol string) (*model.PaperPositionAgg, error) {
	for _, p := range r.positions {
		if p.Symbol == symbol {
			return &p, nil
		}
	}
	return nil, nil
}

func (r *fakeRepository) ListPositions() ([]model.PaperPositionAgg, error) {
	return r.positions, nil
}

func (r *fakeRepository) ResetPaper(initial float64) error {
	return nil
}

type fakeSnapshotProvider struct{}

func (p fakeSnapshotProvider) Snapshot(symbol string) (map[string]any, error) {
	return map[string]any{"price": 120.0}, nil
}

func TestPlaceOrderValidatesRequiredFields(t *testing.T) {
	service := NewService(&fakeRepository{}, fakeSnapshotProvider{})
	if _, err := service.PlaceOrder(context.Background(), OrderInput{Symbol: "600519", Side: "buy", Qty: 1}); err == nil {
		t.Fatalf("expected price validation error")
	}
}

func TestPlaceOrderAppliesDefaultFee(t *testing.T) {
	service := NewService(&fakeRepository{account: &model.PaperAccountRow{Cash: 100000, Initial: 100000}}, fakeSnapshotProvider{})
	order, err := service.PlaceOrder(context.Background(), OrderInput{Symbol: "600519", Side: "buy", Qty: 10, Price: 100})
	if err != nil {
		t.Fatalf("place order failed: %v", err)
	}
	if order.Fee != 5 {
		t.Fatalf("expected min fee 5, got %v", order.Fee)
	}
}

func TestPositionsUsesSnapshotPrice(t *testing.T) {
	service := NewService(&fakeRepository{positions: []model.PaperPositionAgg{{Symbol: "600519", Name: "贵州茅台", Market: "CN-A", Qty: 2, AvgCost: 100}}}, fakeSnapshotProvider{})
	positions, err := service.Positions(context.Background())
	if err != nil {
		t.Fatalf("positions failed: %v", err)
	}
	if len(positions) != 1 || positions[0].LastPrice != 120 {
		t.Fatalf("unexpected positions: %+v", positions)
	}
}

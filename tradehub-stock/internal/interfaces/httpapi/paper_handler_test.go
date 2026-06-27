package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	paperapp "stock-etf-monitor/backend/internal/application/paper"
	"stock-etf-monitor/backend/model"
)

type fakePaperRepository struct {
	account   *model.PaperAccountRow
	orders    []model.PaperOrder
	positions []model.PaperPositionAgg
}

func (r *fakePaperRepository) ListPaperOrders(symbol string, limit int) ([]model.PaperOrder, error) {
	return r.orders, nil
}

func (r *fakePaperRepository) GetPaperAccountRow() (*model.PaperAccountRow, error) {
	if r.account != nil {
		return r.account, nil
	}
	return &model.PaperAccountRow{Cash: 1000000, Initial: 1000000, UpdatedAt: time.Now()}, nil
}

func (r *fakePaperRepository) PlaceOrder(order model.PaperOrder) error {
	r.orders = append(r.orders, order)
	return nil
}

func (r *fakePaperRepository) ComputePosition(symbol string) (*model.PaperPositionAgg, error) {
	for _, p := range r.positions {
		if p.Symbol == symbol {
			return &p, nil
		}
	}
	return nil, nil
}

func (r *fakePaperRepository) ListPositions() ([]model.PaperPositionAgg, error) {
	return r.positions, nil
}

func (r *fakePaperRepository) ResetPaper(initial float64) error {
	return nil
}

type fakePaperSnapshotProvider struct{}

func (p fakePaperSnapshotProvider) Snapshot(symbol string) (map[string]any, error) {
	return map[string]any{"price": 120.0}, nil
}

func TestPaperOrdersPostReturnsUnifiedResponse(t *testing.T) {
	handler := NewPaperHandler(paperapp.NewService(&fakePaperRepository{account: &model.PaperAccountRow{Cash: 100000, Initial: 100000}}, fakePaperSnapshotProvider{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/paper/orders", strings.NewReader(`{"symbol":"600519","side":"buy","qty":10,"price":100}`))
	rec := httptest.NewRecorder()

	handler.Orders(rec, req)

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

func TestPaperResetReturnsUnifiedResponse(t *testing.T) {
	handler := NewPaperHandler(paperapp.NewService(&fakePaperRepository{}, fakePaperSnapshotProvider{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/paper/reset", strings.NewReader(`{"initial":1000000}`))
	rec := httptest.NewRecorder()

	handler.Reset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

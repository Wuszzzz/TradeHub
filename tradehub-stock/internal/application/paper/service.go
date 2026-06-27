package paper

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type Repository interface {
	ListPaperOrders(symbol string, limit int) ([]model.PaperOrder, error)
	GetPaperAccountRow() (*model.PaperAccountRow, error)
	PlaceOrder(model.PaperOrder) error
	ComputePosition(symbol string) (*model.PaperPositionAgg, error)
	ListPositions() ([]model.PaperPositionAgg, error)
	ResetPaper(initial float64) error
}

type SnapshotProvider interface {
	Snapshot(symbol string) (map[string]any, error)
}

type Service struct {
	repo     Repository
	snapshot SnapshotProvider
}

type OrderInput struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Market string  `json:"market"`
	Side   string  `json:"side"`
	Qty    float64 `json:"qty"`
	Price  float64 `json:"price"`
	Fee    float64 `json:"fee"`
	Note   string  `json:"note"`
}

func NewService(repo Repository, snapshot SnapshotProvider) *Service {
	return &Service{repo: repo, snapshot: snapshot}
}

func (s *Service) ListOrders(_ context.Context, symbol string, limit int) ([]model.PaperOrder, error) {
	return s.repo.ListPaperOrders(strings.TrimSpace(symbol), limit)
}

func (s *Service) PlaceOrder(_ context.Context, input OrderInput) (model.PaperOrder, error) {
	symbol := strings.TrimSpace(input.Symbol)
	side := strings.TrimSpace(input.Side)
	if symbol == "" || (side != "buy" && side != "sell") || input.Qty <= 0 || input.Price <= 0 {
		return model.PaperOrder{}, fmt.Errorf("symbol/side/qty/price invalid")
	}
	amount := input.Qty * input.Price
	fee := input.Fee
	if fee <= 0 {
		fee = math.Max(5, amount*0.00025)
	}
	if side == "sell" {
		pos, err := s.repo.ComputePosition(symbol)
		if err != nil {
			return model.PaperOrder{}, err
		}
		if pos == nil || pos.Qty < input.Qty {
			return model.PaperOrder{}, fmt.Errorf("持仓不足，无法卖出")
		}
	}
	if side == "buy" {
		acc, err := s.repo.GetPaperAccountRow()
		if err != nil {
			return model.PaperOrder{}, err
		}
		if acc.Cash < amount+fee {
			return model.PaperOrder{}, fmt.Errorf("现金不足：需要 %.2f，可用 %.2f", amount+fee, acc.Cash)
		}
	}
	now := time.Now().UTC()
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = symbol
	}
	market := strings.TrimSpace(input.Market)
	if market == "" {
		market = "CN-A"
	}
	order := model.PaperOrder{
		OrderID:  fmt.Sprintf("ord_%d", now.UnixNano()),
		Symbol:   symbol,
		Name:     name,
		Market:   market,
		Side:     side,
		Qty:      input.Qty,
		Price:    input.Price,
		Amount:   amount,
		Fee:      fee,
		Status:   "filled",
		Note:     input.Note,
		PlacedAt: now,
		FilledAt: now,
	}
	return order, s.repo.PlaceOrder(order)
}

func (s *Service) Positions(_ context.Context) ([]model.PaperPosition, error) {
	positions, err := s.repo.ListPositions()
	if err != nil {
		return nil, err
	}
	items := make([]model.PaperPosition, 0, len(positions))
	for _, p := range positions {
		last := p.AvgCost
		if snap, err := s.snapshot.Snapshot(p.Symbol); err == nil {
			if v, ok := snap["price"].(float64); ok && v > 0 {
				last = v
			}
		}
		mv := last * p.Qty
		items = append(items, model.PaperPosition{
			Symbol:       p.Symbol,
			Name:         p.Name,
			Market:       p.Market,
			Qty:          p.Qty,
			AvgCost:      p.AvgCost,
			LastPrice:    last,
			MarketValue:  mv,
			UnrealizedPL: (last - p.AvgCost) * p.Qty,
			UpdatedAt:    time.Now(),
		})
	}
	return items, nil
}

func (s *Service) Account(ctx context.Context) (model.PaperAccount, error) {
	acc, err := s.repo.GetPaperAccountRow()
	if err != nil {
		return model.PaperAccount{}, err
	}
	positions, err := s.Positions(ctx)
	if err != nil {
		return model.PaperAccount{}, err
	}
	mv := 0.0
	for _, p := range positions {
		mv += p.MarketValue
	}
	equity := acc.Cash + mv
	totalReturn := 0.0
	if acc.Initial > 0 {
		totalReturn = (equity - acc.Initial) / acc.Initial
	}
	return model.PaperAccount{
		Cash:        acc.Cash,
		Equity:      equity,
		RealizedPL:  acc.RealizedPL,
		TotalReturn: totalReturn,
		Initial:     acc.Initial,
		UpdatedAt:   acc.UpdatedAt,
	}, nil
}

func (s *Service) Reset(_ context.Context, initial float64) error {
	return s.repo.ResetPaper(initial)
}

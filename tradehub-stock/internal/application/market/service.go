package market

import (
	"context"
	"fmt"
	"strings"
)

type SnapshotProvider interface {
	Snapshot(symbol string) (map[string]any, error)
}

type BarsProvider interface {
	QueryBars(symbol, period string, limit int) ([]map[string]any, error)
}

type ETFRiskProvider interface {
	ETFRisk(symbol string, limit int) (map[string]any, error)
}

type Service struct {
	snapshot SnapshotProvider
	bars     BarsProvider
	etfRisk  ETFRiskProvider
}

func NewService(snapshot SnapshotProvider, bars BarsProvider, etfRisk ETFRiskProvider) *Service {
	return &Service{snapshot: snapshot, bars: bars, etfRisk: etfRisk}
}

func (s *Service) Snapshot(_ context.Context, symbol string) (map[string]any, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	return s.snapshot.Snapshot(symbol)
}

func (s *Service) ETFRisk(_ context.Context, symbol string, limit int) (map[string]any, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if limit <= 0 || limit > 2000 {
		limit = 120
	}
	return s.etfRisk.ETFRisk(symbol, limit)
}

func (s *Service) Kline(_ context.Context, symbol, period string, limit int) (map[string]any, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	period = strings.TrimSpace(period)
	if period == "" {
		period = "1m"
	}
	if limit <= 0 || limit > 2000 {
		return nil, fmt.Errorf("limit must be between 1 and 2000")
	}
	items, err := s.bars.QueryBars(symbol, period, limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"symbol": symbol,
		"period": period,
		"items":  items,
	}, nil
}

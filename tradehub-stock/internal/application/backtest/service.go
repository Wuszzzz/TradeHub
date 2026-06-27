package backtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type Repository interface {
	ListBacktestResults(taskID, symbol string, limit int) ([]model.BacktestResult, error)
	ListBacktestSummaries(taskID string, limit int) ([]model.BacktestSummary, error)
}

// KLineProvider K线数据提供接口
type KLineProvider interface {
	GetKLine(symbol, period string, limit int) ([]model.KlineBar, error)
}

// MarketDataProvider 市场数据提供接口
type MarketDataProvider interface {
	Snapshot(symbol string) (map[string]any, error)
}

type Service struct {
	repo     Repository
	klineProvider   KLineProvider
	marketProvider  MarketDataProvider
}

// BacktestInput 回测输入
type BacktestInput struct {
	StrategyID     string            `json:"strategy_id"`
	StrategyName   string            `json:"strategy_name"`
	Symbol         string            `json:"symbol"`
	Period         string            `json:"period"`
	Lookback       int               `json:"lookback"`
	HoldBars       int               `json:"hold_bars"`
	InitialCash    float64           `json:"initial_cash"`
	FeeRate        float64           `json:"fee_rate"`
	SlippageRate   float64           `json:"slippage_rate"`
	StopLoss       float64           `json:"stop_loss"`
	TakeProfit     float64           `json:"take_profit"`
	Benchmark      string            `json:"benchmark"`
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetKLineProvider 设置K线数据提供者
func (s *Service) SetKLineProvider(provider KLineProvider) {
	s.klineProvider = provider
}

// SetMarketDataProvider 设置市场数据提供者
func (s *Service) SetMarketDataProvider(provider MarketDataProvider) {
	s.marketProvider = provider
}

func (s *Service) ListResults(_ context.Context, taskID, symbol string, limit int) ([]model.BacktestResult, error) {
	if s.repo == nil {
		return []model.BacktestResult{}, nil
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.ListBacktestResults(strings.TrimSpace(taskID), strings.TrimSpace(symbol), limit)
}

func (s *Service) ListSummaries(_ context.Context, taskID string, limit int) ([]model.BacktestSummary, error) {
	if s.repo == nil {
		return []model.BacktestSummary{}, nil
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.ListBacktestSummaries(strings.TrimSpace(taskID), limit)
}

// ExecuteBacktest 执行回测
func (s *Service) ExecuteBacktest(_ context.Context, input BacktestInput) (*Result, error) {
	// 获取K线数据
	if s.klineProvider == nil {
		return nil, fmt.Errorf("kline provider not configured")
	}

	period := strings.TrimSpace(input.Period)
	if period == "" {
		period = "1d"
	}

	lookback := input.Lookback
	if lookback <= 0 {
		lookback = 260 // 默认一年
	}

	klines, err := s.klineProvider.GetKLine(input.Symbol, period, lookback)
	if err != nil {
		return nil, fmt.Errorf("failed to get kline: %w", err)
	}

	if len(klines) < 20 {
		return nil, fmt.Errorf("insufficient kline data: %d bars", len(klines))
	}

	// 创建策略
	var strategy Strategy
	switch input.StrategyID {
	case "ma_5_20":
		strategy = NewMAStrategy(5, 20)
	case "ma_10_60":
		strategy = NewMAStrategy(10, 60)
	case "ma_20_60":
		strategy = NewMAStrategy(20, 60)
	case "macd":
		strategy = NewMACDStrategy(12, 26, 9)
	case "buy_and_hold":
		strategy = NewBuyAndHold()
	default:
		// 默认使用均线策略
		strategy = NewMAStrategy(5, 20)
	}

	// 配置回测参数
	config := Config{
		InitialCash:  input.InitialCash,
		FeeRate:      input.FeeRate,
		SlippageRate: input.SlippageRate,
		StopLoss:     input.StopLoss,
		TakeProfit:   input.TakeProfit,
		HoldBars:     input.HoldBars,
		Lookback:     input.Lookback,
		Benchmark:    input.Benchmark,
	}

	// 执行回测
	engine := NewEngine(config)
	result, err := engine.Run(context.Background(), klines, strategy)
	if err != nil {
		return nil, fmt.Errorf("backtest failed: %w", err)
	}

	result.TaskID = fmt.Sprintf("bt_%d", time.Now().UnixNano())
	result.StrategyID = input.StrategyID
	if input.StrategyName != "" {
		result.StrategyName = input.StrategyName
	}

	return result, nil
}

// GetStrategyList 获取可用策略列表
func (s *Service) GetStrategyList() []map[string]any {
	return []map[string]any{
		{
			"id":          "buy_and_hold",
			"name":        "买入持有",
			"description": "在回测开始时买入，持有到最后",
			"params":      []string{},
		},
		{
			"id":          "ma_5_20",
			"name":        "均线策略(MA5/MA20)",
			"description": "MA5上穿MA20买入，下穿卖出",
			"params":      []string{"fast_period", "slow_period"},
		},
		{
			"id":          "ma_10_60",
			"name":        "均线策略(MA10/MA60)",
			"description": "MA10上穿MA60买入，下穿卖出",
			"params":      []string{"fast_period", "slow_period"},
		},
		{
			"id":          "ma_20_60",
			"name":        "均线策略(MA20/MA60)",
			"description": "MA20上穿MA60买入，下穿卖出",
			"params":      []string{"fast_period", "slow_period"},
		},
		{
			"id":          "macd",
			"name":        "MACD策略",
			"description": "MACD金叉买入，死叉卖出",
			"params":      []string{"fast_period", "slow_period", "signal_period"},
		},
	}
}

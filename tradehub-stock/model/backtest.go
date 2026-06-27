package model

import "time"

type BacktestResult struct {
	ResultID           string    `json:"result_id"`
	TaskID            string    `json:"task_id"`
	Symbol            string    `json:"symbol"`
	Period            string    `json:"period"`
	EntryTime         time.Time `json:"entry_time"`
	ExitTime          time.Time `json:"exit_time"`
	EntryPrice        float64   `json:"entry_price"`
	ExitPrice         float64   `json:"exit_price"`
	ReturnPct         float64   `json:"return_pct"`
	BenchmarkSymbol   string    `json:"benchmark_symbol"`
	BenchmarkReturnPct float64 `json:"benchmark_return_pct"`
	ExcessReturnPct   float64   `json:"excess_return_pct"`
	MetaJSON          string    `json:"meta_json"`
	CreatedAt         time.Time `json:"created_at"`
}

// BacktestMetrics 回测指标
type BacktestMetrics struct {
	TotalReturn       float64 `json:"total_return"`        // 总收益率%
	AnnualReturn      float64 `json:"annual_return"`       // 年化收益率%
	SharpeRatio       float64 `json:"sharpe_ratio"`       // 夏普比率
	MaxDrawdown       float64 `json:"max_drawdown"`       // 最大回撤%
	WinRate           float64 `json:"win_rate"`           // 胜率%
	ProfitLossRatio   float64 `json:"profit_loss_ratio"` // 盈亏比
	TotalTrades       int     `json:"total_trades"`       // 总交易次数
	WinTrades         int     `json:"win_trades"`         // 盈利次数
	LoseTrades        int     `json:"lose_trades"`        // 亏损次数
	AvgHoldingBars    float64 `json:"avg_holding_bars"`  // 平均持仓周期
	CalmarRatio       float64 `json:"calmar_ratio"`      // 卡玛比率
	Volatility        float64 `json:"volatility"`        // 波动率%
}

// BacktestEquityPoint 权益曲线点
type BacktestEquityPoint struct {
	Date    time.Time `json:"date"`
	Equity  float64   `json:"equity"`
	Cash    float64   `json:"cash"`
	Return  float64   `json:"return"`
}

// BacktestTrade 单笔交易
type BacktestTrade struct {
	EntryDate  time.Time `json:"entry_date"`
	ExitDate   time.Time `json:"exit_date"`
	Symbol     string    `json:"symbol"`
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	Qty        float64   `json:"qty"`
	PnL        float64   `json:"pnl"`
	PnLRate    float64   `json:"pnl_rate"`
	Reason     string    `json:"reason"`
}

// BacktestSummary 回测汇总
type BacktestSummary struct {
	SummaryID              string              `json:"summary_id"`
	TaskID                 string              `json:"task_id"`
	StrategyID             string              `json:"strategy_id"`
	StrategyName           string              `json:"strategy_name"`
	Symbol                 string              `json:"symbol"`
	Period                 string              `json:"period"`
	StartDate              time.Time           `json:"start_date"`
	EndDate                time.Time           `json:"end_date"`
	InitialCash            float64             `json:"initial_cash"`
	FinalEquity            float64             `json:"final_equity"`
	Metrics                BacktestMetrics     `json:"metrics"`
	EquityCurve            []BacktestEquityPoint `json:"equity_curve"`
	Trades                 []BacktestTrade     `json:"trades"`
	MetaJSON               string              `json:"meta_json"`
	CreatedAt              time.Time           `json:"created_at"`
}

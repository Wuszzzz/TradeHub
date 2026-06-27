package model

import "time"

// PaperOrder 纸交易订单
// Side: buy | sell
// Status: filled | canceled
type PaperOrder struct {
	OrderID  string    `json:"order_id"`
	Symbol   string    `json:"symbol"`
	Name     string    `json:"name"`
	Market   string    `json:"market"`
	Side     string    `json:"side"`
	Qty      float64   `json:"qty"`
	Price    float64   `json:"price"`
	Amount   float64   `json:"amount"`
	Fee      float64   `json:"fee"`
	Status   string    `json:"status"`
	Note     string    `json:"note"`
	PlacedAt time.Time `json:"placed_at"`
	FilledAt time.Time `json:"filled_at,omitempty"`
}

// PaperPosition 持仓快照（聚合视图）
type PaperPosition struct {
	Symbol       string    `json:"symbol"`
	Name         string    `json:"name"`
	Market       string    `json:"market"`
	Qty          float64   `json:"qty"`
	AvgCost      float64   `json:"avg_cost"`
	LastPrice    float64   `json:"last_price"`
	MarketValue  float64   `json:"market_value"`
	UnrealizedPL float64   `json:"unrealized_pl"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PaperPositionAgg 是从 paper_orders 聚合出的持仓基础数据，不含实时市值。
type PaperPositionAgg struct {
	Symbol  string  `json:"symbol"`
	Name    string  `json:"name"`
	Market  string  `json:"market"`
	Qty     float64 `json:"qty"`
	AvgCost float64 `json:"avg_cost"`
}

// PaperAccount 模拟账户
type PaperAccount struct {
	Cash        float64   `json:"cash"`
	Equity      float64   `json:"equity"`       // 现金 + 持仓市值
	RealizedPL  float64   `json:"realized_pl"`  // 已实现盈亏
	TotalReturn float64   `json:"total_return"` // (equity - initial) / initial
	Initial     float64   `json:"initial"`      // 初始资金
	UpdatedAt   time.Time `json:"updated_at"`
}

// PaperAccountRow 是 paper_account 表的原始账户行。
type PaperAccountRow struct {
	Cash       float64   `json:"cash"`
	Initial    float64   `json:"initial"`
	RealizedPL float64   `json:"realized_pl"`
	UpdatedAt  time.Time `json:"updated_at"`
}

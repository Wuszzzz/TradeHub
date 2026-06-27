package model

import "time"

type IngestionTask struct {
	TaskID      string    `json:"task_id"`
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Market      string    `json:"market"`
	Interval    string    `json:"interval"`
	Fields      []string  `json:"fields,omitempty"`
	Enabled     bool      `json:"enabled"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	LastRunAt   time.Time `json:"last_run_at,omitempty"`
	LastMessage string    `json:"last_message"`
}

type StockTask struct {
	TaskID     string    `json:"task_id"`
	TaskType   string    `json:"task_type"`
	Status     string    `json:"status"`
	Params     string    `json:"params"`
	ResultRef  string    `json:"result_ref"`
	Progress   int       `json:"progress"`
	Attempts   int       `json:"attempts"`
	LastError  string    `json:"last_error"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type StockTaskLog struct {
	LogID     string    `json:"log_id"`
	TaskID    string    `json:"task_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Context   string    `json:"context"`
	CreatedAt time.Time `json:"created_at"`
}

// KlineBar K线数据
type KlineBar struct {
	Symbol   string    `json:"symbol"`
	Period   string    `json:"period"`
	TS       time.Time `json:"ts"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
	Amount   float64   `json:"amount"`
}

// WatchlistGroup 自选分组
type WatchlistGroup struct {
	GroupID   string    `json:"group_id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WatchlistItem 自选项
type WatchlistItem struct {
	ItemID    string    `json:"item_id"`
	GroupID   string    `json:"group_id"`
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Market    string    `json:"market"`
	Note      string    `json:"note"`
	SortOrder int      `json:"sort_order"`
	AddedAt   time.Time `json:"added_at"`
	CreatedAt time.Time `json:"created_at"`
}

// WatchlistSnapshot 自选快照（带行情）
type WatchlistSnapshot struct {
	ItemID   string         `json:"item_id"`
	GroupID  string         `json:"group_id"`
	Symbol   string         `json:"symbol"`
	Name     string         `json:"name"`
	Market   string         `json:"market"`
	Quote    *QuoteSnapshot `json:"quote,omitempty"`
}

// QuoteSnapshot 行情快照
type QuoteSnapshot struct {
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	PrevClose     float64 `json:"prev_close"`
	Volume        float64 `json:"volume"`
	Amount        float64 `json:"amount"`
	TurnoverRate  float64 `json:"turnover_rate"`
	PE            float64 `json:"pe"`
	PB            float64 `json:"pb"`
	MarketCap     float64 `json:"market_cap"`
	Amplitude     float64 `json:"amplitude"`
	VolumeRatio   float64 `json:"volume_ratio"`
}

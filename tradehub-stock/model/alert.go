package model

import "time"

// AlertRule 监控规则
// Metric: price | pct_change | volume_ratio | premium_ratio | iopv | turnover_rate
// Op: gt | lt | gte | lte | eq
type AlertRule struct {
	RuleID          string    `json:"rule_id"`
	Symbol          string    `json:"symbol"`
	Name            string    `json:"name"`
	Market          string    `json:"market"`
	Metric          string    `json:"metric"`
	Op              string    `json:"op"`
	Threshold       float64   `json:"threshold"`
	CooldownSeconds int       `json:"cooldown_seconds"`
	Enabled         bool      `json:"enabled"`
	LastTriggeredAt time.Time `json:"last_triggered_at,omitempty"`
	LastValue       float64   `json:"last_value"`
	LastMessage     string    `json:"last_message"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AlertEvent 触发事件
type AlertEvent struct {
	EventID     string    `json:"event_id"`
	RuleID      string    `json:"rule_id"`
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Metric      string    `json:"metric"`
	Op          string    `json:"op"`
	Threshold   float64   `json:"threshold"`
	Value       float64   `json:"value"`
	Status      string    `json:"status"` // open | ack
	Message     string    `json:"message"`
	TriggeredAt time.Time `json:"triggered_at"`
	AckAt       time.Time `json:"ack_at,omitempty"`
}

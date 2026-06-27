// Package integration 预留外部券商 / 同花顺半自动执行的接入点。
// 当前仅提供 Broker 接口与一个 NoopBroker 实现（不做任何真实下单），
// 待后续接入同花顺 dll / 模拟下单 SDK 时替换实现即可。
package integration

import (
	"errors"
	"time"
)

// Order 与 model.PaperOrder 解耦的最小订单结构
type Order struct {
	OrderID   string
	Symbol    string
	Side      string // buy | sell
	Qty       float64
	Price     float64
	Note      string
	CreatedAt time.Time
}

// Position 通用持仓结构
type Position struct {
	Symbol  string
	Qty     float64
	AvgCost float64
}

// Broker 任意券商对接需要实现的最小接口
type Broker interface {
	Name() string
	Connected() bool
	PlaceOrder(o Order) (string, error)
	CancelOrder(orderID string) error
	QueryPositions() ([]Position, error)
}

// NoopBroker 占位实现：所有方法直接返回 not_connected，
// 用于在尚未接入真实券商前保证编译与接口形态稳定。
type NoopBroker struct{}

func (NoopBroker) Name() string { return "noop" }
func (NoopBroker) Connected() bool { return false }

var errNotConnected = errors.New("broker not connected: 同花顺/券商对接尚未启用")

func (NoopBroker) PlaceOrder(_ Order) (string, error) { return "", errNotConnected }
func (NoopBroker) CancelOrder(_ string) error         { return errNotConnected }
func (NoopBroker) QueryPositions() ([]Position, error) { return nil, errNotConnected }

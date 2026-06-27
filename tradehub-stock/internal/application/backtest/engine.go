package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"stock-etf-monitor/backend/model"
)

/**
 * Backtest Engine - 策略回测引擎
 * 支持：买入持有、均线策略、MACD策略等基础策略
 */

type Config struct {
	InitialCash    float64 // 初始资金
	FeeRate        float64 // 手续费率（默认万2.5）
	SlippageRate   float64 // 滑点率（默认万分之5）
	StopLoss       float64 // 止损比例（0表示不设置）
	TakeProfit     float64 // 止盈比例（0表示不设置）
	HoldBars       int     // 持仓周期
	Lookback       int     // 回看周期
	Benchmark      string  // 基准代码
}

type Order struct {
	Date       time.Time
	Symbol     string
	Side       string // "buy" or "sell"
	Price      float64
	Qty        float64
	Fee        float64
	Amount     float64
	Reason     string
}

type Position struct {
	Symbol    string
	Qty       float64
	AvgCost   float64
	HoldingBars int
}

type Trade struct {
	EntryDate  time.Time
	ExitDate   time.Time
	Symbol     string
	EntryPrice float64
	ExitPrice  float64
	Qty        float64
	PnL        float64
	PnLRate    float64
	Reason     string
}

type EquityCurve struct {
	Date      time.Time
	Equity    float64
	Cash      float64
	Position  float64
	Return    float64
	Benchmark float64
}

type Metrics struct {
	TotalReturn       float64   // 总收益率
	AnnualReturn     float64   // 年化收益率
	SharpeRatio      float64   // 夏普比率
	MaxDrawdown      float64   // 最大回撤
	WinRate          float64   // 胜率
	ProfitLossRatio  float64   // 盈亏比
	TotalTrades      int       // 总交易次数
	WinTrades        int       // 盈利次数
	LoseTrades       int       // 亏损次数
	AvgHoldingBars   float64   // 平均持仓周期
	MaxConsecutiveWin int      // 最大连续盈利
	MaxConsecutiveLose int     // 最大连续亏损
	CalmarRatio      float64   // 卡玛比率
	Volatility       float64   // 波动率
}

type Result struct {
	TaskID       string
	StrategyID   string
	StrategyName string
	Symbol       string
	StartDate    time.Time
	EndDate      time.Time
	Metrics      Metrics
	EquityCurve  []EquityCurve
	Trades       []Trade
	Orders       []Order
	CreatedAt    time.Time
}

// Engine 回测引擎
type Engine struct {
	config   Config
	position *Position
	orders   []Order
	trades   []Trade
	equity   []EquityCurve
	cash     float64
}

// NewEngine 创建回测引擎
func NewEngine(cfg Config) *Engine {
	if cfg.InitialCash <= 0 {
		cfg.InitialCash = 1000000
	}
	if cfg.FeeRate <= 0 {
		cfg.FeeRate = 0.00025
	}
	if cfg.SlippageRate <= 0 {
		cfg.SlippageRate = 0.0005
	}
	return &Engine{
		config:  cfg,
		cash:   cfg.InitialCash,
		orders: []Order{},
		trades: []Trade{},
		equity: []EquityCurve{},
	}
}

// Run 执行回测
func (e *Engine) Run(ctx context.Context, klines []model.KlineBar, strategy Strategy) (*Result, error) {
	if len(klines) == 0 {
		return nil, fmt.Errorf("kline data is empty")
	}

	// 排序
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].TS.Before(klines[j].TS)
	})

	startDate := klines[0].TS
	endDate := klines[len(klines)-1].TS

	// 初始化
	e.cash = e.config.InitialCash
	e.position = nil
	e.orders = []Order{}
	e.trades = []Trade{}
	e.equity = []EquityCurve{}

	// 生成信号并执行
	signals := strategy.Generate(klines)

	for i, bar := range klines {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		date := bar.TS
		price := bar.Close
		signal := signals[i]

		// 记录当日权益
		posValue := 0.0
		if e.position != nil {
			posValue = e.position.Qty * price
		}
		e.equity = append(e.equity, EquityCurve{
			Date:     date,
			Equity:   e.cash + posValue,
			Cash:     e.cash,
			Position: posValue,
		})

		// 执行交易
		if signal.Action == ActionBuy && e.position == nil && e.cash > price*100 {
			e.executeBuy(date, signal.Symbol, price, signal.Reason)
		} else if signal.Action == ActionSell && e.position != nil {
			e.executeSell(date, e.position.Symbol, price, signal.Reason)
		} else if e.position != nil {
			e.position.HoldingBars++
			// 止损止盈检查
			if e.config.StopLoss > 0 {
				pnlRate := (price - e.position.AvgCost) / e.position.AvgCost
				if pnlRate <= -e.config.StopLoss {
					e.executeSell(date, e.position.Symbol, price, "止损")
				}
			}
			if e.config.TakeProfit > 0 {
				pnlRate := (price - e.position.AvgCost) / e.position.AvgCost
				if pnlRate >= e.config.TakeProfit {
					e.executeSell(date, e.position.Symbol, price, "止盈")
				}
			}
			// 持仓周期止盈
			if e.config.HoldBars > 0 && e.position.HoldingBars >= e.config.HoldBars {
				e.executeSell(date, e.position.Symbol, price, "到期卖出")
			}
		}
	}

	// 平仓
	if e.position != nil {
		lastBar := klines[len(klines)-1]
		e.executeSell(lastBar.TS, e.position.Symbol, lastBar.Close, "回测结束")
	}

	// 计算指标
	metrics := e.calculateMetrics()

	return &Result{
		StrategyID:  strategy.GetID(),
		StrategyName: strategy.GetName(),
		Symbol:     klines[0].Symbol,
		StartDate:  startDate,
		EndDate:    endDate,
		Metrics:    metrics,
		EquityCurve: e.equity,
		Trades:     e.trades,
		Orders:     e.orders,
		CreatedAt:  time.Now(),
	}, nil
}

func (e *Engine) executeBuy(date time.Time, symbol string, price float64, reason string) {
	// 滑点
	buyPrice := price * (1 + e.config.SlippageRate)
	// 按手买入
	qty := math.Floor(e.cash / (buyPrice * (1 + e.config.FeeRate)) / 100 * 100)
	if qty < 100 {
		return
	}
	amount := qty * buyPrice
	fee := amount * e.config.FeeRate
	if fee < 5 {
		fee = 5
	}

	e.cash -= (amount + fee)
	e.position = &Position{
		Symbol:      symbol,
		Qty:         qty,
		AvgCost:    buyPrice,
		HoldingBars: 0,
	}
	e.orders = append(e.orders, Order{
		Date:   date,
		Symbol: symbol,
		Side:   "buy",
		Price:  buyPrice,
		Qty:    qty,
		Fee:    fee,
		Amount: amount,
		Reason: reason,
	})
}

func (e *Engine) executeSell(date time.Time, symbol string, price float64, reason string) {
	if e.position == nil {
		return
	}
	sellPrice := price * (1 - e.config.SlippageRate)
	qty := e.position.Qty
	amount := qty * sellPrice
	fee := amount * e.config.FeeRate
	if fee < 5 {
		fee = 5
	}
	pnl := (sellPrice - e.position.AvgCost) * qty

	e.trades = append(e.trades, Trade{
		EntryDate:  e.orders[len(e.orders)-1].Date,
		ExitDate:   date,
		Symbol:     symbol,
		EntryPrice: e.position.AvgCost,
		ExitPrice:  sellPrice,
		Qty:        qty,
		PnL:        pnl - fee,
		PnLRate:    (sellPrice - e.position.AvgCost) / e.position.AvgCost,
		Reason:     reason,
	})

	e.cash += (amount - fee)
	e.position = nil
	e.orders = append(e.orders, Order{
		Date:   date,
		Symbol: symbol,
		Side:   "sell",
		Price:  sellPrice,
		Qty:    qty,
		Fee:    fee,
		Amount: amount,
		Reason: reason,
	})
}

func (e *Engine) calculateMetrics() Metrics {
	if len(e.equity) == 0 {
		return Metrics{}
	}

	finalEquity := e.equity[len(e.equity)-1].Equity
	totalReturn := (finalEquity - e.config.InitialCash) / e.config.InitialCash

	// 年化收益率
	days := len(e.equity)
	annualReturn := math.Pow(1+totalReturn, 252/float64(days)) - 1

	// 最大回撤
	maxEquity := 0.0
	maxDrawdown := 0.0
	for _, eq := range e.equity {
		if eq.Equity > maxEquity {
			maxEquity = eq.Equity
		}
		drawdown := (maxEquity - eq.Equity) / maxEquity
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	// 胜率
	winTrades := 0
	loseTrades := 0
	totalPnL := 0.0
	for _, t := range e.trades {
		if t.PnL > 0 {
			winTrades++
		} else {
			loseTrades++
		}
		totalPnL += t.PnL
	}
	totalTrades := len(e.trades)
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winTrades) / float64(totalTrades)
	}

	// 盈亏比
	avgWin := 0.0
	avgLose := 0.0
	if winTrades > 0 {
		avgWin = totalPnL / float64(winTrades)
	}
	if loseTrades > 0 {
		avgLose = math.Abs(totalPnL) / float64(loseTrades)
	}
	profitLossRatio := 0.0
	if avgLose > 0 {
		profitLossRatio = avgWin / avgLose
	}

	// 夏普比率
	returns := make([]float64, len(e.equity)-1)
	for i := 1; i < len(e.equity); i++ {
		returns[i-1] = (e.equity[i].Equity - e.equity[i-1].Equity) / e.equity[i-1].Equity
	}
	sharpeRatio := calculateSharpeRatio(returns)
	volatility := calculateStdDev(returns)

	// 卡玛比率
	calmarRatio := 0.0
	if maxDrawdown > 0 {
		calmarRatio = annualReturn / maxDrawdown
	}

	// 平均持仓周期
	avgHolding := 0.0
	if totalTrades > 0 {
		sumHolding := 0
		for _, t := range e.trades {
			days := int(t.ExitDate.Sub(t.EntryDate).Hours() / 24)
			sumHolding += days
		}
		avgHolding = float64(sumHolding) / float64(totalTrades)
	}

	return Metrics{
		TotalReturn:      totalReturn * 100,
		AnnualReturn:     annualReturn * 100,
		SharpeRatio:      sharpeRatio,
		MaxDrawdown:      maxDrawdown * 100,
		WinRate:          winRate * 100,
		ProfitLossRatio:  profitLossRatio,
		TotalTrades:      totalTrades,
		WinTrades:        winTrades,
		LoseTrades:       loseTrades,
		AvgHoldingBars:   avgHolding,
		CalmarRatio:      calmarRatio,
		Volatility:       volatility * 100,
	}
}

func calculateSharpeRatio(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	mean := calculateMean(returns)
	stdDev := calculateStdDev(returns)
	if stdDev == 0 {
		return 0
	}
	return mean / stdDev * math.Sqrt(252)
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := calculateMean(values)
	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(values)))
}

// Strategy 策略接口
type Strategy interface {
	Generate(klines []model.KlineBar) []Signal
	GetID() string
	GetName() string
}

// Signal 交易信号
type Signal struct {
	Symbol string
	Action Action
	Reason string
}

type Action int

const (
	ActionHold Action = iota
	ActionBuy
	ActionSell
)

func (a Action) String() string {
	switch a {
	case ActionBuy:
		return "buy"
	case ActionSell:
		return "sell"
	default:
		return "hold"
	}
}

// BuyAndHold 买入持有策略
type BuyAndHold struct{}

func NewBuyAndHold() *BuyAndHold { return &BuyAndHold{} }

func (s *BuyAndHold) Generate(klines []model.KlineBar) []Signal {
	signals := make([]Signal, len(klines))
	if len(klines) > 0 {
		signals[0] = Signal{Symbol: klines[0].Symbol, Action: ActionBuy, Reason: "买入持有"}
	}
	for i := 1; i < len(klines); i++ {
		signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionHold}
	}
	return signals
}

func (s *BuyAndHold) GetID() string   { return "buy_and_hold" }
func (s *BuyAndHold) GetName() string { return "买入持有" }

// MAStrategy 均线策略
type MAStrategy struct {
	FastPeriod int
	SlowPeriod int
}

func NewMAStrategy(fast, slow int) *MAStrategy {
	return &MAStrategy{FastPeriod: fast, SlowPeriod: slow}
}

func (s *MAStrategy) Generate(klines []model.KlineBar) []Signal {
	signals := make([]Signal, len(klines))
	if len(klines) < s.SlowPeriod {
		return signals
	}

	// 计算均线
	maFast := make([]float64, len(klines))
	maSlow := make([]float64, len(klines))

	for i := s.FastPeriod - 1; i < len(klines); i++ {
		sumFast := 0.0
		for j := 0; j < s.FastPeriod; j++ {
			sumFast += klines[i-j].Close
		}
		maFast[i] = sumFast / float64(s.FastPeriod)
	}
	for i := s.SlowPeriod - 1; i < len(klines); i++ {
		sumSlow := 0.0
		for j := 0; j < s.SlowPeriod; j++ {
			sumSlow += klines[i-j].Close
		}
		maSlow[i] = sumSlow / float64(s.SlowPeriod)
	}

	prevFastAbove := false
	for i := s.SlowPeriod - 1; i < len(klines); i++ {
		fastAbove := maFast[i] > maSlow[i]
		if i > s.SlowPeriod && !prevFastAbove && fastAbove {
			signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionBuy, Reason: fmt.Sprintf("MA%d上穿MA%d", s.FastPeriod, s.SlowPeriod)}
		} else if i > s.SlowPeriod && prevFastAbove && !fastAbove {
			signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionSell, Reason: fmt.Sprintf("MA%d下穿MA%d", s.FastPeriod, s.SlowPeriod)}
		} else {
			signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionHold}
		}
		prevFastAbove = fastAbove
	}

	return signals
}

func (s *MAStrategy) GetID() string {
	return fmt.Sprintf("ma_%d_%d", s.FastPeriod, s.SlowPeriod)
}

func (s *MAStrategy) GetName() string {
	return fmt.Sprintf("均线策略(MA%d/MA%d)", s.FastPeriod, s.SlowPeriod)
}

// MACDStrategy MACD策略
type MACDStrategy struct {
	FastPeriod   int
	SlowPeriod   int
	SignalPeriod int
}

func NewMACDStrategy(fast, slow, signal int) *MACDStrategy {
	return &MACDStrategy{FastPeriod: fast, SlowPeriod: slow, SignalPeriod: signal}
}

func (s *MACDStrategy) Generate(klines []model.KlineBar) []Signal {
	signals := make([]Signal, len(klines))
	if len(klines) < s.SlowPeriod+s.SignalPeriod {
		return signals
	}

	// 计算EMA
	emaFast := calculateEMA(klines, s.FastPeriod)
	emaSlow := calculateEMA(klines, s.SlowPeriod)

	// 计算DIF和DEA
	dif := make([]float64, len(klines))
	dea := make([]float64, len(klines))
	for i := s.SlowPeriod - 1; i < len(klines); i++ {
		dif[i] = emaFast[i] - emaSlow[i]
	}

	// 计算DEA的EMA
	sum := 0.0
	for i := s.SlowPeriod - 1; i < s.SlowPeriod-1+s.SignalPeriod; i++ {
		sum += dif[i]
	}
	dea[s.SlowPeriod-1+s.SignalPeriod-1] = sum / float64(s.SignalPeriod)

	for i := s.SlowPeriod - 1 + s.SignalPeriod; i < len(klines); i++ {
		dea[i] = dea[i-1]*float64(s.SignalPeriod-1)/float64(s.SignalPeriod) + dif[i]*2/float64(s.SignalPeriod)
	}

	prevDifAbove := false
	for i := s.SlowPeriod - 1 + s.SignalPeriod; i < len(klines); i++ {
		difAbove := dif[i] > dea[i]
		macd := (dif[i] - dea[i]) * 2

		if i > s.SlowPeriod-1+s.SignalPeriod && !prevDifAbove && difAbove && macd > 0 {
			signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionBuy, Reason: "MACD金叉"}
		} else if i > s.SlowPeriod-1+s.SignalPeriod && prevDifAbove && !difAbove && macd < 0 {
			signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionSell, Reason: "MACD死叉"}
		} else {
			signals[i] = Signal{Symbol: klines[i].Symbol, Action: ActionHold}
		}
		prevDifAbove = difAbove
	}

	return signals
}

func (s *MACDStrategy) GetID() string {
	return fmt.Sprintf("macd_%d_%d_%d", s.FastPeriod, s.SlowPeriod, s.SignalPeriod)
}

func (s *MACDStrategy) GetName() string {
	return fmt.Sprintf("MACD策略(%d/%d/%d)", s.FastPeriod, s.SlowPeriod, s.SignalPeriod)
}

func calculateEMA(klines []model.KlineBar, period int) []float64 {
	ema := make([]float64, len(klines))
	if len(klines) < period {
		return ema
	}

	// 计算初始SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema[period-1] = sum / float64(period)

	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema[i] = (klines[i].Close - ema[i-1])*multiplier + ema[i-1]
	}

	return ema
}

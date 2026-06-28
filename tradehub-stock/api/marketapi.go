package main

// market-api 客户端：本地行情聚合网关（plugin/cmd/market-api）的 Go HTTP 适配层。
//
// 设计目的：让 AkshareAdapter 在 CN 市场路径不再 fork Python 子进程，
// 直接走纯 Go HTTP，免去 ~500ms 进程启动 + import akshare 的固定开销，
// 同时享受 market-api 自带的 30 子域名池 + LRU 缓存 + singleflight。
//
// 字段映射严格对齐 akshare_adapter.py 的现有输出结构，让上层 cachedSnapshot /
// QueryBars / 前端解构逻辑零改动。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// MarketAPIClient 是无状态的 HTTP 客户端；多个 goroutine 共享安全。
type MarketAPIClient struct {
	baseURL string
	http    *http.Client
}

// NewMarketAPIClient 读取 MARKET_API_URL 环境变量；为空则禁用（caller 应判 nil 后 fallback）。
func NewMarketAPIClient() *MarketAPIClient {
	base := strings.TrimRight(strings.TrimSpace(env("MARKET_API_URL", "")), "/")
	if base == "" {
		return nil
	}
	return &MarketAPIClient{
		baseURL: base,
		http: &http.Client{
			Timeout: 6 * time.Second, // market-api 内部含 5-8s 超时 + singleflight，6s 足够
		},
	}
}

// Enabled 标识当前实例是否真的可用（即 baseURL 非空）。
func (c *MarketAPIClient) Enabled() bool { return c != nil && c.baseURL != "" }

// envelope 是 market-api 的统一响应格式：{ok, data?, error?}.
type envelope struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// get 发起一次 GET 请求并解码 data 字段到 target。
func (c *MarketAPIClient) get(ctx context.Context, path string, query map[string]string, target any) error {
	if !c.Enabled() {
		return fmt.Errorf("market-api: client disabled")
	}
	u := c.baseURL + path
	if len(query) > 0 {
		parts := make([]string, 0, len(query))
		for k, v := range query {
			if v == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		if len(parts) > 0 {
			u = u + "?" + strings.Join(parts, "&")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "stock-etf-monitor/backend")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("market-api: %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("market-api: %s: decode envelope: %w", path, err)
	}
	if !env.OK {
		return fmt.Errorf("market-api: %s: %s", path, env.Error)
	}
	if target != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, target); err != nil {
			return fmt.Errorf("market-api: %s: decode data: %w", path, err)
		}
	}
	return nil
}

func (c *MarketAPIClient) rawGet(ctx context.Context, path string, query map[string]string) (map[string]any, error) {
	var data map[string]any
	if err := c.get(ctx, path, query, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// ----- 类型定义：与 plugin/cmd/market-api/internal/tencent 中保持一致 -----

type marketAPISnapshot struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Quote  struct {
		Latest        *float64 `json:"latest"`
		ChangePercent *float64 `json:"change_percent"`
		Open          *float64 `json:"open"`
		PreviousClose *float64 `json:"previous_close"`
		High          *float64 `json:"high"`
		Low           *float64 `json:"low"`
	} `json:"quote"`
	VolumeAmount struct {
		VolumeHand          *float64 `json:"volume_hand"`
		AmountYuan          *float64 `json:"amount_yuan"`
		TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
		VolumeRatio         *float64 `json:"volume_ratio"`
	} `json:"volume_amount"`
	OrderBook struct {
		Asks []marketAPIOrderLevel `json:"asks"`
		Bids []marketAPIOrderLevel `json:"bids"`
	} `json:"order_book"`
}

type marketAPIOrderLevel struct {
	Level  int      `json:"level"`
	Price  *float64 `json:"price"`
	Volume *float64 `json:"volume"`
}

type marketAPIKlineRow struct {
	Time       string   `json:"time"`
	Open       *float64 `json:"open"`
	Close      *float64 `json:"close"`
	High       *float64 `json:"high"`
	Low        *float64 `json:"low"`
	VolumeHand *float64 `json:"volume_hand"`
}

type marketAPIKlineResult struct {
	Rows []marketAPIKlineRow `json:"rows"`
}

type marketAPIMinuteRow struct {
	Time       string   `json:"time"`
	Price      *float64 `json:"price"`
	VolumeHand *float64 `json:"volume_hand"`
	AmountYuan *float64 `json:"amount_yuan"`
}

type marketAPIMinuteResult struct {
	Rows []marketAPIMinuteRow `json:"rows"`
}

// ----- 工具：float 安全转换 + 五档拍平 ------------------------------------

func safeFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	x := *v
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	return x
}

func safeFloatPtr(v *float64) any {
	if v == nil {
		return nil
	}
	x := *v
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}
	return x
}

// classifyCNMarket：6 位代码按首位粗判 ETF；与 akshare_adapter.py 保持一致。
func classifyCNMarket(symbol string) string {
	s := strings.TrimSpace(symbol)
	if strings.HasPrefix(s, "5") || strings.HasPrefix(s, "15") || strings.HasPrefix(s, "16") {
		return "CN-ETF"
	}
	return "CN-A"
}

// flattenOrderBook 把 market-api 的 asks/bids 数组拍平成 bid_1_price/bid_1_volume...
// 对齐 akshare_adapter.py 的 snapshot 输出契约。
func flattenOrderBook(item map[string]any, asks, bids []marketAPIOrderLevel) {
	// 先全部填 None（与 Python 端 _safe_get_or_none 同义）
	for _, side := range []string{"bid", "ask"} {
		for i := 1; i <= 5; i++ {
			item[fmt.Sprintf("%s_%d_price", side, i)] = nil
			item[fmt.Sprintf("%s_%d_volume", side, i)] = nil
		}
	}
	put := func(side string, rows []marketAPIOrderLevel) {
		for _, r := range rows {
			if r.Level < 1 || r.Level > 5 {
				continue
			}
			item[fmt.Sprintf("%s_%d_price", side, r.Level)] = safeFloatPtr(r.Price)
			item[fmt.Sprintf("%s_%d_volume", side, r.Level)] = safeFloatPtr(r.Volume)
		}
	}
	put("bid", bids)
	put("ask", asks)
}

// ----- 对外接口：Snapshot / Minute / Daily ---------------------------------

// Snapshot 拉一只标的实时快照；腾讯主，东财备。
// 返回结构与 akshare_adapter.py 完全一致：item dict 含 price/pct_change/五档/IOPV 等字段。
func (c *MarketAPIClient) Snapshot(ctx context.Context, symbol string) (map[string]any, error) {
	var lastErr error
	for _, vendor := range []string{"tencent", "eastmoney"} {
		var snap marketAPISnapshot
		err := c.get(ctx, "/api/v1/"+vendor+"/snapshot", map[string]string{"symbol": symbol}, &snap)
		if err != nil {
			lastErr = err
			continue
		}
		return mapSnapshot(symbol, snap), nil
	}
	return nil, fmt.Errorf("market-api snapshot failed across vendors: %w", lastErr)
}

func mapSnapshot(symbol string, s marketAPISnapshot) map[string]any {
	market := classifyCNMarket(symbol)
	name := s.Name
	if name == "" {
		name = symbol
	}
	amount := safeFloat(s.VolumeAmount.AmountYuan)
	item := map[string]any{
		"symbol":          symbol,
		"name":            name,
		"market":          market,
		"price":           safeFloat(s.Quote.Latest),
		"pct_change":      safeFloat(s.Quote.ChangePercent),
		"amount":          amount,
		"volume":          safeFloat(s.VolumeAmount.VolumeHand),
		"turnover_rate":   safeFloat(s.VolumeAmount.TurnoverRatePercent),
		"turnover_amount": amount,
		"volume_ratio":    safeFloat(s.VolumeAmount.VolumeRatio),
		// 腾讯不提供 ETF IOPV / 溢价率 / 大中小单；留 nil 前端显示「—」。
		// 后续如需补全，可叠加 sohu/aggregate 或 eastmoney/flow/* 端点。
		"iopv":                nil,
		"premium_ratio":       nil,
		"big_order_volume":    nil,
		"medium_order_volume": nil,
		"small_order_volume":  nil,
	}
	flattenOrderBook(item, s.OrderBook.Asks, s.OrderBook.Bids)
	return item
}

// 项目内 period 字面到 market-api tencent kline period 的映射。
// tencent kline 不支持 1m；这里仅处理 5m 及以上。1m 走 Minute 端点。
var klinePeriodMap = map[string]string{
	"5m": "5m", "15m": "15m", "30m": "30m",
	"1h": "60m", "60m": "60m",
	// 10m 上游都不直给，与 akshare 历来一致用 5m 替代。
	"10m": "5m",
}

// Minute 拉分钟级数据。period in {1m, 5m, 10m, 15m, 30m, 1h}。
//   - 1m       → tencent/minute（分时打点，OHLC 全填 price）
//   - 其他周期  → tencent/kline（真正分钟 K，含完整 OHLC）
func (c *MarketAPIClient) Minute(ctx context.Context, symbol, period string) ([]map[string]any, error) {
	if period == "1m" {
		var res marketAPIMinuteResult
		if err := c.get(ctx, "/api/v1/tencent/minute", map[string]string{"symbol": symbol}, &res); err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(res.Rows))
		for _, r := range res.Rows {
			price := safeFloat(r.Price)
			items = append(items, map[string]any{
				"symbol": symbol,
				"ts":     r.Time,
				"open":   price, "close": price, "high": price, "low": price,
				"volume":           safeFloat(r.VolumeHand),
				"amount":           safeFloat(r.AmountYuan),
				"requested_period": "1m",
				"source_period":    "tencent_minute",
			})
		}
		return items, nil
	}
	kp, ok := klinePeriodMap[period]
	if !ok {
		kp = "5m"
	}
	var res marketAPIKlineResult
	if err := c.get(ctx, "/api/v1/tencent/kline", map[string]string{"symbol": symbol, "period": kp, "limit": "240"}, &res); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(res.Rows))
	for _, r := range res.Rows {
		items = append(items, map[string]any{
			"symbol": symbol,
			"ts":     r.Time,
			"open":   safeFloat(r.Open),
			"close":  safeFloat(r.Close),
			"high":   safeFloat(r.High),
			"low":    safeFloat(r.Low),
			"volume": safeFloat(r.VolumeHand),
			// tencent kline 不直接给 amount，与 akshare 路径相同处理为 0
			"amount":           0.0,
			"requested_period": period,
			"source_period":    "tencent_kline_" + kp,
		})
	}
	return items, nil
}

// Daily 拉日 K；按 limit 截取，按时间升序返回。pct_change 由前一日 close 推算。
func (c *MarketAPIClient) Daily(ctx context.Context, symbol string, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 240
	}
	var res marketAPIKlineResult
	source := ""
	var lastErr error
	for _, vendor := range []string{"tencent", "eastmoney", "sohu"} {
		if err := c.get(ctx, "/api/v1/"+vendor+"/kline", map[string]string{
			"symbol": symbol, "period": "day", "limit": fmt.Sprintf("%d", limit),
		}, &res); err != nil {
			lastErr = err
			continue
		}
		source = vendor
		break
	}
	if source == "" {
		return nil, fmt.Errorf("market-api daily failed across vendors: %w", lastErr)
	}
	market := classifyCNMarket(symbol)
	items := make([]map[string]any, 0, len(res.Rows))
	var prevClose float64
	for i, r := range res.Rows {
		close := safeFloat(r.Close)
		pct := 0.0
		if i > 0 && prevClose > 0 {
			pct = (close - prevClose) / prevClose * 100
		}
		ts := r.Time
		if len(ts) >= 10 {
			ts = ts[:10] // 截到 YYYY-MM-DD
		}
		items = append(items, map[string]any{
			"ts":               ts,
			"open":             safeFloat(r.Open),
			"close":            close,
			"high":             safeFloat(r.High),
			"low":              safeFloat(r.Low),
			"volume":           safeFloat(r.VolumeHand),
			"amount":           0.0,
			"turnover_rate":    0.0,
			"pct_change":       pct,
			"requested_period": "1d",
			"source_period":    marketAPISourcePeriod(source, "day"),
		})
		prevClose = close
	}
	return map[string]any{"items": items, "market": market}, nil
}

func marketAPISourcePeriod(source, period string) string {
	switch source {
	case "eastmoney":
		return "em_" + period
	case "tencent":
		return "qq_" + period
	default:
		return source + "_" + period
	}
}

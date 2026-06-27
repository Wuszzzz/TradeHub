// Package ths wraps publicly reachable 10jqka JSONP endpoints that do not
// require an authenticated app session.
//
// Confirmed working on 2026-05-12:
//   - d.10jqka.com.cn/v6/realhead/hs_{code}/last.js         snapshot
//   - d.10jqka.com.cn/v6/time/hs_{code}/last.js             intraday minute
//   - d.10jqka.com.cn/v6/line/hs_{code}/{fq}/all.js         daily history
//   - d.10jqka.com.cn/v6/line/hs_{code}/{fq}/defer/today.js same-day patch
//
// Important limits:
//   - These are A-share oriented "hs_" endpoints only.
//   - We currently expose day K only; no verified free minute-K endpoint is
//     wired here yet.
package ths

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"stock/cmd/market-api/internal/symbol"
)

const (
	userAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	realHeadURL   = "https://d.10jqka.com.cn/v6/realhead/hs_%s/last.js"
	timeURL       = "https://d.10jqka.com.cn/v6/time/hs_%s/last.js"
	lineAllURL    = "https://d.10jqka.com.cn/v6/line/hs_%s/%s/all.js"
	lineTodayURL  = "https://d.10jqka.com.cn/v6/line/hs_%s/%s/defer/today.js"
	defaultFQCode = "01" // qfq
)

var (
	reJSONP = regexp.MustCompile(`(?s)^[^(]+\((.*)\)\s*;?\s*$`)
)

// Client is safe for concurrent use.
type Client struct {
	http *http.Client
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &Client{http: &http.Client{Timeout: timeout, Transport: transport}}
}

func (c *Client) get(ctx context.Context, rawURL string, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func unwrapJSONP(body []byte) ([]byte, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, errors.New("empty body")
	}
	m := reJSONP.FindStringSubmatch(text)
	if len(m) != 2 {
		return nil, fmt.Errorf("unexpected jsonp payload: %.120s", text)
	}
	return []byte(m[1]), nil
}

func floatPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func int64Ptr(s string) *int64 {
	f := floatPtr(s)
	if f == nil {
		return nil
	}
	v := int64(*f)
	return &v
}

func mustSpecHS(spec symbol.Spec) error {
	if spec.IsIndex {
		return errors.New("ths currently only supports stock/etf symbols, not indices")
	}
	if spec.Market != "sh" && spec.Market != "sz" {
		return errors.New("ths currently only supports sh/sz symbols")
	}
	return nil
}

type Snapshot struct {
	Dataset      string         `json:"dataset"`
	Source       string         `json:"source"`
	Symbol       string         `json:"symbol"`
	Code         string         `json:"code"`
	Name         string         `json:"name,omitempty"`
	TradeTime    string         `json:"trade_time,omitempty"`
	CapturedAt   int64          `json:"captured_at"`
	Quote        SnapshotQuote  `json:"quote"`
	VolumeAmount SnapshotVolume `json:"volume_amount"`
	Meta         SnapshotMeta   `json:"meta"`
	Raw          map[string]any `json:"raw,omitempty"`
}

type SnapshotQuote struct {
	Latest        *float64 `json:"latest"`
	Average       *float64 `json:"average"`
	ChangePercent *float64 `json:"change_percent"`
	Open          *float64 `json:"open"`
	PreviousClose *float64 `json:"previous_close"`
	High          *float64 `json:"high"`
	Low           *float64 `json:"low"`
	LimitUp       *float64 `json:"limit_up"`
	LimitDown     *float64 `json:"limit_down"`
}

type SnapshotVolume struct {
	VolumeHand          *int64   `json:"volume_hand"`
	AmountYuan          *float64 `json:"amount_yuan"`
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	VolumeRatio         *float64 `json:"volume_ratio"`
	AmplitudePercent    *float64 `json:"amplitude_percent"`
}

type SnapshotMeta struct {
	Stop        bool   `json:"stop"`
	MarketType  string `json:"market_type,omitempty"`
	StockStatus string `json:"stock_status,omitempty"`
	UpdateTime  string `json:"update_time,omitempty"`
}

func (c *Client) Snapshot(ctx context.Context, spec symbol.Spec) (*Snapshot, error) {
	if err := mustSpecHS(spec); err != nil {
		return nil, err
	}
	body, err := c.get(ctx, fmt.Sprintf(realHeadURL, spec.Code), "https://basic.10jqka.com.cn/"+spec.Code+"/")
	if err != nil {
		return nil, err
	}
	payload, err := unwrapJSONP(body)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Items       map[string]any `json:"items"`
		Stop        int            `json:"stop"`
		Time        string         `json:"time"`
		Name        string         `json:"name"`
		MarketType  string         `json:"marketType"`
		StockStatus string         `json:"stockStatus"`
		UpdateTime  string         `json:"updateTime"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	item := func(key string) string {
		v, ok := raw.Items[key]
		if !ok || v == nil {
			return ""
		}
		switch x := v.(type) {
		case string:
			return x
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			if x {
				return "1"
			}
			return "0"
		default:
			return fmt.Sprint(x)
		}
	}
	changePercent := floatPtr(item("199112"))
	latest := floatPtr(item("6"))
	prev := floatPtr(item("10"))
	var avg *float64
	if latest != nil && prev != nil {
		avg = floatPtr(item("1378761"))
	}
	return &Snapshot{
		Dataset:    "snapshot",
		Source:     "ths",
		Symbol:     spec.Tencent(),
		Code:       spec.Code,
		Name:       raw.Name,
		TradeTime:  strings.TrimSpace(raw.Time),
		CapturedAt: time.Now().Unix(),
		Quote: SnapshotQuote{
			Latest:        latest,
			Average:       avg,
			ChangePercent: changePercent,
			Open:          floatPtr(item("7")),
			PreviousClose: prev,
			High:          floatPtr(item("8")),
			Low:           floatPtr(item("9")),
			LimitUp:       floatPtr(item("69")),
			LimitDown:     floatPtr(item("70")),
		},
		VolumeAmount: SnapshotVolume{
			VolumeHand:          int64Ptr(item("13")),
			AmountYuan:          floatPtr(item("19")),
			TurnoverRatePercent: floatPtr(item("1968584")),
			VolumeRatio:         floatPtr(item("526792")),
			AmplitudePercent:    floatPtr(item("1149395")),
		},
		Meta: SnapshotMeta{
			Stop:        raw.Stop != 0,
			MarketType:  raw.MarketType,
			StockStatus: raw.StockStatus,
			UpdateTime:  raw.UpdateTime,
		},
		Raw: map[string]any{
			"items": raw.Items,
		},
	}, nil
}

type MinuteRow struct {
	Time         string   `json:"time"`
	Price        *float64 `json:"price"`
	AmountYuan   *float64 `json:"amount_yuan"`
	AveragePrice *float64 `json:"average_price"`
	VolumeHand   *float64 `json:"volume_hand"`
	Raw          string   `json:"raw"`
}

type MinuteResult struct {
	Dataset       string         `json:"dataset"`
	Source        string         `json:"source"`
	Symbol        string         `json:"symbol"`
	Code          string         `json:"code"`
	Name          string         `json:"name,omitempty"`
	TradeDate     string         `json:"trade_date,omitempty"`
	PreviousClose *float64       `json:"previous_close,omitempty"`
	Count         int            `json:"count"`
	Rows          []MinuteRow    `json:"rows"`
	Meta          map[string]any `json:"meta,omitempty"`
}

func (c *Client) Minute(ctx context.Context, spec symbol.Spec) (*MinuteResult, error) {
	if err := mustSpecHS(spec); err != nil {
		return nil, err
	}
	body, err := c.get(ctx, fmt.Sprintf(timeURL, spec.Code), "https://basic.10jqka.com.cn/"+spec.Code+"/")
	if err != nil {
		return nil, err
	}
	payload, err := unwrapJSONP(body)
	if err != nil {
		return nil, err
	}
	var raw map[string]struct {
		Name           string   `json:"name"`
		Open           int      `json:"open"`
		Stop           int      `json:"stop"`
		IsTrading      int      `json:"isTrading"`
		RT             string   `json:"rt"`
		TradeTime      []string `json:"tradeTime"`
		Pre            string   `json:"pre"`
		Date           string   `json:"date"`
		Data           string   `json:"data"`
		DotsCount      int      `json:"dotsCount"`
		Dates          []string `json:"dates"`
		AfterTradeTime string   `json:"afterTradeTime"`
		MarketType     string   `json:"marketType"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	key := "hs_" + spec.Code
	node, ok := raw[key]
	if !ok {
		return nil, fmt.Errorf("missing minute payload for %s", key)
	}
	out := &MinuteResult{
		Dataset:       "minute",
		Source:        "ths",
		Symbol:        spec.Tencent(),
		Code:          spec.Code,
		Name:          node.Name,
		TradeDate:     node.Date,
		PreviousClose: floatPtr(node.Pre),
		Meta: map[string]any{
			"is_trading":       node.IsTrading != 0,
			"market_type":      node.MarketType,
			"trade_time":       node.TradeTime,
			"after_trade_time": node.AfterTradeTime,
		},
	}
	for _, item := range strings.Split(node.Data, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.Split(item, ",")
		if len(parts) < 5 {
			continue
		}
		var volHand *float64
		if amt := floatPtr(parts[2]); amt != nil {
			v := *amt / 100.0
			volHand = &v
		}
		out.Rows = append(out.Rows, MinuteRow{
			Time:         parts[0],
			Price:        floatPtr(parts[1]),
			AmountYuan:   floatPtr(parts[2]),
			AveragePrice: floatPtr(parts[3]),
			VolumeHand:   volHand,
			Raw:          item,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

type KlineRow struct {
	Time       string   `json:"time"`
	Open       *float64 `json:"open"`
	High       *float64 `json:"high"`
	Low        *float64 `json:"low"`
	Close      *float64 `json:"close"`
	VolumeHand *float64 `json:"volume_hand"`
	Raw        string   `json:"raw"`
}

type KlineResult struct {
	Dataset     string     `json:"dataset"`
	Source      string     `json:"source"`
	Symbol      string     `json:"symbol"`
	Code        string     `json:"code"`
	Name        string     `json:"name,omitempty"`
	Period      string     `json:"period"`
	Adjust      string     `json:"adjust"`
	Count       int        `json:"count"`
	Rows        []KlineRow `json:"rows"`
	PriceFactor int64      `json:"price_factor,omitempty"`
}

func fqCode(adjust string) string {
	switch strings.ToLower(strings.TrimSpace(adjust)) {
	case "", "qfq":
		return "01"
	case "hfq":
		return "02"
	case "none":
		return "00"
	default:
		return defaultFQCode
	}
}

func (c *Client) Kline(ctx context.Context, spec symbol.Spec, period string, adjust string, limit int) (*KlineResult, error) {
	if err := mustSpecHS(spec); err != nil {
		return nil, err
	}
	period = strings.ToLower(strings.TrimSpace(period))
	if period == "" {
		period = "day"
	}
	if period != "day" {
		return nil, errors.New("ths kline currently supports period=day only")
	}
	_ = fqCode(adjust)
	_ = limit
	return nil, errors.New("ths kline day parser is not enabled yet: upstream all.js uses a compressed format and still needs a dedicated decoder")
}

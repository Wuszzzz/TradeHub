// Additional sohu endpoints discovered behind q.stock.sohu.com / hq.stock.sohu.com.
//
// All payloads here are GB18030-encoded JSONP-flavoured fragments. They are
// NOT standard JSON, so we use targeted regex extraction instead of a JSON
// decoder. Each endpoint is tied to a specific "fortune_hq" / "deal_data" /
// "time_data" / "div_price_data" callback name.
//
// IMPORTANT semantics:
//
//   * cn_{code}-3.html (deal_data)        逐笔成交明细 / tick stream
//                                         row layout: [time, price, change_pct, volume_hand, count]
//
//   * cn_{code}-4.html (time_data)        当日分时 / per-minute trend
//                                         first row: [prev_close, open, high, low, total_amount_yuan]
//                                         then rows: [time HH:MM, price, avg_price, vol_hand_delta, vol_hand_cum]
//
//   * cn_{code}-5.html (div_price_data)   逐价位成交分布 / volume-by-price.
//                                         row layout: [price, vol1_hand, vol2_hand, ratio%]
//                                         vol1/vol2 represent inner/outer volume buckets sohu groups
//                                         (their exact buy-vs-sell labelling is not documented, raw
//                                         array is preserved for downstream interpretation).
//                                         NOTE: this is NOT level-2 order book depth.
//
//   * ushq.stock.sohu.com/AFundFlow/STOCKS/{code}.html
//                                         详细资金流瞬时统计 (industry, active/passive split,
//                                         tier breakdown, intra-day net-in series).
//                                         Body is a JS object literal (unquoted keys, trailing
//                                         commas) wrapped in fortune_hq(...).
//
//   * ushq.stock.sohu.com/AFundFlow/STOCKS/{code}-1.html
//                                         简版资金流 (tradestat tier counters + 3 net-in series).

package sohu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"stock/cmd/market-api/internal/symbol"
)

const (
	hqStockURL  = "https://hq.stock.sohu.com/cn/%s/cn_%s-%d.html?_=%d"
	ushqStockURL = "https://ushq.stock.sohu.com/AFundFlow/STOCKS/%s%s.html?_=%d"
)

var (
	// reSubArr extracts each [...] tuple from a flat JSONP body.
	reSubArr = regexp.MustCompile(`\[[^\[\]]*\]`)
	// reKVPair extracts unquoted-key:value pairs from sohu's pseudo-JS objects.
	reKVPair = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_]*)\s*:\s*("(?:[^"\\]|\\.)*"|-?\d+(?:\.\d+)?|\[[^\]]*\]|null)`)
	// reArrayBlock extracts named array blocks like StockNetIn:[...]
	reArrayBlock = regexp.MustCompile(`(?s)([A-Za-z][A-Za-z0-9_]*)\s*:\s*\[([^\[]*?(?:\[[^\]]*\][^\[]*?)*?)\]`)
)

// fetchJSONP downloads a sohu JSONP fragment and returns the decoded UTF-8 text.
func (c *Client) fetchJSONP(ctx context.Context, rawURL string) (string, error) {
	body, err := c.get(ctx, rawURL, "https://q.stock.sohu.com/", true)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// stripCallback removes the leading "name(" and trailing ")".
func stripCallback(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '('); i > 0 && strings.HasSuffix(s, ")") {
		return s[i+1 : len(s)-1]
	}
	return s
}

// parseTuples extracts every [...] sub-array as a string slice.
// "['a','b','c'],['d','e']" -> [["a","b","c"], ["d","e"]]
func parseTuples(payload string) [][]string {
	var out [][]string
	for _, m := range reSubArr.FindAllString(payload, -1) {
		inner := strings.TrimPrefix(strings.TrimSuffix(m, "]"), "[")
		row := splitCSVQuoted(inner)
		out = append(out, row)
	}
	return out
}

// splitCSVQuoted splits a comma-separated list while respecting single/double quotes.
func splitCSVQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case inQuote != 0:
			if ch == inQuote {
				inQuote = 0
				continue
			}
			cur.WriteByte(ch)
		case ch == '\'' || ch == '"':
			inQuote = ch
		case ch == ',':
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			if ch != ' ' && ch != '\n' && ch != '\r' && ch != '\t' {
				cur.WriteByte(ch)
			}
		}
	}
	if cur.Len() > 0 || len(out) > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

// ---- Ticks (-3 deal_data) ----------------------------------------------

type Tick struct {
	Time          string   `json:"time"`           // HH:MM:SS
	Price         *float64 `json:"price"`
	ChangePercent *float64 `json:"change_percent"` // %
	VolumeHand    *int64   `json:"volume_hand"`
	Count         *int64   `json:"count"`          // 笔数
	Raw           []string `json:"raw"`
}

type TicksResult struct {
	Dataset       string   `json:"dataset"`
	Source        string   `json:"source"`
	Symbol        string   `json:"symbol"`
	SohuCode      string   `json:"sohu_code"`
	Period        string   `json:"period,omitempty"`
	Count         int      `json:"count"`
	Rows          []Tick   `json:"rows"`
}

// Ticks fetches sohu's per-tick deal stream (a small recent window only).
func (c *Client) Ticks(ctx context.Context, spec symbol.Spec) (*TicksResult, error) {
	url := fmt.Sprintf(hqStockURL, spec.SohuLast3(), spec.Code, 3, time.Now().UnixMilli())
	text, err := c.fetchJSONP(ctx, url)
	if err != nil {
		return nil, err
	}
	body := stripCallback(text)
	tuples := parseTuples(body)
	if len(tuples) == 0 {
		return nil, errors.New("sohu ticks: empty payload")
	}
	out := &TicksResult{Dataset: "ticks", Source: "sohu", Symbol: spec.Tencent(), SohuCode: spec.Sohu()}
	for _, row := range tuples {
		// Skip leading marker tuples like ['dealdetail_r'] or trailing ['dealdetail_p',['18']]
		if len(row) < 4 {
			if len(row) == 1 && strings.HasPrefix(row[0], "dealdetail") {
				// dealdetail_r marker
				continue
			}
			if len(row) == 1 && strings.Contains(row[0], ":") {
				// time-window marker like '15:15-15:30'
				out.Period = row[0]
				continue
			}
			continue
		}
		// Some rows may carry 5 fields (time,price,pct,vol,count); some 4
		// (time,price,pct,vol). Pad to 5.
		pad := append([]string(nil), row...)
		for len(pad) < 5 {
			pad = append(pad, "")
		}
		out.Rows = append(out.Rows, Tick{
			Time:          pad[0],
			Price:         parseFloatPtr(pad[1]),
			ChangePercent: percentToFloat(pad[2]),
			VolumeHand:    parseInt64Ptr(pad[3]),
			Count:         parseInt64Ptr(pad[4]),
			Raw:           row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

// ---- Minute (-4 time_data) ---------------------------------------------

type MinuteSummary struct {
	PreviousClose *float64 `json:"previous_close"`
	Open          *float64 `json:"open"`
	High          *float64 `json:"high"`
	Low           *float64 `json:"low"`
	AmountYuan    *float64 `json:"amount_yuan"`
}

type MinuteRow struct {
	Time            string   `json:"time"`            // HH:MM
	Price           *float64 `json:"price"`
	AveragePrice    *float64 `json:"average_price"`
	VolumeHandDelta *int64   `json:"volume_hand_delta"`
	VolumeHandTotal *int64   `json:"volume_hand_total"`
	Raw             []string `json:"raw"`
}

type MinuteResult struct {
	Dataset  string         `json:"dataset"`
	Source   string         `json:"source"`
	Symbol   string         `json:"symbol"`
	SohuCode string         `json:"sohu_code"`
	Summary  MinuteSummary  `json:"summary"`
	Count    int            `json:"count"`
	Rows     []MinuteRow    `json:"rows"`
}

func (c *Client) Minute(ctx context.Context, spec symbol.Spec) (*MinuteResult, error) {
	url := fmt.Sprintf(hqStockURL, spec.SohuLast3(), spec.Code, 4, time.Now().UnixMilli())
	text, err := c.fetchJSONP(ctx, url)
	if err != nil {
		return nil, err
	}
	body := stripCallback(text)
	tuples := parseTuples(body)
	if len(tuples) == 0 {
		return nil, errors.New("sohu minute: empty payload")
	}
	out := &MinuteResult{Dataset: "minute", Source: "sohu", Symbol: spec.Tencent(), SohuCode: spec.Sohu()}
	// First tuple is the summary header.
	header := tuples[0]
	hpad := append([]string(nil), header...)
	for len(hpad) < 5 {
		hpad = append(hpad, "")
	}
	out.Summary = MinuteSummary{
		PreviousClose: parseFloatPtr(hpad[0]),
		Open:          parseFloatPtr(hpad[1]),
		High:          parseFloatPtr(hpad[2]),
		Low:           parseFloatPtr(hpad[3]),
		AmountYuan:    parseFloatPtr(hpad[4]),
	}
	for _, row := range tuples[1:] {
		if len(row) < 5 {
			continue
		}
		out.Rows = append(out.Rows, MinuteRow{
			Time:            row[0],
			Price:           parseFloatPtr(row[1]),
			AveragePrice:    parseFloatPtr(row[2]),
			VolumeHandDelta: parseInt64Ptr(row[3]),
			VolumeHandTotal: parseInt64Ptr(row[4]),
			Raw:             row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

// ---- Price distribution (-5 div_price_data) ----------------------------

type PriceDistRow struct {
	Price         *float64 `json:"price"`
	Volume1Hand   *int64   `json:"volume1_hand"`    // 内盘量 / 主卖累计 (sohu raw column 1)
	Volume2Hand   *int64   `json:"volume2_hand"`    // 外盘量 / 主买累计 (sohu raw column 2)
	RatioPercent  *float64 `json:"ratio_percent"`   // 主动占比%
	Raw           []string `json:"raw"`
}

type PriceDistResult struct {
	Dataset  string         `json:"dataset"`
	Source   string         `json:"source"`
	Symbol   string         `json:"symbol"`
	SohuCode string         `json:"sohu_code"`
	Note     string         `json:"note"`
	Count    int            `json:"count"`
	Rows     []PriceDistRow `json:"rows"`
}

// PriceDistribution returns the per-price volume bucket histogram. This is the
// dataset that feeds the right-side price-volume column on sohu's quote page.
// It is NOT a level-2 order book depth snapshot.
func (c *Client) PriceDistribution(ctx context.Context, spec symbol.Spec) (*PriceDistResult, error) {
	url := fmt.Sprintf(hqStockURL, spec.SohuLast3(), spec.Code, 5, time.Now().UnixMilli())
	text, err := c.fetchJSONP(ctx, url)
	if err != nil {
		return nil, err
	}
	body := stripCallback(text)
	tuples := parseTuples(body)
	out := &PriceDistResult{
		Dataset:  "price_distribution",
		Source:   "sohu",
		Symbol:   spec.Tencent(),
		SohuCode: spec.Sohu(),
		Note:     "volume-by-price histogram, not level-2 depth",
	}
	for _, row := range tuples {
		if len(row) < 4 {
			continue
		}
		out.Rows = append(out.Rows, PriceDistRow{
			Price:        parseFloatPtr(row[0]),
			Volume1Hand:  parseInt64Ptr(row[1]),
			Volume2Hand:  parseInt64Ptr(row[2]),
			RatioPercent: percentToFloat(row[3]),
			Raw:          row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

// ---- Fund flow (ushq /STOCKS/{code}.html) ------------------------------

type FundFlow struct {
	Dataset    string         `json:"dataset"`
	Source     string         `json:"source"`
	Symbol     string         `json:"symbol"`
	SohuCode   string         `json:"sohu_code"`
	StockName  string         `json:"stock_name,omitempty"`
	Industry   string         `json:"industry,omitempty"`
	InValue    *float64       `json:"in_value"`     // 总流入 (元)
	OutValue   *float64       `json:"out_value"`    // 总流出 (元)
	NetValue   *float64       `json:"net_value"`    // in - out
	ActiveBuy  *float64       `json:"active_buy"`   // ABuy 主动买入
	ActiveBuyRatio   *float64 `json:"active_buy_ratio"`
	PassiveBuy *float64       `json:"passive_buy"`
	PassiveBuyRatio  *float64 `json:"passive_buy_ratio"`
	ActiveSell *float64       `json:"active_sell"`
	ActiveSellRatio  *float64 `json:"active_sell_ratio"`
	PassiveSell *float64      `json:"passive_sell"`
	PassiveSellRatio *float64 `json:"passive_sell_ratio"`
	Tier       FundFlowTier   `json:"tier"`
	BigOrderActive   *float64 `json:"big_order_active_buy,omitempty"`   // BABuy
	BigOrderPassiveBuy *float64 `json:"big_order_passive_buy,omitempty"` // BPBuy
	BigOrderActiveSell *float64 `json:"big_order_active_sell,omitempty"` // BASell
	BigOrderPassiveSell *float64 `json:"big_order_passive_sell,omitempty"` // BPSell
	SmallOrderActiveBuy  *float64 `json:"small_order_active_buy,omitempty"`
	SmallOrderPassiveBuy *float64 `json:"small_order_passive_buy,omitempty"`
	SmallOrderActiveSell *float64 `json:"small_order_active_sell,omitempty"`
	SmallOrderPassiveSell *float64 `json:"small_order_passive_sell,omitempty"`
	NetSeries    [][]string   `json:"net_series,omitempty"`     // StockNetIn 时序
	CorpSeries   [][]string   `json:"corp_series,omitempty"`    // StockCorpIn
	QuickSeries  [][]string   `json:"quick_series,omitempty"`   // StockQuickIn
	Time         string       `json:"time,omitempty"`
}

type FundFlowTier struct {
	SuperBuy   *float64 `json:"super_buy"`   // HBuy
	BigBuy     *float64 `json:"big_buy"`     // BBuy
	MediumBuy  *float64 `json:"medium_buy"`  // MBuy
	SmallBuy   *float64 `json:"small_buy"`   // SBuy
	SuperSell  *float64 `json:"super_sell"`  // HSell
	BigSell    *float64 `json:"big_sell"`    // BSell
	MediumSell *float64 `json:"medium_sell"` // MSell
	SmallSell  *float64 `json:"small_sell"`  // SSell
	SuperNet   *float64 `json:"super_net"`
	BigNet     *float64 `json:"big_net"`
	MediumNet  *float64 `json:"medium_net"`
	SmallNet   *float64 `json:"small_net"`
}

func (c *Client) FundFlow(ctx context.Context, spec symbol.Spec) (*FundFlow, error) {
	url := fmt.Sprintf(ushqStockURL, "", spec.Code, time.Now().UnixMilli())
	text, err := c.fetchJSONP(ctx, url)
	if err != nil {
		return nil, err
	}
	body := stripCallback(text)
	kv := parseSohuObject(body)
	out := &FundFlow{
		Dataset:  "fund_flow",
		Source:   "sohu",
		Symbol:   spec.Tencent(),
		SohuCode: spec.Sohu(),
		StockName: kv.Str("StockName"),
		Industry:  kv.Str("SecName"),
		InValue:   kv.Num("InVaule"),
		OutValue:  kv.Num("OutVaule"),
		ActiveBuy: kv.Num("ABuy"), ActiveBuyRatio: kv.Num("ABuyRate"),
		PassiveBuy: kv.Num("PBuy"), PassiveBuyRatio: kv.Num("PBuyRate"),
		ActiveSell: kv.Num("ASell"), ActiveSellRatio: kv.Num("ASellRate"),
		PassiveSell: kv.Num("PSell"), PassiveSellRatio: kv.Num("PSellRate"),
		BigOrderActive: kv.Num("BABuy"), BigOrderPassiveBuy: kv.Num("BPBuy"),
		BigOrderActiveSell: kv.Num("BASell"), BigOrderPassiveSell: kv.Num("BPSell"),
		SmallOrderActiveBuy: kv.Num("SABuy"), SmallOrderPassiveBuy: kv.Num("SPBuy"),
		SmallOrderActiveSell: kv.Num("SASell"), SmallOrderPassiveSell: kv.Num("SPSell"),
		Time: kv.Str("Time"),
	}
	if out.InValue != nil && out.OutValue != nil {
		v := *out.InValue - *out.OutValue
		out.NetValue = &v
	}
	out.Tier = FundFlowTier{
		SuperBuy:  kv.Num("HBuy"),
		BigBuy:    kv.Num("BBuy"),
		MediumBuy: kv.Num("MBuy"),
		SmallBuy:  kv.Num("SBuy"),
		SuperSell:  kv.Num("HSell"),
		BigSell:    kv.Num("BSell"),
		MediumSell: kv.Num("MSell"),
		SmallSell:  kv.Num("SSell"),
	}
	out.Tier.SuperNet = subPtr(out.Tier.SuperBuy, out.Tier.SuperSell)
	out.Tier.BigNet = subPtr(out.Tier.BigBuy, out.Tier.BigSell)
	out.Tier.MediumNet = subPtr(out.Tier.MediumBuy, out.Tier.MediumSell)
	out.Tier.SmallNet = subPtr(out.Tier.SmallBuy, out.Tier.SmallSell)
	out.NetSeries = parseSohuSeries(body, "StockNetIn")
	out.CorpSeries = parseSohuSeries(body, "StockCorpIn")
	out.QuickSeries = parseSohuSeries(body, "StockQuickIn")
	return out, nil
}

// ---- Fund flow series (ushq /STOCKS/{code}-1.html) ----------------------

type FundFlowSeries struct {
	Dataset     string         `json:"dataset"`
	Source      string         `json:"source"`
	Symbol      string         `json:"symbol"`
	SohuCode    string         `json:"sohu_code"`
	Tier        FundFlowTier   `json:"tier"`
	NetSeries   [][]string     `json:"net_series"`
	CorpSeries  [][]string     `json:"corp_series"`
	QuickSeries [][]string     `json:"quick_series"`
	Time        string         `json:"time,omitempty"`
}

func (c *Client) FundFlowSeries(ctx context.Context, spec symbol.Spec) (*FundFlowSeries, error) {
	url := fmt.Sprintf(ushqStockURL, "", spec.Code+"-1", time.Now().UnixMilli())
	text, err := c.fetchJSONP(ctx, url)
	if err != nil {
		return nil, err
	}
	body := stripCallback(text)
	// tradestat is nested: tradestat:{HBuy:..., ...}.
	statBlock := extractNamedBraceBlock(body, "tradestat")
	kv := parseSohuObject(statBlock)
	out := &FundFlowSeries{
		Dataset:  "fund_flow_series",
		Source:   "sohu",
		Symbol:   spec.Tencent(),
		SohuCode: spec.Sohu(),
		Tier: FundFlowTier{
			SuperBuy:  kv.Num("HBuy"), BigBuy: kv.Num("BBuy"),
			MediumBuy: kv.Num("MBuy"), SmallBuy: kv.Num("SBuy"),
			SuperSell:  kv.Num("HSell"), BigSell: kv.Num("BSell"),
			MediumSell: kv.Num("MSell"), SmallSell: kv.Num("SSell"),
		},
	}
	out.Tier.SuperNet = subPtr(out.Tier.SuperBuy, out.Tier.SuperSell)
	out.Tier.BigNet = subPtr(out.Tier.BigBuy, out.Tier.BigSell)
	out.Tier.MediumNet = subPtr(out.Tier.MediumBuy, out.Tier.MediumSell)
	out.Tier.SmallNet = subPtr(out.Tier.SmallBuy, out.Tier.SmallSell)
	out.NetSeries = parseSohuSeries(body, "StockNetIn")
	out.CorpSeries = parseSohuSeries(body, "StockCorpIn")
	out.QuickSeries = parseSohuSeries(body, "StockQuickIn")
	out.Time = parseTimeField(body)
	return out, nil
}

// ---- helpers for sohu's pseudo-JS object -------------------------------

type sohuKV map[string]string

func (m sohuKV) Str(k string) string {
	v := strings.TrimSpace(m[k])
	v = strings.Trim(v, `"`)
	return v
}

func (m sohuKV) Num(k string) *float64 {
	v := strings.TrimSpace(m[k])
	v = strings.Trim(v, `"`)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

// parseSohuObject extracts top-level scalar key:value pairs from sohu's
// pseudo-JS object body. Nested arrays are skipped (use parseSohuSeries).
func parseSohuObject(body string) sohuKV {
	out := sohuKV{}
	for _, m := range reKVPair.FindAllStringSubmatch(body, -1) {
		key := m[1]
		val := strings.TrimSpace(m[2])
		// Skip array-valued fields here.
		if strings.HasPrefix(val, "[") {
			continue
		}
		out[key] = val
	}
	return out
}

// parseSohuSeries extracts a named array-of-arrays block, e.g.
// StockNetIn:[["1","5269082999","108013504"],["1","5485841730","112913922"]].
func parseSohuSeries(body, name string) [][]string {
	idx := strings.Index(body, name+":")
	if idx < 0 {
		idx = strings.Index(body, name+" :")
		if idx < 0 {
			return nil
		}
	}
	rest := body[idx:]
	open := strings.IndexByte(rest, '[')
	if open < 0 {
		return nil
	}
	depth := 0
	end := -1
	for i := open; i < len(rest); i++ {
		switch rest[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}
	block := rest[open+1 : end]
	return parseTuples(block)
}

// extractNamedBraceBlock returns the matched-brace contents for `name:{ ... }`.
func extractNamedBraceBlock(body, name string) string {
	idx := strings.Index(body, name+":{")
	if idx < 0 {
		return ""
	}
	rest := body[idx:]
	open := strings.IndexByte(rest, '{')
	depth := 0
	end := -1
	for i := open; i < len(rest); i++ {
		switch rest[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return ""
	}
	return rest[open+1 : end]
}

func parseTimeField(body string) string {
	idx := strings.Index(body, "Time:")
	if idx < 0 {
		return ""
	}
	rest := body[idx+len("Time:"):]
	end := strings.IndexAny(rest, ",}")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func subPtr(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

// silence unused json import when only this file is changed.
var _ = json.Marshal

package tencent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	quoteURL       = "https://qt.gtimg.cn/q=%s"
	detailURL      = "https://stock.gtimg.cn/data/index.php?appn=detail&action=data&c=%s&p=%d"
	minuteURL      = "https://ifzq.gtimg.cn/appstock/app/minute/query?code=%s"
	minuteKlineURL = "https://ifzq.gtimg.cn/appstock/app/kline/mkline?param=%s,%s,,%d"
	fqKlineURL     = "https://ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,%s"
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

var (
	quoteRe  = regexp.MustCompile(`v_([a-z]{2}\d{6})="(.*?)";`)
	detailRe = regexp.MustCompile(`v_detail_data_[a-z]{2}\d{6}=\[(\d+),"(.*)"\];?`)
)

// Client wraps Tencent free market-data endpoints. It is safe to reuse.
type Client struct {
	http *http.Client
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &Client{http: &http.Client{Timeout: timeout, Transport: transport}}
}

func NormalizeSymbol(input string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return "", errors.New("empty symbol")
	}
	if matched, _ := regexp.MatchString(`^\d{6}$`, s); matched {
		if strings.HasPrefix(s, "5") || strings.HasPrefix(s, "6") || strings.HasPrefix(s, "9") {
			return "sh" + s, nil
		}
		return "sz" + s, nil
	}
	if matched, _ := regexp.MatchString(`^(sh|sz|bj)\d{6}$`, s); matched {
		return s, nil
	}
	return "", fmt.Errorf("invalid symbol: %s", input)
}

func SymbolToSecID(symbol string) string {
	if strings.HasPrefix(symbol, "sh") {
		return "1." + symbol[2:]
	}
	if strings.HasPrefix(symbol, "sz") {
		return "0." + symbol[2:]
	}
	return ""
}

func (c *Client) get(ctx context.Context, rawURL string, gb18030 bool) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://gu.qq.com/")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	var reader io.Reader = resp.Body
	if gb18030 {
		reader = transform.NewReader(resp.Body, simplifiedchinese.GB18030.NewDecoder())
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func f(parts []string, idx int) string {
	if idx < 0 || idx >= len(parts) {
		return ""
	}
	return parts[idx]
}

func floatPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func intPtr(s string) *int64 {
	p := floatPtr(s)
	if p == nil {
		return nil
	}
	v := int64(*p)
	return &v
}

func floatVal(s string) float64 {
	p := floatPtr(s)
	if p == nil {
		return 0
	}
	return *p
}

func parseTradeTime(s string) string {
	if len(s) == 14 {
		return fmt.Sprintf("%s-%s-%s %s:%s:%s", s[0:4], s[4:6], s[6:8], s[8:10], s[10:12], s[12:14])
	}
	return s
}

func compactTime(s string) string {
	if len(s) == 12 {
		return fmt.Sprintf("%s-%s-%s %s:%s", s[0:4], s[4:6], s[6:8], s[8:10], s[10:12])
	}
	if len(s) == 8 && regexp.MustCompile(`^\d{8}$`).MatchString(s) {
		return fmt.Sprintf("%s-%s-%s", s[0:4], s[4:6], s[6:8])
	}
	return s
}

type Snapshot struct {
	Dataset        string         `json:"dataset"`
	Source         string         `json:"source"`
	Symbol         string         `json:"symbol"`
	SecID          string         `json:"secid,omitempty"`
	MarketCode     string         `json:"market_code,omitempty"`
	Code           string         `json:"code,omitempty"`
	Name           string         `json:"name,omitempty"`
	TradeTime      string         `json:"trade_time,omitempty"`
	CapturedAt     int64          `json:"captured_at"`
	Quote          Quote          `json:"quote"`
	VolumeAmount   VolumeAmount   `json:"volume_amount"`
	OrderBookStats OrderBookStats `json:"order_book_stats"`
	OrderBook      OrderBook      `json:"order_book"`
	Raw            []string       `json:"raw,omitempty"`
}

type Quote struct {
	Latest        *float64 `json:"latest"`
	Average       *float64 `json:"average"`
	ChangeAmount  *float64 `json:"change_amount"`
	ChangePercent *float64 `json:"change_percent"`
	Open          *float64 `json:"open"`
	PreviousClose *float64 `json:"previous_close"`
	High          *float64 `json:"high"`
	Low           *float64 `json:"low"`
	LimitUp       *float64 `json:"limit_up"`
	LimitDown     *float64 `json:"limit_down"`
}

type VolumeAmount struct {
	VolumeHand          *int64   `json:"volume_hand"`
	AmountYuan          *float64 `json:"amount_yuan"`
	Amount10KYuan       *float64 `json:"amount_10k_yuan"`
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	VolumeRatio         *float64 `json:"volume_ratio"`
	AmplitudePercent    *float64 `json:"amplitude_percent"`
	OuterVolumeHand     *int64   `json:"outer_volume_hand"`
	InnerVolumeHand     *int64   `json:"inner_volume_hand"`
}

type OrderBookStats struct {
	EntrustDiffHand *int64 `json:"entrust_diff_hand"`
}

type OrderBook struct {
	Asks []OrderLevel `json:"asks"`
	Bids []OrderLevel `json:"bids"`
}

type OrderLevel struct {
	Level  int      `json:"level"`
	Price  *float64 `json:"price"`
	Volume *int64   `json:"volume"`
}

func (c *Client) Snapshots(ctx context.Context, symbols []string) ([]Snapshot, error) {
	normalized := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		n, err := NormalizeSymbol(symbol)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, n)
	}
	if len(normalized) == 0 {
		return nil, errors.New("symbols required")
	}
	text, err := c.get(ctx, fmt.Sprintf(quoteURL, url.QueryEscape(strings.Join(normalized, ","))), true)
	if err != nil {
		return nil, err
	}
	matches := quoteRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no quote records parsed: %.160s", text)
	}
	out := make([]Snapshot, 0, len(matches))
	for _, match := range matches {
		out = append(out, parseSnapshot(match[1], strings.Split(match[2], "~")))
	}
	return out, nil
}

func parseSnapshot(symbol string, fields []string) Snapshot {
	amount := floatPtr(f(fields, 35))
	compound := strings.Split(f(fields, 35), "/")
	if len(compound) >= 3 {
		amount = floatPtr(compound[2])
	}
	if amount == nil {
		if p := floatPtr(f(fields, 37)); p != nil {
			v := *p * 10000
			amount = &v
		}
	}
	return Snapshot{
		Dataset:    "snapshot",
		Source:     "tencent",
		Symbol:     symbol,
		SecID:      SymbolToSecID(symbol),
		MarketCode: f(fields, 0),
		Code:       f(fields, 2),
		Name:       f(fields, 1),
		TradeTime:  parseTradeTime(f(fields, 30)),
		CapturedAt: time.Now().Unix(),
		Quote: Quote{
			Latest:        floatPtr(f(fields, 3)),
			Average:       floatPtr(f(fields, 51)),
			ChangeAmount:  floatPtr(f(fields, 31)),
			ChangePercent: floatPtr(f(fields, 32)),
			Open:          floatPtr(f(fields, 5)),
			PreviousClose: floatPtr(f(fields, 4)),
			High:          floatPtr(f(fields, 33)),
			Low:           floatPtr(f(fields, 34)),
			LimitUp:       floatPtr(f(fields, 47)),
			LimitDown:     floatPtr(f(fields, 48)),
		},
		VolumeAmount: VolumeAmount{
			VolumeHand:          intPtr(f(fields, 36)),
			AmountYuan:          amount,
			Amount10KYuan:       floatPtr(f(fields, 37)),
			TurnoverRatePercent: floatPtr(f(fields, 38)),
			VolumeRatio:         floatPtr(f(fields, 49)),
			AmplitudePercent:    floatPtr(f(fields, 43)),
			OuterVolumeHand:     intPtr(f(fields, 7)),
			InnerVolumeHand:     intPtr(f(fields, 8)),
		},
		OrderBookStats: OrderBookStats{EntrustDiffHand: intPtr(f(fields, 50))},
		OrderBook:      parseOrderBook(fields),
		Raw:            fields,
	}
}

func parseOrderBook(fields []string) OrderBook {
	book := OrderBook{Asks: make([]OrderLevel, 0, 5), Bids: make([]OrderLevel, 0, 5)}
	for level := 1; level <= 5; level++ {
		bidIdx := 9 + (level-1)*2
		askIdx := 19 + (level-1)*2
		book.Bids = append(book.Bids, OrderLevel{Level: level, Price: floatPtr(f(fields, bidIdx)), Volume: intPtr(f(fields, bidIdx+1))})
		book.Asks = append(book.Asks, OrderLevel{Level: level, Price: floatPtr(f(fields, askIdx)), Volume: intPtr(f(fields, askIdx+1))})
	}
	return book
}

type Tick struct {
	Index        *int64   `json:"index"`
	Time         string   `json:"time"`
	Price        *float64 `json:"price"`
	Change       *float64 `json:"change"`
	VolumeShare  *int64   `json:"volume_share"`
	VolumeHand   *float64 `json:"volume_hand"`
	AmountYuan   *float64 `json:"amount_yuan"`
	SideCode     string   `json:"side_code"`
	Side         string   `json:"side"`
	LargeLevel   string   `json:"large_level"`
	IsLargeTrade bool     `json:"is_large_trade"`
	Page         int      `json:"page"`
	Symbol       string   `json:"symbol"`
	Raw          string   `json:"raw"`
}

type TicksResult struct {
	Dataset        string  `json:"dataset"`
	Source         string  `json:"source"`
	Symbol         string  `json:"symbol"`
	SecID          string  `json:"secid,omitempty"`
	Pages          []int   `json:"pages"`
	Count          int     `json:"count"`
	LargeThreshold float64 `json:"large_threshold"`
	SuperThreshold float64 `json:"super_threshold"`
	Rows           []Tick  `json:"rows"`
}

func sideName(code string) string {
	switch code {
	case "B":
		return "buy"
	case "S":
		return "sell"
	case "M":
		return "neutral"
	default:
		return "unknown"
	}
}

func largeLevel(amount *float64, largeThreshold, superThreshold float64) string {
	if amount == nil {
		return "unknown"
	}
	if *amount >= superThreshold {
		return "super_large"
	}
	if *amount >= largeThreshold {
		return "large"
	}
	return "normal"
}

func (c *Client) Ticks(ctx context.Context, symbol string, pages int, largeThreshold, superThreshold float64) (TicksResult, error) {
	n, err := NormalizeSymbol(symbol)
	if err != nil {
		return TicksResult{}, err
	}
	if pages <= 0 {
		pages = 1
	}
	if largeThreshold <= 0 {
		largeThreshold = 1_000_000
	}
	if superThreshold <= 0 {
		superThreshold = 5_000_000
	}
	result := TicksResult{Dataset: "ticks", Source: "tencent", Symbol: n, SecID: SymbolToSecID(n), LargeThreshold: largeThreshold, SuperThreshold: superThreshold}

	type pageResult struct {
		idx    int
		pageNo int
		rows   []Tick
		empty  bool
		err    error
	}

	// 腾讯 detail 接口社区经验上限约 5 req/s，并发度限定为 3，兼顾速度与限流风险。
	concurrency := 3
	if pages < concurrency {
		concurrency = pages
	}
	results := make([]pageResult, pages)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i := 0; i < pages; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			if cctx.Err() != nil {
				results[idx] = pageResult{idx: idx, err: cctx.Err()}
				return
			}
			text, err := c.get(cctx, fmt.Sprintf(detailURL, n, idx), true)
			if err != nil {
				results[idx] = pageResult{idx: idx, err: err}
				cancel()
				return
			}
			pr := pageResult{idx: idx}
			if strings.TrimSpace(text) == "" {
				pr.empty = true
				results[idx] = pr
				return
			}
			m := detailRe.FindStringSubmatch(text)
			if len(m) < 3 {
				pr.err = fmt.Errorf("cannot parse detail payload: %.160s", text)
				results[idx] = pr
				cancel()
				return
			}
			pr.pageNo, _ = strconv.Atoi(m[1])
			if m[2] == "" {
				pr.empty = true
				results[idx] = pr
				return
			}
			for _, item := range strings.Split(m[2], "|") {
				if item == "" {
					continue
				}
				parts := strings.Split(item, "/")
				if len(parts) < 7 {
					continue
				}
				amount := floatPtr(parts[5])
				volumeShare := intPtr(parts[4])
				var volumeHand *float64
				if volumeShare != nil {
					v := float64(*volumeShare) / 100.0
					volumeHand = &v
				}
				isLarge := amount != nil && *amount >= largeThreshold
				pr.rows = append(pr.rows, Tick{
					Index:        intPtr(parts[0]),
					Time:         parts[1],
					Price:        floatPtr(parts[2]),
					Change:       floatPtr(parts[3]),
					VolumeShare:  volumeShare,
					VolumeHand:   volumeHand,
					AmountYuan:   amount,
					SideCode:     parts[6],
					Side:         sideName(parts[6]),
					LargeLevel:   largeLevel(amount, largeThreshold, superThreshold),
					IsLargeTrade: isLarge,
					Page:         pr.pageNo,
					Symbol:       n,
					Raw:          item,
				})
			}
			results[idx] = pr
		}(i)
	}
	wg.Wait()

	// 按页序合并；若中间某页为空，腾讯一般意味着已无更多数据，截断。
	for i := 0; i < pages; i++ {
		pr := results[i]
		if pr.err != nil {
			return result, pr.err
		}
		if pr.empty {
			break
		}
		result.Pages = append(result.Pages, pr.pageNo)
		result.Rows = append(result.Rows, pr.rows...)
	}

	sort.Slice(result.Rows, func(i, j int) bool {
		if result.Rows[i].Index == nil {
			return false
		}
		if result.Rows[j].Index == nil {
			return true
		}
		return *result.Rows[i].Index < *result.Rows[j].Index
	})
	result.Count = len(result.Rows)
	return result, nil
}

type LargeTradesResult struct {
	Dataset      string                    `json:"dataset"`
	Source       string                    `json:"source"`
	Symbol       string                    `json:"symbol"`
	SecID        string                    `json:"secid,omitempty"`
	MinAmount    float64                   `json:"min_amount"`
	Count        int                       `json:"count"`
	DisplayCount int                       `json:"display_count,omitempty"`
	BySide       map[string]LargeTradeSide `json:"by_side"`
	Rows         []Tick                    `json:"rows"`
}

type LargeTradeSide struct {
	Count       int     `json:"count"`
	AmountYuan  float64 `json:"amount_yuan"`
	VolumeShare int64   `json:"volume_share"`
}

func LargeTrades(ticks TicksResult, minAmount float64) LargeTradesResult {
	if minAmount <= 0 {
		minAmount = 1_000_000
	}
	out := LargeTradesResult{Dataset: "large_trades", Source: "tencent", Symbol: ticks.Symbol, SecID: ticks.SecID, MinAmount: minAmount, BySide: map[string]LargeTradeSide{}}
	for _, row := range ticks.Rows {
		if row.AmountYuan == nil || *row.AmountYuan < minAmount {
			continue
		}
		out.Rows = append(out.Rows, row)
		bucket := out.BySide[row.Side]
		bucket.Count++
		bucket.AmountYuan += *row.AmountYuan
		if row.VolumeShare != nil {
			bucket.VolumeShare += *row.VolumeShare
		}
		out.BySide[row.Side] = bucket
	}
	out.Count = len(out.Rows)
	return out
}

type MinuteRow struct {
	Time       string   `json:"time"`
	Price      *float64 `json:"price"`
	VolumeHand *float64 `json:"volume_hand"`
	AmountYuan *float64 `json:"amount_yuan"`
	Raw        string   `json:"raw"`
}

type MinuteResult struct {
	Dataset string      `json:"dataset"`
	Source  string      `json:"source"`
	Symbol  string      `json:"symbol"`
	SecID   string      `json:"secid,omitempty"`
	Count   int         `json:"count"`
	Rows    []MinuteRow `json:"rows"`
}

func (c *Client) Minute(ctx context.Context, symbol string) (MinuteResult, error) {
	n, err := NormalizeSymbol(symbol)
	if err != nil {
		return MinuteResult{}, err
	}
	text, err := c.get(ctx, fmt.Sprintf(minuteURL, n), false)
	if err != nil {
		return MinuteResult{}, err
	}
	var payload struct {
		Data map[string]struct {
			Data struct {
				Date string   `json:"date"`
				Data []string `json:"data"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return MinuteResult{}, err
	}
	node, ok := payload.Data[n]
	if !ok {
		return MinuteResult{}, fmt.Errorf("missing minute data for %s", n)
	}
	tradeDate := compactTime(node.Data.Date)
	out := MinuteResult{Dataset: "minute", Source: "tencent", Symbol: n, SecID: SymbolToSecID(n)}
	for _, item := range node.Data.Data {
		parts := strings.Fields(item)
		if len(parts) < 4 {
			continue
		}
		minute := parts[0]
		if len(minute) == 4 {
			minute = minute[:2] + ":" + minute[2:]
		}
		out.Rows = append(out.Rows, MinuteRow{Time: strings.TrimSpace(tradeDate + " " + minute), Price: floatPtr(parts[1]), VolumeHand: floatPtr(parts[2]), AmountYuan: floatPtr(parts[3]), Raw: item})
	}
	out.Count = len(out.Rows)
	return out, nil
}

type KlineRow struct {
	Time       string   `json:"time"`
	Open       *float64 `json:"open"`
	Close      *float64 `json:"close"`
	High       *float64 `json:"high"`
	Low        *float64 `json:"low"`
	VolumeHand *float64 `json:"volume_hand"`
	Extension  any      `json:"extension_f7,omitempty"`
	Raw        []any    `json:"raw"`
}

type KlineResult struct {
	Dataset string     `json:"dataset"`
	Source  string     `json:"source"`
	Symbol  string     `json:"symbol"`
	SecID   string     `json:"secid,omitempty"`
	Period  string     `json:"period"`
	Adjust  string     `json:"adjust"`
	Code    string     `json:"code,omitempty"`
	Name    string     `json:"name,omitempty"`
	Count   int        `json:"count"`
	Rows    []KlineRow `json:"rows"`
}

var periods = map[string]string{"5m": "m5", "15m": "m15", "30m": "m30", "60m": "m60", "day": "day", "week": "week", "month": "month"}
var adjusts = map[string]string{"none": "", "qfq": "qfq", "hfq": "hfq"}

func (c *Client) Kline(ctx context.Context, symbol, period, adjust string, limit int) (KlineResult, error) {
	n, err := NormalizeSymbol(symbol)
	if err != nil {
		return KlineResult{}, err
	}
	remotePeriod, ok := periods[period]
	if !ok {
		return KlineResult{}, fmt.Errorf("invalid period: %s", period)
	}
	if limit <= 0 {
		limit = 100
	}
	if adjust == "" {
		adjust = "qfq"
	}
	remoteAdjust, ok := adjusts[adjust]
	if !ok {
		return KlineResult{}, fmt.Errorf("invalid adjust: %s", adjust)
	}
	var rawURL string
	if strings.HasSuffix(period, "m") {
		rawURL = fmt.Sprintf(minuteKlineURL, n, remotePeriod, limit)
	} else {
		rawURL = fmt.Sprintf(fqKlineURL, n, remotePeriod, limit, remoteAdjust)
	}
	text, err := c.get(ctx, rawURL, false)
	if err != nil {
		return KlineResult{}, err
	}
	var payload struct {
		Data map[string]map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return KlineResult{}, err
	}
	node, ok := payload.Data[n]
	if !ok {
		return KlineResult{}, fmt.Errorf("missing kline data for %s", n)
	}
	var rawRows [][]any
	if err := json.Unmarshal(node[remotePeriod], &rawRows); err != nil {
		return KlineResult{}, err
	}
	out := KlineResult{Dataset: "kline", Source: "tencent", Symbol: n, SecID: SymbolToSecID(n), Period: period, Adjust: adjust}
	if rawQT, ok := node["qt"]; ok {
		var qt map[string][]string
		if json.Unmarshal(rawQT, &qt) == nil {
			if arr := qt[n]; len(arr) > 2 {
				out.Name = arr[1]
				out.Code = arr[2]
			}
		}
	}
	for _, row := range rawRows {
		out.Rows = append(out.Rows, parseKlineRow(row))
	}
	out.Count = len(out.Rows)
	return out, nil
}

func parseKlineRow(row []any) KlineRow {
	get := func(idx int) string {
		if idx >= len(row) || row[idx] == nil {
			return ""
		}
		return fmt.Sprint(row[idx])
	}
	var ext any
	if len(row) > 7 {
		ext = row[7]
	}
	return KlineRow{Time: compactTime(get(0)), Open: floatPtr(get(1)), Close: floatPtr(get(2)), High: floatPtr(get(3)), Low: floatPtr(get(4)), VolumeHand: floatPtr(get(5)), Extension: ext, Raw: row}
}

func TrimTicks(rows []Tick, limit int) []Tick {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[len(rows)-limit:]
}

func TrimMinute(rows []MinuteRow, limit int) []MinuteRow {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[len(rows)-limit:]
}

func TrimKline(rows []KlineRow, limit int) []KlineRow {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[len(rows)-limit:]
}

func Round(v float64) float64 {
	return math.Round(v*100) / 100
}

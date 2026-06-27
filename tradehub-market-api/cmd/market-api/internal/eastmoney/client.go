// Package eastmoney wraps the public push2/push2his JSON endpoints used by
// quote.eastmoney.com and data.eastmoney.com.
//
// Why this looks different from the tencent client:
//   - Eastmoney enforces an aggressive per-IP WAF on data-heavy paths
//     (kline / details / trends2 / ulist.np / fflow/kline). A single host
//     direct hit often returns "Empty reply from server" while the same path
//     succeeds on a different numbered sub-domain.
//   - Therefore every call rotates through a randomized pool of
//     "N.push2[his].eastmoney.com" hosts and the first one that returns a
//     non-empty 2xx body wins.
//   - Snapshot has a fallback path to push2his kline/get (latest daily bar)
//     so callers always get a degraded but consistent payload.
package eastmoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"stock/cmd/market-api/internal/symbol"
)

const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	utQuote   = "fa5fd1943c7b386f172d6893dbfba10b"
	utFlow    = "b2884a393a59ad64002292a3e90d46a5"
	pageWbp2u = "|0|0|0|web"

	// hostPoolSize keeps enough variation to dodge per-host throttling while
	// bounding the worst-case latency when the IP is fully blacklisted.
	hostPoolSize = 30
)

var (
	push2Hosts    []string
	push2hisHosts []string
)

func init() {
	push2Hosts = append([]string{"push2.eastmoney.com"}, numbered("push2.eastmoney.com", hostPoolSize-1)...)
	push2hisHosts = append([]string{"push2his.eastmoney.com"}, numbered("push2his.eastmoney.com", hostPoolSize-1)...)
}

func numbered(suffix string, n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = fmt.Sprintf("%d.%s", i+1, suffix)
	}
	return out
}

// Client is safe for concurrent use.
type Client struct {
	http           *http.Client
	maxHostRetries int

	rngMu sync.Mutex
	rng   *rand.Rand
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &Client{
		http:           &http.Client{Timeout: timeout, Transport: transport},
		maxHostRetries: 8,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetMaxHostRetries bounds how many sub-domains will be tried.
func (c *Client) SetMaxHostRetries(n int) {
	if n > 0 {
		c.maxHostRetries = n
	}
}

func (c *Client) hostsFor(base string) []string {
	var pool []string
	switch base {
	case "push2.eastmoney.com":
		pool = push2Hosts
	case "push2his.eastmoney.com":
		pool = push2hisHosts
	default:
		return []string{base}
	}
	out := make([]string, len(pool))
	copy(out, pool)
	c.rngMu.Lock()
	c.rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	c.rngMu.Unlock()
	if c.maxHostRetries > 0 && c.maxHostRetries < len(out) {
		out = out[:c.maxHostRetries]
	}
	return out
}

// fetch performs a host-rotating GET. The first host that yields a non-empty
// 2xx body wins.
func (c *Client) fetch(ctx context.Context, base, path string, q url.Values, referer string) ([]byte, error) {
	var lastErr error
	for _, host := range c.hostsFor(base) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		body, err := c.doOnce(ctx, host, path, q, referer)
		if err == nil && len(body) > 0 {
			return body, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", host, err)
		} else {
			lastErr = fmt.Errorf("%s: empty body", host)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("all hosts exhausted")
	}
	return nil, lastErr
}

func (c *Client) doOnce(ctx context.Context, host, path string, q url.Values, referer string) ([]byte, error) {
	full := "https://" + host + path + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", referer)
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

// ---- Snapshot ----------------------------------------------------------

// Snapshot payload mirrors the field names of tencent.Snapshot for consistency
// at the API layer.
type Snapshot struct {
	Dataset        string         `json:"dataset"`
	Source         string         `json:"source"`
	Secid          string         `json:"secid"`
	Symbol         string         `json:"symbol"`
	Market         string         `json:"market,omitempty"`
	Code           string         `json:"code,omitempty"`
	Name           string         `json:"name,omitempty"`
	TradeTime      string         `json:"trade_time,omitempty"`
	CapturedAt     int64          `json:"captured_at"`
	TradeStatus    any            `json:"trade_status,omitempty"`
	PriceDecimals  any            `json:"price_decimals,omitempty"`
	PercentDecs    any            `json:"percent_decimals,omitempty"`
	Quote          Quote          `json:"quote"`
	VolumeAmount   VolumeAmount   `json:"volume_amount"`
	Valuation      Valuation      `json:"valuation"`
	OrderBookStats OrderBookStats `json:"order_book_stats"`
	OrderBook      OrderBook      `json:"order_book"`
	Degraded       bool           `json:"degraded"`
	DegradedReason string         `json:"degraded_reason,omitempty"`
	FallbackFrom   string         `json:"fallback_from,omitempty"`
	Raw            map[string]any `json:"raw,omitempty"`
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
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	VolumeRatio         *float64 `json:"volume_ratio"`
	AmplitudePercent    *float64 `json:"amplitude_percent"`
	OuterVolumeHand     *int64   `json:"outer_volume_hand"`
	InnerVolumeHand     *int64   `json:"inner_volume_hand"`
	CurrentHand         *int64   `json:"current_hand"`
	CurrentHandSide     string   `json:"current_hand_side,omitempty"`
}

type Valuation struct {
	TotalShares     *float64 `json:"total_shares"`
	FloatShares     *float64 `json:"float_shares"`
	TotalMarketCap  *float64 `json:"total_market_cap"`
	FloatMarketCap  *float64 `json:"float_market_cap"`
	PETTM           *float64 `json:"pe_ttm"`
	PB              *float64 `json:"pb"`
	EPSTTM          *float64 `json:"eps_ttm"`
	NavPerShare     *float64 `json:"nav_per_share"`
}

type OrderBookStats struct {
	EntrustRatioPercent *float64 `json:"entrust_ratio_percent"`
	EntrustDiffHand     *int64   `json:"entrust_diff_hand"`
}

type OrderBook struct {
	Asks []OrderLevel `json:"asks"`
	Bids []OrderLevel `json:"bids"`
}

type OrderLevel struct {
	Level       int      `json:"level"`
	Price       *float64 `json:"price"`
	Volume      *int64   `json:"volume"`
	DeltaVolume *int64   `json:"delta_volume,omitempty"`
}

var quoteFields = strings.Join([]string{
	"f57", "f58", "f107", "f43", "f59", "f169", "f170", "f152",
	"f46", "f60", "f44", "f45", "f47", "f48", "f86", "f292",
	"f49", "f50", "f71", "f161", "f168", "f171", "f452", "f51", "f52",
	"f191", "f192", "f84", "f85", "f92", "f108", "f116", "f117",
	"f154", "f164", "f167", "f177", "f600", "f601",
	"f19", "f20", "f17", "f18", "f15", "f16", "f13", "f14", "f11", "f12",
	"f39", "f40", "f37", "f38", "f35", "f36", "f33", "f34", "f31", "f32",
	"f211", "f212", "f213", "f214", "f215", "f210", "f209", "f208", "f207", "f206",
	"f531", "f532",
}, ",")

func quoteReferer(spec symbol.Spec) string {
	prefix := spec.Market
	if prefix == "bj" {
		prefix = "sz"
	}
	return "https://quote.eastmoney.com/" + prefix + spec.Code + ".html"
}

func flowReferer(spec symbol.Spec) string {
	return "https://data.eastmoney.com/zjlx/" + spec.Code + ".html"
}

// Snapshots fetches the real-time quote payload for a single security.
// Degradation: on failure we attempt a kline/get fallback so the JSON shape
// stays consistent for callers.
func (c *Client) Snapshot(ctx context.Context, spec symbol.Spec) (*Snapshot, error) {
	secid := spec.Eastmoney()
	q := url.Values{
		"fields": {quoteFields},
		"fltt":   {"1"},
		"invt":   {"2"},
		"dect":   {"1"},
		"secid":  {secid},
		"ut":     {utQuote},
		"wbp2u":  {pageWbp2u},
	}
	body, err := c.fetch(ctx, "push2.eastmoney.com", "/api/qt/stock/get", q, quoteReferer(spec))
	if err == nil {
		var payload struct {
			Data map[string]any `json:"data"`
		}
		if jerr := json.Unmarshal(body, &payload); jerr == nil && payload.Data != nil {
			return parseSnapshot(spec, payload.Data), nil
		}
	}
	// Fallback: latest daily kline.
	fb, fbErr := c.fallbackSnapshotFromKline(ctx, spec)
	if fbErr == nil {
		fb.Degraded = true
		fb.DegradedReason = summarizeErr(err)
		fb.FallbackFrom = "push2his_kline_day"
		return fb, nil
	}
	return nil, fmt.Errorf("snapshot failed and fallback failed: snap=%v fallback=%v", err, fbErr)
}

func parseSnapshot(spec symbol.Spec, data map[string]any) *Snapshot {
	decs := getInt(data, "f59")
	pctDecs := getInt(data, "f152")
	cur := getInt(data, "f452")
	var curAbs *int64
	var curSide string
	if cur != nil {
		v := *cur
		if v < 0 {
			v = -v
		}
		curAbs = &v
		switch {
		case *cur > 0:
			curSide = "buy"
		case *cur < 0:
			curSide = "sell"
		default:
			curSide = "neutral"
		}
	}
	return &Snapshot{
		Dataset:    "snapshot",
		Source:     "eastmoney",
		Secid:      spec.Eastmoney(),
		Symbol:     spec.Tencent(),
		Market:     fmt.Sprint(data["f107"]),
		Code:       fmt.Sprint(data["f57"]),
		Name:       fmt.Sprint(data["f58"]),
		TradeStatus: data["f292"],
		TradeTime:   fmt.Sprint(data["f86"]),
		CapturedAt:  time.Now().Unix(),
		PriceDecimals: data["f59"],
		PercentDecs:   data["f152"],
		Quote: Quote{
			Latest:        scaleDec(data["f43"], decs),
			Average:       scaleDec(data["f71"], decs),
			ChangeAmount:  scaleDec(data["f169"], decs),
			ChangePercent: scalePct(data["f170"], pctDecs),
			Open:          scaleDec(data["f46"], decs),
			PreviousClose: scaleDec(data["f60"], decs),
			High:          scaleDec(data["f44"], decs),
			Low:           scaleDec(data["f45"], decs),
			LimitUp:       scaleDec(data["f51"], decs),
			LimitDown:     scaleDec(data["f52"], decs),
		},
		VolumeAmount: VolumeAmount{
			VolumeHand:          getInt(data, "f47"),
			AmountYuan:          getFloat(data, "f48"),
			TurnoverRatePercent: scalePct(data["f168"], pctDecs),
			VolumeRatio:         scalePct(data["f50"], pctDecs),
			AmplitudePercent:    scalePct(data["f171"], pctDecs),
			OuterVolumeHand:     getInt(data, "f49"),
			InnerVolumeHand:     getInt(data, "f161"),
			CurrentHand:         curAbs,
			CurrentHandSide:     curSide,
		},
		Valuation: Valuation{
			TotalShares:    getFloat(data, "f84"),
			FloatShares:    getFloat(data, "f85"),
			TotalMarketCap: getFloat(data, "f116"),
			FloatMarketCap: getFloat(data, "f117"),
			PETTM:          scalePct(data["f164"], pctDecs),
			PB:             scalePct(data["f167"], pctDecs),
			EPSTTM:         getFloat(data, "f108"),
			NavPerShare:    getFloat(data, "f92"),
		},
		OrderBookStats: OrderBookStats{
			EntrustRatioPercent: scalePct(data["f191"], pctDecs),
			EntrustDiffHand:     getInt(data, "f192"),
		},
		OrderBook: parseOrderBook(data, decs),
	}
}

// orderBookSpec lists (level, priceKey, volumeKey, deltaKey) for asks/bids.
var (
	asksSpec = [][4]string{
		{"1", "f39", "f40", "f210"},
		{"2", "f37", "f38", "f209"},
		{"3", "f35", "f36", "f208"},
		{"4", "f33", "f34", "f207"},
		{"5", "f31", "f32", "f206"},
	}
	bidsSpec = [][4]string{
		{"1", "f19", "f20", "f211"},
		{"2", "f17", "f18", "f212"},
		{"3", "f15", "f16", "f213"},
		{"4", "f13", "f14", "f214"},
		{"5", "f11", "f12", "f215"},
	}
)

func parseOrderBook(data map[string]any, decs *int64) OrderBook {
	build := func(spec [][4]string) []OrderLevel {
		out := make([]OrderLevel, 0, 5)
		for _, row := range spec {
			lvl, _ := strconv.Atoi(row[0])
			out = append(out, OrderLevel{
				Level:       lvl,
				Price:       scaleDec(data[row[1]], decs),
				Volume:      getInt(data, row[2]),
				DeltaVolume: getInt(data, row[3]),
			})
		}
		return out
	}
	return OrderBook{Asks: build(asksSpec), Bids: build(bidsSpec)}
}

// ---- Kline -------------------------------------------------------------

var klinePeriods = map[string]string{
	"1m": "1", "5m": "5", "15m": "15", "30m": "30", "60m": "60",
	"day": "101", "week": "102", "month": "103",
}
var adjusts = map[string]string{"none": "0", "qfq": "1", "hfq": "2"}

type KlineRow struct {
	Time                string   `json:"time"`
	Open                *float64 `json:"open"`
	Close               *float64 `json:"close"`
	High                *float64 `json:"high"`
	Low                 *float64 `json:"low"`
	VolumeHand          *float64 `json:"volume_hand"`
	AmountYuan          *float64 `json:"amount_yuan"`
	AmplitudePercent    *float64 `json:"amplitude_percent"`
	ChangePercent       *float64 `json:"change_percent"`
	ChangeAmount        *float64 `json:"change_amount"`
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	Raw                 []string `json:"raw"`
}

type KlineResult struct {
	Dataset string     `json:"dataset"`
	Source  string     `json:"source"`
	Secid   string     `json:"secid"`
	Symbol  string     `json:"symbol"`
	Period  string     `json:"period"`
	Adjust  string     `json:"adjust"`
	Code    string     `json:"code,omitempty"`
	Name    string     `json:"name,omitempty"`
	Count   int        `json:"count"`
	Rows    []KlineRow `json:"rows"`
}

func (c *Client) Kline(ctx context.Context, spec symbol.Spec, period, adjust string, limit int) (*KlineResult, error) {
	remotePeriod, ok := klinePeriods[period]
	if !ok {
		return nil, fmt.Errorf("invalid period: %s", period)
	}
	if adjust == "" {
		adjust = "qfq"
	}
	remoteAdjust, ok := adjusts[adjust]
	if !ok {
		return nil, fmt.Errorf("invalid adjust: %s", adjust)
	}
	if limit <= 0 {
		limit = 100
	}
	q := url.Values{
		"fields1": {"f1,f2,f3,f4,f5,f6"},
		"fields2": {"f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"},
		"secid":   {spec.Eastmoney()},
		"ut":      {utQuote},
		"klt":     {remotePeriod},
		"fqt":     {remoteAdjust},
		"lmt":     {strconv.Itoa(limit)},
		"end":     {"20500101"},
	}
	body, err := c.fetch(ctx, "push2his.eastmoney.com", "/api/qt/stock/kline/get", q, quoteReferer(spec))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Code   string   `json:"code"`
			Name   string   `json:"name"`
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &KlineResult{
		Dataset: "kline", Source: "eastmoney", Secid: spec.Eastmoney(), Symbol: spec.Tencent(),
		Period: period, Adjust: adjust, Code: payload.Data.Code, Name: payload.Data.Name,
	}
	for _, line := range payload.Data.Klines {
		parts := strings.Split(line, ",")
		pad := append(parts, make([]string, 11-len(parts))...)
		out.Rows = append(out.Rows, KlineRow{
			Time:                pad[0],
			Open:                parseFloatPtr(pad[1]),
			Close:               parseFloatPtr(pad[2]),
			High:                parseFloatPtr(pad[3]),
			Low:                 parseFloatPtr(pad[4]),
			VolumeHand:          parseFloatPtr(pad[5]),
			AmountYuan:          parseFloatPtr(pad[6]),
			AmplitudePercent:    parseFloatPtr(pad[7]),
			ChangePercent:       parseFloatPtr(pad[8]),
			ChangeAmount:        parseFloatPtr(pad[9]),
			TurnoverRatePercent: parseFloatPtr(pad[10]),
			Raw:                 parts,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

func (c *Client) fallbackSnapshotFromKline(ctx context.Context, spec symbol.Spec) (*Snapshot, error) {
	kl, err := c.Kline(ctx, spec, "day", "qfq", 2)
	if err != nil {
		return nil, err
	}
	if len(kl.Rows) == 0 {
		return nil, errors.New("fallback kline empty")
	}
	row := kl.Rows[len(kl.Rows)-1]
	var prevClose *float64
	if row.Close != nil && row.ChangeAmount != nil {
		v := *row.Close - *row.ChangeAmount
		prevClose = &v
	}
	return &Snapshot{
		Dataset: "snapshot", Source: "eastmoney",
		Secid: spec.Eastmoney(), Symbol: spec.Tencent(),
		Code: kl.Code, Name: kl.Name, CapturedAt: time.Now().Unix(),
		Quote: Quote{
			Latest: row.Close, Open: row.Open, High: row.High, Low: row.Low,
			ChangeAmount: row.ChangeAmount, ChangePercent: row.ChangePercent,
			PreviousClose: prevClose,
		},
		VolumeAmount: VolumeAmount{
			VolumeHand: int64Ptr(row.VolumeHand), AmountYuan: row.AmountYuan,
			TurnoverRatePercent: row.TurnoverRatePercent, AmplitudePercent: row.AmplitudePercent,
		},
	}, nil
}

// ---- Trends ------------------------------------------------------------

type TrendRow struct {
	Time             string   `json:"time"`
	Price            *float64 `json:"price"`
	AveragePrice     *float64 `json:"average_price"`
	VolumeHand       *float64 `json:"volume_hand"`
	AmountYuan       *float64 `json:"amount_yuan"`
	Raw              []string `json:"raw"`
}

type TrendsResult struct {
	Dataset string     `json:"dataset"`
	Source  string     `json:"source"`
	Secid   string     `json:"secid"`
	Symbol  string     `json:"symbol"`
	Code    string     `json:"code,omitempty"`
	Name    string     `json:"name,omitempty"`
	Count   int        `json:"count"`
	Rows    []TrendRow `json:"rows"`
}

func (c *Client) Trends(ctx context.Context, spec symbol.Spec, days int) (*TrendsResult, error) {
	if days <= 0 {
		days = 1
	}
	q := url.Values{
		"fields1": {"f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13,f17"},
		"fields2": {"f51,f52,f53,f54,f55,f56,f57,f58"},
		"secid":   {spec.Eastmoney()},
		"ndays":   {strconv.Itoa(days)},
		"iscr":    {"0"},
		"iscca":   {"0"},
		"ut":      {utQuote},
		"wbp2u":   {pageWbp2u},
	}
	body, err := c.fetch(ctx, "push2his.eastmoney.com", "/api/qt/stock/trends2/get", q, quoteReferer(spec))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Code   string   `json:"code"`
			Name   string   `json:"name"`
			Trends []string `json:"trends"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &TrendsResult{
		Dataset: "trends", Source: "eastmoney",
		Secid: spec.Eastmoney(), Symbol: spec.Tencent(),
		Code: payload.Data.Code, Name: payload.Data.Name,
	}
	for _, line := range payload.Data.Trends {
		parts := strings.Split(line, ",")
		pad := append(parts, make([]string, 8-len(parts))...)
		out.Rows = append(out.Rows, TrendRow{
			Time:         pad[0],
			Price:        parseFloatPtr(pad[1]),
			AveragePrice: parseFloatPtr(pad[2]),
			VolumeHand:   parseFloatPtr(pad[3]),
			AmountYuan:   parseFloatPtr(pad[4]),
			Raw:          parts,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

// ---- Flow --------------------------------------------------------------

type FlowKlineRow struct {
	Time                     string   `json:"time"`
	MainNetAmount            *float64 `json:"main_net_amount"`
	SmallNetAmount           *float64 `json:"small_net_amount"`
	MediumNetAmount          *float64 `json:"medium_net_amount"`
	BigNetAmount             *float64 `json:"big_net_amount"`
	SuperBigNetAmount        *float64 `json:"super_big_net_amount"`
	MainNetRatioPercent      *float64 `json:"main_net_ratio_percent"`
	SmallNetRatioPercent     *float64 `json:"small_net_ratio_percent"`
	MediumNetRatioPercent    *float64 `json:"medium_net_ratio_percent"`
	BigNetRatioPercent       *float64 `json:"big_net_ratio_percent"`
	SuperBigNetRatioPercent  *float64 `json:"super_big_net_ratio_percent"`
	Close                    *float64 `json:"close,omitempty"`
	ChangePercent            *float64 `json:"change_percent,omitempty"`
	Raw                      []string `json:"raw"`
}

type FlowKlineResult struct {
	Dataset string         `json:"dataset"`
	Source  string         `json:"source"`
	Secid   string         `json:"secid"`
	Symbol  string         `json:"symbol"`
	Mode    string         `json:"mode"`
	Count   int            `json:"count"`
	Rows    []FlowKlineRow `json:"rows"`
}

func (c *Client) flowKline(ctx context.Context, spec symbol.Spec, mode string, limit int) (*FlowKlineResult, error) {
	var base, klt string
	switch mode {
	case "intraday":
		base = "push2.eastmoney.com"
		klt = "1"
	case "daily":
		base = "push2his.eastmoney.com"
		klt = "101"
	default:
		return nil, fmt.Errorf("invalid flow mode: %s", mode)
	}
	if limit < 0 {
		limit = 0
	}
	q := url.Values{
		"lmt":     {strconv.Itoa(limit)},
		"klt":     {klt},
		"fields1": {"f1,f2,f3,f7"},
		"fields2": {"f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65"},
		"secid":   {spec.Eastmoney()},
		"ut":      {utFlow},
	}
	path := "/api/qt/stock/fflow/kline/get"
	if mode == "daily" {
		path = "/api/qt/stock/fflow/daykline/get"
	}
	body, err := c.fetch(ctx, base, path, q, flowReferer(spec))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Code   string   `json:"code"`
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &FlowKlineResult{
		Dataset: "flow_" + mode, Source: "eastmoney",
		Secid: spec.Eastmoney(), Symbol: spec.Tencent(), Mode: mode,
	}
	for _, line := range payload.Data.Klines {
		parts := strings.Split(line, ",")
		pad := append(parts, make([]string, 15-len(parts))...)
		out.Rows = append(out.Rows, FlowKlineRow{
			Time:                    pad[0],
			MainNetAmount:           parseFloatPtr(pad[1]),
			SmallNetAmount:          parseFloatPtr(pad[2]),
			MediumNetAmount:         parseFloatPtr(pad[3]),
			BigNetAmount:            parseFloatPtr(pad[4]),
			SuperBigNetAmount:       parseFloatPtr(pad[5]),
			MainNetRatioPercent:     parseFloatPtr(pad[6]),
			SmallNetRatioPercent:    parseFloatPtr(pad[7]),
			MediumNetRatioPercent:   parseFloatPtr(pad[8]),
			BigNetRatioPercent:      parseFloatPtr(pad[9]),
			SuperBigNetRatioPercent: parseFloatPtr(pad[10]),
			Close:                   parseFloatPtr(pad[11]),
			ChangePercent:           parseFloatPtr(pad[12]),
			Raw:                     parts,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

func (c *Client) FlowIntraday(ctx context.Context, spec symbol.Spec, limit int) (*FlowKlineResult, error) {
	return c.flowKline(ctx, spec, "intraday", limit)
}
func (c *Client) FlowDaily(ctx context.Context, spec symbol.Spec, limit int) (*FlowKlineResult, error) {
	return c.flowKline(ctx, spec, "daily", limit)
}

type FlowSnapshot struct {
	Dataset    string         `json:"dataset"`
	Source     string         `json:"source"`
	Secid      string         `json:"secid"`
	Symbol     string         `json:"symbol"`
	Code       string         `json:"code,omitempty"`
	Name       string         `json:"name,omitempty"`
	AmountYuan *float64       `json:"amount_yuan,omitempty"`
	Timestamp  any            `json:"timestamp,omitempty"`
	Net        FlowNet        `json:"net"`
	InOut      FlowInOut      `json:"in_out"`
	Raw        map[string]any `json:"raw,omitempty"`
}

type FlowNet struct {
	MainNetAmount           *float64 `json:"main_net_amount"`
	MainNetRatioPercent     *float64 `json:"main_net_ratio_percent"`
	SuperBigNetAmount       *float64 `json:"super_big_net_amount"`
	SuperBigNetRatioPercent *float64 `json:"super_big_net_ratio_percent"`
	BigNetAmount            *float64 `json:"big_net_amount"`
	BigNetRatioPercent      *float64 `json:"big_net_ratio_percent"`
	MediumNetAmount         *float64 `json:"medium_net_amount"`
	MediumNetRatioPercent   *float64 `json:"medium_net_ratio_percent"`
	SmallNetAmount          *float64 `json:"small_net_amount"`
	SmallNetRatioPercent    *float64 `json:"small_net_ratio_percent"`
}

type FlowInOut struct {
	SuperBigIn  *float64 `json:"super_big_in"`
	SuperBigOut *float64 `json:"super_big_out"`
	BigIn       *float64 `json:"big_in"`
	BigOut      *float64 `json:"big_out"`
	MediumIn    *float64 `json:"medium_in"`
	MediumOut   *float64 `json:"medium_out"`
	SmallIn     *float64 `json:"small_in"`
	SmallOut    *float64 `json:"small_out"`
}

func (c *Client) FlowSnapshot(ctx context.Context, spec symbol.Spec) (*FlowSnapshot, error) {
	q := url.Values{
		"fltt":    {"2"},
		"secids":  {spec.Eastmoney()},
		"fields":  {"f12,f13,f14,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f64,f65,f70,f71,f76,f77,f82,f83,f124,f6"},
		"ut":      {utFlow},
	}
	body, err := c.fetch(ctx, "push2.eastmoney.com", "/api/qt/ulist.np/get", q, flowReferer(spec))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Diff []map[string]any `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data.Diff) == 0 {
		return nil, errors.New("empty flow snapshot")
	}
	data := payload.Data.Diff[0]
	return &FlowSnapshot{
		Dataset:    "flow_snapshot",
		Source:     "eastmoney",
		Secid:      spec.Eastmoney(),
		Symbol:     spec.Tencent(),
		Code:       fmt.Sprint(data["f12"]),
		Name:       fmt.Sprint(data["f14"]),
		AmountYuan: getFloat(data, "f6"),
		Timestamp:  data["f124"],
		Net: FlowNet{
			MainNetAmount:           getFloat(data, "f62"),
			MainNetRatioPercent:     getFloat(data, "f184"),
			SuperBigNetAmount:       getFloat(data, "f66"),
			SuperBigNetRatioPercent: getFloat(data, "f69"),
			BigNetAmount:            getFloat(data, "f72"),
			BigNetRatioPercent:      getFloat(data, "f75"),
			MediumNetAmount:         getFloat(data, "f78"),
			MediumNetRatioPercent:   getFloat(data, "f81"),
			SmallNetAmount:          getFloat(data, "f84"),
			SmallNetRatioPercent:    getFloat(data, "f87"),
		},
		InOut: FlowInOut{
			SuperBigIn:  getFloat(data, "f64"),
			SuperBigOut: getFloat(data, "f65"),
			BigIn:       getFloat(data, "f70"),
			BigOut:      getFloat(data, "f71"),
			MediumIn:    getFloat(data, "f76"),
			MediumOut:   getFloat(data, "f77"),
			SmallIn:     getFloat(data, "f82"),
			SmallOut:    getFloat(data, "f83"),
		},
	}, nil
}

// ---- helpers -----------------------------------------------------------

func summarizeErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 400 {
		s = s[:400]
	}
	return s
}

func parseFloatPtr(s string) *float64 {
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

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func getFloat(m map[string]any, key string) *float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if s, isStr := v.(string); isStr && (s == "" || s == "-") {
		return nil
	}
	f, ok := toFloat(v)
	if !ok {
		return nil
	}
	return &f
}

func getInt(m map[string]any, key string) *int64 {
	f := getFloat(m, key)
	if f == nil {
		return nil
	}
	v := int64(*f)
	return &v
}

// scaleDec divides by 10^decimals if the raw value is already an integer
// counter (the eastmoney "fltt=1" mode multiplies prices by 10^decimals).
func scaleDec(v any, decimals *int64) *float64 {
	if v == nil {
		return nil
	}
	f, ok := toFloat(v)
	if !ok {
		return nil
	}
	if decimals == nil || *decimals <= 0 {
		return &f
	}
	threshold := pow10(int(*decimals))
	if absFloat(f) >= threshold {
		out := f / threshold
		return &out
	}
	return &f
}

// scalePct divides by 10^pctDecimals when the field is a scaled percentage.
func scalePct(v any, decs *int64) *float64 {
	if v == nil {
		return nil
	}
	f, ok := toFloat(v)
	if !ok {
		return nil
	}
	if decs == nil || *decs <= 0 {
		return &f
	}
	out := f / pow10(int(*decs))
	return &out
}

func pow10(n int) float64 {
	if n <= 0 {
		return 1
	}
	v := 1.0
	for i := 0; i < n; i++ {
		v *= 10
	}
	return v
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func int64Ptr(f *float64) *int64 {
	if f == nil {
		return nil
	}
	v := int64(*f)
	return &v
}

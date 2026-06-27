// Package sohu wraps the two long-lived Sohu finance JSON endpoints:
//
//	hqm.stock.sohu.com/getqjson   real-time snapshot (no level-2 depth)
//	q.stock.sohu.com/hisHq        day/week/month historical K-line (JSONP)
//
// Both endpoints emit GB18030-encoded payloads.
//
// Sohu's level-2 / tick / fund-flow APIs were retired years ago (every
// candidate URL returns 404 in production). Sohu therefore acts as the third
// fallback source after tencent (primary) and eastmoney (secondary) for
// the limited subset above.
package sohu

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

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"stock/cmd/market-api/internal/symbol"
)

const (
	snapshotURL = "https://hqm.stock.sohu.com/getqjson?code=%s"
	klineURL    = "https://q.stock.sohu.com/hisHq?code=%s&start=%s&end=%s&stat=1&order=D&period=%s&rt=jsonp&r=%d"
	userAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

var (
	jsonpRe = regexp.MustCompile(`(?s)^[\w$.]+\((.*)\)\s*;?\s*$`)
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

func (c *Client) get(ctx context.Context, rawURL, referer string, gb18030 bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Referer", referer)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var r io.Reader = resp.Body
	if gb18030 {
		r = transform.NewReader(resp.Body, simplifiedchinese.GB18030.NewDecoder())
	}
	return io.ReadAll(r)
}

// ---- Snapshot ----------------------------------------------------------

// Snapshot covers the fields returned by hqm.stock.sohu.com/getqjson.
// Order in the raw array (verified 2026-05-11):
//
//	0  code (cn_xxxxxx)
//	1  name
//	2  latest price
//	3  change percent string (e.g. "6.11%")
//	4  change amount string (e.g. "+54.13")
//	5  volume in hands (or shares for indexes)
//	6  status flag (0/1/2)
//	7  amount in thousand yuan
//	8  turnover rate string ("2.91%")
//	9  volume ratio
//	10 high
//	11 low
//	12 amplitude proxy
//	13 previous close
//	14 open
//	15 desktop URL path
//	16 total market cap (in 100M yuan, may be empty for ETFs)
//	17 trade time
//	18 mobile URL path
type Snapshot struct {
	Dataset       string       `json:"dataset"`
	Source        string       `json:"source"`
	Symbol        string       `json:"symbol"`
	SohuCode      string       `json:"sohu_code"`
	Name          string       `json:"name,omitempty"`
	TradeTime     string       `json:"trade_time,omitempty"`
	CapturedAt    int64        `json:"captured_at"`
	Quote         SnapshotQuote `json:"quote"`
	VolumeAmount  SnapshotVA    `json:"volume_amount"`
	Valuation     SnapshotVal   `json:"valuation"`
	Raw           []string      `json:"raw"`
}

type SnapshotQuote struct {
	Latest        *float64 `json:"latest"`
	ChangeAmount  *float64 `json:"change_amount"`
	ChangePercent *float64 `json:"change_percent"`
	Open          *float64 `json:"open"`
	PreviousClose *float64 `json:"previous_close"`
	High          *float64 `json:"high"`
	Low           *float64 `json:"low"`
}

type SnapshotVA struct {
	VolumeHand          *int64   `json:"volume_hand"`
	AmountYuan          *float64 `json:"amount_yuan"`
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	VolumeRatio         *float64 `json:"volume_ratio"`
}

type SnapshotVal struct {
	// TotalMarketCapYuan is reconstructed from sohu's 亿元 column.
	TotalMarketCapYuan *float64 `json:"total_market_cap_yuan"`
}

// Snapshot returns a single security quote.
func (c *Client) Snapshot(ctx context.Context, spec symbol.Spec) (*Snapshot, error) {
	body, err := c.get(ctx, fmt.Sprintf(snapshotURL, spec.Sohu()), "https://q.stock.sohu.com/", true)
	if err != nil {
		return nil, err
	}
	parsed, err := parseSnapshotPayload(spec, body)
	if err != nil {
		return nil, err
	}
	if len(parsed) == 0 {
		return nil, errors.New("sohu snapshot empty")
	}
	return parsed[0], nil
}

// SnapshotBatch fetches multiple securities in a single request. Sohu accepts
// a comma-separated codes list.
func (c *Client) SnapshotBatch(ctx context.Context, specs []symbol.Spec) ([]*Snapshot, error) {
	if len(specs) == 0 {
		return nil, errors.New("symbols required")
	}
	codes := make([]string, 0, len(specs))
	for _, s := range specs {
		codes = append(codes, s.Sohu())
	}
	body, err := c.get(ctx, fmt.Sprintf(snapshotURL, strings.Join(codes, ",")), "https://q.stock.sohu.com/", true)
	if err != nil {
		return nil, err
	}
	// Build a lookup by sohu code so the output order matches the input.
	rows, err := parseSnapshotPayloadMap(body)
	if err != nil {
		return nil, err
	}
	out := make([]*Snapshot, 0, len(specs))
	for _, s := range specs {
		if r, ok := rows[s.Sohu()]; ok {
			snap := buildSnapshot(s, r)
			out = append(out, snap)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("sohu snapshot batch: no rows matched")
	}
	return out, nil
}

func parseSnapshotPayload(spec symbol.Spec, body []byte) ([]*Snapshot, error) {
	rows, err := parseSnapshotPayloadMap(body)
	if err != nil {
		return nil, err
	}
	if r, ok := rows[spec.Sohu()]; ok {
		return []*Snapshot{buildSnapshot(spec, r)}, nil
	}
	// Sohu sometimes echoes the code without prefix when the symbol is unknown.
	for _, r := range rows {
		if len(r) > 0 && r[0] == spec.Sohu() {
			return []*Snapshot{buildSnapshot(spec, r)}, nil
		}
	}
	return nil, fmt.Errorf("sohu snapshot: no row for %s", spec.Sohu())
}

func parseSnapshotPayloadMap(body []byte) (map[string][]string, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, errors.New("sohu snapshot empty body")
	}
	// Endpoint returns either pure JSON or jsonp-wrapped JSON.
	if m := jsonpRe.FindStringSubmatch(text); len(m) == 2 {
		text = m[1]
	}
	var raw map[string][]any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("sohu snapshot decode: %w; head=%.120s", err, text)
	}
	out := make(map[string][]string, len(raw))
	for k, arr := range raw {
		row := make([]string, 0, len(arr))
		for _, v := range arr {
			switch x := v.(type) {
			case string:
				row = append(row, x)
			case nil:
				row = append(row, "")
			default:
				row = append(row, fmt.Sprint(x))
			}
		}
		out[k] = row
	}
	return out, nil
}

func buildSnapshot(spec symbol.Spec, r []string) *Snapshot {
	pad := append([]string(nil), r...)
	for len(pad) < 19 {
		pad = append(pad, "")
	}
	totalMcap := percentToFloat(pad[16])
	var totalMcapYuan *float64
	if totalMcap != nil {
		v := *totalMcap * 1e8 // sohu reports 亿元
		totalMcapYuan = &v
	}
	amount := parseFloatPtr(pad[7])
	if amount != nil {
		v := *amount * 1000 // sohu reports 千元
		amount = &v
	}
	return &Snapshot{
		Dataset:    "snapshot",
		Source:     "sohu",
		Symbol:     spec.Tencent(),
		SohuCode:   spec.Sohu(),
		Name:       pad[1],
		TradeTime:  pad[17],
		CapturedAt: time.Now().Unix(),
		Quote: SnapshotQuote{
			Latest:        parseFloatPtr(pad[2]),
			ChangePercent: percentToFloat(pad[3]),
			ChangeAmount:  parseFloatPtr(strings.TrimPrefix(pad[4], "+")),
			High:          parseFloatPtr(pad[10]),
			Low:           parseFloatPtr(pad[11]),
			PreviousClose: parseFloatPtr(pad[13]),
			Open:          parseFloatPtr(pad[14]),
		},
		VolumeAmount: SnapshotVA{
			VolumeHand:          parseInt64Ptr(pad[5]),
			AmountYuan:          amount,
			TurnoverRatePercent: percentToFloat(pad[8]),
			VolumeRatio:         parseFloatPtr(pad[9]),
		},
		Valuation: SnapshotVal{TotalMarketCapYuan: totalMcapYuan},
		Raw:       pad,
	}
}

// ---- Kline -------------------------------------------------------------

// KlineRow keeps the historical fields returned by hisHq.
type KlineRow struct {
	Time                string   `json:"time"`
	Open                *float64 `json:"open"`
	Close               *float64 `json:"close"`
	ChangeAmount        *float64 `json:"change_amount"`
	ChangePercent       *float64 `json:"change_percent"`
	Low                 *float64 `json:"low"`
	High                *float64 `json:"high"`
	VolumeHand          *float64 `json:"volume_hand"`
	AmountYuan          *float64 `json:"amount_yuan"`
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	Extension           string   `json:"extension,omitempty"`
	Raw                 []string `json:"raw"`
}

type KlineResult struct {
	Dataset string     `json:"dataset"`
	Source  string     `json:"source"`
	Symbol  string     `json:"symbol"`
	SohuCode string    `json:"sohu_code"`
	Period  string     `json:"period"`
	Count   int        `json:"count"`
	Rows    []KlineRow `json:"rows"`
}

var sohuPeriods = map[string]string{"day": "d", "week": "w", "month": "m"}

// Kline returns the historical OHLC series. Period must be one of day/week/month.
// Sohu disabled the intraday periods (1/5/15/30/60 minutes); use tencent for those.
func (c *Client) Kline(ctx context.Context, spec symbol.Spec, period, begin, end string, limit int) (*KlineResult, error) {
	p, ok := sohuPeriods[period]
	if !ok {
		return nil, fmt.Errorf("sohu kline period unsupported: %s (only day/week/month)", period)
	}
	if end == "" {
		end = time.Now().Format("20060102")
	}
	if begin == "" {
		begin = "19900101"
	}
	rawURL := fmt.Sprintf(klineURL, spec.Sohu(), begin, end, p, time.Now().UnixNano()%1_000_000_000)
	body, err := c.get(ctx, rawURL, "https://q.stock.sohu.com/", true)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(body))
	if m := jsonpRe.FindStringSubmatch(text); len(m) == 2 {
		text = m[1]
	}
	var payload []struct {
		Status int        `json:"status"`
		Msg    string     `json:"msg,omitempty"`
		Code   string     `json:"code,omitempty"`
		Hq     [][]string `json:"hq"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, fmt.Errorf("sohu kline decode: %w; head=%.140s", err, text)
	}
	if len(payload) == 0 {
		return nil, errors.New("sohu kline: empty payload")
	}
	if payload[0].Status != 0 {
		return nil, fmt.Errorf("sohu kline status=%d msg=%s", payload[0].Status, payload[0].Msg)
	}
	hq := payload[0].Hq
	// hisHq returns rows newest-first by default; sort caller-friendly: oldest-first.
	rows := make([]KlineRow, 0, len(hq))
	for i := len(hq) - 1; i >= 0; i-- {
		row := hq[i]
		pad := append([]string(nil), row...)
		for len(pad) < 11 {
			pad = append(pad, "")
		}
		rows = append(rows, KlineRow{
			Time:                pad[0],
			Open:                parseFloatPtr(pad[1]),
			Close:               parseFloatPtr(pad[2]),
			ChangeAmount:        parseFloatPtr(pad[3]),
			ChangePercent:       percentToFloat(pad[4]),
			Low:                 parseFloatPtr(pad[5]),
			High:                parseFloatPtr(pad[6]),
			VolumeHand:          parseFloatPtr(pad[7]),
			AmountYuan:          scaleAmount(pad[8]), // raw is 万元
			TurnoverRatePercent: percentToFloat(pad[9]),
			Extension:           pad[10],
			Raw:                 row,
		})
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return &KlineResult{
		Dataset:  "kline",
		Source:   "sohu",
		Symbol:   spec.Tencent(),
		SohuCode: spec.Sohu(),
		Period:   period,
		Count:    len(rows),
		Rows:     rows,
	}, nil
}

// ---- helpers -----------------------------------------------------------

func parseFloatPtr(s string) *float64 {
	s = strings.TrimSpace(strings.Trim(s, "\""))
	if s == "" || s == "-" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseInt64Ptr(s string) *int64 {
	f := parseFloatPtr(s)
	if f == nil {
		return nil
	}
	v := int64(*f)
	return &v
}

// percentToFloat strips trailing "%" and parses; "6.11%" -> 6.11.
func percentToFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimPrefix(s, "+")
	return parseFloatPtr(s)
}

// scaleAmount converts sohu's "amount in 万元" string into raw yuan.
func scaleAmount(s string) *float64 {
	v := parseFloatPtr(s)
	if v == nil {
		return nil
	}
	out := *v * 10000
	return &out
}

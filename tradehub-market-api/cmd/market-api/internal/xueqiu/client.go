package xueqiu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"stock/cmd/market-api/internal/symbol"
)

const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	quoteURL  = "https://stock.xueqiu.com/v5/stock/quote.json"
	klineURL  = "https://stock.xueqiu.com/v5/stock/chart/kline.json"
)

type Client struct {
	http          *http.Client
	defaultCookie string
}

func (c *Client) HasDefaultCookie() bool {
	return strings.TrimSpace(c.defaultCookie) != ""
}

func New(timeout time.Duration, defaultCookie string) *Client {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &Client{
		http:          &http.Client{Timeout: timeout, Transport: transport},
		defaultCookie: strings.TrimSpace(defaultCookie),
	}
}

type authError struct {
	msg string
}

func (e *authError) Error() string { return e.msg }

func (c *Client) resolveCookie(override string) (string, error) {
	cookie := strings.TrimSpace(override)
	if cookie == "" {
		cookie = c.defaultCookie
	}
	if cookie == "" {
		return "", &authError{msg: "xueqiu cookie is required: set env XUEQIU_COOKIE or header X-Xueqiu-Cookie"}
	}
	return cookie, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, referer string, cookie string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://xueqiu.com")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %.160s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode xueqiu: %w; head=%.200s", err, strings.TrimSpace(string(body)))
	}
	return nil
}

func ensureSupported(spec symbol.Spec) error {
	if spec.Market != "sh" && spec.Market != "sz" && spec.Market != "bj" {
		return fmt.Errorf("xueqiu unsupported market: %s", spec.Market)
	}
	return nil
}

type Snapshot struct {
	Dataset      string      `json:"dataset"`
	Source       string      `json:"source"`
	Symbol       string      `json:"symbol"`
	XQSymbol     string      `json:"xq_symbol"`
	Name         string      `json:"name,omitempty"`
	TradeTime    string      `json:"trade_time,omitempty"`
	CapturedAt   int64       `json:"captured_at"`
	Quote        Quote       `json:"quote"`
	VolumeAmount Volume      `json:"volume_amount"`
	Valuation    Valuation   `json:"valuation"`
	Meta         Meta        `json:"meta"`
	Raw          interface{} `json:"raw,omitempty"`
}

type Quote struct {
	Latest        *float64 `json:"latest"`
	ChangeAmount  *float64 `json:"change_amount"`
	ChangePercent *float64 `json:"change_percent"`
	Open          *float64 `json:"open"`
	PreviousClose *float64 `json:"previous_close"`
	High          *float64 `json:"high"`
	Low           *float64 `json:"low"`
	LimitUp       *float64 `json:"limit_up"`
	LimitDown     *float64 `json:"limit_down"`
}

type Volume struct {
	VolumeShare         *float64 `json:"volume_share"`
	AmountYuan          *float64 `json:"amount_yuan"`
	TurnoverRatePercent *float64 `json:"turnover_rate_percent"`
	VolumeRatio         *float64 `json:"volume_ratio"`
	FloatShares         *float64 `json:"float_shares"`
	TotalShares         *float64 `json:"total_shares"`
	FloatMarketCapYuan  *float64 `json:"float_market_cap_yuan"`
	TotalMarketCapYuan  *float64 `json:"total_market_cap_yuan"`
}

type Valuation struct {
	PE *float64 `json:"pe_ttm"`
	PB *float64 `json:"pb"`
}

type Meta struct {
	StatusCode  int      `json:"status_code,omitempty"`
	Exchange    string   `json:"exchange,omitempty"`
	Type        int64    `json:"type,omitempty"`
	CurrentYear *float64 `json:"current_year_percent,omitempty"`
}

type quoteResp struct {
	ErrorCode        string `json:"error_code"`
	ErrorDescription string `json:"error_description"`
	Data             struct {
		Quote struct {
			Symbol             string   `json:"symbol"`
			Name               string   `json:"name"`
			Timestamp          int64    `json:"timestamp"`
			Current            *float64 `json:"current"`
			Percent            *float64 `json:"percent"`
			Change             *float64 `json:"chg"`
			Open               *float64 `json:"open"`
			LastClose          *float64 `json:"last_close"`
			High               *float64 `json:"high"`
			Low                *float64 `json:"low"`
			LimitUp            *float64 `json:"limit_up"`
			LimitDown          *float64 `json:"limit_down"`
			Volume             *float64 `json:"volume"`
			Amount             *float64 `json:"amount"`
			TurnoverRate       *float64 `json:"turnover_rate"`
			VolumeRatio        *float64 `json:"volume_ratio"`
			FloatShares        *float64 `json:"float_shares"`
			TotalShares        *float64 `json:"total_shares"`
			FloatMarketCapital *float64 `json:"float_market_capital"`
			MarketCapital      *float64 `json:"market_capital"`
			PE                 *float64 `json:"pe_ttm"`
			PB                 *float64 `json:"pb"`
			Exchange           string   `json:"exchange"`
			Type               int64    `json:"type"`
			CurrentYearPercent *float64 `json:"current_year_percent"`
		} `json:"quote"`
	} `json:"data"`
}

func (c *Client) Snapshot(ctx context.Context, spec symbol.Spec, cookieOverride string) (*Snapshot, error) {
	if err := ensureSupported(spec); err != nil {
		return nil, err
	}
	cookie, err := c.resolveCookie(cookieOverride)
	if err != nil {
		return nil, err
	}
	xq := spec.Xueqiu()
	q := url.Values{}
	q.Set("symbol", xq)
	q.Set("extend", "detail")
	rawURL := quoteURL + "?" + q.Encode()
	var resp quoteResp
	if err := c.getJSON(ctx, rawURL, "https://xueqiu.com/S/"+xq, cookie, &resp); err != nil {
		return nil, err
	}
	if resp.ErrorCode != "" {
		return nil, &authError{msg: fmt.Sprintf("xueqiu upstream error %s: %s", resp.ErrorCode, strings.TrimSpace(resp.ErrorDescription))}
	}
	quote := resp.Data.Quote
	out := &Snapshot{
		Dataset:    "snapshot",
		Source:     "xueqiu",
		Symbol:     spec.Tencent(),
		XQSymbol:   xq,
		Name:       quote.Name,
		CapturedAt: time.Now().Unix(),
		Quote: Quote{
			Latest:        quote.Current,
			ChangeAmount:  quote.Change,
			ChangePercent: quote.Percent,
			Open:          quote.Open,
			PreviousClose: quote.LastClose,
			High:          quote.High,
			Low:           quote.Low,
			LimitUp:       quote.LimitUp,
			LimitDown:     quote.LimitDown,
		},
		VolumeAmount: Volume{
			VolumeShare:         quote.Volume,
			AmountYuan:          quote.Amount,
			TurnoverRatePercent: quote.TurnoverRate,
			VolumeRatio:         quote.VolumeRatio,
			FloatShares:         quote.FloatShares,
			TotalShares:         quote.TotalShares,
			FloatMarketCapYuan:  quote.FloatMarketCapital,
			TotalMarketCapYuan:  quote.MarketCapital,
		},
		Valuation: Valuation{
			PE: quote.PE,
			PB: quote.PB,
		},
		Meta: Meta{
			Exchange:    quote.Exchange,
			Type:        quote.Type,
			CurrentYear: quote.CurrentYearPercent,
		},
		Raw: quote,
	}
	if quote.Timestamp > 0 {
		out.TradeTime = time.UnixMilli(quote.Timestamp).Format("2006-01-02 15:04:05")
	}
	return out, nil
}

type KlineRow struct {
	Time       string   `json:"time"`
	Open       *float64 `json:"open"`
	High       *float64 `json:"high"`
	Low        *float64 `json:"low"`
	Close      *float64 `json:"close"`
	Volume     *float64 `json:"volume"`
	AmountYuan *float64 `json:"amount_yuan"`
	Change     *float64 `json:"change"`
	Percent    *float64 `json:"percent"`
	Turnover   *float64 `json:"turnover_rate_percent"`
}

type KlineResult struct {
	Dataset   string     `json:"dataset"`
	Source    string     `json:"source"`
	Symbol    string     `json:"symbol"`
	XQSymbol  string     `json:"xq_symbol"`
	Period    string     `json:"period"`
	Adjust    string     `json:"adjust"`
	Count     int        `json:"count"`
	Rows      []KlineRow `json:"rows"`
	ColumnMap []string   `json:"column_map,omitempty"`
}

type klineResp struct {
	ErrorCode        string `json:"error_code"`
	ErrorDescription string `json:"error_description"`
	Data             struct {
		Column []string        `json:"column"`
		Item   [][]interface{} `json:"item"`
	} `json:"data"`
}

func normalizePeriod(period string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "", "day":
		return "day", nil
	case "week":
		return "week", nil
	case "month":
		return "month", nil
	default:
		return "", fmt.Errorf("xueqiu kline period unsupported: %s (only day/week/month)", period)
	}
}

func normalizeAdjust(adjust string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(adjust)) {
	case "", "qfq":
		return "before", nil
	case "hfq":
		return "after", nil
	case "none":
		return "normal", nil
	default:
		return "", fmt.Errorf("xueqiu kline adjust unsupported: %s", adjust)
	}
}

func (c *Client) Kline(ctx context.Context, spec symbol.Spec, period string, adjust string, limit int, cookieOverride string) (*KlineResult, error) {
	if err := ensureSupported(spec); err != nil {
		return nil, err
	}
	cookie, err := c.resolveCookie(cookieOverride)
	if err != nil {
		return nil, err
	}
	p, err := normalizePeriod(period)
	if err != nil {
		return nil, err
	}
	a, err := normalizeAdjust(adjust)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	xq := spec.Xueqiu()
	q := url.Values{}
	q.Set("symbol", xq)
	q.Set("period", p)
	q.Set("type", a)
	q.Set("count", fmt.Sprintf("-%d", limit))
	q.Set("begin", fmt.Sprintf("%d", time.Now().UnixMilli()))
	q.Set("indicator", "kline,pe,pb,ps,pcf,market_capital,agt,ggt,balance")
	rawURL := klineURL + "?" + q.Encode()
	var resp klineResp
	if err := c.getJSON(ctx, rawURL, "https://xueqiu.com/S/"+xq, cookie, &resp); err != nil {
		return nil, err
	}
	if resp.ErrorCode != "" {
		return nil, &authError{msg: fmt.Sprintf("xueqiu upstream error %s: %s", resp.ErrorCode, strings.TrimSpace(resp.ErrorDescription))}
	}
	result := &KlineResult{
		Dataset:   "kline",
		Source:    "xueqiu",
		Symbol:    spec.Tencent(),
		XQSymbol:  xq,
		Period:    p,
		Adjust:    strings.TrimSpace(adjust),
		ColumnMap: append([]string(nil), resp.Data.Column...),
	}
	if result.Adjust == "" {
		result.Adjust = "qfq"
	}
	index := make(map[string]int, len(resp.Data.Column))
	for i, name := range resp.Data.Column {
		index[name] = i
	}
	for _, row := range resp.Data.Item {
		result.Rows = append(result.Rows, KlineRow{
			Time:       asTime(row, index, "timestamp"),
			Open:       asFloat(row, index, "open"),
			High:       asFloat(row, index, "high"),
			Low:        asFloat(row, index, "low"),
			Close:      asFloat(row, index, "close"),
			Volume:     asFloat(row, index, "volume"),
			AmountYuan: asFloat(row, index, "amount"),
			Change:     asFloat(row, index, "chg"),
			Percent:    asFloat(row, index, "percent"),
			Turnover:   asFloat(row, index, "turnoverrate"),
		})
	}
	result.Count = len(result.Rows)
	return result, nil
}

func asFloat(row []interface{}, index map[string]int, key string) *float64 {
	pos, ok := index[key]
	if !ok || pos < 0 || pos >= len(row) || row[pos] == nil {
		return nil
	}
	switch v := row[pos].(type) {
	case float64:
		return &v
	case int64:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	default:
		return nil
	}
}

func asTime(row []interface{}, index map[string]int, key string) string {
	pos, ok := index[key]
	if !ok || pos < 0 || pos >= len(row) || row[pos] == nil {
		return ""
	}
	switch v := row[pos].(type) {
	case float64:
		return time.UnixMilli(int64(v)).Format("2006-01-02 15:04:05")
	case int64:
		return time.UnixMilli(v).Format("2006-01-02 15:04:05")
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func IsAuthError(err error) bool {
	var ae *authError
	return errors.As(err, &ae)
}

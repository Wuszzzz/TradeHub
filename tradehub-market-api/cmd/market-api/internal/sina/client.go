package sina

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"

type Client struct {
	http *http.Client
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Client{http: &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          64,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: timeout,
		},
	}}
}

type FinancialReportResult struct {
	Dataset string           `json:"dataset"`
	Source  string           `json:"source"`
	Symbol  string           `json:"symbol"`
	Type    string           `json:"type"`
	Count   int              `json:"count"`
	Rows    []map[string]any `json:"rows"`
}

func (c *Client) FinancialReport(ctx context.Context, code, reportType string, num int) (*FinancialReportResult, error) {
	if reportType == "" {
		reportType = "lrb"
	}
	if num <= 0 || num > 40 {
		num = 8
	}
	prefix := "sz"
	if strings.HasPrefix(code, "6") {
		prefix = "sh"
	}
	q := url.Values{
		"paperCode": {prefix + code},
		"source":    {reportType},
		"type":      {"0"},
		"page":      {"1"},
		"num":       {strconv.Itoa(num)},
	}
	body, err := c.get(ctx, "https://quotes.sina.cn/cn/api/openapi.php/CompanyFinanceService.getFinanceReport2022", q, "https://finance.sina.com.cn/")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Result struct {
			Data struct {
				ReportList map[string]struct {
					Data []map[string]any `json:"data"`
				} `json:"report_list"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(payload.Result.Data.ReportList))
	for period := range payload.Result.Data.ReportList {
		keys = append(keys, period)
	}
	sortDesc(keys)
	out := &FinancialReportResult{Dataset: "financial_report", Source: "sina", Symbol: code, Type: reportType}
	for _, period := range keys {
		if len(out.Rows) >= num {
			break
		}
		rec := map[string]any{"report_date": formatPeriod(period)}
		for _, item := range payload.Result.Data.ReportList[period].Data {
			title := strings.TrimSpace(fmt.Sprint(item["item_title"]))
			if title == "" || item["item_value"] == nil {
				continue
			}
			rec[title] = item["item_value"]
			if tb := item["item_tongbi"]; tb != nil && strings.TrimSpace(fmt.Sprint(tb)) != "" {
				rec[title+"_同比"] = tb
			}
		}
		out.Rows = append(out.Rows, rec)
	}
	out.Count = len(out.Rows)
	return out, nil
}

type OptionCodesResult struct {
	Dataset    string              `json:"dataset"`
	Source     string              `json:"source"`
	Underlying string              `json:"underlying"`
	Call       bool                `json:"call"`
	Months     map[string][]string `json:"months"`
}

func (c *Client) OptionCodes(ctx context.Context, underlying string, call bool) (*OptionCodesResult, error) {
	if underlying == "" {
		underlying = "510050"
	}
	cate := map[string]string{
		"510050": "50ETF",
		"510300": "300ETF",
		"588000": "科创50ETF",
		"510500": "500ETF",
	}[underlying]
	if cate == "" {
		cate = "50ETF"
	}
	rawURL := "https://stock.finance.sina.com.cn/futures/api/openapi.php/StockOptionService.getStockName"
	body, err := c.get(ctx, rawURL, url.Values{"exchange": {"null"}, "cate": {cate}}, "https://stock.finance.sina.com.cn/")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Result struct {
			Data struct {
				ContractMonth []string `json:"contractMonth"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	flag := "OP_UP_"
	if !call {
		flag = "OP_DOWN_"
	}
	months := map[string][]string{}
	for i, month := range payload.Result.Data.ContractMonth {
		if i == 0 {
			continue
		}
		key := strings.ReplaceAll(month, "-", "")
		if len(key) >= 6 {
			key = key[2:6]
		}
		rows, err := c.hqList(ctx, flag+underlying+key)
		if err != nil {
			continue
		}
		for _, row := range rows {
			if strings.HasPrefix(row, "CON_OP_") {
				months[key] = append(months[key], strings.TrimPrefix(row, "CON_OP_"))
			}
		}
	}
	return &OptionCodesResult{Dataset: "option_codes", Source: "sina", Underlying: underlying, Call: call, Months: months}, nil
}

func (c *Client) OptionTQuote(ctx context.Context, code string) (map[string]any, error) {
	rows, err := c.hqList(ctx, "CON_OP_"+code)
	if err != nil {
		return nil, err
	}
	if len(rows) < 43 {
		return nil, fmt.Errorf("option quote fields too short: %d", len(rows))
	}
	return map[string]any{
		"dataset": "option_tquote", "source": "sina", "code": code,
		"bid_vol": f(rows[0]), "bid": f(rows[1]), "last": f(rows[2]), "ask": f(rows[3]), "ask_vol": f(rows[4]),
		"open_interest": f(rows[5]), "pct": f(rows[6]), "strike": f(rows[7]), "prev_close": f(rows[8]),
		"open": f(rows[9]), "limit_up": f(rows[10]), "limit_down": f(rows[11]), "name": rows[37],
		"amplitude": f(rows[38]), "high": f(rows[39]), "low": f(rows[40]), "volume": f(rows[41]), "amount": f(rows[42]),
	}, nil
}

func (c *Client) OptionGreeks(ctx context.Context, code string) (map[string]any, error) {
	raw, err := c.hqList(ctx, "CON_SO_"+code)
	if err != nil {
		return nil, err
	}
	if len(raw) < 16 {
		return nil, fmt.Errorf("option greeks fields too short: %d", len(raw))
	}
	rows := append([]string{raw[0]}, raw[4:]...)
	return map[string]any{
		"dataset": "option_greeks", "source": "sina", "code": code,
		"name": rows[0], "volume": f(rows[1]), "delta": f(rows[2]), "gamma": f(rows[3]),
		"theta": f(rows[4]), "vega": f(rows[5]), "iv": f(rows[6]), "high": f(rows[7]),
		"low": f(rows[8]), "trade_code": rows[9], "strike": f(rows[10]), "last": f(rows[11]), "theory": f(rows[12]),
	}, nil
}

func (c *Client) get(ctx context.Context, rawURL string, q url.Values, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
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

func (c *Client) hqList(ctx context.Context, param string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://hq.sinajs.cn/list="+url.QueryEscape(param), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://stock.finance.sina.com.cn/")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var reader io.Reader = resp.Body
	reader = transform.NewReader(reader, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	text := string(body)
	parts := strings.Split(text, `"`)
	if len(parts) < 2 {
		return nil, fmt.Errorf("unexpected hq payload")
	}
	return strings.Split(parts[1], ","), nil
}

func f(s string) any {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return strings.TrimSpace(s)
	}
	return v
}

func sortDesc(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] > items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func formatPeriod(period string) string {
	if len(period) == 8 {
		return period[:4] + "-" + period[4:6] + "-" + period[6:8]
	}
	return period
}

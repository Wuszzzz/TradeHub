package eastmoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"stock/cmd/market-api/internal/symbol"
)

type IndustryRankRow struct {
	BoardCode           string         `json:"board_code"`
	BoardName           string         `json:"board_name"`
	ChangePercent       *float64       `json:"change_percent"`
	Latest              *float64       `json:"latest"`
	RiseCount           *int64         `json:"rise_count"`
	FallCount           *int64         `json:"fall_count"`
	LeadingStockCode    string         `json:"leading_stock_code,omitempty"`
	LeadingStockName    string         `json:"leading_stock_name,omitempty"`
	LeadingStockPercent *float64       `json:"leading_stock_percent,omitempty"`
	Raw                 map[string]any `json:"raw,omitempty"`
}

type IndustryRankResult struct {
	Dataset string            `json:"dataset"`
	Source  string            `json:"source"`
	Count   int               `json:"count"`
	Rows    []IndustryRankRow `json:"rows"`
}

func (c *Client) IndustryRank(ctx context.Context, top int) (*IndustryRankResult, error) {
	if top <= 0 || top > 500 {
		top = 100
	}
	q := url.Values{
		"pn":     {"1"},
		"pz":     {strconv.Itoa(top)},
		"po":     {"1"},
		"np":     {"1"},
		"ut":     {utQuote},
		"fltt":   {"2"},
		"invt":   {"2"},
		"fid":    {"f3"},
		"fs":     {"m:90+t:2"},
		"fields": {"f12,f14,f2,f3,f62,f104,f105,f128,f136,f140"},
	}
	body, err := c.fetch(ctx, "push2.eastmoney.com", "/api/qt/clist/get", q, "https://quote.eastmoney.com/center/boardlist.html")
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
	out := &IndustryRankResult{Dataset: "industry_rank", Source: "eastmoney"}
	for _, row := range payload.Data.Diff {
		out.Rows = append(out.Rows, IndustryRankRow{
			BoardCode:           strings.TrimSpace(fmt.Sprint(row["f12"])),
			BoardName:           strings.TrimSpace(fmt.Sprint(row["f14"])),
			Latest:              getFloat(row, "f2"),
			ChangePercent:       getFloat(row, "f3"),
			RiseCount:           getInt(row, "f104"),
			FallCount:           getInt(row, "f105"),
			LeadingStockCode:    strings.TrimSpace(fmt.Sprint(row["f128"])),
			LeadingStockName:    strings.TrimSpace(fmt.Sprint(row["f140"])),
			LeadingStockPercent: getFloat(row, "f136"),
			Raw:                 row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

type ConceptBlockRow struct {
	BoardCode           string         `json:"board_code"`
	BoardName           string         `json:"board_name"`
	BoardType           string         `json:"board_type,omitempty"`
	ChangePercent       *float64       `json:"change_percent,omitempty"`
	LeadingStockCode    string         `json:"leading_stock_code,omitempty"`
	LeadingStockName    string         `json:"leading_stock_name,omitempty"`
	LeadingStockPercent *float64       `json:"leading_stock_percent,omitempty"`
	Raw                 map[string]any `json:"raw,omitempty"`
}

type ConceptBlocksResult struct {
	Dataset     string            `json:"dataset"`
	Source      string            `json:"source"`
	Secid       string            `json:"secid"`
	Symbol      string            `json:"symbol"`
	Count       int               `json:"count"`
	ConceptTags []string          `json:"concept_tags"`
	Rows        []ConceptBlockRow `json:"rows"`
}

func (c *Client) ConceptBlocks(ctx context.Context, spec symbol.Spec) (*ConceptBlocksResult, error) {
	q := url.Values{
		"ut":     {utQuote},
		"spt":    {"3"},
		"pi":     {"0"},
		"pz":     {"200"},
		"po":     {"1"},
		"fid":    {"f3"},
		"secid":  {spec.Eastmoney()},
		"fields": {"f12,f14,f3,f128,f136,f140,f1,f2,f13"},
	}
	body, err := c.fetch(ctx, "push2.eastmoney.com", "/api/qt/slist/get", q, quoteReferer(spec))
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
	out := &ConceptBlocksResult{
		Dataset: "concept_blocks",
		Source:  "eastmoney",
		Secid:   spec.Eastmoney(),
		Symbol:  spec.Tencent(),
	}
	seen := map[string]bool{}
	for _, row := range payload.Data.Diff {
		name := strings.TrimSpace(fmt.Sprint(row["f14"]))
		code := strings.TrimSpace(fmt.Sprint(row["f12"]))
		item := ConceptBlockRow{
			BoardCode:           code,
			BoardName:           name,
			ChangePercent:       getFloat(row, "f3"),
			LeadingStockCode:    strings.TrimSpace(fmt.Sprint(row["f128"])),
			LeadingStockName:    strings.TrimSpace(fmt.Sprint(row["f140"])),
			LeadingStockPercent: getFloat(row, "f136"),
			Raw:                 row,
		}
		if strings.HasPrefix(code, "BK") {
			item.BoardType = "board"
		}
		out.Rows = append(out.Rows, item)
		if name != "" && !seen[name] {
			seen[name] = true
			out.ConceptTags = append(out.ConceptTags, name)
		}
	}
	sort.Strings(out.ConceptTags)
	out.Count = len(out.Rows)
	return out, nil
}

type ReportRow struct {
	InfoCode              string         `json:"info_code"`
	Title                 string         `json:"title"`
	PublishDate           string         `json:"publish_date,omitempty"`
	OrgName               string         `json:"org_name,omitempty"`
	Rating                string         `json:"rating,omitempty"`
	IndustryName          string         `json:"industry_name,omitempty"`
	IndustryCode          string         `json:"industry_code,omitempty"`
	PredictThisYearEPS    any            `json:"predict_this_year_eps,omitempty"`
	PredictNextYearEPS    any            `json:"predict_next_year_eps,omitempty"`
	PredictNextTwoYearEPS any            `json:"predict_next_two_year_eps,omitempty"`
	PDFURL                string         `json:"pdf_url,omitempty"`
	Raw                   map[string]any `json:"raw,omitempty"`
}

type ReportsResult struct {
	Dataset string      `json:"dataset"`
	Source  string      `json:"source"`
	QType   string      `json:"q_type"`
	Count   int         `json:"count"`
	Rows    []ReportRow `json:"rows"`
}

type DataCenterResult struct {
	Dataset    string           `json:"dataset"`
	Source     string           `json:"source"`
	Kind       string           `json:"kind"`
	ReportName string           `json:"report_name"`
	Count      int              `json:"count"`
	Rows       []map[string]any `json:"rows"`
}

func (c *Client) DataCenter(ctx context.Context, kind, code string, pageSize int) (*DataCenterResult, error) {
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 50
	}
	spec, ok := dataCenterSpec(kind, code)
	if !ok {
		return nil, fmt.Errorf("unsupported datacenter kind: %s", kind)
	}
	q := url.Values{
		"reportName":  {spec.reportName},
		"columns":     {"ALL"},
		"filter":      {spec.filter},
		"pageNumber":  {"1"},
		"pageSize":    {strconv.Itoa(pageSize)},
		"sortTypes":   {spec.sortTypes},
		"sortColumns": {spec.sortColumns},
		"source":      {"WEB"},
		"client":      {"WEB"},
	}
	body, err := c.doJSONGet(ctx, "https://datacenter-web.eastmoney.com/api/data/v1/get", q, "https://data.eastmoney.com/")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Result struct {
			Data []map[string]any `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &DataCenterResult{
		Dataset:    "datacenter",
		Source:     "eastmoney",
		Kind:       kind,
		ReportName: spec.reportName,
		Rows:       payload.Result.Data,
	}
	out.Count = len(out.Rows)
	return out, nil
}

type dataCenterQuerySpec struct {
	reportName  string
	filter      string
	sortColumns string
	sortTypes   string
}

func dataCenterSpec(kind, code string) (dataCenterQuerySpec, bool) {
	code = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(code), "sh"), "sz")
	switch strings.TrimSpace(kind) {
	case "margin", "margin_trading":
		return dataCenterQuerySpec{
			reportName:  "RPTA_WEB_RZRQ_GGMX",
			filter:      fmt.Sprintf(`(SCODE="%s")`, code),
			sortColumns: "DATE",
			sortTypes:   "-1",
		}, code != ""
	case "lockup", "lockup_expiry":
		return dataCenterQuerySpec{
			reportName:  "RPT_LIFT_STAGE",
			filter:      fmt.Sprintf(`(SECURITY_CODE="%s")`, code),
			sortColumns: "FREE_DATE",
			sortTypes:   "-1",
		}, code != ""
	case "holders", "holder_num":
		return dataCenterQuerySpec{
			reportName:  "RPT_HOLDERNUMLATEST",
			filter:      fmt.Sprintf(`(SECURITY_CODE="%s")`, code),
			sortColumns: "END_DATE",
			sortTypes:   "-1",
		}, code != ""
	case "dividend", "dividends":
		return dataCenterQuerySpec{
			reportName:  "RPT_SHAREBONUS_DET",
			filter:      fmt.Sprintf(`(SECURITY_CODE="%s")`, code),
			sortColumns: "EX_DIVIDEND_DATE",
			sortTypes:   "-1",
		}, code != ""
	case "lhb_details":
		return dataCenterQuerySpec{
			reportName:  "RPT_DAILYBILLBOARD_DETAILSNEW",
			filter:      fmt.Sprintf(`(SECURITY_CODE="%s")`, code),
			sortColumns: "TRADE_DATE",
			sortTypes:   "-1",
		}, code != ""
	default:
		return dataCenterQuerySpec{}, false
	}
}

func (c *Client) Reports(ctx context.Context, code, qType, industryCode string, page, pageSize int) (*ReportsResult, error) {
	if qType == "" {
		qType = "0"
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := url.Values{
		"pageNo":     {strconv.Itoa(page)},
		"pageSize":   {strconv.Itoa(pageSize)},
		"qType":      {qType},
		"orgCode":    {""},
		"rcode":      {""},
		"p":          {strconv.Itoa(page)},
		"pageNum":    {strconv.Itoa(page)},
		"pageNumber": {strconv.Itoa(page)},
		"cb":         {""},
	}
	if code != "" {
		q.Set("code", code)
	}
	if industryCode != "" && industryCode != "*" {
		q.Set("industryCode", industryCode)
	}
	body, err := c.doJSONGet(ctx, "https://reportapi.eastmoney.com/report/list", q, "https://data.eastmoney.com/report/")
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	rows := extractReportRows(payload)
	out := &ReportsResult{Dataset: "reports", Source: "eastmoney", QType: qType}
	for _, row := range rows {
		info := strings.TrimSpace(fmt.Sprint(firstAny(row, "infoCode", "INFO_CODE", "info_code")))
		item := ReportRow{
			InfoCode:              info,
			Title:                 strings.TrimSpace(fmt.Sprint(firstAny(row, "title", "TITLE"))),
			PublishDate:           strings.TrimSpace(fmt.Sprint(firstAny(row, "publishDate", "PUBLISH_DATE", "publish_date"))),
			OrgName:               strings.TrimSpace(fmt.Sprint(firstAny(row, "orgSName", "orgName", "ORG_S_NAME"))),
			Rating:                strings.TrimSpace(fmt.Sprint(firstAny(row, "emRatingName", "rating", "EM_RATING_NAME"))),
			IndustryName:          strings.TrimSpace(fmt.Sprint(firstAny(row, "indvInduName", "industryName", "INDUSTRY_NAME"))),
			IndustryCode:          strings.TrimSpace(fmt.Sprint(firstAny(row, "industryCode", "INDUSTRY_CODE"))),
			PredictThisYearEPS:    firstAny(row, "predictThisYearEps", "PREDICT_THIS_YEAR_EPS"),
			PredictNextYearEPS:    firstAny(row, "predictNextYearEps", "PREDICT_NEXT_YEAR_EPS"),
			PredictNextTwoYearEPS: firstAny(row, "predictNextTwoYearEps", "PREDICT_NEXT_TWO_YEAR_EPS"),
			Raw:                   row,
		}
		if info != "" {
			item.PDFURL = "https://pdf.dfcfw.com/pdf/H3_" + info + "_1.pdf"
		}
		out.Rows = append(out.Rows, item)
	}
	out.Count = len(out.Rows)
	return out, nil
}

func extractReportRows(payload map[string]any) []map[string]any {
	candidates := []any{payload["data"]}
	if result, ok := payload["result"].(map[string]any); ok {
		candidates = append(candidates, result["data"], result["list"])
	}
	for _, candidate := range candidates {
		switch v := candidate.(type) {
		case []any:
			return mapsFromAnySlice(v)
		case map[string]any:
			for _, key := range []string{"data", "list", "records"} {
				if rows, ok := v[key].([]any); ok {
					return mapsFromAnySlice(rows)
				}
			}
		}
	}
	return nil
}

type NewsRow struct {
	Title       string         `json:"title"`
	URL         string         `json:"url,omitempty"`
	SourceName  string         `json:"source_name,omitempty"`
	PublishTime string         `json:"publish_time,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

type NewsResult struct {
	Dataset string    `json:"dataset"`
	Source  string    `json:"source"`
	Count   int       `json:"count"`
	Rows    []NewsRow `json:"rows"`
}

func (c *Client) StockNews(ctx context.Context, code string, pageSize int) (*NewsResult, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := url.Values{
		"keyword":   {code},
		"pageSize":  {strconv.Itoa(pageSize)},
		"pageIndex": {"1"},
		"cb":        {""},
	}
	body, err := c.doJSONGet(ctx, "https://search-api-web.eastmoney.com/search/jsonp", q, "https://so.eastmoney.com/")
	if err != nil {
		return nil, err
	}
	payload, err := decodeMaybeJSONP(body)
	if err != nil {
		return nil, err
	}
	rows := findMapsByKey(payload, []string{"cmsArticleWebOld", "list", "data"})
	out := &NewsResult{Dataset: "stock_news", Source: "eastmoney"}
	for _, row := range rows {
		out.Rows = append(out.Rows, NewsRow{
			Title:       strings.TrimSpace(fmt.Sprint(firstAny(row, "title", "Title"))),
			URL:         strings.TrimSpace(fmt.Sprint(firstAny(row, "url", "Url", "showUrl"))),
			SourceName:  strings.TrimSpace(fmt.Sprint(firstAny(row, "source", "sourceName", "mediaName"))),
			PublishTime: strings.TrimSpace(fmt.Sprint(firstAny(row, "date", "publishTime", "showTime"))),
			Summary:     strings.TrimSpace(fmt.Sprint(firstAny(row, "content", "summary", "digest"))),
			Raw:         row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

func (c *Client) GlobalNews(ctx context.Context, pageSize int) (*NewsResult, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	q := url.Values{
		"client":           {"web"},
		"biz":              {"web_news_col"},
		"column":           {"351"},
		"order":            {"1"},
		"needInteractData": {"0"},
		"page_index":       {strconv.Itoa(1)},
		"page_size":        {strconv.Itoa(pageSize)},
		"types":            {"1"},
	}
	body, err := c.doJSONGet(ctx, "https://np-listapi.eastmoney.com/comm/web/getNewsByColumns", q, "https://kuaixun.eastmoney.com/")
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	rows := findMapsByKey(payload, []string{"data", "list", "news"})
	out := &NewsResult{Dataset: "global_news", Source: "eastmoney"}
	for _, row := range rows {
		out.Rows = append(out.Rows, NewsRow{
			Title:       strings.TrimSpace(fmt.Sprint(firstAny(row, "title", "Title"))),
			URL:         strings.TrimSpace(fmt.Sprint(firstAny(row, "url", "Url"))),
			SourceName:  strings.TrimSpace(fmt.Sprint(firstAny(row, "source", "sourceName", "mediaName"))),
			PublishTime: strings.TrimSpace(fmt.Sprint(firstAny(row, "showTime", "publishTime", "date"))),
			Summary:     strings.TrimSpace(fmt.Sprint(firstAny(row, "digest", "summary", "content"))),
			Raw:         row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

type LimitPoolRow struct {
	Code              string         `json:"code"`
	Name              string         `json:"name"`
	Latest            *float64       `json:"latest,omitempty"`
	ChangePercent     *float64       `json:"change_percent,omitempty"`
	SealedAmount      *float64       `json:"sealed_amount,omitempty"`
	FirstLimitTime    string         `json:"first_limit_time,omitempty"`
	LastLimitTime     string         `json:"last_limit_time,omitempty"`
	ConsecutiveBoards *int64         `json:"consecutive_boards,omitempty"`
	Industry          string         `json:"industry,omitempty"`
	Raw               map[string]any `json:"raw,omitempty"`
}

type LimitPoolResult struct {
	Dataset string         `json:"dataset"`
	Source  string         `json:"source"`
	Date    string         `json:"date"`
	Pool    string         `json:"pool"`
	Count   int            `json:"count"`
	Rows    []LimitPoolRow `json:"rows"`
}

func (c *Client) LimitPool(ctx context.Context, pool, date string) (*LimitPoolResult, error) {
	if date == "" {
		date = time.Now().Format("20060102")
	}
	endpoint := "getTopicZTPool"
	sortField := "fbt:asc"
	switch pool {
	case "", "zt", "limit_up":
		pool = "limit_up"
	case "zb", "broken":
		pool = "broken"
		endpoint = "getTopicZBPool"
	case "dt", "limit_down":
		pool = "limit_down"
		endpoint = "getTopicDTPool"
		sortField = "fund:asc"
	case "yzt", "yesterday":
		pool = "yesterday"
		endpoint = "getYesterdayZTPool"
		sortField = "zs:desc"
	default:
		return nil, fmt.Errorf("invalid pool: %s", pool)
	}
	q := url.Values{
		"ut":   {utQuote},
		"d":    {date},
		"sort": {sortField},
	}
	body, err := c.doJSONGet(ctx, "https://push2ex.eastmoney.com/"+endpoint, q, "https://quote.eastmoney.com/ztb/")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Pool []map[string]any `json:"pool"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &LimitPoolResult{Dataset: "limit_pool", Source: "eastmoney", Date: date, Pool: pool}
	for _, row := range payload.Data.Pool {
		out.Rows = append(out.Rows, LimitPoolRow{
			Code:              strings.TrimSpace(fmt.Sprint(firstAny(row, "c", "code"))),
			Name:              strings.TrimSpace(fmt.Sprint(firstAny(row, "n", "name"))),
			Latest:            getFloat(row, "p"),
			ChangePercent:     getFloat(row, "zdp"),
			SealedAmount:      getFloat(row, "fund"),
			FirstLimitTime:    formatHHMMSS(firstAny(row, "fbt")),
			LastLimitTime:     formatHHMMSS(firstAny(row, "lbt")),
			ConsecutiveBoards: getInt(row, "lbc"),
			Industry:          strings.TrimSpace(fmt.Sprint(firstAny(row, "hybk", "hy"))),
			Raw:               row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

func (c *Client) doJSONGet(ctx context.Context, rawURL string, q url.Values, referer string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return c.fetch(ctx, u.Host, u.Path, q, referer)
}

func decodeMaybeJSONP(body []byte) (map[string]any, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, errors.New("empty response")
	}
	if i := strings.Index(text, "("); i >= 0 && strings.HasSuffix(text, ")") {
		text = strings.TrimSuffix(text[i+1:], ")")
	}
	text = strings.TrimSuffix(text, ";")
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func mapsFromAnySlice(rows []any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func firstAny(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := row[key]; ok && v != nil {
			return v
		}
	}
	return ""
}

func findMapsByKey(payload map[string]any, keys []string) []map[string]any {
	var walk func(any) []map[string]any
	walk = func(v any) []map[string]any {
		switch x := v.(type) {
		case []any:
			return mapsFromAnySlice(x)
		case map[string]any:
			for _, key := range keys {
				if child, ok := x[key]; ok {
					if rows := walk(child); len(rows) > 0 {
						return rows
					}
				}
			}
			for _, child := range x {
				if rows := walk(child); len(rows) > 0 {
					return rows
				}
			}
		}
		return nil
	}
	return walk(payload)
}

func formatHHMMSS(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if len(s) == 6 {
		return s[0:2] + ":" + s[2:4] + ":" + s[4:6]
	}
	return s
}

package research

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultXiaoBeiBaseURL = "https://api.xiaobeiyangji.com/yangji-api/api"
const defaultXiaoBeiVersion = "3.8.2.3"

type XiaoBeiClient struct {
	client  *http.Client
	baseURL string
	version string
}

type XiaoBeiRequest struct {
	Token    string `json:"token"`
	UnionID  string `json:"union_id,omitempty"`
	FundCode string `json:"fund_code"`
	Version  string `json:"version,omitempty"`
}

type XiaoBeiAllocation struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio,omitempty"`
}

type XiaoBeiHolding struct {
	StockCode     string         `json:"stock_code"`
	StockName     string         `json:"stock_name"`
	Weight        float64        `json:"weight"`
	Price         float64        `json:"price,omitempty"`
	ChangePercent float64        `json:"change_percent,omitempty"`
	HoldingType   string         `json:"holding_type"`
	RawData       map[string]any `json:"raw_data,omitempty"`
}

type XiaoBeiFundProfile struct {
	FundCode   string              `json:"fund_code"`
	FundName   string              `json:"fund_name,omitempty"`
	Source     string              `json:"source"`
	ReportDate string              `json:"report_date,omitempty"`
	Asset      []XiaoBeiAllocation `json:"asset"`
	Industry   []XiaoBeiAllocation `json:"industry"`
	Holdings   []XiaoBeiHolding    `json:"holdings"`
	RawData    map[string]any      `json:"raw_data"`
}

func NewXiaoBeiClient(timeout time.Duration) *XiaoBeiClient {
	return &XiaoBeiClient{
		client:  &http.Client{Timeout: timeout},
		baseURL: defaultXiaoBeiBaseURL,
		version: defaultXiaoBeiVersion,
	}
}

func (c *XiaoBeiClient) FundProfile(ctx context.Context, req XiaoBeiRequest) (XiaoBeiFundProfile, error) {
	req.FundCode = normalizeFundCode(req.FundCode)
	if strings.TrimSpace(req.Token) == "" {
		return XiaoBeiFundProfile{}, fmt.Errorf("xiaobeiyangji token required")
	}
	if req.FundCode == "" {
		return XiaoBeiFundProfile{}, fmt.Errorf("fund_code required")
	}

	detail, detailErr := c.post(ctx, req, "/get-fund-detail-v310", map[string]any{
		"code":             req.FundCode,
		"accountId":        0,
		"dataResources":    "4",
		"dataSourceSwitch": true,
		"isHasPosition":    true,
		"fromType":         "home",
	})
	if detailErr != nil {
		return XiaoBeiFundProfile{}, detailErr
	}
	positionRows, _ := c.post(ctx, req, "/get-fund-position-ratio", map[string]any{
		"code": req.FundCode,
	})

	profile := XiaoBeiFundProfile{
		FundCode: req.FundCode,
		FundName: firstString(detail, "name", "fundName", "shortName"),
		Source:   "xiaobeiyangji",
		Asset:    normalizeAssetAllocations(detail),
		Industry: normalizeIndustryAllocations(detail),
		Holdings: normalizeXiaoBeiHoldings(positionRows),
		RawData: map[string]any{
			"detail":              detail,
			"fund_position_ratio": positionRows,
		},
	}
	profile.ReportDate = inferXiaoBeiReportDate(positionRows)
	return profile, nil
}

func (c *XiaoBeiClient) FundHoldings(ctx context.Context, req XiaoBeiRequest) ([]XiaoBeiHolding, error) {
	req.FundCode = normalizeFundCode(req.FundCode)
	if strings.TrimSpace(req.Token) == "" {
		return nil, fmt.Errorf("xiaobeiyangji token required")
	}
	if req.FundCode == "" {
		return nil, fmt.Errorf("fund_code required")
	}
	positionRows, err := c.post(ctx, req, "/get-fund-position-ratio", map[string]any{
		"code": req.FundCode,
	})
	if err != nil {
		return nil, err
	}
	return normalizeXiaoBeiHoldings(positionRows), nil
}

func (c *XiaoBeiClient) post(ctx context.Context, req XiaoBeiRequest, path string, body map[string]any) (any, error) {
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = c.version
	}
	body["unionId"] = req.UnionID
	body["version"] = version
	body["clientType"] = "APP"

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	httpReq.Header.Set("User-Agent", "TradeHub fund-research")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xiaobeiyangji %s http %d: %s", path, resp.StatusCode, string(raw))
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 200 {
		if envelope.Msg == "" {
			envelope.Msg = "unknown error"
		}
		return nil, fmt.Errorf("xiaobeiyangji %s: %s", path, envelope.Msg)
	}
	var data any
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func normalizeFundCode(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "sh")
	value = strings.TrimPrefix(value, "sz")
	return value
}

func normalizeAssetAllocations(detail any) []XiaoBeiAllocation {
	items := []XiaoBeiAllocation{}
	for _, key := range []string{"asset", "assets", "fundAssetRatio", "assetRatio", "fundPosition"} {
		for _, raw := range anySlice(mapValue(detail, key)) {
			if item := allocationFromMap(raw); item.Name != "" {
				items = append(items, item)
			}
		}
	}
	return dedupeAllocations(items)
}

func normalizeIndustryAllocations(detail any) []XiaoBeiAllocation {
	items := []XiaoBeiAllocation{}
	for _, key := range []string{"relatedIndustryV2", "relatedIndustry", "industry", "industryList", "themeList"} {
		for _, raw := range anySlice(mapValue(detail, key)) {
			if item := allocationFromMap(raw); item.Name != "" {
				items = append(items, item)
			}
		}
	}
	return dedupeAllocations(items)
}

func allocationFromMap(raw any) XiaoBeiAllocation {
	m, ok := raw.(map[string]any)
	if !ok {
		return XiaoBeiAllocation{}
	}
	name := firstString(m, "name", "themeName", "industryName", "sectorName", "label", "title")
	ratio := firstFloat(m, "ratio", "weight", "prop", "percent", "value")
	return XiaoBeiAllocation{Name: name, Ratio: ratio}
}

func normalizeXiaoBeiHoldings(raw any) []XiaoBeiHolding {
	rows := anySlice(raw)
	if m, ok := raw.(map[string]any); ok {
		rows = anySlice(firstValue(m, "position", "positions", "list", "data"))
	}
	if len(rows) > 0 {
		if first, ok := rows[0].(map[string]any); ok {
			if nested := anySlice(firstValue(first, "position", "positions", "list", "data")); len(nested) > 0 {
				rows = nested
			}
		}
	}
	holdings := []XiaoBeiHolding{}
	for _, rawItem := range rows {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		name := firstString(item, "stock_name", "stockName", "asset_name", "assetName", "name")
		code := firstString(item, "stock_code", "stockCode", "asset_code", "assetCode", "code")
		weight := firstFloat(item, "weight", "ratio", "holdRatio", "positionRatio")
		if name == "" && code == "" {
			continue
		}
		holdingType := "stock"
		if code == "" {
			holdingType = "product"
		}
		holdings = append(holdings, XiaoBeiHolding{
			StockCode:     code,
			StockName:     name,
			Weight:        weight,
			Price:         firstFloat(item, "price", "nav"),
			ChangePercent: firstFloat(item, "change", "changePercent", "rate"),
			HoldingType:   holdingType,
			RawData:       item,
		})
	}
	return holdings
}

func inferXiaoBeiReportDate(raw any) string {
	rows := anySlice(raw)
	if len(rows) == 0 {
		return ""
	}
	first, ok := rows[0].(map[string]any)
	if !ok {
		return ""
	}
	if dateText := firstString(first, "reportDate", "date", "dataDate", "pubDate"); dateText != "" {
		return dateText
	}
	year := parseInt(first["year"])
	quarter := parseInt(first["quarter"])
	if year == 0 || quarter == 0 {
		return ""
	}
	switch quarter {
	case 1:
		return fmt.Sprintf("%04d-03-31", year)
	case 2:
		return fmt.Sprintf("%04d-06-30", year)
	case 3:
		return fmt.Sprintf("%04d-09-30", year)
	case 4:
		return fmt.Sprintf("%04d-12-31", year)
	default:
		return ""
	}
}

func dedupeAllocations(items []XiaoBeiAllocation) []XiaoBeiAllocation {
	out := []XiaoBeiAllocation{}
	seen := map[string]int{}
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		if idx, ok := seen[item.Name]; ok {
			if out[idx].Ratio == 0 && item.Ratio != 0 {
				out[idx].Ratio = item.Ratio
			}
			continue
		}
		seen[item.Name] = len(out)
		out = append(out, item)
	}
	return out
}

func mapValue(raw any, key string) any {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return m[key]
}

func anySlice(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func firstValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := m[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func firstString(raw any, keys ...string) string {
	m, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range keys {
		if text := strings.TrimSpace(fmt.Sprint(m[key])); text != "" && text != "<nil>" && text != "--" {
			return text
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value := parseFloat(m[key])
		if value != 0 {
			return value
		}
	}
	return 0
}

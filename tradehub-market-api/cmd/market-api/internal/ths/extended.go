package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HotReasonResult struct {
	Dataset string           `json:"dataset"`
	Source  string           `json:"source"`
	Date    string           `json:"date"`
	Count   int              `json:"count"`
	Rows    []map[string]any `json:"rows"`
}

func (c *Client) HotReason(ctx context.Context, date string) (*HotReasonResult, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	rawURL := "http://zx.10jqka.com.cn/event/api/getharden/date/" + date + "/orderby/date/orderway/desc/charset/GBK/"
	body, err := c.get(ctx, rawURL, "http://zx.10jqka.com.cn/")
	if err != nil {
		return nil, err
	}
	var payload struct {
		ErrorCode any              `json:"errocode"`
		ErrorMsg  string           `json:"errormsg"`
		Data      []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if fmt.Sprint(payload.ErrorCode) != "0" && fmt.Sprint(payload.ErrorCode) != "<nil>" {
		return nil, fmt.Errorf("ths hot reason: %s", payload.ErrorMsg)
	}
	return &HotReasonResult{Dataset: "hot_reason", Source: "ths", Date: date, Count: len(payload.Data), Rows: payload.Data}, nil
}

type NorthboundResult struct {
	Dataset string           `json:"dataset"`
	Source  string           `json:"source"`
	Count   int              `json:"count"`
	Rows    []map[string]any `json:"rows"`
}

func (c *Client) Northbound(ctx context.Context) (*NorthboundResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://data.hexin.cn/market/hsgtApi/method/dayChart/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Host", "data.hexin.cn")
	req.Header.Set("Referer", "https://data.hexin.cn/")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Time []any `json:"time"`
		HGT  []any `json:"hgt"`
		SGT  []any `json:"sgt"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &NorthboundResult{Dataset: "northbound_intraday", Source: "ths"}
	for i, t := range payload.Time {
		row := map[string]any{"time": t}
		if i < len(payload.HGT) {
			row["hgt_yi"] = payload.HGT[i]
		}
		if i < len(payload.SGT) {
			row["sgt_yi"] = payload.SGT[i]
		}
		out.Rows = append(out.Rows, row)
	}
	out.Count = len(out.Rows)
	return out, nil
}

type HotListResult struct {
	Dataset string           `json:"dataset"`
	Source  string           `json:"source"`
	Period  string           `json:"period"`
	Count   int              `json:"count"`
	Rows    []map[string]any `json:"rows"`
}

func (c *Client) HotList(ctx context.Context, period string) (*HotListResult, error) {
	if period == "" {
		period = "hour"
	}
	q := url.Values{"stock_type": {"a"}, "type": {period}, "list_type": {"normal"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://dq.10jqka.com.cn/fuyao/hot_list_data/out/hot_list/v1/stock?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			StockList []map[string]any `json:"stock_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &HotListResult{Dataset: "hot_list", Source: "ths", Period: period}
	for _, item := range payload.Data.StockList {
		tag, _ := item["tag"].(map[string]any)
		out.Rows = append(out.Rows, map[string]any{
			"rank":     item["order"],
			"code":     item["code"],
			"name":     item["name"],
			"heat":     item["rate"],
			"pct":      item["rise_and_fall"],
			"rank_chg": item["hot_rank_chg"],
			"concepts": tag["concept_tag"],
			"tag":      tag["popularity_tag"],
			"raw":      item,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

func trimCodePrefix(code string) string {
	code = strings.TrimSpace(code)
	if len(code) > 2 && (strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ")) {
		return code[2:]
	}
	return code
}

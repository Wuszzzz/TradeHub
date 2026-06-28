package cninfo

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

type AnnouncementRow struct {
	ID          string         `json:"id,omitempty"`
	Code        string         `json:"code,omitempty"`
	Name        string         `json:"name,omitempty"`
	Title       string         `json:"title"`
	PublishTime string         `json:"publish_time,omitempty"`
	Category    string         `json:"category,omitempty"`
	DownloadURL string         `json:"download_url,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

type AnnouncementsResult struct {
	Dataset string            `json:"dataset"`
	Source  string            `json:"source"`
	Code    string            `json:"code,omitempty"`
	Count   int               `json:"count"`
	Rows    []AnnouncementRow `json:"rows"`
}

func (c *Client) Announcements(ctx context.Context, code string, page, pageSize int) (*AnnouncementsResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 30
	}
	form := url.Values{
		"pageNum":   {strconv.Itoa(page)},
		"pageSize":  {strconv.Itoa(pageSize)},
		"column":    {"szse"},
		"tabName":   {"fulltext"},
		"plate":     {""},
		"stock":     {code},
		"searchkey": {""},
		"secid":     {""},
		"category":  {""},
		"trade":     {""},
		"seDate":    {""},
		"sortName":  {""},
		"sortType":  {""},
		"isHLtitle": {"true"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.cninfo.com.cn/new/hisAnnouncement/query", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.cninfo.com.cn/new/commonUrl/pageOfSearch")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Announcements []map[string]any `json:"announcements"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := &AnnouncementsResult{Dataset: "announcements", Source: "cninfo", Code: code}
	for _, row := range payload.Announcements {
		adjunct := strings.TrimSpace(fmt.Sprint(row["adjunctUrl"]))
		download := ""
		if adjunct != "" {
			download = "https://static.cninfo.com.cn/" + strings.TrimLeft(adjunct, "/")
		}
		out.Rows = append(out.Rows, AnnouncementRow{
			ID:          strings.TrimSpace(fmt.Sprint(row["announcementId"])),
			Code:        strings.TrimSpace(fmt.Sprint(first(row, "secCode", "证券代码"))),
			Name:        strings.TrimSpace(fmt.Sprint(first(row, "secName", "证券简称"))),
			Title:       cleanTitle(strings.TrimSpace(fmt.Sprint(row["announcementTitle"]))),
			PublishTime: formatMillis(row["announcementTime"]),
			Category:    strings.TrimSpace(fmt.Sprint(row["category"])),
			DownloadURL: download,
			Raw:         row,
		})
	}
	out.Count = len(out.Rows)
	return out, nil
}

func first(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := row[key]; ok && v != nil {
			return v
		}
	}
	return ""
}

func cleanTitle(s string) string {
	s = strings.ReplaceAll(s, "<em>", "")
	s = strings.ReplaceAll(s, "</em>", "")
	return s
}

func formatMillis(v any) string {
	switch x := v.(type) {
	case float64:
		return time.UnixMilli(int64(x)).Format("2006-01-02 15:04:05")
	case int64:
		return time.UnixMilli(x).Format("2006-01-02 15:04:05")
	case string:
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return time.UnixMilli(n).Format("2006-01-02 15:04:05")
		}
		return x
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

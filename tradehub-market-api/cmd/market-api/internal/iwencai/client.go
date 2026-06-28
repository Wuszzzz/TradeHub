package iwencai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

func New(timeout time.Duration, baseURL, apiKey string) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if baseURL == "" {
		baseURL = getenv("IWENCAI_BASE_URL", "https://openapi.iwencai.com")
	}
	if apiKey == "" {
		apiKey = os.Getenv("IWENCAI_API_KEY")
	}
	return &Client{http: &http.Client{Timeout: timeout}, baseURL: baseURL, apiKey: apiKey}
}

func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func (c *Client) Search(ctx context.Context, query, channel string, size int) (map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("IWENCAI_API_KEY is not configured")
	}
	if channel == "" {
		channel = "report"
	}
	if size <= 0 || size > 100 {
		size = 50
	}
	payload := map[string]any{"channels": []string{channel}, "app_id": "AIME_SKILL", "query": query, "size": size}
	return c.post(ctx, "/v1/comprehensive/search", payload)
}

func (c *Client) Query(ctx context.Context, query string, page, limit int) (map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("IWENCAI_API_KEY is not configured")
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	payload := map[string]any{"query": query, "page": fmt.Sprint(page), "limit": fmt.Sprint(limit), "is_cache": "1", "expand_index": "true"}
	return c.post(ctx, "/v1/query2data", payload)
}

func (c *Client) post(ctx context.Context, path string, payload map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claw-Call-Type", "normal")
	req.Header.Set("X-Claw-Skill-Id", "report-search")
	req.Header.Set("X-Claw-Skill-Version", "2.0.0")
	req.Header.Set("X-Claw-Plugin-Id", "none")
	req.Header.Set("X-Claw-Plugin-Version", "none")
	req.Header.Set("X-Claw-Trace-Id", traceID())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("iwencai http %d: %.200s", resp.StatusCode, string(raw))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func traceID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

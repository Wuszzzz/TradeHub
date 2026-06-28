package research

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func errorJSON(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, APIResponse{OK: false, Error: err.Error()})
}

func splitFields(raw string) []string {
	raw = strings.NewReplacer(",", " ", "\n", " ", "\t", " ", "，", " ").Replace(raw)
	parts := strings.Fields(raw)
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func intQuery(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func floatQuery(r *http.Request, key string, fallback float64) float64 {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func boolQuery(r *http.Request, key string, fallback bool) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes")
}

func parseFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		text := strings.TrimSpace(strings.TrimSuffix(v, "%"))
		if text == "" || text == "--" {
			return 0
		}
		f, _ := strconv.ParseFloat(text, 64)
		return f
	default:
		return 0
	}
}

func parseInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	default:
		return 0
	}
}

func rankRatio(rank, total int) float64 {
	if rank <= 0 || total <= 0 {
		return 0
	}
	return float64(rank) / float64(total) * 100
}

func avgNonZero(values ...float64) float64 {
	sum := 0.0
	count := 0
	for _, value := range values {
		if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		sum += value
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func yearsSince(dateText string) float64 {
	if strings.TrimSpace(dateText) == "" || dateText == "--" {
		return 0
	}
	t, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		return 0
	}
	return time.Since(t).Hours() / 24 / 365
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

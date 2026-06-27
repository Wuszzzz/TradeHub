package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"stock/cmd/market-api/internal/eastmoney"
	"stock/cmd/market-api/internal/sohu"
	"stock/cmd/market-api/internal/tencent"
	"stock/cmd/market-api/internal/ths"
	"stock/cmd/market-api/internal/xueqiu"
)

type server struct {
	tencent   *tencent.Client
	eastmoney *eastmoney.Client
	sohu      *sohu.Client
	ths       *ths.Client
	xueqiu    *xueqiu.Client
	cache     *memoryCache
	sf        singleflight.Group
}

type cachedResult struct {
	status int
	body   []byte
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

func main() {
	addr := getenv("MARKET_API_ADDR", ":18080")
	s := &server{
		tencent:   tencent.New(5 * time.Second),
		eastmoney: eastmoney.New(8 * time.Second),
		sohu:      sohu.New(6 * time.Second),
		ths:       ths.New(6 * time.Second),
		xueqiu:    xueqiu.New(8*time.Second, getenv("XUEQIU_COOKIE", "")),
		cache:     newMemoryCache(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/v1/cache/stats", s.cacheStats)

	mux.HandleFunc("GET /api/v1/tencent/snapshot", s.tencentSnapshot)
	mux.HandleFunc("GET /api/v1/tencent/ticks", s.tencentTicks)
	mux.HandleFunc("GET /api/v1/tencent/large-trades", s.tencentLargeTrades)
	mux.HandleFunc("GET /api/v1/tencent/minute", s.tencentMinute)
	mux.HandleFunc("GET /api/v1/tencent/kline", s.tencentKline)
	mux.HandleFunc("GET /api/v1/tencent/fund-asset", s.tencentFundAsset)

	mux.HandleFunc("GET /api/v1/eastmoney/snapshot", s.eastmoneySnapshot)
	mux.HandleFunc("GET /api/v1/eastmoney/kline", s.eastmoneyKline)
	mux.HandleFunc("GET /api/v1/eastmoney/trends", s.eastmoneyTrends)
	mux.HandleFunc("GET /api/v1/eastmoney/flow/snapshot", s.eastmoneyFlowSnapshot)
	mux.HandleFunc("GET /api/v1/eastmoney/flow/intraday", s.eastmoneyFlowIntraday)
	mux.HandleFunc("GET /api/v1/eastmoney/flow/daily", s.eastmoneyFlowDaily)

	mux.HandleFunc("GET /api/v1/sohu/snapshot", s.sohuSnapshot)
	mux.HandleFunc("GET /api/v1/sohu/kline", s.sohuKline)
	mux.HandleFunc("GET /api/v1/sohu/ticks", s.sohuTicks)
	mux.HandleFunc("GET /api/v1/sohu/minute", s.sohuMinute)
	mux.HandleFunc("GET /api/v1/sohu/price-distribution", s.sohuPriceDistribution)
	mux.HandleFunc("GET /api/v1/sohu/flow", s.sohuFlow)
	mux.HandleFunc("GET /api/v1/sohu/flow/series", s.sohuFlowSeries)
	mux.HandleFunc("GET /api/v1/sohu/order-book", s.sohuOrderBook)
	mux.HandleFunc("GET /api/v1/sohu/aggregate", s.sohuAggregate)

	mux.HandleFunc("GET /api/v1/ths/snapshot", s.thsSnapshot)
	mux.HandleFunc("GET /api/v1/ths/minute", s.thsMinute)
	mux.HandleFunc("GET /api/v1/ths/kline", s.thsKline)

	mux.HandleFunc("GET /api/v1/xueqiu/snapshot", s.xueqiuSnapshot)
	mux.HandleFunc("GET /api/v1/xueqiu/kline", s.xueqiuKline)

	wrapped := logging(cors(mux))
	log.Printf("market-api listening on %s", addr)
	if err := http.ListenAndServe(addr, wrapped); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]any{"service": "market-api", "time": time.Now().Unix()}})
}

func (s *server) cacheStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{OK: true, Data: s.cache.stats()})
}

func (s *server) tencentSnapshot(w http.ResponseWriter, r *http.Request) {
	ttl := 800 * time.Millisecond
	s.serveWithCache(w, r, ttl, func() (any, error) {
		symbols := readSymbols(r)
		data, err := s.tencent.Snapshots(r.Context(), symbols)
		if err != nil {
			return nil, err
		}
		if len(data) == 1 {
			return data[0], nil
		}
		return map[string]any{"dataset": "snapshot_batch", "source": "tencent", "count": len(data), "rows": data}, nil
	})
}

func (s *server) tencentTicks(w http.ResponseWriter, r *http.Request) {
	ttl := 2 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		symbol := firstSymbol(r)
		pages := intQuery(r, "pages", 1)
		limit := intQuery(r, "limit", 0)
		largeThreshold := floatQuery(r, "large_threshold", 1_000_000)
		superThreshold := floatQuery(r, "super_threshold", 5_000_000)
		data, err := s.tencent.Ticks(r.Context(), symbol, pages, largeThreshold, superThreshold)
		if err != nil {
			return nil, err
		}
		data.Rows = tencent.TrimTicks(data.Rows, limit)
		data.Count = len(data.Rows)
		return data, nil
	})
}

func (s *server) tencentLargeTrades(w http.ResponseWriter, r *http.Request) {
	ttl := 3 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		symbol := firstSymbol(r)
		pages := intQuery(r, "pages", 5)
		limit := intQuery(r, "limit", 0)
		minAmount := floatQuery(r, "min_amount", 1_000_000)
		largeThreshold := floatQuery(r, "large_threshold", 1_000_000)
		superThreshold := floatQuery(r, "super_threshold", 5_000_000)
		ticks, err := s.tencent.Ticks(r.Context(), symbol, pages, largeThreshold, superThreshold)
		if err != nil {
			return nil, err
		}
		data := tencent.LargeTrades(ticks, minAmount)
		if limit > 0 && len(data.Rows) > limit {
			data.Rows = tencent.TrimTicks(data.Rows, limit)
			data.DisplayCount = len(data.Rows)
		}
		return data, nil
	})
}

func (s *server) tencentMinute(w http.ResponseWriter, r *http.Request) {
	ttl := 8 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		symbol := firstSymbol(r)
		limit := intQuery(r, "limit", 0)
		data, err := s.tencent.Minute(r.Context(), symbol)
		if err != nil {
			return nil, err
		}
		data.Rows = tencent.TrimMinute(data.Rows, limit)
		data.Count = len(data.Rows)
		return data, nil
	})
}

func (s *server) tencentKline(w http.ResponseWriter, r *http.Request) {
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "5m"
	}
	ttl := klineCacheTTL(period)
	s.serveWithCache(w, r, ttl, func() (any, error) {
		symbol := firstSymbol(r)
		adjust := strings.TrimSpace(r.URL.Query().Get("adjust"))
		if adjust == "" {
			adjust = "qfq"
		}
		limit := intQuery(r, "limit", 100)
		return s.tencent.Kline(r.Context(), symbol, period, adjust, limit)
	})
}

func (s *server) tencentFundAsset(w http.ResponseWriter, r *http.Request) {
	ttl := 30 * time.Minute
	s.serveWithCache(w, r, ttl, func() (any, error) {
		symbol := firstSymbol(r)
		return s.tencent.FundAsset(r.Context(), symbol)
	})
}

func readSymbols(r *http.Request) []string {
	query := r.URL.Query()
	var symbols []string
	if raw := strings.TrimSpace(query.Get("symbols")); raw != "" {
		for _, item := range strings.Split(raw, ",") {
			if strings.TrimSpace(item) != "" {
				symbols = append(symbols, strings.TrimSpace(item))
			}
		}
	}
	if raw := strings.TrimSpace(query.Get("symbol")); raw != "" {
		symbols = append(symbols, raw)
	}
	if len(symbols) == 0 {
		symbols = []string{"513310"}
	}
	return symbols
}

func firstSymbol(r *http.Request) string {
	return readSymbols(r)[0]
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatQuery(r *http.Request, key string, fallback float64) float64 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadGateway, response{OK: false, Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// serveWithCache 统一处理 缓存命中 -> singleflight 合并冷请求 -> 写缓存 的流程。
func (s *server) serveWithCache(w http.ResponseWriter, r *http.Request, ttl time.Duration, build func() (any, error)) {
	key := cacheKey(r)
	if ttl > 0 {
		if body, ok := s.cache.get(key); ok {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}
	}

	res, err, shared := s.sf.Do(key, func() (any, error) {
		// 二次检查，singleflight 排队期间可能已有人写了缓存。
		if ttl > 0 {
			if body, ok := s.cache.get(key); ok {
				return cachedResult{status: http.StatusOK, body: body}, nil
			}
		}
		value, buildErr := build()
		if buildErr != nil {
			return nil, buildErr
		}
		var buf bytes.Buffer
		if encErr := json.NewEncoder(&buf).Encode(response{OK: true, Data: value}); encErr != nil {
			return nil, encErr
		}
		body := buf.Bytes()
		if ttl > 0 {
			s.cache.set(key, body, ttl)
		}
		return cachedResult{status: http.StatusOK, body: body}, nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	cr := res.(cachedResult)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cacheTag := "MISS"
	if shared {
		cacheTag = "COALESCED"
	}
	w.Header().Set("X-Cache", cacheTag)
	w.WriteHeader(cr.status)
	_, _ = w.Write(cr.body)
}

func cacheKey(r *http.Request) string {
	return r.Method + " " + r.URL.Path + "?" + r.URL.Query().Encode()
}

func klineCacheTTL(period string) time.Duration {
	switch period {
	case "5m", "15m", "30m", "60m":
		return 30 * time.Second
	case "day":
		return 5 * time.Minute
	case "week", "month":
		return 30 * time.Minute
	default:
		return 30 * time.Second
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Xueqiu-Cookie, X-XQ-Cookie")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start).Truncate(time.Millisecond))
	})
}

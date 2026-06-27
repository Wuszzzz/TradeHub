package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	alertapp "stock-etf-monitor/backend/internal/application/alert"
	backtestapp "stock-etf-monitor/backend/internal/application/backtest"
	brokerapp "stock-etf-monitor/backend/internal/application/broker"
	datacenter "stock-etf-monitor/backend/internal/application/datacenter"
	instrumentapp "stock-etf-monitor/backend/internal/application/instrument"
	marketapp "stock-etf-monitor/backend/internal/application/market"
	paperapp "stock-etf-monitor/backend/internal/application/paper"
	quantapp "stock-etf-monitor/backend/internal/application/quant"
	screenerapp "stock-etf-monitor/backend/internal/application/screener"
	strategyapp "stock-etf-monitor/backend/internal/application/strategy"
	systemapp "stock-etf-monitor/backend/internal/application/system"
	taskapp "stock-etf-monitor/backend/internal/application/task"
	watchlistapp "stock-etf-monitor/backend/internal/application/watchlist"
	"stock-etf-monitor/backend/internal/interfaces/httpapi"
	"stock-etf-monitor/backend/model"

	_ "github.com/lib/pq"
)

// ttlCache 极简带 TTL 的内存缓存，用于挡住 Python 子进程冷启动的高频请求
type ttlCache struct {
	mu   sync.RWMutex
	ttl  time.Duration
	data map[string]ttlEntry
}

type ttlEntry struct {
	value     map[string]any
	expiresAt time.Time
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, data: map[string]ttlEntry{}}
}

func (c *ttlCache) Get(key string) (map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.data[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

// GetStale 返回任意存在的 entry（无论是否过期）与是否新鲜。
// 用于 stale-while-revalidate：过期值立刻返给客户端，后台异步刷新。
func (c *ttlCache) GetStale(key string) (value map[string]any, fresh bool, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.data[key]
	if !exists {
		return nil, false, false
	}
	return entry.value, time.Now().Before(entry.expiresAt), true
}

func (c *ttlCache) Set(key string, value map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = ttlEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
}

// inflight 记录正在后台刷新的 key，避免同一 symbol 并发触发多次子进程。
type inflightSet struct {
	mu  sync.Mutex
	set map[string]struct{}
}

func newInflightSet() *inflightSet { return &inflightSet{set: map[string]struct{}{}} }

// TryAcquire 抢占；返回 true 表示当前 goroutine 拿到刷新权限。
func (s *inflightSet) TryAcquire(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, busy := s.set[key]; busy {
		return false
	}
	s.set[key] = struct{}{}
	return true
}

func (s *inflightSet) Release(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.set, key)
}

// cachedSnapshot 走 snapshotCache 拿快照。
// 策略（stale-while-revalidate）：
//  1. 新鲜 → 立刻返回，无 I/O。
//  2. 过期但存在旧值 → 立刻返回旧值，后台 goroutine 异步刷新；下次请求就会拿到新值。
//  3. 完全缺失 → 阻塞同步拉一次（首次访问无可避免）。
//
// 这样把跨市场（尤其美股 yfinance ~2-3s、汇率 ~1s）的远程调用对前端的可见延迟，
// 从「每 TTL 周期一次抖动」削成「只有冷启动第一次」。
func (s *Server) cachedSnapshot(symbol string) (map[string]any, error) {
	if cached, fresh, ok := s.snapshotCache.GetStale(symbol); ok {
		if !fresh {
			s.refreshSnapshotAsync(symbol)
		}
		return cached, nil
	}
	// 完全缺失：必须同步等远程返回
	return s.refreshSnapshotSync(symbol)
}

func (s *Server) refreshSnapshotSync(symbol string) (map[string]any, error) {
	item, err := s.adapter.Snapshot(symbol)
	if err != nil {
		return nil, err
	}
	s.snapshotCache.Set(symbol, item)
	return item, nil
}

func (s *Server) refreshSnapshotAsync(symbol string) {
	if !s.snapshotInflight.TryAcquire(symbol) {
		return // 已有 goroutine 在刷
	}
	go func() {
		defer s.snapshotInflight.Release(symbol)
		// 后台失败静默：当前请求已经拿到 stale 值，下次再试即可
		if item, err := s.adapter.Snapshot(symbol); err == nil {
			s.snapshotCache.Set(symbol, item)
		}
	}()
}

type Server struct {
	mux               *http.ServeMux
	taskRepo          *PostgresRepository
	tdRepo            *TdengineRepository
	adapter           *MarketDataService
	taskRunner        *TaskRunner
	snapshotCache     *ttlCache    // 实时快照：10s TTL（stale-while-revalidate 下实际"陈旧门槛"）
	profileCache      *ttlCache    // 板块/行业资料：1 小时
	listCache         *ttlCache    // 全市场浏览：30s
	etfRiskCache      *ttlCache    // ETF 风控：15s
	snapshotInflight  *inflightSet // 防止同 symbol 多个后台刷新并发
	systemHandler     *httpapi.SystemHandler
	instrumentHandler *httpapi.InstrumentHandler
	marketHandler     *httpapi.MarketHandler
	watchlistHandler  *httpapi.WatchlistHandler
	taskHandler       *httpapi.TaskHandler
	alertHandler      *httpapi.AlertHandler
	paperHandler      *httpapi.PaperHandler
	brokerHandler     *httpapi.BrokerHandler
	quantHandler      *httpapi.QuantHandler
	screenerHandler   *httpapi.ScreenerHandler
	backtestHandler   *httpapi.BacktestHandler
	strategyHandler   *httpapi.StrategyHandler
	datacenterHandler *httpapi.DataCenterHandler
	datacenterService *datacenter.ServiceImpl
}

type serverSnapshotProvider struct {
	server *Server
}

func (p serverSnapshotProvider) Snapshot(symbol string) (map[string]any, error) {
	return p.server.cachedSnapshot(symbol)
}

type liveBarsProvider struct {
	tdRepo  *TdengineRepository
	adapter *MarketDataService
}

func (p liveBarsProvider) QueryBars(symbol, period string, limit int) ([]map[string]any, error) {
	if p.tdRepo != nil {
		if rows, err := p.tdRepo.QueryBars(symbol, period, limit); err == nil && len(rows) > 0 {
			return rows, nil
		}
	}
	if p.adapter == nil {
		return []map[string]any{}, nil
	}
	if period == "1d" || period == "daily" || period == "day" {
		data, err := p.adapter.Daily(symbol, "", "", limit)
		if err != nil {
			return nil, err
		}
		return limitBars(mapItems(data), limit), nil
	}
	items, err := p.adapter.MinuteLimit(symbol, period, limit)
	if err != nil {
		return nil, err
	}
	return limitBars(items, limit), nil
}

func mapItems(data map[string]any) []map[string]any {
	raw, ok := data["items"]
	if !ok || raw == nil {
		return []map[string]any{}
	}
	if items, ok := raw.([]map[string]any); ok {
		return items
	}
	rawItems, ok := raw.([]any)
	if !ok {
		return []map[string]any{}
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		if row, ok := item.(map[string]any); ok {
			items = append(items, row)
		}
	}
	return items
}

func limitBars(items []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[len(items)-limit:]
}

// klineProviderForBacktest 实现 KLineProvider 接口
type klineProviderForBacktest struct {
	tdRepo *TdengineRepository
}

func (p klineProviderForBacktest) GetKLine(symbol, period string, limit int) ([]model.KlineBar, error) {
	return p.tdRepo.GetKLine(symbol, period, limit)
}

type TdengineRepository struct {
	url      string
	auth     string
	database string
	client   *http.Client
}

// PostgresRepository 使用 database/sql + lib/pq 连接，所有写入/查询走 $N 占位参数化，
// 杜绝 SQL 注入。结果查询统一为 select row_to_json(t)，单列 JSON 文本由调用方反序列化。
type PostgresRepository struct {
	db *sql.DB
}

// MarketDataService 是行情服务层：API / Worker 只应该依赖它。
// 它负责按市场选择底层 provider、fallback、以及统一输出契约。
type MarketDataService struct {
	cn     *MarketAPIClient
	python *PythonProvider
	tdx    *PythonProvider
}

// PythonProvider 是历史 Python provider：akshare / yfinance / Frankfurter fallback。
// 它不是服务层，只是一个底层数据源适配器。
type PythonProvider struct {
	pythonBin string
	script    string
}

// isCNSymbol 仅按字面识别国内 A 股/ETF 代码（与 market-api 主源覆盖范围一致）。
func isCNSymbol(symbol string) bool {
	s := strings.TrimSpace(symbol)
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type TaskRunner struct {
	repo    *PostgresRepository
	td      *TdengineRepository
	adapter *MarketDataService
}

type SearchResponse struct {
	Items []map[string]any `json:"items"`
}

type SnapshotResponse struct {
	Item map[string]any `json:"item"`
}

type BarsResponse struct {
	Items []map[string]any `json:"items"`
}

type SectorBoardsResponse struct {
	Items []map[string]any `json:"items"`
}

type createTaskRequest struct {
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name"`
	Market   string   `json:"market"`
	Interval string   `json:"interval"`
	Fields   []string `json:"fields"`
}

var defaultFields = []string{
	"price",
	"volume",
	"amount",
	"turnover_rate",
	"turnover_amount",
	"volume_ratio",
	"premium_ratio",
	"buy_sell_5",
	"order_flow",
}

func main() {
	server := newServer()
	addr := env("API_HOST", "0.0.0.0") + ":" + env("API_PORT", "8000")

	auth := newAuthConfig()
	if !auth.enabled {
		log.Printf("[backend-api] 警告：JWT 鉴权已通过 AUTH_ENABLED=false 关闭，仅限本地联调")
	} else if len(auth.secret) == 0 {
		log.Printf("[backend-api] 警告：未配置 SECRET_KEY，JWT 鉴权降级为放行，请在 .env 中设置")
	}

	log.Printf("[backend-api] listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, withAuth(server.mux, auth)))
}

// publicPaths 为无需鉴权的公开端点（健康检查、服务概览）。
var publicPaths = map[string]bool{
	"/healthz":                      true,
	"/api/v1/overview":              true,
	"/api/stock/v1/overview":        true,
	"/api/stock/v1/system/health":   true,
	"/api/stock/v1/system/overview": true,
}

// withAuth 在 mux 外层包一道鉴权：公开路径直接放行，其余走 JWT 校验。
func withAuth(mux *http.ServeMux, auth authConfig) http.Handler {
	protected := auth.requireAuth(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			mux.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})
}

func newServer() *Server {
	configRepo := NewPostgresRepository()
	if err := configRepo.Initialize(); err != nil {
		log.Printf("[backend-api] postgres initialize failed: %v", err)
	}
	tdRepo := NewTdengineRepository()
	if err := tdRepo.Initialize(); err != nil {
		log.Printf("[backend-api] tdengine initialize failed: %v", err)
	}
	adapter := NewMarketDataService()

	// 创建数据中心数据库连接
	dcDB := newDataCenterDB()

	server := &Server{
		mux:      http.NewServeMux(),
		taskRepo: configRepo,
		tdRepo:   tdRepo,
		adapter:  adapter,
		taskRunner: &TaskRunner{
			repo:    configRepo,
			td:      tdRepo,
			adapter: adapter,
		},
		// stale-while-revalidate 下，TTL 是"陈旧门槛"：超过后旧值仍立刻返回但触发后台刷新。
		// 10s 对秒级监控足够；用户感知是首次访问外永远命中缓存。
		snapshotCache:     newTTLCache(10 * time.Second),
		profileCache:      newTTLCache(60 * time.Minute),
		listCache:         newTTLCache(30 * time.Second),
		etfRiskCache:      newTTLCache(15 * time.Second),
		snapshotInflight:  newInflightSet(),
		systemHandler:     httpapi.NewSystemHandler(systemapp.NewService()),
		instrumentHandler: httpapi.NewInstrumentHandler(instrumentapp.NewService(adapter)),
		watchlistHandler:  httpapi.NewWatchlistHandler(watchlistapp.NewService(configRepo)),
		taskHandler:       httpapi.NewTaskHandler(taskapp.NewService(configRepo)),
		alertHandler:      httpapi.NewAlertHandler(alertapp.NewService(configRepo)),
		brokerHandler:     httpapi.NewBrokerHandler(brokerapp.NewService()),
		quantHandler:      httpapi.NewQuantHandler(quantapp.NewService(configRepo, tdRepo)),
		screenerHandler:   httpapi.NewScreenerHandler(screenerapp.NewService(tdRepo, configRepo)),
		backtestHandler:   httpapi.NewBacktestHandler(backtestapp.NewService(nil)),
		strategyHandler:   httpapi.NewStrategyHandler(strategyapp.NewService(configRepo)),
	}

	// 初始化数据中心服务
	server.datacenterService = datacenter.NewService(dcDB, nil)
	server.datacenterHandler = httpapi.NewDataCenterHandler(server.datacenterService)

	server.backtestHandler.SetKLineProvider(klineProviderForBacktest{tdRepo: tdRepo})
	server.marketHandler = httpapi.NewMarketHandler(marketapp.NewService(
		serverSnapshotProvider{server: server},
		liveBarsProvider{tdRepo: tdRepo, adapter: adapter},
		adapter,
	))
	server.paperHandler = httpapi.NewPaperHandler(paperapp.NewService(configRepo, serverSnapshotProvider{server: server}))
	server.routes()
	return server
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/api/v1/overview", s.handleOverview)
	s.mux.HandleFunc("/api/stock/v1/overview", s.handleOverview)

	s.systemHandler.Register(s.mux)
	s.instrumentHandler.Register(s.mux)
	s.marketHandler.Register(s.mux)
	s.watchlistHandler.Register(s.mux)
	s.taskHandler.Register(s.mux)
	s.alertHandler.Register(s.mux)
	s.paperHandler.Register(s.mux)
	s.brokerHandler.Register(s.mux)
	s.quantHandler.Register(s.mux)
	s.screenerHandler.Register(s.mux)
	s.backtestHandler.Register(s.mux)
	s.strategyHandler.Register(s.mux)
	s.datacenterHandler.Register(s.mux)

	// 初始化数据中心数据库表
	go func() {
		if err := s.datacenterService.InitSchema(context.Background()); err != nil {
			log.Printf("数据中心表初始化失败: %v", err)
		} else {
			log.Println("数据中心表初始化成功")
		}
	}()

	// 旧路由兼容
	s.mux.HandleFunc("/api/v1/instruments/search", s.handleSearch)
	s.mux.HandleFunc("/api/v1/instruments", s.handleInstruments)
	s.mux.HandleFunc("/api/v1/instruments/profile", s.handleProfile)
	s.mux.HandleFunc("/api/v1/market/sector-boards", s.handleSectorBoards)
	s.mux.HandleFunc("/api/v1/ingestion/tasks", s.handleTasks)
	s.mux.HandleFunc("/api/v1/market/realtime", s.handleRealtime)
	s.mux.HandleFunc("/api/v1/market/history", s.handleHistory)
	s.mux.HandleFunc("/api/v1/market/daily", s.handleDaily)
	s.mux.HandleFunc("/api/v1/etf/risk", s.handleETFRisk)
	s.mux.HandleFunc("/api/v1/alerts/rules", s.handleAlertRules)
	s.mux.HandleFunc("/api/v1/alerts/events", s.handleAlertEvents)
	s.mux.HandleFunc("/api/v1/alerts/events/ack", s.handleAlertEventAck)
	s.mux.HandleFunc("/api/v1/paper/orders", s.handlePaperOrders)
	s.mux.HandleFunc("/api/v1/paper/positions", s.handlePaperPositions)
	s.mux.HandleFunc("/api/v1/paper/account", s.handlePaperAccount)
	s.mux.HandleFunc("/api/v1/paper/reset", s.handlePaperReset)
	s.mux.HandleFunc("/api/v1/broker/status", s.handleBrokerStatus)
	s.mux.HandleFunc("/api/v1/watchlist/groups", s.handleWatchlistGroups)
	s.mux.HandleFunc("/api/v1/watchlist/items", s.handleWatchlistItems)
	s.mux.HandleFunc("/api/v1/watchlist/snapshot", s.handleWatchlistSnapshot)

	// 新路由 /api/stock/v1/* 别名（仅保留 internal handler 未覆盖的）
	s.mux.HandleFunc("/api/stock/v1/system/health", s.handleHealth)
	s.mux.HandleFunc("/api/stock/v1/system/overview", s.handleOverview)
	s.mux.HandleFunc("/api/stock/v1/market/sector-boards", s.handleSectorBoards)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "backend-api"})
}

func (s *Server) handleOverview(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"project": "stock-etf-monitor",
		"service": "backend-api",
		"status":  "ok",
		"architecture": map[string]any{
			"frontend": []string{"web"},
			"backend":  []string{"controller", "network", "logic", "model", "repository", "service"},
			"strategy": []string{"signals", "factors", "backtest"},
		},
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "keyword is required"})
		return
	}
	items, err := s.adapter.Search(keyword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleInstruments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	market := strings.TrimSpace(q.Get("market"))
	if market == "" {
		market = "all"
	}
	keyword := strings.TrimSpace(q.Get("keyword"))
	board := strings.TrimSpace(q.Get("board"))
	limit := 200
	offset := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit <= 0 || limit > 5000 {
			limit = 200
		}
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &offset); err != nil || offset < 0 {
			offset = 0
		}
	}
	cacheKey := fmt.Sprintf("%s|%s|%s|%d|%d", market, keyword, board, limit, offset)
	if cached, ok := s.listCache.Get(cacheKey); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	data, err := s.adapter.Instruments(market, keyword, board, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	s.listCache.Set(cacheKey, data)
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol is required"})
		return
	}
	// 60min TTL：板块/行业基本不变，Python 端也有 6h 文件缓存
	if cached, ok := s.profileCache.Get(symbol); ok {
		writeJSON(w, http.StatusOK, map[string]any{"item": cached, "cached": true})
		return
	}
	item, err := s.adapter.Profile(symbol)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	s.profileCache.Set(symbol, item)
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleSectorBoards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	boardType := strings.TrimSpace(q.Get("board_type"))
	if boardType == "" {
		boardType = "industry"
	}
	limit := 500
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit <= 0 || limit > 5000 {
			limit = 500
		}
	}
	cacheKey := fmt.Sprintf("sector_boards|%s|%d", boardType, limit)
	if cached, ok := s.listCache.Get(cacheKey); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	data, err := s.adapter.SectorBoards(boardType, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	s.listCache.Set(cacheKey, data)
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	symbol := strings.TrimSpace(q.Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol is required"})
		return
	}
	start := strings.TrimSpace(q.Get("start"))
	end := strings.TrimSpace(q.Get("end"))
	limit := 240
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit <= 0 || limit > 5000 {
			limit = 240
		}
	}
	data, err := s.adapter.Daily(symbol, start, end, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks, err := s.taskRepo.ListTasks()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": tasks})
	case http.MethodPost:
		var req createTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
			return
		}
		if strings.TrimSpace(req.Symbol) == "" || strings.TrimSpace(req.Interval) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol and interval are required"})
			return
		}
		now := time.Now().UTC()
		fields := normalizeFields(req.Fields)
		task := model.IngestionTask{
			TaskID:      fmt.Sprintf("task_%d", now.UnixNano()),
			Symbol:      strings.TrimSpace(req.Symbol),
			Name:        strings.TrimSpace(req.Name),
			Market:      fallback(strings.TrimSpace(req.Market), "CN-A"),
			Interval:    strings.TrimSpace(req.Interval),
			Fields:      fields,
			Enabled:     true,
			Status:      "pending",
			Source:      "akshare",
			CreatedAt:   now,
			UpdatedAt:   now,
			LastMessage: "created",
		}
		if err := s.taskRepo.UpsertTask(task); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": task})
	case http.MethodDelete:
		taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
		if taskID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "task_id is required"})
			return
		}
		if err := s.taskRepo.DeleteTask(taskID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"task_id": taskID, "deleted": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRealtime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol is required"})
		return
	}
	// 走统一的 stale-while-revalidate 路径：
	// - 新鲜值 → 0 延迟
	// - 陈旧值 → 0 延迟 + 后台异步刷新
	// - 完全缺失 → 阻塞同步等（仅冷启动首次）
	item, err := s.cachedSnapshot(symbol)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol is required"})
		return
	}
	interval := fallback(strings.TrimSpace(r.URL.Query().Get("interval")), "1m")
	limit := 120
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if _, err := fmt.Sscanf(rawLimit, "%d", &limit); err != nil || limit <= 0 || limit > 2000 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "limit must be between 1 and 2000"})
			return
		}
	}
	items, err := s.tdRepo.QueryBars(symbol, interval, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"symbol":   symbol,
		"interval": interval,
		"items":    items,
	})
}

func NewPostgresRepository() *PostgresRepository {
	host := env("POSTGRES_HOST", "postgres")
	port := env("POSTGRES_PORT", "5432") // docker-compose 设置的是 POSTGRES_PORT，不是 STOCK_DB_PORT
	db := env("STOCK_DB", "stock_etf")
	user := env("STOCK_DB_USER", "stock")
	password := env("STOCK_DB_PASSWORD", "tradehub_local_password")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, db)
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[backend-api] 打开 postgres 连接失败: %v", err)
		return &PostgresRepository{}
	}
	database.SetMaxOpenConns(20)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(5 * time.Minute)
	return &PostgresRepository{db: database}
}

// newDataCenterDB 创建数据中心专用数据库连接
func newDataCenterDB() *sql.DB {
	host := env("POSTGRES_HOST", "postgres")
	port := env("POSTGRES_PORT", "5432") // docker-compose 设置的是 POSTGRES_PORT，不是 STOCK_DB_PORT
	db := env("STOCK_DB", "stock_etf")
	user := env("STOCK_DB_USER", "stock")
	password := env("STOCK_DB_PASSWORD", "tradehub_local_password")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, db)

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[datacenter] 打开数据库失败: %v", err)
		return nil
	}

	// 设置连接池
	database.SetMaxOpenConns(20)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(5 * time.Minute)

	// 验证连接
	if err := database.Ping(); err != nil {
		log.Printf("[datacenter] 数据库连接失败: %v", err)
		return nil
	}

	return database
}

func (r *PostgresRepository) Initialize() error {
	sql := `
	create table if not exists ingestion_task_configs (
	  task_id varchar(64) primary key,
	  symbol varchar(32) not null,
	  name varchar(128) not null,
	  market varchar(32) not null,
	  interval varchar(16) not null,
	  fields jsonb not null default '[]'::jsonb,
	  enabled boolean not null default true,
	  status varchar(32) not null default 'pending',
	  source varchar(32) not null default 'akshare',
	  last_message text not null default '',
	  last_run_at timestamptz,
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	alter table ingestion_task_configs add column if not exists last_message text not null default '';
	alter table ingestion_task_configs add column if not exists last_run_at timestamptz;
	alter table ingestion_task_configs add column if not exists fields jsonb not null default '[]'::jsonb;
	create table if not exists instrument_configs (
	  symbol varchar(32) primary key,
	  name varchar(128) not null,
	  market varchar(32) not null,
	  enabled boolean not null default true,
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create table if not exists stock_tasks (
	  task_id varchar(64) primary key,
	  task_type varchar(64) not null,
	  status varchar(32) not null default 'pending',
	  params jsonb not null default '{}'::jsonb,
	  result_ref text not null default '',
	  progress int not null default 0,
	  attempts int not null default 0,
	  last_error text not null default '',
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  started_at timestamptz,
	  finished_at timestamptz,
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_tasks_type_status on stock_tasks(task_type, status, created_at desc);
	create table if not exists stock_task_logs (
	  log_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  level varchar(16) not null default 'info',
	  message text not null default '',
	  context jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	create index if not exists idx_stock_task_logs_task on stock_task_logs(task_id, created_at asc);
	create table if not exists stock_indicator_definitions (
	  indicator_code varchar(64) primary key,
	  name varchar(128) not null,
	  category varchar(64) not null default 'technical',
	  description text not null default '',
	  params_schema jsonb not null default '{}'::jsonb,
	  output_fields jsonb not null default '[]'::jsonb,
	  enabled boolean not null default true,
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_indicator_definitions_category on stock_indicator_definitions(category, enabled);
	create table if not exists stock_pattern_definitions (
	  pattern_code varchar(96) primary key,
	  name varchar(128) not null,
	  category varchar(64) not null default 'candlestick',
	  talib_function varchar(64) not null,
	  direction varchar(32) not null default 'both',
	  description text not null default '',
	  params_schema jsonb not null default '{}'::jsonb,
	  enabled boolean not null default true,
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_pattern_definitions_category on stock_pattern_definitions(category, enabled);
	create table if not exists stock_screening_templates (
	  template_id varchar(64) primary key,
	  name varchar(128) not null,
	  description text not null default '',
	  conditions jsonb not null default '{}'::jsonb,
	  enabled boolean not null default true,
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_screening_templates_enabled on stock_screening_templates(enabled, updated_at desc);
	create table if not exists stock_strategy_templates (
	  strategy_id varchar(64) primary key,
	  name varchar(128) not null,
	  description text not null default '',
	  screening_template_id varchar(64) not null default '',
	  conditions jsonb not null default '{}'::jsonb,
	  backtest_params jsonb not null default '{}'::jsonb,
	  risk_params jsonb not null default '{}'::jsonb,
	  enabled boolean not null default true,
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_strategy_templates_enabled on stock_strategy_templates(enabled, updated_at desc);
	create table if not exists stock_strategy_runs (
	  run_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  task_type varchar(64) not null,
	  strategy_id varchar(64) not null,
	  snapshot_id varchar(64) not null default '',
	  strategy_name varchar(128) not null default '',
	  status varchar(32) not null default 'pending',
	  result_ref text not null default '',
	  summary_ref text not null default '',
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_strategy_runs_strategy on stock_strategy_runs(strategy_id, created_at desc);
	create index if not exists idx_stock_strategy_runs_task on stock_strategy_runs(task_id, created_at desc);
	create table if not exists stock_screening_results (
	  result_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  template_id varchar(64) not null default '',
	  symbol varchar(32) not null,
	  score double precision not null default 0,
	  matched_conditions jsonb not null default '[]'::jsonb,
	  snapshot jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	create index if not exists idx_stock_screening_results_task on stock_screening_results(task_id, score desc, created_at desc);
	create index if not exists idx_stock_screening_results_template on stock_screening_results(template_id, created_at desc);
	create table if not exists stock_backtest_results (
	  result_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  symbol varchar(32) not null,
	  period varchar(16) not null default '1d',
	  entry_time timestamptz not null,
	  exit_time timestamptz not null,
	  entry_price double precision not null default 0,
	  exit_price double precision not null default 0,
	  return_pct double precision not null default 0,
	  benchmark_symbol varchar(32) not null default '',
	  benchmark_return_pct double precision not null default 0,
	  excess_return_pct double precision not null default 0,
	  meta jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	alter table stock_backtest_results add column if not exists benchmark_symbol varchar(32) not null default '';
	alter table stock_backtest_results add column if not exists benchmark_return_pct double precision not null default 0;
	alter table stock_backtest_results add column if not exists excess_return_pct double precision not null default 0;
	create index if not exists idx_stock_backtest_results_task on stock_backtest_results(task_id, return_pct desc, created_at desc);
	create index if not exists idx_stock_backtest_results_symbol on stock_backtest_results(symbol, created_at desc);
	create table if not exists stock_backtest_summaries (
	  summary_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  total_trades int not null default 0,
	  win_rate double precision not null default 0,
	  avg_return_pct double precision not null default 0,
	  total_return_pct double precision not null default 0,
	  max_drawdown_pct double precision not null default 0,
	  best_return_pct double precision not null default 0,
	  worst_return_pct double precision not null default 0,
	  benchmark_symbol varchar(32) not null default '',
	  benchmark_return_pct double precision not null default 0,
	  avg_excess_return_pct double precision not null default 0,
	  return_distribution jsonb not null default '{}'::jsonb,
	  meta jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	create index if not exists idx_stock_backtest_summaries_task on stock_backtest_summaries(task_id, created_at desc);
	insert into stock_indicator_definitions(indicator_code, name, category, description, params_schema, output_fields)
	values
	  ('MA', '移动平均线', 'trend', '按窗口计算收盘价简单移动平均，支撑均线选股、趋势判断和 K 线叠加。', '{"windows":[5,10,20,30,60,120,250]}'::jsonb, '["ma5","ma10","ma20","ma30","ma60","ma120","ma250"]'::jsonb),
	  ('EMA', '指数移动平均线', 'trend', '按窗口计算指数移动平均，是 MACD 等指标的基础。', '{"windows":[5,10,12,20,26,60]}'::jsonb, '["ema5","ema10","ema12","ema20","ema26","ema60"]'::jsonb),
	  ('MACD', '指数平滑异同移动平均线', 'trend', '计算 DIF、DEA、MACD 柱，支撑金叉、死叉、背离和 pythonstock 指标猜想。', '{"fast":12,"slow":26,"signal":9}'::jsonb, '["dif","dea","macd"]'::jsonb),
	  ('KDJ', '随机指标', 'momentum', '计算 K、D、J，支撑超买超卖、KDJ 均值猜想买入和风险卖出。', '{"n":9,"m1":3,"m2":3}'::jsonb, '["kdjk","kdjd","kdjj"]'::jsonb),
	  ('BOLL', '布林线', 'volatility', '计算中轨、上轨、下轨，支撑通道突破和波动率判断。', '{"window":20,"std":2}'::jsonb, '["boll_mid","boll_upper","boll_lower"]'::jsonb),
	  ('RSI', '相对强弱指标', 'momentum', '计算多窗口 RSI，支撑超买超卖和动量选股。', '{"windows":[6,12,24]}'::jsonb, '["rsi6","rsi12","rsi24"]'::jsonb),
	  ('TRIX', '三重指数平滑平均线', 'trend', '计算 TRIX 和矩阵均线，支撑中期趋势判断。', '{"window":12,"ma":20}'::jsonb, '["trix","trix_ma"]'::jsonb),
	  ('VR', '成交量变异率', 'volume', '基于成交量上涨/下跌关系衡量量能强弱，支撑 pythonstock 指标猜想。', '{"window":26}'::jsonb, '["vr"]'::jsonb),
	  ('WR', '威廉指标', 'momentum', '衡量收盘价在周期高低区间的位置，支撑短线超买超卖。', '{"windows":[10,14]}'::jsonb, '["wr10","wr14"]'::jsonb)
	on conflict (indicator_code) do nothing;
	insert into stock_pattern_definitions(pattern_code, name, category, talib_function, direction, description, params_schema)
	values
	  ('tow_crows', '两只乌鸦', 'candlestick', 'CDL2CROWS', 'bearish', 'InStock / myhhub 形态字段：两只乌鸦。', '{}'::jsonb),
	  ('upside_gap_two_crows', '向上跳空的两只乌鸦', 'candlestick', 'CDLUPSIDEGAP2CROWS', 'bearish', 'InStock / myhhub 形态字段：向上跳空的两只乌鸦。', '{}'::jsonb),
	  ('three_black_crows', '三只乌鸦', 'candlestick', 'CDL3BLACKCROWS', 'bearish', 'InStock / myhhub 形态字段：三只乌鸦。', '{}'::jsonb),
	  ('identical_three_crows', '三胞胎乌鸦', 'candlestick', 'CDLIDENTICAL3CROWS', 'bearish', 'InStock / myhhub 形态字段：三胞胎乌鸦。', '{}'::jsonb),
	  ('three_line_strike', '三线打击', 'candlestick', 'CDL3LINESTRIKE', 'both', 'InStock / myhhub 形态字段：三线打击。', '{}'::jsonb),
	  ('dark_cloud_cover', '乌云压顶', 'candlestick', 'CDLDARKCLOUDCOVER', 'bearish', 'InStock / myhhub 形态字段：乌云压顶。', '{}'::jsonb),
	  ('evening_doji_star', '十字暮星', 'candlestick', 'CDLEVENINGDOJISTAR', 'bearish', 'InStock / myhhub 形态字段：十字暮星。', '{}'::jsonb),
	  ('doji_Star', '十字星', 'candlestick', 'CDLDOJISTAR', 'neutral', 'InStock / myhhub 形态字段：十字星。', '{}'::jsonb),
	  ('hanging_man', '上吊线', 'candlestick', 'CDLHANGINGMAN', 'bearish', 'InStock / myhhub 形态字段：上吊线。', '{}'::jsonb),
	  ('hikkake_pattern', '陷阱', 'candlestick', 'CDLHIKKAKE', 'both', 'InStock / myhhub 形态字段：陷阱。', '{}'::jsonb),
	  ('modified_hikkake_pattern', '修正陷阱', 'candlestick', 'CDLHIKKAKEMOD', 'both', 'InStock / myhhub 形态字段：修正陷阱。', '{}'::jsonb),
	  ('in_neck_pattern', '颈内线', 'candlestick', 'CDLINNECK', 'bearish', 'InStock / myhhub 形态字段：颈内线。', '{}'::jsonb),
	  ('on_neck_pattern', '颈上线', 'candlestick', 'CDLONNECK', 'bearish', 'InStock / myhhub 形态字段：颈上线。', '{}'::jsonb),
	  ('thrusting_pattern', '插入', 'candlestick', 'CDLTHRUSTING', 'bearish', 'InStock / myhhub 形态字段：插入。', '{}'::jsonb),
	  ('shooting_star', '射击之星', 'candlestick', 'CDLSHOOTINGSTAR', 'bearish', 'InStock / myhhub 形态字段：射击之星。', '{}'::jsonb),
	  ('stalled_pattern', '停顿形态', 'candlestick', 'CDLSTALLEDPATTERN', 'bearish', 'InStock / myhhub 形态字段：停顿形态。', '{}'::jsonb),
	  ('advance_block', '大敌当前', 'candlestick', 'CDLADVANCEBLOCK', 'bearish', 'InStock / myhhub 形态字段：大敌当前。', '{}'::jsonb),
	  ('high_wave_candle', '风高浪大线', 'candlestick', 'CDLHIGHWAVE', 'neutral', 'InStock / myhhub 形态字段：风高浪大线。', '{}'::jsonb),
	  ('engulfing_pattern', '吞噬模式', 'candlestick', 'CDLENGULFING', 'both', 'InStock / myhhub 形态字段：吞噬模式。', '{}'::jsonb),
	  ('abandoned_baby', '弃婴', 'candlestick', 'CDLABANDONEDBABY', 'both', 'InStock / myhhub 形态字段：弃婴。', '{}'::jsonb),
	  ('closing_marubozu', '收盘缺影线', 'candlestick', 'CDLCLOSINGMARUBOZU', 'both', 'InStock / myhhub 形态字段：收盘缺影线。', '{}'::jsonb),
	  ('doji', '十字', 'candlestick', 'CDLDOJI', 'neutral', 'InStock / myhhub 形态字段：十字。', '{}'::jsonb),
	  ('up_down_gap', '向上/下跳空并列阳线', 'candlestick', 'CDLGAPSIDESIDEWHITE', 'both', 'InStock / myhhub 形态字段：向上/下跳空并列阳线。', '{}'::jsonb),
	  ('long_legged_doji', '长脚十字', 'candlestick', 'CDLLONGLEGGEDDOJI', 'neutral', 'InStock / myhhub 形态字段：长脚十字。', '{}'::jsonb),
	  ('rickshaw_man', '黄包车夫', 'candlestick', 'CDLRICKSHAWMAN', 'neutral', 'InStock / myhhub 形态字段：黄包车夫。', '{}'::jsonb),
	  ('marubozu', '光头光脚/缺影线', 'candlestick', 'CDLMARUBOZU', 'both', 'InStock / myhhub 形态字段：光头光脚/缺影线。', '{}'::jsonb),
	  ('three_inside_up_down', '三内部上涨和下跌', 'candlestick', 'CDL3INSIDE', 'both', 'InStock / myhhub 形态字段：三内部上涨和下跌。', '{}'::jsonb),
	  ('three_outside_up_down', '三外部上涨和下跌', 'candlestick', 'CDL3OUTSIDE', 'both', 'InStock / myhhub 形态字段：三外部上涨和下跌。', '{}'::jsonb),
	  ('three_stars_in_the_south', '南方三星', 'candlestick', 'CDL3STARSINSOUTH', 'bullish', 'InStock / myhhub 形态字段：南方三星。', '{}'::jsonb),
	  ('three_white_soldiers', '三个白兵', 'candlestick', 'CDL3WHITESOLDIERS', 'bullish', 'InStock / myhhub 形态字段：三个白兵。', '{}'::jsonb),
	  ('belt_hold', '捉腰带线', 'candlestick', 'CDLBELTHOLD', 'both', 'InStock / myhhub 形态字段：捉腰带线。', '{}'::jsonb),
	  ('breakaway', '脱离', 'candlestick', 'CDLBREAKAWAY', 'both', 'InStock / myhhub 形态字段：脱离。', '{}'::jsonb),
	  ('concealing_baby_swallow', '藏婴吞没', 'candlestick', 'CDLCONCEALBABYSWALL', 'bullish', 'InStock / myhhub 形态字段：藏婴吞没。', '{}'::jsonb),
	  ('counterattack', '反击线', 'candlestick', 'CDLCOUNTERATTACK', 'both', 'InStock / myhhub 形态字段：反击线。', '{}'::jsonb),
	  ('dragonfly_doji', '蜻蜓十字/T形十字', 'candlestick', 'CDLDRAGONFLYDOJI', 'neutral', 'InStock / myhhub 形态字段：蜻蜓十字/T形十字。', '{}'::jsonb),
	  ('evening_star', '暮星', 'candlestick', 'CDLEVENINGSTAR', 'bearish', 'InStock / myhhub 形态字段：暮星。', '{}'::jsonb),
	  ('gravestone_doji', '墓碑十字/倒T十字', 'candlestick', 'CDLGRAVESTONEDOJI', 'neutral', 'InStock / myhhub 形态字段：墓碑十字/倒T十字。', '{}'::jsonb),
	  ('hammer', '锤头', 'candlestick', 'CDLHAMMER', 'bullish', 'InStock / myhhub 形态字段：锤头。', '{}'::jsonb),
	  ('harami_pattern', '母子线', 'candlestick', 'CDLHARAMI', 'both', 'InStock / myhhub 形态字段：母子线。', '{}'::jsonb),
	  ('harami_cross_pattern', '十字孕线', 'candlestick', 'CDLHARAMICROSS', 'both', 'InStock / myhhub 形态字段：十字孕线。', '{}'::jsonb),
	  ('homing_pigeon', '家鸽', 'candlestick', 'CDLHOMINGPIGEON', 'bullish', 'InStock / myhhub 形态字段：家鸽。', '{}'::jsonb),
	  ('inverted_hammer', '倒锤头', 'candlestick', 'CDLINVERTEDHAMMER', 'bullish', 'InStock / myhhub 形态字段：倒锤头。', '{}'::jsonb),
	  ('kicking', '反冲形态', 'candlestick', 'CDLKICKING', 'both', 'InStock / myhhub 形态字段：反冲形态。', '{}'::jsonb),
	  ('kicking_bull_bear', '由较长缺影线决定的反冲形态', 'candlestick', 'CDLKICKINGBYLENGTH', 'both', 'InStock / myhhub 形态字段：由较长缺影线决定的反冲形态。', '{}'::jsonb),
	  ('ladder_bottom', '梯底', 'candlestick', 'CDLLADDERBOTTOM', 'bullish', 'InStock / myhhub 形态字段：梯底。', '{}'::jsonb),
	  ('long_line_candle', '长蜡烛', 'candlestick', 'CDLLONGLINE', 'both', 'InStock / myhhub 形态字段：长蜡烛。', '{}'::jsonb),
	  ('matching_low', '相同低价', 'candlestick', 'CDLMATCHINGLOW', 'bullish', 'InStock / myhhub 形态字段：相同低价。', '{}'::jsonb),
	  ('mat_hold', '铺垫', 'candlestick', 'CDLMATHOLD', 'bullish', 'InStock / myhhub 形态字段：铺垫。', '{}'::jsonb),
	  ('morning_doji_star', '十字晨星', 'candlestick', 'CDLMORNINGDOJISTAR', 'bullish', 'InStock / myhhub 形态字段：十字晨星。', '{}'::jsonb),
	  ('morning_star', '晨星', 'candlestick', 'CDLMORNINGSTAR', 'bullish', 'InStock / myhhub 形态字段：晨星。', '{}'::jsonb),
	  ('piercing_pattern', '刺透形态', 'candlestick', 'CDLPIERCING', 'bullish', 'InStock / myhhub 形态字段：刺透形态。', '{}'::jsonb),
	  ('rising_falling_three', '上升/下降三法', 'candlestick', 'CDLRISEFALL3METHODS', 'both', 'InStock / myhhub 形态字段：上升/下降三法。', '{}'::jsonb),
	  ('separating_lines', '分离线', 'candlestick', 'CDLSEPARATINGLINES', 'both', 'InStock / myhhub 形态字段：分离线。', '{}'::jsonb),
	  ('short_line_candle', '短蜡烛', 'candlestick', 'CDLSHORTLINE', 'both', 'InStock / myhhub 形态字段：短蜡烛。', '{}'::jsonb),
	  ('spinning_top', '纺锤', 'candlestick', 'CDLSPINNINGTOP', 'both', 'InStock / myhhub 形态字段：纺锤。', '{}'::jsonb),
	  ('stick_sandwich', '条形三明治', 'candlestick', 'CDLSTICKSANDWICH', 'bullish', 'InStock / myhhub 形态字段：条形三明治。', '{}'::jsonb),
	  ('takuri', '探水竿', 'candlestick', 'CDLTAKURI', 'bullish', 'InStock / myhhub 形态字段：探水竿。', '{}'::jsonb),
	  ('tasuki_gap', '跳空并列阴阳线', 'candlestick', 'CDLTASUKIGAP', 'both', 'InStock / myhhub 形态字段：跳空并列阴阳线。', '{}'::jsonb),
	  ('tristar_pattern', '三星', 'candlestick', 'CDLTRISTAR', 'both', 'InStock / myhhub 形态字段：三星。', '{}'::jsonb),
	  ('unique_3_river', '奇特三河床', 'candlestick', 'CDLUNIQUE3RIVER', 'bullish', 'InStock / myhhub 形态字段：奇特三河床。', '{}'::jsonb),
	  ('upside_downside_gap', '上升/下降跳空三法', 'candlestick', 'CDLXSIDEGAP3METHODS', 'both', 'InStock / myhhub 形态字段：上升/下降跳空三法。', '{}'::jsonb)
	on conflict (pattern_code) do nothing;
	create table if not exists alert_rules (
	  rule_id varchar(64) primary key,
	  symbol varchar(32) not null,
	  name varchar(128) not null,
	  market varchar(32) not null default 'CN-A',
	  metric varchar(32) not null,
	  op varchar(8) not null,
	  threshold double precision not null,
	  cooldown_seconds int not null default 300,
	  enabled boolean not null default true,
	  last_triggered_at timestamptz,
	  last_value double precision not null default 0,
	  last_message text not null default '',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create table if not exists alert_events (
	  event_id varchar(64) primary key,
	  rule_id varchar(64) not null,
	  symbol varchar(32) not null,
	  name varchar(128) not null,
	  metric varchar(32) not null,
	  op varchar(8) not null,
	  threshold double precision not null,
	  value double precision not null,
	  status varchar(16) not null default 'open',
	  message text not null default '',
	  triggered_at timestamptz not null default now(),
	  ack_at timestamptz
	);
	create index if not exists idx_alert_events_status on alert_events(status, triggered_at desc);
	create table if not exists paper_orders (
	  order_id varchar(64) primary key,
	  symbol varchar(32) not null,
	  name varchar(128) not null default '',
	  market varchar(32) not null default 'CN-A',
	  side varchar(8) not null,
	  qty double precision not null,
	  price double precision not null,
	  amount double precision not null,
	  fee double precision not null default 0,
	  status varchar(16) not null default 'filled',
	  note text not null default '',
	  placed_at timestamptz not null default now(),
	  filled_at timestamptz
	);
	create index if not exists idx_paper_orders_symbol on paper_orders(symbol, placed_at desc);
	create table if not exists paper_account (
	  id int primary key default 1,
	  cash double precision not null default 1000000,
	  initial double precision not null default 1000000,
	  realized_pl double precision not null default 0,
	  updated_at timestamptz not null default now()
	);
	insert into paper_account(id) values (1) on conflict (id) do nothing;
	-- 自选池：分组 + 标的；删除分组级联删除标的
	create table if not exists watchlist_groups (
	  group_id varchar(64) primary key,
	  name varchar(128) not null,
	  sort_order int not null default 0,
	  created_at timestamptz not null default now()
	);
	create table if not exists watchlist_items (
	  item_id varchar(64) primary key,
	  group_id varchar(64) not null references watchlist_groups(group_id) on delete cascade,
	  symbol varchar(32) not null,
	  name varchar(128) not null default '',
	  market varchar(32) not null default '',
	  note text not null default '',
	  sort_order int not null default 0,
	  created_at timestamptz not null default now(),
	  unique (group_id, symbol)
	);
	create index if not exists idx_watchlist_items_group on watchlist_items(group_id, sort_order);
	-- 默认分组：首次启动给一个空的默认组，让前端有承载
	insert into watchlist_groups(group_id, name, sort_order)
	  values ('default', '默认分组', 0) on conflict (group_id) do nothing;
	`
	return r.execRaw(sql)
}

func (r *PostgresRepository) UpsertTask(task model.IngestionTask) error {
	fieldsJSON, err := json.Marshal(normalizeFields(task.Fields))
	if err != nil {
		return err
	}
	taskStmt := sqlStmt{
		query: `
	insert into ingestion_task_configs
	  (task_id, symbol, name, market, interval, fields, enabled, status, source, last_message, last_run_at, created_at, updated_at)
	values
	  ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12, $13)
	on conflict (task_id) do update set
	  symbol = excluded.symbol,
	  name = excluded.name,
	  market = excluded.market,
	  interval = excluded.interval,
	  fields = excluded.fields,
	  enabled = excluded.enabled,
	  status = excluded.status,
	  source = excluded.source,
	  last_message = excluded.last_message,
	  last_run_at = excluded.last_run_at,
	  updated_at = excluded.updated_at;`,
		args: []any{
			task.TaskID,
			task.Symbol,
			task.Name,
			task.Market,
			task.Interval,
			string(fieldsJSON),
			task.Enabled,
			task.Status,
			task.Source,
			task.LastMessage,
			nullableTime(task.LastRunAt),
			task.CreatedAt,
			task.UpdatedAt,
		},
	}
	instrumentStmt := sqlStmt{
		query: `
	insert into instrument_configs
	  (symbol, name, market, enabled, created_at, updated_at)
	values
	  ($1, $2, $3, true, $4, $5)
	on conflict (symbol) do update set
	  name = excluded.name,
	  market = excluded.market,
	  updated_at = excluded.updated_at;`,
		args: []any{
			task.Symbol,
			task.Name,
			task.Market,
			task.CreatedAt,
			task.UpdatedAt,
		},
	}
	return r.execTx([]sqlStmt{taskStmt, instrumentStmt})
}

func (r *PostgresRepository) ListTasks() ([]model.IngestionTask, error) {
	sql := `
	select row_to_json(t)
	from (
	  select
	    task_id,
	    symbol,
	    name,
	    market,
	    interval,
	    fields,
	    enabled,
	    status,
	    source,
	    created_at,
	    updated_at,
	    coalesce(last_run_at, 'epoch'::timestamptz) as last_run_at,
	    last_message
	  from ingestion_task_configs
	  order by created_at desc
	) t;
	`
	lines, err := r.queryLines(sql)
	if err != nil {
		return nil, err
	}
	tasks := make([]model.IngestionTask, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var task model.IngestionTask
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (r *PostgresRepository) GetTask(taskID string) (*model.IngestionTask, error) {
	sql := `
	select row_to_json(t)
	from (
	  select
	    task_id,
	    symbol,
	    name,
	    market,
	    interval,
	    fields,
	    enabled,
	    status,
	    source,
	    created_at,
	    updated_at,
	    coalesce(last_run_at, 'epoch'::timestamptz) as last_run_at,
	    last_message
	  from ingestion_task_configs
	  where task_id = $1
	) t;
	`
	lines, err := r.queryLines(sql, taskID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	var task model.IngestionTask
	if err := json.Unmarshal([]byte(lines[0]), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// DeleteTask 删除一条采集任务。
// 注意：当前不级联删除 TDengine 里已落库的历史 K 线 —— 历史数据本身有保留价值，
// 任务删除只表示"停止继续采集"。若需清理时间序列数据，走单独的清理流程。
func (r *PostgresRepository) DeleteTask(taskID string) error {
	return r.exec(`delete from ingestion_task_configs where task_id = $1;`, taskID)
}

func (r *PostgresRepository) UpdateTaskStatus(taskID, status, message string) error {
	return r.exec(`
	update ingestion_task_configs
	set status = $1,
	    last_message = $2,
	    last_run_at = now(),
	    updated_at = now()
	where task_id = $3;
	`, status, message, taskID)
}

// exec 执行一条写入语句，参数通过 $N 占位符绑定，杜绝注入。
func (r *PostgresRepository) exec(query string, args ...any) error {
	if r.db == nil {
		return fmt.Errorf("postgres 未连接")
	}
	if _, err := r.db.Exec(query, args...); err != nil {
		return fmt.Errorf("postgres exec failed: %w", err)
	}
	return nil
}

// execTx 在单个事务里按顺序执行多条语句（每条可带自己的参数）。
// lib/pq 扩展协议不支持一次 Exec 跑多条带占位符的语句，故拆成事务。
func (r *PostgresRepository) execTx(stmts []sqlStmt) error {
	if r.db == nil {
		return fmt.Errorf("postgres 未连接")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("postgres begin failed: %w", err)
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s.query, s.args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("postgres tx exec failed: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres commit failed: %w", err)
	}
	return nil
}

// execRaw 执行不带参数的多语句脚本（建表/初始化），走简单协议。
func (r *PostgresRepository) execRaw(script string) error {
	if r.db == nil {
		return fmt.Errorf("postgres 未连接")
	}
	if _, err := r.db.Exec(script); err != nil {
		return fmt.Errorf("postgres exec failed: %w", err)
	}
	return nil
}

// sqlStmt 是事务内的一条带参数语句。
type sqlStmt struct {
	query string
	args  []any
}

// queryLines 执行 select row_to_json(t) 查询，把单列 JSON 文本逐行收集为 []string，
// 调用方按既有逻辑 json.Unmarshal 每行即可，输出契约与原 psql -t -A 一致。
func (r *PostgresRepository) queryLines(query string, args ...any) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("postgres 未连接")
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query failed: %w", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line sql.NullString
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("postgres scan failed: %w", err)
		}
		if line.Valid && strings.TrimSpace(line.String) != "" {
			lines = append(lines, line.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres rows failed: %w", err)
	}
	return lines, nil
}

func NewTdengineRepository() *TdengineRepository {
	host := env("TDENGINE_HOST", "tdengine")
	port := env("TDENGINE_PORT", "6041")
	user := env("TDENGINE_USER", "root")
	password := env("TDENGINE_PASSWORD", "taosdata")
	return &TdengineRepository{
		url:      fmt.Sprintf("http://%s:%s/rest/sql", host, port),
		auth:     "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password)),
		database: env("TDENGINE_DATABASE", "stock_etf_ts"),
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (r *TdengineRepository) Initialize() error {
	if _, err := r.Exec("create database if not exists " + r.database); err != nil {
		return err
	}
	if _, err := r.Exec(fmt.Sprintf(`
create stable if not exists %s.market_bars (
  ts timestamp,
  open double,
  close double,
  high double,
  low double,
  volume double,
  amount double,
  turnover_rate double,
  turnover_amount double,
  volume_ratio double,
  premium_ratio double,
  big_order_volume double,
  medium_order_volume double,
  small_order_volume double,
  bid_1_price double,
  bid_1_volume double,
  bid_2_price double,
  bid_2_volume double,
  bid_3_price double,
  bid_3_volume double,
  bid_4_price double,
  bid_4_volume double,
  bid_5_price double,
  bid_5_volume double,
  ask_1_price double,
  ask_1_volume double,
  ask_2_price double,
  ask_2_volume double,
  ask_3_price double,
  ask_3_volume double,
  ask_4_price double,
  ask_4_volume double,
  ask_5_price double,
  ask_5_volume double,
  requested_period binary(16),
  source_period binary(16)
) tags (
  symbol binary(16),
  market binary(16),
  period binary(16)
)`, r.database)); err != nil {
		return err
	}
	if _, err := r.Exec(fmt.Sprintf(`
create stable if not exists %s.indicator_values (
  ts timestamp,
  indicator_value double,
  values_json nchar(2048)
) tags (
  symbol binary(16),
  period binary(16),
  indicator_code binary(64)
)`, r.database)); err != nil {
		return err
	}
	if _, err := r.Exec(fmt.Sprintf(`
create stable if not exists %s.pattern_hits (
  ts timestamp,
  pattern_value int,
  direction binary(16),
  extra_json nchar(2048),
  algorithm_version binary(64)
) tags (
  symbol binary(16),
  period binary(16),
  pattern_code binary(96)
)`, r.database)); err != nil {
		return err
	}
	// 兼容老库：超级表已存在但没有 period tag 的情况，尝试 ALTER ADD TAG（已存在则忽略）
	if _, err := r.Exec(fmt.Sprintf(`alter stable %s.market_bars add tag period binary(16)`, r.database)); err != nil {
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "already") && !strings.Contains(msg, "duplicate") && !strings.Contains(msg, "exist") {
			return err
		}
	}
	return nil
}

// periodSuffix 把任务周期映射成 TDengine 子表名安全的后缀。
// 「秒级」无法作为标识符，统一映射成 tick；其余周期天然合法直接复用。
func periodSuffix(interval string) string {
	switch interval {
	case "秒级", "tick", "":
		return "tick"
	case "5s", "10s", "30s", "1m", "5m", "10m", "30m", "1h", "1d":
		return interval
	default:
		return sanitizeIdentifier(interval)
	}
}

func (r *TdengineRepository) InsertBars(symbol, market, interval string, fields []string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	selected := makeFieldSet(fields)
	suffix := periodSuffix(interval)
	table := fmt.Sprintf("%s.bars_%s_%s", r.database, sanitizeIdentifier(symbol), suffix)
	if _, err := r.Exec(fmt.Sprintf(`create table if not exists %s using %s.market_bars tags ("%s", "%s", "%s")`,
		table, r.database, escapeDoubleQuote(symbol), escapeDoubleQuote(market), escapeDoubleQuote(suffix))); err != nil {
		return err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, fmt.Sprintf(
			`(%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
			tdTimestampLiteral(row["ts"]),
			numericValue(row, selected, "price", "open"),
			numericValue(row, selected, "price", "close"),
			numericValue(row, selected, "price", "high"),
			numericValue(row, selected, "price", "low"),
			numericValue(row, selected, "volume", "volume"),
			numericValue(row, selected, "amount", "amount"),
			numericValue(row, selected, "turnover_rate", "turnover_rate"),
			numericValue(row, selected, "turnover_amount", "turnover_amount"),
			numericValue(row, selected, "volume_ratio", "volume_ratio"),
			numericValue(row, selected, "premium_ratio", "premium_ratio"),
			numericValue(row, selected, "order_flow", "big_order_volume"),
			numericValue(row, selected, "order_flow", "medium_order_volume"),
			numericValue(row, selected, "order_flow", "small_order_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_1_price"),
			numericValue(row, selected, "buy_sell_5", "bid_1_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_2_price"),
			numericValue(row, selected, "buy_sell_5", "bid_2_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_3_price"),
			numericValue(row, selected, "buy_sell_5", "bid_3_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_4_price"),
			numericValue(row, selected, "buy_sell_5", "bid_4_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_5_price"),
			numericValue(row, selected, "buy_sell_5", "bid_5_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_1_price"),
			numericValue(row, selected, "buy_sell_5", "ask_1_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_2_price"),
			numericValue(row, selected, "buy_sell_5", "ask_2_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_3_price"),
			numericValue(row, selected, "buy_sell_5", "ask_3_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_4_price"),
			numericValue(row, selected, "buy_sell_5", "ask_4_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_5_price"),
			numericValue(row, selected, "buy_sell_5", "ask_5_volume"),
			sqlStringLiteral(fmt.Sprintf("%v", row["requested_period"])),
			sqlStringLiteral(fmt.Sprintf("%v", row["source_period"])),
		))
	}
	_, err := r.Exec("insert into " + table + " values " + strings.Join(values, ","))
	return err
}

func (r *TdengineRepository) Exec(sql string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.url, bytes.NewBufferString(sql))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", r.auth)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tdengine error: %s", string(body))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if code, ok := payload["code"].(float64); ok && code != 0 {
		return nil, fmt.Errorf("tdengine error: %s", string(body))
	}
	return payload, nil
}

func (r *TdengineRepository) QueryBars(symbol, requestedPeriod string, limit int) ([]map[string]any, error) {
	suffix := periodSuffix(requestedPeriod)
	table := fmt.Sprintf("%s.bars_%s_%s", r.database, sanitizeIdentifier(symbol), suffix)
	sql := fmt.Sprintf(`
select
  ts,
  open,
  close,
  high,
  low,
  volume,
  amount,
  turnover_rate,
  turnover_amount,
  volume_ratio,
  premium_ratio,
  big_order_volume,
  medium_order_volume,
  small_order_volume,
  bid_1_price,
  bid_1_volume,
  bid_2_price,
  bid_2_volume,
  bid_3_price,
  bid_3_volume,
  bid_4_price,
  bid_4_volume,
  bid_5_price,
  bid_5_volume,
  ask_1_price,
  ask_1_volume,
  ask_2_price,
  ask_2_volume,
  ask_3_price,
  ask_3_volume,
  ask_4_price,
  ask_4_volume,
  ask_5_price,
  ask_5_volume,
  requested_period,
  source_period
from %s
order by ts desc
limit %d`, table, limit)
	payload, err := r.Exec(sql)
	if err != nil {
		// 子表不存在视为空结果，避免任务还没跑前端就报错
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "not exist") ||
			strings.Contains(msg, "table does not exist") || strings.Contains(msg, "invalid") {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	columnMeta, ok := payload["column_meta"].([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	dataRows, ok := payload["data"].([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	columns := make([]string, 0, len(columnMeta))
	for _, item := range columnMeta {
		meta, ok := item.([]any)
		if !ok || len(meta) == 0 {
			continue
		}
		columns = append(columns, fmt.Sprintf("%v", meta[0]))
	}
	items := make([]map[string]any, 0, len(dataRows))
	for i := len(dataRows) - 1; i >= 0; i-- {
		row, ok := dataRows[i].([]any)
		if !ok {
			continue
		}
		record := map[string]any{}
		for idx, column := range columns {
			if idx < len(row) {
				record[column] = row[idx]
			}
		}
		items = append(items, record)
	}
	return items, nil
}

// GetKLine 获取K线数据，返回标准K线格式
func (r *TdengineRepository) GetKLine(symbol, period string, limit int) ([]model.KlineBar, error) {
	rows, err := r.QueryBars(symbol, period, limit)
	if err != nil {
		return nil, err
	}

	bars := make([]model.KlineBar, 0, len(rows))
	for _, row := range rows {
		ts, ok := row["ts"].(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			// 尝试其他格式
			t, _ = time.Parse("2006-01-02 15:04:05", ts)
		}

		bar := model.KlineBar{
			Symbol: symbol,
			Period: period,
			TS:     t,
			Open:   toFloat64(row["open"]),
			High:   toFloat64(row["high"]),
			Low:    toFloat64(row["low"]),
			Close:  toFloat64(row["close"]),
			Volume: toFloat64(row["volume"]),
			Amount: toFloat64(row["amount"]),
		}
		bars = append(bars, bar)
	}

	// 反转使时间升序
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
	return bars, nil
}

func toFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return 0
	}
}

func (r *TdengineRepository) QueryIndicatorValues(symbol, period, indicatorCode string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 5000 {
		limit = 200
	}
	suffix := periodSuffix(period)
	table := fmt.Sprintf("%s.ind_%s_%s_%s", r.database, sanitizeIdentifier(symbol), suffix, sanitizeIdentifier(indicatorCode))
	sql := fmt.Sprintf(`
select ts, indicator_value, values_json
from %s
order by ts desc
limit %d`, table, limit)
	items, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	for _, item := range items {
		if _, ok := item["value"]; !ok {
			item["value"] = item["indicator_value"]
		}
		item["symbol"] = symbol
		item["period"] = suffix
		item["indicator_code"] = strings.ToUpper(indicatorCode)
	}
	return reverseRows(items), nil
}

func (r *TdengineRepository) QueryPatternHits(symbol, period, patternCode string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 5000 {
		limit = 200
	}
	suffix := periodSuffix(period)
	var sql string
	if strings.TrimSpace(patternCode) != "" {
		table := fmt.Sprintf("%s.pattern_%s_%s_%s", r.database, sanitizeIdentifier(symbol), suffix, sanitizeIdentifier(patternCode))
		sql = fmt.Sprintf(`
select ts, pattern_value, direction, extra_json, algorithm_version
from %s
order by ts desc
limit %d`, table, limit)
	} else {
		sql = fmt.Sprintf(`
select ts, pattern_value, direction, extra_json, algorithm_version, pattern_code
from %s.pattern_hits
where symbol = '%s' and period = '%s'
order by ts desc
limit %d`, r.database, escape(symbol), escape(suffix), limit)
	}
	items, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	for _, item := range items {
		item["symbol"] = symbol
		item["period"] = suffix
		if strings.TrimSpace(patternCode) != "" {
			item["pattern_code"] = patternCode
		}
	}
	return reverseRows(items), nil
}

func (r *TdengineRepository) ScreenByIndicator(period, indicatorCode, field, op string, threshold float64, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	suffix := periodSuffix(period)
	code := strings.ToUpper(strings.TrimSpace(indicatorCode))
	sql := fmt.Sprintf(`
select ts, indicator_value, values_json, symbol, period, indicator_code
from %s.indicator_values
where period = '%s' and indicator_code = '%s'
order by ts desc
limit %d`, r.database, escape(suffix), escape(code), limit*20)
	rows, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	items := make([]map[string]any, 0, limit)
	seen := map[string]struct{}{}
	for _, row := range rows {
		if _, ok := row["value"]; !ok {
			row["value"] = row["indicator_value"]
		}
		symbol := strings.TrimSpace(fmt.Sprintf("%v", row["symbol"]))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		value, ok := indicatorFieldValue(row, field)
		if !ok || !compareFloat(value, op, threshold) {
			continue
		}
		row["field"] = field
		row["field_value"] = value
		row["threshold"] = threshold
		seen[symbol] = struct{}{}
		items = append(items, row)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *TdengineRepository) ScreenByPattern(period, patternCode, direction string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	suffix := periodSuffix(period)
	code := strings.TrimSpace(patternCode)
	sql := fmt.Sprintf(`
select ts, pattern_value, direction, extra_json, algorithm_version, symbol, period, pattern_code
from %s.pattern_hits
where period = '%s' and pattern_code = '%s'
order by ts desc
limit %d`, r.database, escape(suffix), escape(code), limit*20)
	rows, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	items := make([]map[string]any, 0, limit)
	seen := map[string]struct{}{}
	for _, row := range rows {
		symbol := strings.TrimSpace(fmt.Sprintf("%v", row["symbol"]))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		if direction != "" && strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", row["direction"]))) != direction {
			continue
		}
		seen[symbol] = struct{}{}
		items = append(items, row)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *TdengineRepository) queryRows(sql string) ([]map[string]any, error) {
	payload, err := r.Exec(sql)
	if err != nil {
		return nil, err
	}
	columnMeta, ok := payload["column_meta"].([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	dataRows, ok := payload["data"].([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	columns := make([]string, 0, len(columnMeta))
	for _, item := range columnMeta {
		meta, ok := item.([]any)
		if !ok || len(meta) == 0 {
			continue
		}
		columns = append(columns, fmt.Sprintf("%v", meta[0]))
	}
	items := make([]map[string]any, 0, len(dataRows))
	for _, dataRow := range dataRows {
		row, ok := dataRow.([]any)
		if !ok {
			continue
		}
		record := map[string]any{}
		for idx, column := range columns {
			if idx < len(row) {
				record[column] = row[idx]
			}
		}
		items = append(items, record)
	}
	return items, nil
}

func reverseRows(items []map[string]any) []map[string]any {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items
}

func isMissingTDTable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "table does not exist") || strings.Contains(msg, "invalid")
}

func indicatorFieldValue(row map[string]any, field string) (float64, bool) {
	field = strings.TrimSpace(field)
	if field == "" || field == "value" {
		return anyFloat(row["value"])
	}
	raw := strings.TrimSpace(fmt.Sprintf("%v", row["values_json"]))
	if raw == "" || raw == "<nil>" {
		return 0, false
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return 0, false
	}
	value, ok := values[field]
	if !ok {
		return 0, false
	}
	return anyFloat(value)
}

func anyFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		parsed, err := strconv.ParseFloat(v.String(), 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func compareFloat(value float64, op string, threshold float64) bool {
	switch op {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

func NewMarketDataService() *MarketDataService {
	return &MarketDataService{
		cn: NewMarketAPIClient(),
		python: &PythonProvider{
			pythonBin: env("AKSHARE_PYTHON_BIN", "python3.11"),
			script:    env("AKSHARE_SCRIPT_PATH", "/app/network/akshare_adapter.py"),
		},
		tdx: &PythonProvider{
			pythonBin: env("MOOTDX_PYTHON_BIN", env("AKSHARE_PYTHON_BIN", "python3.11")),
			script:    env("MOOTDX_SCRIPT_PATH", "/app/network/mootdx_adapter.py"),
		},
	}
}

func (s *MarketDataService) Search(keyword string) ([]map[string]any, error) {
	return s.python.Search(keyword)
}

func (s *MarketDataService) Snapshot(symbol string) (map[string]any, error) {
	if isCNSymbol(symbol) && s.cn.Enabled() {
		if item, err := s.cn.Snapshot(context.Background(), symbol); err == nil {
			return item, nil
		}
	}
	return s.python.Snapshot(symbol)
}

func (s *MarketDataService) Minute(symbol, period string) ([]map[string]any, error) {
	return s.MinuteLimit(symbol, period, 0)
}

func (s *MarketDataService) MinuteLimit(symbol, period string, limit int) ([]map[string]any, error) {
	if isCNSymbol(symbol) {
		if items, err := s.tdx.MinuteLimit(symbol, period, limit); err == nil && len(items) > 0 {
			return items, nil
		}
	}
	if isCNSymbol(symbol) && s.cn.Enabled() {
		if items, err := s.cn.Minute(context.Background(), symbol, period); err == nil {
			return items, nil
		}
	}
	return s.python.Minute(symbol, period)
}

func (s *MarketDataService) Instruments(market, keyword, board string, limit, offset int) (map[string]any, error) {
	return s.python.Instruments(market, keyword, board, limit, offset)
}

func (s *MarketDataService) Profile(symbol string) (map[string]any, error) {
	return s.python.Profile(symbol)
}

func (s *MarketDataService) Daily(symbol, start, end string, limit int) (map[string]any, error) {
	if isCNSymbol(symbol) {
		if data, err := s.tdx.Daily(symbol, start, end, limit); err == nil {
			if items, ok := data["items"].([]any); ok && len(items) > 0 {
				return data, nil
			}
			if items, ok := data["items"].([]map[string]any); ok && len(items) > 0 {
				return data, nil
			}
		}
	}
	if isCNSymbol(symbol) && s.cn.Enabled() {
		if data, err := s.cn.Daily(context.Background(), symbol, limit); err == nil {
			return data, nil
		}
	}
	return s.python.Daily(symbol, start, end, limit)
}

func (s *MarketDataService) ETFRisk(symbol string, limit int) (map[string]any, error) {
	return s.python.ETFRisk(symbol, limit)
}

func (s *MarketDataService) SectorBoards(boardType string, limit int) (map[string]any, error) {
	return s.python.SectorBoards(boardType, limit)
}

func (a *PythonProvider) Search(keyword string) ([]map[string]any, error) {
	var response SearchResponse
	if err := a.run(&response, "search", keyword); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (a *PythonProvider) Snapshot(symbol string) (map[string]any, error) {
	var response SnapshotResponse
	if err := a.run(&response, "snapshot", symbol); err != nil {
		return nil, err
	}
	return response.Item, nil
}

func (a *PythonProvider) Minute(symbol, period string) ([]map[string]any, error) {
	return a.MinuteLimit(symbol, period, 0)
}

func (a *PythonProvider) MinuteLimit(symbol, period string, limit int) ([]map[string]any, error) {
	var response BarsResponse
	args := []string{"minute", symbol, period}
	if limit > 0 {
		args = append(args, fmt.Sprintf("%d", limit))
	}
	if err := a.run(&response, args...); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (a *PythonProvider) Instruments(market, keyword, board string, limit, offset int) (map[string]any, error) {
	var response map[string]any
	if err := a.run(&response, "instruments", market, keyword, board, fmt.Sprintf("%d", limit), fmt.Sprintf("%d", offset)); err != nil {
		return nil, err
	}
	return response, nil
}

func (a *PythonProvider) Profile(symbol string) (map[string]any, error) {
	var response SnapshotResponse
	if err := a.run(&response, "profile", symbol); err != nil {
		return nil, err
	}
	return response.Item, nil
}

func (a *PythonProvider) Daily(symbol, start, end string, limit int) (map[string]any, error) {
	var response map[string]any
	if err := a.run(&response, "daily", symbol, start, end, fmt.Sprintf("%d", limit)); err != nil {
		return nil, err
	}
	return response, nil
}

func (a *PythonProvider) ETFRisk(symbol string, limit int) (map[string]any, error) {
	var response map[string]any
	if err := a.run(&response, "etf_risk", symbol, fmt.Sprintf("%d", limit)); err != nil {
		return nil, err
	}
	return response, nil
}

func (a *PythonProvider) SectorBoards(boardType string, limit int) (map[string]any, error) {
	var response map[string]any
	if err := a.run(&response, "sector_boards", boardType, fmt.Sprintf("%d", limit)); err != nil {
		return nil, err
	}
	return response, nil
}

func (a *PythonProvider) run(target any, args ...string) error {
	cmd := exec.Command(a.pythonBin, append([]string{a.script}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("akshare adapter failed: %v: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("akshare adapter failed: %v", err)
	}
	return json.Unmarshal(output, target)
}

func (r *TaskRunner) Run(taskID string) error {
	if err := r.repo.UpdateTaskStatus(taskID, "running", "started"); err != nil {
		return err
	}
	task, err := r.repo.GetTask(taskID)
	if err != nil {
		return err
	}
	switch task.Interval {
	case "1m", "5m", "10m", "30m", "1h":
		rows, err := r.adapter.Minute(task.Symbol, task.Interval)
		if err != nil {
			_ = r.repo.UpdateTaskStatus(taskID, "failed", err.Error())
			return err
		}
		if err := r.td.InsertBars(task.Symbol, task.Market, task.Interval, task.Fields, rows); err != nil {
			_ = r.repo.UpdateTaskStatus(taskID, "failed", err.Error())
			return err
		}
		return r.repo.UpdateTaskStatus(taskID, "completed", fmt.Sprintf("inserted %d bars", len(rows)))
	default:
		snapshot, err := r.adapter.Snapshot(task.Symbol)
		if err != nil {
			_ = r.repo.UpdateTaskStatus(taskID, "failed", err.Error())
			return err
		}
		rows := []map[string]any{snapshotRow(task.Interval, snapshot)}
		if err := r.td.InsertBars(task.Symbol, task.Market, task.Interval, task.Fields, rows); err != nil {
			_ = r.repo.UpdateTaskStatus(taskID, "failed", err.Error())
			return err
		}
		return r.repo.UpdateTaskStatus(taskID, "completed", "inserted snapshot")
	}
}

func snapshotRow(interval string, snapshot map[string]any) map[string]any {
	return map[string]any{
		"ts":                  time.Now().Format(time.RFC3339),
		"open":                snapshot["price"],
		"close":               snapshot["price"],
		"high":                snapshot["price"],
		"low":                 snapshot["price"],
		"volume":              snapshot["volume"],
		"amount":              snapshot["amount"],
		"turnover_rate":       snapshot["turnover_rate"],
		"turnover_amount":     snapshot["turnover_amount"],
		"volume_ratio":        snapshot["volume_ratio"],
		"premium_ratio":       snapshot["premium_ratio"],
		"big_order_volume":    snapshot["big_order_volume"],
		"medium_order_volume": snapshot["medium_order_volume"],
		"small_order_volume":  snapshot["small_order_volume"],
		"bid_1_price":         snapshot["bid_1_price"],
		"bid_1_volume":        snapshot["bid_1_volume"],
		"bid_2_price":         snapshot["bid_2_price"],
		"bid_2_volume":        snapshot["bid_2_volume"],
		"bid_3_price":         snapshot["bid_3_price"],
		"bid_3_volume":        snapshot["bid_3_volume"],
		"bid_4_price":         snapshot["bid_4_price"],
		"bid_4_volume":        snapshot["bid_4_volume"],
		"bid_5_price":         snapshot["bid_5_price"],
		"bid_5_volume":        snapshot["bid_5_volume"],
		"ask_1_price":         snapshot["ask_1_price"],
		"ask_1_volume":        snapshot["ask_1_volume"],
		"ask_2_price":         snapshot["ask_2_price"],
		"ask_2_volume":        snapshot["ask_2_volume"],
		"ask_3_price":         snapshot["ask_3_price"],
		"ask_3_volume":        snapshot["ask_3_volume"],
		"ask_4_price":         snapshot["ask_4_price"],
		"ask_4_volume":        snapshot["ask_4_volume"],
		"ask_5_price":         snapshot["ask_5_price"],
		"ask_5_volume":        snapshot["ask_5_volume"],
		"requested_period":    interval,
		"source_period":       interval,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// env 返回环境变量值，找不到时返回 fallback。支持从 .env 文件读取（仅首次调用时加载）。
func env(key, fallback string) string {
	v := os.Getenv(key)
	if v != "" {
		return v
	}
	return envFallback(key, fallback)
}

// envFallback 从本地 .env 文件中查找 key（无锁单次加载，协程安全）。
func envFallback(key, fallback string) string {
	const envFile = ".env"
	f, err := os.Open(envFile)
	if err != nil {
		return fallback
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	_ = sc.Err()
	return fallback
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func escape(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func escapeDoubleQuote(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func sqlStringLiteral(value string) string {
	return "'" + escape(value) + "'"
}

func tdTimestampLiteral(value any) string {
	raw := strings.TrimSpace(fmt.Sprintf("%v", value))
	if raw == "" || raw == "<nil>" {
		return sqlStringLiteral(time.Now().Format("2006-01-02 15:04:05"))
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05.000"))
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05"))
	}
	if ts, err := time.Parse("2006-01-02 15:04", raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05"))
	}
	if ts, err := time.Parse("2006-01-02", raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05"))
	}
	return sqlStringLiteral(raw)
}

func parseTime(value time.Time) string {
	if value.IsZero() {
		return "null"
	}
	return fmt.Sprintf("'%s'", value.Format(time.RFC3339Nano))
}

// nullableTime 用于参数化查询：时间为零时传 nil，让数据库自行处理。
func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func normalizeFields(fields []string) []string {
	if len(fields) == 0 {
		return append([]string{}, defaultFields...)
	}
	set := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		set[field] = struct{}{}
	}
	items := make([]string, 0, len(set))
	for field := range set {
		items = append(items, field)
	}
	sort.Strings(items)
	return items
}

func makeFieldSet(fields []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, field := range normalizeFields(fields) {
		set[field] = struct{}{}
	}
	return set
}

func numericValue(row map[string]any, selected map[string]struct{}, feature, key string) string {
	if _, ok := selected[feature]; !ok {
		return "0"
	}
	value, ok := row[key]
	if !ok || value == nil {
		return "0"
	}
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%v", v)
	case float32:
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return v.String()
	case string:
		if strings.TrimSpace(v) == "" {
			return "0"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sanitizeIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			b.WriteRune(ch)
			continue
		}
		b.WriteRune('_')
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "unknown"
	}
	return result
}

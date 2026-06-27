package httpapi

import (
	"encoding/json"
	"net/http"

	backtestapp "stock-etf-monitor/backend/internal/application/backtest"
)

type BacktestHandler struct {
	service *backtestapp.Service
}

func NewBacktestHandler(service *backtestapp.Service) *BacktestHandler {
	return &BacktestHandler{service: service}
}

// SetKLineProvider 设置K线数据提供者
func (h *BacktestHandler) SetKLineProvider(provider backtestapp.KLineProvider) {
	h.service.SetKLineProvider(provider)
}

func (h *BacktestHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/backtest/results", h.Results)
	mux.HandleFunc("/api/stock/v1/backtest/summaries", h.Summaries)
	mux.HandleFunc("/api/stock/v1/backtest/execute", h.Execute)
	mux.HandleFunc("/api/stock/v1/backtest/strategies", h.Strategies)
}

// Execute 执行回测
func (h *BacktestHandler) Execute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}

	var input struct {
		StrategyID   string  `json:"strategy_id"`
		StrategyName string  `json:"strategy_name"`
		Symbol       string  `json:"symbol"`
		Period       string  `json:"period"`
		Lookback     int     `json:"lookback"`
		HoldBars     int     `json:"hold_bars"`
		InitialCash  float64 `json:"initial_cash"`
		FeeRate      float64 `json:"fee_rate"`
		SlippageRate float64 `json:"slippage_rate"`
		StopLoss     float64 `json:"stop_loss"`
		TakeProfit   float64 `json:"take_profit"`
		Benchmark    string  `json:"benchmark"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
		return
	}

	if input.Symbol == "" {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "symbol is required", "symbol", "")
		return
	}

	backtestInput := backtestapp.BacktestInput{
		StrategyID:   input.StrategyID,
		StrategyName: input.StrategyName,
		Symbol:       input.Symbol,
		Period:       input.Period,
		Lookback:     input.Lookback,
		HoldBars:     input.HoldBars,
		InitialCash:  input.InitialCash,
		FeeRate:      input.FeeRate,
		SlippageRate: input.SlippageRate,
		StopLoss:     input.StopLoss,
		TakeProfit:   input.TakeProfit,
		Benchmark:    input.Benchmark,
	}

	result, err := h.service.ExecuteBacktest(r.Context(), backtestInput)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BACKTEST_FAILED", err.Error(), "", err.Error())
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"item": result})
}

// Strategies 获取策略列表
func (h *BacktestHandler) Strategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}

	strategies := h.service.GetStrategyList()
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": strategies})
}

func (h *BacktestHandler) Results(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	limit, err := parseInt(q.Get("limit"), 200)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
		return
	}
	items, err := h.service.ListResults(r.Context(), q.Get("task_id"), q.Get("symbol"), limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func (h *BacktestHandler) Summaries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	limit, err := parseInt(q.Get("limit"), 200)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
		return
	}
	items, err := h.service.ListSummaries(r.Context(), q.Get("task_id"), limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

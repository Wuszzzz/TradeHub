package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"stock-etf-monitor/backend/internal/application/market"
)

type MarketHandler struct {
	service *market.Service
}

func NewMarketHandler(service *market.Service) *MarketHandler {
	return &MarketHandler{service: service}
}

func (h *MarketHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/market/snapshot", h.Snapshot)
	mux.HandleFunc("/api/stock/v1/market/kline", h.Kline)
	mux.HandleFunc("/api/stock/v1/etf/risk", h.ETFRisk)
}

func (h *MarketHandler) Snapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	item, err := h.service.Snapshot(r.Context(), symbol)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "symbol", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"item": item})
}

func (h *MarketHandler) ETFRisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	symbol := strings.TrimSpace(q.Get("symbol"))
	limit := 120
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
			return
		}
		limit = parsed
	}
	data, err := h.service.ETFRisk(r.Context(), symbol, limit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "symbol", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", data)
}

func (h *MarketHandler) Kline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	symbol := strings.TrimSpace(q.Get("symbol"))
	period := strings.TrimSpace(q.Get("period"))
	if period == "" {
		period = strings.TrimSpace(q.Get("interval"))
	}
	limit := 120
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be between 1 and 2000", "limit", err.Error())
			return
		}
		limit = parsed
	}
	data, err := h.service.Kline(r.Context(), symbol, period, limit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", data)
}

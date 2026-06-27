package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	quantapp "stock-etf-monitor/backend/internal/application/quant"
)

type QuantHandler struct {
	service *quantapp.Service
}

func NewQuantHandler(service *quantapp.Service) *QuantHandler {
	return &QuantHandler{service: service}
}

func (h *QuantHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/quant/indicators", h.Indicators)
	mux.HandleFunc("/api/stock/v1/quant/indicator-values", h.IndicatorValues)
	mux.HandleFunc("/api/stock/v1/quant/patterns", h.Patterns)
	mux.HandleFunc("/api/stock/v1/quant/pattern-hits", h.PatternHits)
}

func (h *QuantHandler) Indicators(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		definitions, err := h.service.ListIndicators(r.Context(), q.Get("category"), enabledOnly(q.Get("enabled_only")))
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": definitions})
	case http.MethodPost:
		var input quantapp.IndicatorInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		definition, err := h.service.CreateIndicator(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": definition})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *QuantHandler) IndicatorValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	limit, err := parseLimit(q.Get("limit"), 200)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
		return
	}
	items, err := h.service.ListIndicatorValues(r.Context(), q.Get("symbol"), q.Get("period"), q.Get("indicator_code"), limit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func (h *QuantHandler) Patterns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		definitions, err := h.service.ListPatterns(r.Context(), q.Get("category"), enabledOnly(q.Get("enabled_only")))
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": definitions})
	case http.MethodPost:
		var input quantapp.PatternInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		definition, err := h.service.CreatePattern(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": definition})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *QuantHandler) PatternHits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	limit, err := parseLimit(q.Get("limit"), 200)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
		return
	}
	items, err := h.service.ListPatternHits(r.Context(), q.Get("symbol"), q.Get("period"), q.Get("pattern_code"), limit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func enabledOnly(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

func parseLimit(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

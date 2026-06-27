package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	strategyapp "stock-etf-monitor/backend/internal/application/strategy"
)

type StrategyHandler struct {
	service *strategyapp.Service
}

func NewStrategyHandler(service *strategyapp.Service) *StrategyHandler {
	return &StrategyHandler{service: service}
}

func (h *StrategyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/strategies/templates", h.Templates)
	mux.HandleFunc("/api/stock/v1/strategies/runs", h.Runs)
}

func (h *StrategyHandler) Templates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.service.ListTemplates(r.Context(), enabledOnly(r.URL.Query().Get("enabled_only")))
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
	case http.MethodPost:
		var input strategyapp.TemplateInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		template, err := h.service.SaveTemplate(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": template})
	case http.MethodDelete:
		strategyID := strings.TrimSpace(r.URL.Query().Get("strategy_id"))
		if err := h.service.DeleteTemplate(r.Context(), strategyID); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "strategy_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"strategy_id": strategyID, "deleted": true})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *StrategyHandler) Runs(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.service.ListRuns(r.Context(), q.Get("strategy_id"), q.Get("task_id"), q.Get("status"), limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

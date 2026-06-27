package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	screenerapp "stock-etf-monitor/backend/internal/application/screener"
)

type ScreenerHandler struct {
	service *screenerapp.Service
}

func NewScreenerHandler(service *screenerapp.Service) *ScreenerHandler {
	return &ScreenerHandler{service: service}
}

func (h *ScreenerHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/screener/indicator", h.Indicator)
	mux.HandleFunc("/api/stock/v1/screener/pattern", h.Pattern)
	mux.HandleFunc("/api/stock/v1/screener/templates", h.Templates)
	mux.HandleFunc("/api/stock/v1/screener/results", h.Results)
}

func (h *ScreenerHandler) Indicator(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	threshold, err := parseFloat(q.Get("threshold"), 0)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "threshold must be a number", "threshold", err.Error())
		return
	}
	limit, err := parseInt(q.Get("limit"), 200)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
		return
	}
	items, err := h.service.Indicator(r.Context(), q.Get("period"), q.Get("indicator_code"), q.Get("field"), q.Get("op"), threshold, limit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func (h *ScreenerHandler) Pattern(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.service.Pattern(r.Context(), q.Get("period"), q.Get("pattern_code"), q.Get("direction"), limit)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func (h *ScreenerHandler) Templates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.service.ListTemplates(r.Context(), enabledOnly(r.URL.Query().Get("enabled_only")))
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
	case http.MethodPost:
		var input screenerapp.TemplateInput
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
		templateID := strings.TrimSpace(r.URL.Query().Get("template_id"))
		if err := h.service.DeleteTemplate(r.Context(), templateID); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "template_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"template_id": templateID, "deleted": true})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *ScreenerHandler) Results(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.service.ListResults(r.Context(), q.Get("task_id"), q.Get("template_id"), limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func parseFloat(value string, fallback float64) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(value, 64)
}

func parseInt(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

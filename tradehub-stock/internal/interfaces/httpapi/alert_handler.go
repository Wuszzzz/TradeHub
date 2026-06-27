package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	alertapp "stock-etf-monitor/backend/internal/application/alert"
)

type AlertHandler struct {
	service *alertapp.Service
}

func NewAlertHandler(service *alertapp.Service) *AlertHandler {
	return &AlertHandler{service: service}
}

func (h *AlertHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/alerts/rules", h.Rules)
	mux.HandleFunc("/api/stock/v1/alerts/events", h.Events)
	mux.HandleFunc("/api/stock/v1/alerts/events/ack", h.AckEvent)
}

func (h *AlertHandler) Rules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := h.service.ListRules(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": rules})
	case http.MethodPost:
		var input alertapp.RuleInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		rule, err := h.service.CreateRule(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": rule})
	case http.MethodDelete:
		ruleID := strings.TrimSpace(r.URL.Query().Get("rule_id"))
		if err := h.service.DeleteRule(r.Context(), ruleID); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "rule_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"deleted": ruleID})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *AlertHandler) Events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	limit := 200
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
			return
		}
		limit = parsed
	}
	events, err := h.service.ListEvents(r.Context(), status, limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": events})
}

func (h *AlertHandler) AckEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	eventID := strings.TrimSpace(r.URL.Query().Get("event_id"))
	if err := h.service.AckEvent(r.Context(), eventID); err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "event_id", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"acked": eventID})
}

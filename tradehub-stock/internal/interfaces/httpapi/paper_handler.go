package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	paperapp "stock-etf-monitor/backend/internal/application/paper"
)

type PaperHandler struct {
	service *paperapp.Service
}

func NewPaperHandler(service *paperapp.Service) *PaperHandler {
	return &PaperHandler{service: service}
}

func (h *PaperHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/paper/orders", h.Orders)
	mux.HandleFunc("/api/stock/v1/paper/positions", h.Positions)
	mux.HandleFunc("/api/stock/v1/paper/account", h.Account)
	mux.HandleFunc("/api/stock/v1/paper/reset", h.Reset)
}

func (h *PaperHandler) Orders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
		limit := 200
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
				return
			}
			limit = parsed
		}
		orders, err := h.service.ListOrders(r.Context(), symbol, limit)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": orders})
	case http.MethodPost:
		var input paperapp.OrderInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		order, err := h.service.PlaceOrder(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": order})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *PaperHandler) Positions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	items, err := h.service.Positions(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func (h *PaperHandler) Account(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	account, err := h.service.Account(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"item": account})
}

func (h *PaperHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	var input struct {
		Initial float64 `json:"initial"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if err := h.service.Reset(r.Context(), input.Initial); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"reset": true})
}

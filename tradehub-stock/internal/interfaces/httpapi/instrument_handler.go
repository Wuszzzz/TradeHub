package httpapi

import (
	"net/http"
	"strings"

	"stock-etf-monitor/backend/internal/application/instrument"
)

type InstrumentHandler struct {
	service *instrument.Service
}

func NewInstrumentHandler(service *instrument.Service) *InstrumentHandler {
	return &InstrumentHandler{service: service}
}

func (h *InstrumentHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/instruments/search", h.Search)
	mux.HandleFunc("/api/stock/v1/instruments/profile", h.Profile)
}

func (h *InstrumentHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	items, err := h.service.Search(r.Context(), keyword)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "keyword", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
}

func (h *InstrumentHandler) Profile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	item, err := h.service.Profile(r.Context(), symbol)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "symbol", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"item": item})
}

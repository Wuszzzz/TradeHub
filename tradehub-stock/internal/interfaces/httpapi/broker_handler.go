package httpapi

import (
	"net/http"

	"stock-etf-monitor/backend/internal/application/broker"
)

type BrokerHandler struct {
	service *broker.Service
}

func NewBrokerHandler(service *broker.Service) *BrokerHandler {
	return &BrokerHandler{service: service}
}

func (h *BrokerHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/broker/status", h.Status)
}

func (h *BrokerHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", h.service.Status())
}

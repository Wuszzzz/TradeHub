package httpapi

import (
	"net/http"

	"stock-etf-monitor/backend/internal/application/system"
)

type SystemHandler struct {
	service *system.Service
}

func NewSystemHandler(service *system.Service) *SystemHandler {
	return &SystemHandler{service: service}
}

func (h *SystemHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/system/v1/health", h.Health)
	mux.HandleFunc("/api/system/v1/overview", h.Overview)
}

func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", h.service.Health())
}

func (h *SystemHandler) Overview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", h.service.Overview())
}

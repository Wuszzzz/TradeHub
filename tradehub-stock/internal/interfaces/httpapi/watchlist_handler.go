package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"stock-etf-monitor/backend/internal/application/watchlist"
)

type WatchlistHandler struct {
	service *watchlist.Service
}

func NewWatchlistHandler(service *watchlist.Service) *WatchlistHandler {
	return &WatchlistHandler{service: service}
}

func (h *WatchlistHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/watchlist/groups", h.Groups)
	mux.HandleFunc("/api/stock/v1/watchlist/items", h.Items)
}

func (h *WatchlistHandler) Groups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups, err := h.service.ListGroups(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": groups})
	case http.MethodPost:
		var input watchlist.GroupInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		group, err := h.service.CreateGroup(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "name", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": group})
	case http.MethodDelete:
		groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
		if err := h.service.DeleteGroup(r.Context(), groupID); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "group_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"deleted": groupID})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *WatchlistHandler) Items(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
		items, err := h.service.ListItems(r.Context(), groupID)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": items})
	case http.MethodPost:
		var input watchlist.ItemInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		item, err := h.service.CreateItem(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "symbol", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": item})
	case http.MethodDelete:
		itemID := strings.TrimSpace(r.URL.Query().Get("item_id"))
		if err := h.service.DeleteItem(r.Context(), itemID); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "item_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"deleted": itemID})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

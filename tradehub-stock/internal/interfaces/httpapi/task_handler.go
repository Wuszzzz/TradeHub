package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	taskapp "stock-etf-monitor/backend/internal/application/task"
)

type TaskHandler struct {
	service *taskapp.Service
}

func NewTaskHandler(service *taskapp.Service) *TaskHandler {
	return &TaskHandler{service: service}
}

func (h *TaskHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/tasks/ingestion", h.Ingestion)
	mux.HandleFunc("/api/stock/v1/tasks", h.Tasks)
	mux.HandleFunc("/api/stock/v1/tasks/status", h.Status)
	mux.HandleFunc("/api/stock/v1/tasks/logs", h.Logs)
}

func (h *TaskHandler) Ingestion(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks, err := h.service.ListIngestion(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": tasks})
	case http.MethodPost:
		var input taskapp.IngestionInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		task, err := h.service.CreateIngestion(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": task})
	case http.MethodDelete:
		taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
		if err := h.service.DeleteIngestion(r.Context(), taskID); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "task_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"task_id": taskID, "deleted": true})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *TaskHandler) Tasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		taskID := strings.TrimSpace(q.Get("task_id"))
		if taskID != "" {
			task, err := h.service.GetStockTask(r.Context(), taskID)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "task_id", err.Error())
				return
			}
			WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"item": task})
			return
		}
		limit := 200
		if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
				return
			}
			limit = parsed
		}
		tasks, err := h.service.ListStockTasks(r.Context(), q.Get("task_type"), q.Get("status"), limit)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": tasks})
	case http.MethodPost:
		var input taskapp.StockTaskInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		task, err := h.service.CreateStockTask(r.Context(), input)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "task_type", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"item": task})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

func (h *TaskHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
	var input taskapp.StatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
		return
	}
	if err := h.service.UpdateStatus(r.Context(), taskID, input); err != nil {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
		return
	}
	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"task_id": taskID, "status": input.Status})
}

func (h *TaskHandler) Logs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		limit := 200
		if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit must be a number", "limit", err.Error())
				return
			}
			limit = parsed
		}
		logs, err := h.service.ListLogs(r.Context(), q.Get("task_id"), limit)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "task_id", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"items": logs})
	case http.MethodPost:
		taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
		var input taskapp.LogInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid body", "", err.Error())
			return
		}
		if err := h.service.AppendLog(r.Context(), taskID, input); err != nil {
			WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "", err.Error())
			return
		}
		WriteJSON(w, r, http.StatusCreated, "OK", "created", map[string]any{"task_id": taskID})
	default:
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
	}
}

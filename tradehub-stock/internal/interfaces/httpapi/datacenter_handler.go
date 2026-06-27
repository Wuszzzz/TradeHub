package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"stock-etf-monitor/backend/internal/application/datacenter"
)

// DataCenterHandler 数据中心处理器
type DataCenterHandler struct {
	service *datacenter.ServiceImpl
}

// NewDataCenterHandler 创建数据中心处理器
func NewDataCenterHandler(service *datacenter.ServiceImpl) *DataCenterHandler {
	return &DataCenterHandler{service: service}
}

// Register 注册路由
func (h *DataCenterHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/stock/v1/datacenter/collect", h.handleCollect)
	mux.HandleFunc("/api/stock/v1/datacenter/query/daily", h.handleQueryDaily)
	mux.HandleFunc("/api/stock/v1/datacenter/query/lhb", h.handleQueryLHB)
	mux.HandleFunc("/api/stock/v1/datacenter/query/dzjy", h.handleQueryDZJY)
	mux.HandleFunc("/api/stock/v1/datacenter/tasks", h.handleTasks)
	mux.HandleFunc("/api/stock/v1/datacenter/tasks/logs", h.handleTaskLogs)
	mux.HandleFunc("/api/stock/v1/datacenter/health", h.handleHealth)
}

// handleCollect 触发数据采集
func (h *DataCenterHandler) handleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSON(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
		return
	}

	taskType := r.URL.Query().Get("task_type")
	targetDate := r.URL.Query().Get("target_date")

	req := &datacenter.DataCollectionRequest{
		TaskType:   taskType,
		TargetDate: targetDate,
	}

	resp, err := h.service.Collect(r.Context(), req)
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", resp)
}

// handleQueryDaily 查询每日行情
func (h *DataCenterHandler) handleQueryDaily(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	code := r.URL.Query().Get("code")
	name := r.URL.Query().Get("name")
	page := 1
	pageSize := 100

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}

	q := &datacenter.DailySpotQuery{
		Date:     date,
		Code:     code,
		Name:     name,
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.service.QueryDailySpot(r.Context(), q)
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", result)
}

// handleQueryLHB 查询龙虎榜
func (h *DataCenterHandler) handleQueryLHB(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	dateStart := r.URL.Query().Get("date_start")
	dateEnd := r.URL.Query().Get("date_end")
	code := r.URL.Query().Get("code")
	page := 1
	pageSize := 100

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}

	q := &datacenter.LHBGGQuery{
		Date:      date,
		DateStart: dateStart,
		DateEnd:   dateEnd,
		Code:      code,
		Page:      page,
		PageSize:  pageSize,
	}

	result, err := h.service.QueryLHBGG(r.Context(), q)
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", result)
}

// handleQueryDZJY 查询大宗交易
func (h *DataCenterHandler) handleQueryDZJY(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	dateStart := r.URL.Query().Get("date_start")
	dateEnd := r.URL.Query().Get("date_end")
	code := r.URL.Query().Get("code")
	page := 1
	pageSize := 100

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}

	q := &datacenter.DZJYQuery{
		Date:      date,
		DateStart: dateStart,
		DateEnd:   dateEnd,
		Code:      code,
		Page:      page,
		PageSize:  pageSize,
	}

	result, err := h.service.QueryDZJY(r.Context(), q)
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", result)
}

// handleTasks 查询采集任务列表
func (h *DataCenterHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	taskType := r.URL.Query().Get("task_type")
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	tasks, err := h.service.ListCollectionTasks(r.Context(), taskType, limit)
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"data": tasks, "total": len(tasks)})
}

// handleTaskLogs 查询任务日志
func (h *DataCenterHandler) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		WriteJSON(w, r, http.StatusBadRequest, "BAD_REQUEST", "task_id required", nil)
		return
	}

	logs, err := h.service.GetCollectionLogs(r.Context(), taskID)
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", map[string]any{"data": logs, "total": len(logs)})
}

// handleHealth 健康检查
func (h *DataCenterHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.GetHealthStatus(r.Context())
	if err != nil {
		WriteJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	WriteJSON(w, r, http.StatusOK, "OK", "ok", status)
}

// 避免未使用的导入错误
var _ = json.Marshal

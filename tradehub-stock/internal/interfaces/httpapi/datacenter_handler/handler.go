package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	datacenter "stock-etf-monitor/backend/internal/application/datacenter"
)

// DataCenterHandler 数据中心 HTTP 处理器
type DataCenterHandler struct {
	service *datacenter.ServiceImpl
}

// NewDataCenterHandler 创建数据中心处理器
func NewDataCenterHandler(service *datacenter.ServiceImpl) *DataCenterHandler {
	return &DataCenterHandler{service: service}
}

// Register 注册路由
func (h *DataCenterHandler) Register(mux *http.ServeMux) {
	// 每日行情
	mux.HandleFunc("/api/stock/v1/datacenter/daily-spot", h.QueryDailySpot)
	mux.HandleFunc("/api/stock/v1/datacenter/daily-spot/collect", h.CollectDailySpot)

	// 龙虎榜
	mux.HandleFunc("/api/stock/v1/datacenter/lhb", h.QueryLHB)
	mux.HandleFunc("/api/stock/v1/datacenter/lhb/collect", h.CollectLHB)

	// 大宗交易
	mux.HandleFunc("/api/stock/v1/datacenter/dzjy", h.QueryDZJY)
	mux.HandleFunc("/api/stock/v1/datacenter/dzjy/collect", h.CollectDZJY)

	// 采集任务
	mux.HandleFunc("/api/stock/v1/datacenter/tasks", h.ListTasks)
	mux.HandleFunc("/api/stock/v1/datacenter/tasks/status", h.GetTaskStatus)

	// 健康检查
	mux.HandleFunc("/api/stock/v1/datacenter/health", h.Health)

	// 数据源状态
	mux.HandleFunc("/api/stock/v1/datacenter/sources", h.ListSources)
}

// QueryDailySpot 查询每日行情
func (h *DataCenterHandler) QueryDailySpot(w http.ResponseWriter, r *http.Request) {
	q := &datacenter.DailySpotQuery{
		Date:     r.URL.Query().Get("date"),
		Code:     r.URL.Query().Get("code"),
		Name:     r.URL.Query().Get("name"),
		SortField: r.URL.Query().Get("sort_field"),
		SortOrder: r.URL.Query().Get("sort_order"),
		Page:     1,
		PageSize: 50,
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if page, err := strconv.Atoi(p); err == nil {
			q.Page = page
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if pageSize, err := strconv.Atoi(ps); err == nil {
			q.PageSize = pageSize
		}
	}
	if cm := r.URL.Query().Get("change_min"); cm != "" {
		if f, err := strconv.ParseFloat(cm, 64); err == nil {
			q.ChangeMin = f
		}
	}
	if cm := r.URL.Query().Get("change_max"); cm != "" {
		if f, err := strconv.ParseFloat(cm, 64); err == nil {
			q.ChangeMax = f
		}
	}

	result, err := h.service.QueryDailySpot(r.Context(), q)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, result)
}

// CollectDailySpot 采集每日行情
func (h *DataCenterHandler) CollectDailySpot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("20060102")
	}

	resp, err := h.service.Collect(r.Context(), &datacenter.DataCollectionRequest{
		TaskType:   "daily_spot",
		TargetDate: date,
	})
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "COLLECT_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, resp)
}

// QueryLHB 查询龙虎榜
func (h *DataCenterHandler) QueryLHB(w http.ResponseWriter, r *http.Request) {
	q := &datacenter.LHBGGQuery{
		Date:      r.URL.Query().Get("date"),
		DateStart: r.URL.Query().Get("date_start"),
		DateEnd:   r.URL.Query().Get("date_end"),
		Code:      r.URL.Query().Get("code"),
		Name:      r.URL.Query().Get("name"),
		SortField: r.URL.Query().Get("sort_field"),
		SortOrder: r.URL.Query().Get("sort_order"),
		Page:      1,
		PageSize:  50,
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if page, err := strconv.Atoi(p); err == nil {
			q.Page = page
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if pageSize, err := strconv.Atoi(ps); err == nil {
			q.PageSize = pageSize
		}
	}
	if nm := r.URL.Query().Get("net_min"); nm != "" {
		if f, err := strconv.ParseFloat(nm, 64); err == nil {
			q.NetMin = f
		}
	}

	result, err := h.service.QueryLHBGG(r.Context(), q)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, result)
}

// CollectLHB 采集龙虎榜
func (h *DataCenterHandler) CollectLHB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("20060102")
	}

	resp, err := h.service.Collect(r.Context(), &datacenter.DataCollectionRequest{
		TaskType:   "lhb",
		TargetDate: date,
	})
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "COLLECT_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, resp)
}

// QueryDZJY 查询大宗交易
func (h *DataCenterHandler) QueryDZJY(w http.ResponseWriter, r *http.Request) {
	q := &datacenter.DZJYQuery{
		Date:      r.URL.Query().Get("date"),
		DateStart: r.URL.Query().Get("date_start"),
		DateEnd:   r.URL.Query().Get("date_end"),
		Code:      r.URL.Query().Get("code"),
		Name:      r.URL.Query().Get("name"),
		SortField: r.URL.Query().Get("sort_field"),
		SortOrder: r.URL.Query().Get("sort_order"),
		Page:      1,
		PageSize:  50,
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if page, err := strconv.Atoi(p); err == nil {
			q.Page = page
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if pageSize, err := strconv.Atoi(ps); err == nil {
			q.PageSize = pageSize
		}
	}
	if om := r.URL.Query().Get("overflow_min"); om != "" {
		if f, err := strconv.ParseFloat(om, 64); err == nil {
			q.OverflowMin = f
		}
	}

	result, err := h.service.QueryDZJY(r.Context(), q)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, result)
}

// CollectDZJY 采集大宗交易
func (h *DataCenterHandler) CollectDZJY(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("20060102")
	}

	resp, err := h.service.Collect(r.Context(), &datacenter.DataCollectionRequest{
		TaskType:   "dzjy",
		TargetDate: date,
	})
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "COLLECT_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, resp)
}

// ListTasks 获取采集任务列表
func (h *DataCenterHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	taskType := r.URL.Query().Get("task_type")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	tasks, err := h.service.ListCollectionTasks(r.Context(), taskType, limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, map[string]interface{}{
		"tasks": tasks,
	})
}

// GetTaskStatus 获取任务状态
func (h *DataCenterHandler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		WriteError(w, r, http.StatusBadRequest, "MISSING_TASK_ID", "task_id is required", "", "")
		return
	}

	task, err := h.service.GetCollectionTask(r.Context(), taskID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, task)
}

// Health 健康检查
func (h *DataCenterHandler) Health(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.GetHealthStatus(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "HEALTH_ERROR", err.Error(), "", "")
		return
	}

	WriteSuccess(w, r, status)
}

// ListSources 数据源列表
func (h *DataCenterHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources := []map[string]string{
		{"name": "eastmoney", "display_name": "东方财富", "status": "ok"},
		{"name": "sina", "display_name": "新浪财经", "status": "ok"},
		{"name": "tencent", "display_name": "腾讯财经", "status": "ok"},
	}

	WriteSuccess(w, r, map[string]interface{}{
		"sources": sources,
	})
}

// WriteSuccess 统一成功响应
func WriteSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	now := time.Now()
	resp := map[string]interface{}{
		"success": true,
		"code":    "OK",
		"message": "ok",
		"data":    data,
		"meta": map[string]interface{}{
			"request_id": "dc_" + now.Format("20060102150405") + "_" + strconv.FormatInt(now.UnixNano()/1000000%1000000, 10),
			"timestamp":  now.Format(time.RFC3339),
			"version":    "v1",
		},
	}

	json.NewEncoder(w).Encode(resp)
}

// WriteError 统一错误响应
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message, field, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	now := time.Now()
	resp := map[string]interface{}{
		"success": false,
		"code":    code,
		"message": message,
		"data":    nil,
		"meta": map[string]interface{}{
			"request_id": "dc_" + now.Format("20060102150405") + "_" + strconv.FormatInt(now.UnixNano()/1000000%1000000, 10),
			"timestamp":  now.Format(time.RFC3339),
			"version":   "v1",
		},
		"error": map[string]interface{}{
			"field":  field,
			"detail": detail,
		},
	}

	json.NewEncoder(w).Encode(resp)
}

// NormalizeDate 标准化日期格式
func NormalizeDate(date string) string {
	date = strings.ReplaceAll(date, "-", "")
	date = strings.ReplaceAll(date, "/", "")
	return date
}

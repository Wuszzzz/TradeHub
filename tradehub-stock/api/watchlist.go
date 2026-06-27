package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

// =============== Watchlist Repository ===============

// ListWatchlistGroups 返回所有分组（含每组当前 item_count），按 sort_order 升序。
func (r *PostgresRepository) ListWatchlistGroups() ([]model.WatchlistGroup, error) {
	sql := `
	select row_to_json(t) from (
	  select g.group_id, g.name, g.sort_order, g.created_at,
	         coalesce(c.cnt, 0) as item_count
	    from watchlist_groups g
	    left join (
	      select group_id, count(*) as cnt
	        from watchlist_items
	       group by group_id
	    ) c on c.group_id = g.group_id
	   order by g.sort_order asc, g.created_at asc
	) t;`
	lines, err := r.queryLines(sql)
	if err != nil {
		return nil, err
	}
	groups := make([]model.WatchlistGroup, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var g model.WatchlistGroup
		if err := json.Unmarshal([]byte(line), &g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (r *PostgresRepository) UpsertWatchlistGroup(g model.WatchlistGroup) error {
	return r.exec(`
	insert into watchlist_groups (group_id, name, sort_order, created_at)
	values ($1, $2, $3, $4)
	on conflict (group_id) do update set
	  name = excluded.name,
	  sort_order = excluded.sort_order;`,
		g.GroupID, g.Name, g.SortOrder, g.CreatedAt)
}

func (r *PostgresRepository) DeleteWatchlistGroup(groupID string) error {
	if groupID == "default" {
		return fmt.Errorf("default group cannot be deleted")
	}
	return r.exec(`delete from watchlist_groups where group_id = $1;`, groupID)
}

// ListWatchlistItems 列出指定分组的标的，按 sort_order 升序；groupID 空则列全部。
func (r *PostgresRepository) ListWatchlistItems(groupID string) ([]model.WatchlistItem, error) {
	sql := `
	select row_to_json(t) from (
	  select item_id, group_id, symbol, name, market, note, sort_order, created_at
	    from watchlist_items
	   where ($1 = '' or group_id = $1)
	   order by sort_order asc, created_at asc
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(groupID))
	if err != nil {
		return nil, err
	}
	items := make([]model.WatchlistItem, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var it model.WatchlistItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *PostgresRepository) UpsertWatchlistItem(it model.WatchlistItem) error {
	return r.exec(`
	insert into watchlist_items (item_id, group_id, symbol, name, market, note, sort_order, created_at)
	values ($1, $2, $3, $4, $5, $6, $7, $8)
	on conflict (group_id, symbol) do update set
	  name = excluded.name,
	  market = excluded.market,
	  note = excluded.note,
	  sort_order = excluded.sort_order;`,
		it.ItemID, it.GroupID, it.Symbol,
		it.Name, it.Market, it.Note,
		it.SortOrder, it.CreatedAt)
}

func (r *PostgresRepository) DeleteWatchlistItem(itemID string) error {
	return r.exec(`delete from watchlist_items where item_id = $1;`, itemID)
}

// =============== Watchlist Handlers ===============

type watchlistGroupReq struct {
	GroupID   string `json:"group_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type watchlistItemReq struct {
	ItemID    string `json:"item_id"`
	GroupID   string `json:"group_id"`
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Market    string `json:"market"`
	Note      string `json:"note"`
	SortOrder int    `json:"sort_order"`
}

func (s *Server) handleWatchlistGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups, err := s.taskRepo.ListWatchlistGroups()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": groups})
	case http.MethodPost:
		var req watchlistGroupReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "name is required"})
			return
		}
		now := time.Now().UTC()
		groupID := strings.TrimSpace(req.GroupID)
		if groupID == "" {
			groupID = fmt.Sprintf("grp_%d", now.UnixNano())
		}
		g := model.WatchlistGroup{
			GroupID:   groupID,
			Name:      req.Name,
			SortOrder: req.SortOrder,
			CreatedAt: now,
		}
		if err := s.taskRepo.UpsertWatchlistGroup(g); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": g})
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("group_id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "group_id is required"})
			return
		}
		if err := s.taskRepo.DeleteWatchlistGroup(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWatchlistItems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
		items, err := s.taskRepo.ListWatchlistItems(groupID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req watchlistItemReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
			return
		}
		req.Symbol = strings.TrimSpace(req.Symbol)
		req.GroupID = strings.TrimSpace(req.GroupID)
		if req.Symbol == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol is required"})
			return
		}
		if req.GroupID == "" {
			req.GroupID = "default"
		}
		now := time.Now().UTC()
		itemID := strings.TrimSpace(req.ItemID)
		if itemID == "" {
			itemID = fmt.Sprintf("wli_%d", now.UnixNano())
		}
		it := model.WatchlistItem{
			ItemID:    itemID,
			GroupID:   req.GroupID,
			Symbol:    req.Symbol,
			Name:      fallback(req.Name, req.Symbol),
			Market:    fallback(req.Market, "CN-A"),
			Note:      req.Note,
			SortOrder: req.SortOrder,
			CreatedAt: now,
		}
		if err := s.taskRepo.UpsertWatchlistItem(it); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": it})
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("item_id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "item_id is required"})
			return
		}
		if err := s.taskRepo.DeleteWatchlistItem(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleWatchlistSnapshot 把一个分组下所有标的 + 最新行情快照一次性返回。
// 前端工作台/看板用一次 HTTP 拉满，避免 N 次 realtime 请求。
//
// 注意：单次会向 AKShare 发起 N 次子进程调用，N 增大时性能会下降。
// 当前实现 N≤30 内可用，后续接入 efinance 批量接口或自建缓存层时再优化。
func (s *Server) handleWatchlistSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	items, err := s.taskRepo.ListWatchlistItems(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	// 控制最多 30 只避免子进程风暴
	if len(items) > 30 {
		items = items[:30]
	}
	results := make([]map[string]any, 0, len(items))
	for _, it := range items {
		entry := map[string]any{
			"item_id":   it.ItemID,
			"group_id":  it.GroupID,
			"symbol":    it.Symbol,
			"name":      it.Name,
			"market":    it.Market,
			"note":      it.Note,
			"quote":     nil,
			"quote_err": "",
		}
		// 走缓存版本 snapshot，5s TTL 内 N 只标的不会真的调 N 次子进程
		payload, err := s.cachedSnapshot(it.Symbol)
		if err != nil {
			entry["quote_err"] = err.Error()
		} else {
			entry["quote"] = payload
		}
		results = append(results, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": results,
		"count": len(results),
	})
}

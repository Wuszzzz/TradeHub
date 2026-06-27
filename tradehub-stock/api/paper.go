package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

// =============== Paper Repository ===============

func (r *PostgresRepository) ListPaperOrders(symbol string, limit int) ([]model.PaperOrder, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t) from (
	  select order_id, symbol, name, market, side, qty, price, amount, fee,
	         status, note, placed_at,
	         coalesce(filled_at, 'epoch'::timestamptz) as filled_at
	  from paper_orders
	  where ($1 = '' or symbol = $1)
	  order by placed_at desc
	  limit $2
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(symbol), limit)
	if err != nil {
		return nil, err
	}
	orders := make([]model.PaperOrder, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var o model.PaperOrder
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (r *PostgresRepository) GetPaperAccountRow() (*model.PaperAccountRow, error) {
	sql := `
	select row_to_json(t) from (
	  select cash, initial, realized_pl, updated_at from paper_account where id = 1
	) t;`
	lines, err := r.queryLines(sql)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return &model.PaperAccountRow{Cash: 1000000, Initial: 1000000, UpdatedAt: time.Now()}, nil
	}
	var row model.PaperAccountRow
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		return nil, err
	}
	return &row, nil
}

// PlaceOrder 完成下单：扣 / 加现金，记录 realized_pl，写入 orders
// 简化模型：立即按传入价成交，不撮合不滑点；卖出按 FIFO 平均成本计算已实现盈亏。
func (r *PostgresRepository) PlaceOrder(o model.PaperOrder) error {
	deltaCash := -o.Amount - o.Fee
	deltaRealized := 0.0
	if o.Side == "sell" {
		deltaCash = o.Amount - o.Fee
		pos, err := r.computePosition(o.Symbol)
		if err == nil && pos != nil && pos.Qty > 0 {
			deltaRealized = (o.Price - pos.AvgCost) * o.Qty
		}
	}
	insertStmt := sqlStmt{
		query: `
	insert into paper_orders
	  (order_id, symbol, name, market, side, qty, price, amount, fee, status, note, placed_at, filled_at)
	values
	  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);`,
		args: []any{
			o.OrderID, o.Symbol, o.Name, o.Market,
			o.Side, o.Qty, o.Price, o.Amount, o.Fee, o.Status, o.Note,
			o.PlacedAt, o.FilledAt,
		},
	}
	updateStmt := sqlStmt{
		query: `
	update paper_account
	set cash = cash + ($1),
	    realized_pl = realized_pl + ($2),
	    updated_at = now()
	where id = 1;`,
		args: []any{deltaCash, deltaRealized},
	}
	return r.execTx([]sqlStmt{insertStmt, updateStmt})
}

func (r *PostgresRepository) computePosition(symbol string) (*model.PaperPositionAgg, error) {
	sql := `
	select row_to_json(t) from (
	  select symbol,
	         coalesce(max(name), symbol) as name,
	         coalesce(max(market), 'CN-A') as market,
	         coalesce(sum(case when side='buy' then qty else -qty end), 0) as qty,
	         case
	           when coalesce(sum(case when side='buy' then qty else 0 end),0) = 0 then 0
	           else coalesce(sum(case when side='buy' then qty*price else 0 end),0)
	                / nullif(sum(case when side='buy' then qty else 0 end),0)
	         end as avg_cost
	  from paper_orders
	  where symbol = $1 and status = 'filled'
	  group by symbol
	) t;`
	lines, err := r.queryLines(sql, symbol)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, nil
	}
	var pos model.PaperPositionAgg
	if err := json.Unmarshal([]byte(lines[0]), &pos); err != nil {
		return nil, err
	}
	return &pos, nil
}

func (r *PostgresRepository) ComputePosition(symbol string) (*model.PaperPositionAgg, error) {
	return r.computePosition(symbol)
}

// ListPositions 聚合所有持仓（不含市值，由 handler 拿实时价填）
func (r *PostgresRepository) ListPositions() ([]model.PaperPositionAgg, error) {
	sql := `
	select row_to_json(t) from (
	  select symbol,
	         coalesce(max(name), symbol) as name,
	         coalesce(max(market), 'CN-A') as market,
	         coalesce(sum(case when side='buy' then qty else -qty end), 0) as qty,
	         case
	           when coalesce(sum(case when side='buy' then qty else 0 end),0) = 0 then 0
	           else coalesce(sum(case when side='buy' then qty*price else 0 end),0)
	                / nullif(sum(case when side='buy' then qty else 0 end),0)
	         end as avg_cost
	  from paper_orders
	  where status = 'filled'
	  group by symbol
	  having coalesce(sum(case when side='buy' then qty else -qty end), 0) > 0
	  order by symbol
	) t;`
	lines, err := r.queryLines(sql)
	if err != nil {
		return nil, err
	}
	out := make([]model.PaperPositionAgg, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var pos model.PaperPositionAgg
		if err := json.Unmarshal([]byte(line), &pos); err != nil {
			return nil, err
		}
		out = append(out, pos)
	}
	return out, nil
}

func (r *PostgresRepository) ResetPaper(initial float64) error {
	if initial <= 0 {
		initial = 1000000
	}
	return r.execRaw(fmt.Sprintf(`
	delete from paper_orders;
	update paper_account set cash = %v, initial = %v, realized_pl = 0, updated_at = now() where id = 1;`,
		initial, initial))
}

// =============== Paper Handlers ===============

type placeOrderReq struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Market string  `json:"market"`
	Side   string  `json:"side"`
	Qty    float64 `json:"qty"`
	Price  float64 `json:"price"`
	Fee    float64 `json:"fee"`
	Note   string  `json:"note"`
}

func (s *Server) handlePaperOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
		limit := 200
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			_, _ = fmt.Sscanf(raw, "%d", &limit)
		}
		orders, err := s.taskRepo.ListPaperOrders(symbol, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": orders})
	case http.MethodPost:
		var req placeOrderReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
			return
		}
		req.Symbol = strings.TrimSpace(req.Symbol)
		req.Side = strings.TrimSpace(req.Side)
		if req.Symbol == "" || (req.Side != "buy" && req.Side != "sell") || req.Qty <= 0 || req.Price <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "symbol/side/qty/price invalid"})
			return
		}
		amount := req.Qty * req.Price
		// 默认手续费：成交额 * 万 2.5，最低 5 元
		if req.Fee <= 0 {
			req.Fee = math.Max(5, amount*0.00025)
		}
		// 卖出校验持仓
		if req.Side == "sell" {
			pos, err := s.taskRepo.computePosition(req.Symbol)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
				return
			}
			if pos == nil || pos.Qty < req.Qty {
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "持仓不足，无法卖出"})
				return
			}
		}
		// 买入校验现金
		if req.Side == "buy" {
			acc, err := s.taskRepo.GetPaperAccountRow()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
				return
			}
			if acc.Cash < amount+req.Fee {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"message": fmt.Sprintf("现金不足：需要 %.2f，可用 %.2f", amount+req.Fee, acc.Cash),
				})
				return
			}
		}
		now := time.Now().UTC()
		order := model.PaperOrder{
			OrderID:  fmt.Sprintf("ord_%d", now.UnixNano()),
			Symbol:   req.Symbol,
			Name:     fallback(req.Name, req.Symbol),
			Market:   fallback(req.Market, "CN-A"),
			Side:     req.Side,
			Qty:      req.Qty,
			Price:    req.Price,
			Amount:   amount,
			Fee:      req.Fee,
			Status:   "filled",
			Note:     req.Note,
			PlacedAt: now,
			FilledAt: now,
		}
		if err := s.taskRepo.PlaceOrder(order); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": order})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePaperPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	positions, err := s.taskRepo.ListPositions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	items := make([]model.PaperPosition, 0, len(positions))
	for _, p := range positions {
		// 拉实时价，走 cache 避免 N+1 个 Python 子进程
		last := p.AvgCost
		if snap, err := s.cachedSnapshot(p.Symbol); err == nil {
			if v, ok := snap["price"].(float64); ok && v > 0 {
				last = v
			}
		}
		mv := last * p.Qty
		items = append(items, model.PaperPosition{
			Symbol:       p.Symbol,
			Name:         p.Name,
			Market:       p.Market,
			Qty:          p.Qty,
			AvgCost:      p.AvgCost,
			LastPrice:    last,
			MarketValue:  mv,
			UnrealizedPL: (last - p.AvgCost) * p.Qty,
			UpdatedAt:    time.Now(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handlePaperAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	acc, err := s.taskRepo.GetPaperAccountRow()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	// 计算市值，走 cachedSnapshot
	positions, _ := s.taskRepo.ListPositions()
	mv := 0.0
	for _, p := range positions {
		last := p.AvgCost
		if snap, err := s.cachedSnapshot(p.Symbol); err == nil {
			if v, ok := snap["price"].(float64); ok && v > 0 {
				last = v
			}
		}
		mv += last * p.Qty
	}
	equity := acc.Cash + mv
	totalReturn := 0.0
	if acc.Initial > 0 {
		totalReturn = (equity - acc.Initial) / acc.Initial
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item": model.PaperAccount{
			Cash:        acc.Cash,
			Equity:      equity,
			RealizedPL:  acc.RealizedPL,
			TotalReturn: totalReturn,
			Initial:     acc.Initial,
			UpdatedAt:   acc.UpdatedAt,
		},
	})
}

func (s *Server) handlePaperReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Initial float64 `json:"initial"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.taskRepo.ResetPaper(req.Initial); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}

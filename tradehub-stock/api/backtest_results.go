package main

import (
	"encoding/json"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

func (r *PostgresRepository) ListBacktestResults(taskID, symbol string, limit int) ([]model.BacktestResult, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t)
	from (
	  select result_id, task_id, symbol, period, entry_time, exit_time,
	         entry_price, exit_price, return_pct,
	         benchmark_symbol, benchmark_return_pct, excess_return_pct,
	         meta::text as meta_json, created_at
	  from stock_backtest_results
	  where ($1 = '' or task_id = $1)
	    and ($2 = '' or symbol = $2)
	  order by return_pct desc, created_at desc
	  limit $3
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(taskID), strings.TrimSpace(symbol), limit)
	if err != nil {
		return nil, err
	}
	results := make([]model.BacktestResult, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var result model.BacktestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (r *PostgresRepository) ListBacktestSummaries(taskID string, limit int) ([]model.BacktestSummary, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t)
	from (
	  select summary_id, task_id, total_trades, win_rate, avg_return_pct, total_return_pct,
	         max_drawdown_pct, best_return_pct, worst_return_pct,
	         benchmark_symbol, benchmark_return_pct, avg_excess_return_pct,
	         return_distribution::text as return_distribution_json,
	         meta::text as meta_json, created_at
	  from stock_backtest_summaries
	  where ($1 = '' or task_id = $1)
	  order by created_at desc
	  limit $2
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(taskID), limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.BacktestSummary, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row struct {
			SummaryID              string    `json:"summary_id"`
			TaskID                 string    `json:"task_id"`
			TotalTrades            int       `json:"total_trades"`
			WinRate                float64   `json:"win_rate"`
			AvgReturnPct           float64   `json:"avg_return_pct"`
			TotalReturnPct         float64   `json:"total_return_pct"`
			MaxDrawdownPct         float64   `json:"max_drawdown_pct"`
			BestReturnPct          float64   `json:"best_return_pct"`
			WorstReturnPct         float64   `json:"worst_return_pct"`
			BenchmarkSymbol        string    `json:"benchmark_symbol"`
			BenchmarkReturnPct     float64   `json:"benchmark_return_pct"`
			AvgExcessReturnPct     float64   `json:"avg_excess_return_pct"`
			ReturnDistributionJSON string    `json:"return_distribution_json"`
			MetaJSON               string    `json:"meta_json"`
			CreatedAt              time.Time `json:"created_at"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		summary := model.BacktestSummary{
			SummaryID: row.SummaryID,
			TaskID:    row.TaskID,
			MetaJSON:  row.MetaJSON,
			CreatedAt: row.CreatedAt,
			Metrics: model.BacktestMetrics{
				TotalTrades: row.TotalTrades,
				WinRate:     row.WinRate,
				TotalReturn: row.TotalReturnPct,
				MaxDrawdown: row.MaxDrawdownPct,
			},
		}
		summary.MetaJSON = mergeBacktestSummaryMeta(row.MetaJSON, row.ReturnDistributionJSON, row.AvgReturnPct, row.BestReturnPct, row.WorstReturnPct, row.BenchmarkSymbol, row.BenchmarkReturnPct, row.AvgExcessReturnPct)
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func mergeBacktestSummaryMeta(metaJSON, distributionJSON string, avgReturn, bestReturn, worstReturn float64, benchmarkSymbol string, benchmarkReturn, avgExcessReturn float64) string {
	meta := map[string]any{}
	if strings.TrimSpace(metaJSON) != "" {
		_ = json.Unmarshal([]byte(metaJSON), &meta)
	}
	if strings.TrimSpace(distributionJSON) != "" {
		var distribution map[string]int
		if err := json.Unmarshal([]byte(distributionJSON), &distribution); err == nil {
			meta["return_distribution"] = distribution
		}
	}
	meta["avg_return_pct"] = avgReturn
	meta["best_return_pct"] = bestReturn
	meta["worst_return_pct"] = worstReturn
	meta["benchmark_symbol"] = benchmarkSymbol
	meta["benchmark_return_pct"] = benchmarkReturn
	meta["avg_excess_return_pct"] = avgExcessReturn
	data, err := json.Marshal(meta)
	if err != nil {
		return metaJSON
	}
	return string(data)
}

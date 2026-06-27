package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"stock-etf-monitor/backend/model"
)

func (r *PostgresRepository) UpsertStockTask(task model.StockTask) error {
	params := task.Params
	if strings.TrimSpace(params) == "" {
		params = "{}"
	}
	return r.exec(`
	insert into stock_tasks
	  (task_id, task_type, status, params, result_ref, progress, attempts, last_error, created_by, created_at, started_at, finished_at, updated_at)
	values
	  ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	on conflict (task_id) do update set
	  task_type = excluded.task_type,
	  status = excluded.status,
	  params = excluded.params,
	  result_ref = excluded.result_ref,
	  progress = excluded.progress,
	  attempts = excluded.attempts,
	  last_error = excluded.last_error,
	  started_at = excluded.started_at,
	  finished_at = excluded.finished_at,
	  updated_at = excluded.updated_at;`,
		task.TaskID,
		task.TaskType,
		task.Status,
		params,
		task.ResultRef,
		task.Progress,
		task.Attempts,
		task.LastError,
		task.CreatedBy,
		task.CreatedAt,
		nullableTime(task.StartedAt),
		nullableTime(task.FinishedAt),
		task.UpdatedAt,
	)
}

func (r *PostgresRepository) ListStockTasks(taskType, status string, limit int) ([]model.StockTask, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t)
	from (
	  select task_id, task_type, status, params::text as params, result_ref, progress, attempts,
	         last_error, created_by, created_at,
	         coalesce(started_at, 'epoch'::timestamptz) as started_at,
	         coalesce(finished_at, 'epoch'::timestamptz) as finished_at,
	         updated_at
	  from stock_tasks
	  where ($1 = '' or task_type = $1)
	    and ($2 = '' or status = $2)
	  order by created_at desc
	  limit $3
	) t;`
	lines, err := r.queryLines(sql, strings.TrimSpace(taskType), strings.TrimSpace(status), limit)
	if err != nil {
		return nil, err
	}
	tasks := make([]model.StockTask, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var task model.StockTask
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (r *PostgresRepository) GetStockTask(taskID string) (*model.StockTask, error) {
	sql := `
	select row_to_json(t)
	from (
	  select task_id, task_type, status, params::text as params, result_ref, progress, attempts,
	         last_error, created_by, created_at,
	         coalesce(started_at, 'epoch'::timestamptz) as started_at,
	         coalesce(finished_at, 'epoch'::timestamptz) as finished_at,
	         updated_at
	  from stock_tasks
	  where task_id = $1
	) t;`
	lines, err := r.queryLines(sql, taskID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	var task model.StockTask
	if err := json.Unmarshal([]byte(lines[0]), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *PostgresRepository) UpdateStockTaskStatus(taskID, status, resultRef, lastError string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	startExpr := "coalesce(started_at, now())"
	finishExpr := "null"
	if status == "running" {
		startExpr = "coalesce(started_at, now())"
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		finishExpr = "now()"
	}
	if err := r.exec(fmt.Sprintf(`
	update stock_tasks
	set status = $1::text,
	    result_ref = $2,
	    last_error = $3,
	    progress = $4,
	    started_at = %s,
	    finished_at = %s,
	    updated_at = now()
	where task_id = $5;`, startExpr, finishExpr),
		status, resultRef, lastError, progress, taskID); err != nil {
		return err
	}
	return r.updateStrategyRunStatus(taskID, status, resultRef)
}

func (r *PostgresRepository) AppendStockTaskLog(log model.StockTaskLog) error {
	context := log.Context
	if strings.TrimSpace(context) == "" {
		context = "{}"
	}
	return r.exec(`
	insert into stock_task_logs (log_id, task_id, level, message, context, created_at)
	values ($1, $2, $3, $4, $5::jsonb, $6);`,
		log.LogID,
		log.TaskID,
		log.Level,
		log.Message,
		context,
		log.CreatedAt,
	)
}

func (r *PostgresRepository) ListStockTaskLogs(taskID string, limit int) ([]model.StockTaskLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select row_to_json(t)
	from (
	  select log_id, task_id, level, message, context::text as context, created_at
	  from stock_task_logs
	  where task_id = $1
	  order by created_at asc
	  limit $2
	) t;`
	lines, err := r.queryLines(sql, taskID, limit)
	if err != nil {
		return nil, err
	}
	logs := make([]model.StockTaskLog, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var log model.StockTaskLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (r *PostgresRepository) updateStrategyRunStatus(taskID, status, resultRef string) error {
	summaryRef := ""
	if strings.HasPrefix(resultRef, "postgres:stock_backtest_results:") {
		summaryRef = strings.Replace(resultRef, "postgres:stock_backtest_results:", "postgres:stock_backtest_summaries:", 1)
	}
	return r.exec(`
	update stock_strategy_runs
	set status = $1,
	    result_ref = $2,
	    summary_ref = $3,
	    updated_at = now()
	where task_id = $4;`,
		status, resultRef, summaryRef, taskID)
}

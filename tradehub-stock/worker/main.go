package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"stock-etf-monitor/backend/internal/domain/indicator"
	"stock-etf-monitor/backend/internal/domain/pattern"
	"stock-etf-monitor/backend/model"
)

// PostgresRepository 使用 database/sql + lib/pq 连接，所有写入/查询走 $N 占位参数化，
// 杜绝 SQL 注入。结果查询统一为 select row_to_json(t)，单列 JSON 文本由调用方反序列化。
type PostgresRepository struct {
	db *sql.DB
}

type TdengineRepository struct {
	url      string
	auth     string
	database string
	client   *http.Client
}

// MarketDataService 是 Worker 的行情服务层：按市场选择底层 provider + fallback。
type MarketDataService struct {
	cn     *MarketAPIClient
	python *PythonProvider
	tdx    *PythonProvider
}

// PythonProvider 是历史 Python provider：akshare / yfinance / Frankfurter fallback。
type PythonProvider struct {
	pythonBin string
	script    string
}

func isCNSymbol(symbol string) bool {
	s := strings.TrimSpace(symbol)
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type TaskRunner struct {
	repo                  *PostgresRepository
	td                    *TdengineRepository
	adapter               *MarketDataService
	runningTimeoutSeconds int
}

type SnapshotResponse struct {
	Item map[string]any `json:"item"`
}

type BarsResponse struct {
	Items []map[string]any `json:"items"`
}

type ScreeningConditionSet struct {
	Logic               string                 `json:"logic"`
	IndicatorConditions []IndicatorCondition   `json:"indicator_conditions"`
	PatternConditions   []PatternCondition     `json:"pattern_conditions"`
	Extra               map[string]interface{} `json:"-"`
}

type IndicatorCondition struct {
	Period        string  `json:"period"`
	IndicatorCode string  `json:"indicator_code"`
	Field         string  `json:"field"`
	Op            string  `json:"op"`
	Threshold     float64 `json:"threshold"`
}

type PatternCondition struct {
	Period      string `json:"period"`
	PatternCode string `json:"pattern_code"`
	Direction   string `json:"direction"`
}

type ScreeningCandidate struct {
	Symbol            string           `json:"symbol"`
	Score             float64          `json:"score"`
	MatchedConditions []map[string]any `json:"matched_conditions"`
	Snapshot          map[string]any   `json:"snapshot"`
}

type BacktestTrade struct {
	Symbol             string         `json:"symbol"`
	Period             string         `json:"period"`
	EntryTime          time.Time      `json:"entry_time"`
	ExitTime           time.Time      `json:"exit_time"`
	EntryPrice         float64        `json:"entry_price"`
	ExitPrice          float64        `json:"exit_price"`
	ReturnPct          float64        `json:"return_pct"`
	BenchmarkSymbol    string         `json:"benchmark_symbol"`
	BenchmarkReturnPct float64        `json:"benchmark_return_pct"`
	ExcessReturnPct    float64        `json:"excess_return_pct"`
	Meta               map[string]any `json:"meta"`
}

type BacktestSummary struct {
	TaskID             string         `json:"task_id"`
	TotalTrades        int            `json:"total_trades"`
	WinRate            float64        `json:"win_rate"`
	AvgReturnPct       float64        `json:"avg_return_pct"`
	TotalReturnPct     float64        `json:"total_return_pct"`
	MaxDrawdownPct     float64        `json:"max_drawdown_pct"`
	BestReturnPct      float64        `json:"best_return_pct"`
	WorstReturnPct     float64        `json:"worst_return_pct"`
	BenchmarkSymbol    string         `json:"benchmark_symbol"`
	BenchmarkReturnPct float64        `json:"benchmark_return_pct"`
	AvgExcessReturnPct float64        `json:"avg_excess_return_pct"`
	ReturnDistribution map[string]int `json:"return_distribution"`
	Meta               map[string]any `json:"meta"`
}

type BacktestBar struct {
	TS    time.Time
	Close float64
}

var repoGetStrategyTemplate = (*PostgresRepository).GetStrategyTemplate

var defaultFields = []string{
	"price",
	"volume",
	"amount",
	"turnover_rate",
	"turnover_amount",
	"volume_ratio",
	"premium_ratio",
	"buy_sell_5",
	"order_flow",
}

func main() {
	repo := NewPostgresRepository()
	if err := repo.Initialize(); err != nil {
		log.Fatalf("[backend-worker] postgres initialize failed: %v", err)
	}
	td := NewTdengineRepository()
	if err := td.Initialize(); err != nil {
		log.Fatalf("[backend-worker] tdengine initialize failed: %v", err)
	}
	seconds := envInt("WORKER_HEARTBEAT_SECONDS", 10)
	if seconds < 10 {
		log.Printf("[backend-worker] WORKER_HEARTBEAT_SECONDS=%d is too low, clamped to 10s", seconds)
		seconds = 10
	}
	runningTimeoutSeconds := envInt("WORKER_RUNNING_TIMEOUT_SECONDS", 120)
	if runningTimeoutSeconds < 30 {
		log.Printf("[backend-worker] WORKER_RUNNING_TIMEOUT_SECONDS=%d is too low, clamped to 30s", runningTimeoutSeconds)
		runningTimeoutSeconds = 30
	}
	runner := &TaskRunner{
		repo:                  repo,
		td:                    td,
		adapter:               NewMarketDataService(),
		runningTimeoutSeconds: runningTimeoutSeconds,
	}
	alertSeconds := envInt("WORKER_ALERT_SECONDS", 10)
	log.Printf("[backend-worker] ingestion poll every %ds, alert scan every %ds, running timeout %ds", seconds, alertSeconds, runningTimeoutSeconds)

	// alert 扫描独立 goroutine，避免拖慢采集主循环
	go func() {
		for {
			if err := runner.RunAlertScan(); err != nil {
				log.Printf("[backend-worker][alert] scan failed: %v", err)
			}
			time.Sleep(time.Duration(alertSeconds) * time.Second)
		}
	}()

	for {
		if err := runner.RunPending(); err != nil {
			log.Printf("[backend-worker] poll failed: %v", err)
		}
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

func NewPostgresRepository() *PostgresRepository {
	host := env("POSTGRES_HOST", "postgres")
	port := env("POSTGRES_PORT", "5432")
	db := env("POSTGRES_DB", "stock_etf")
	user := env("POSTGRES_USER", "stock")
	password := env("POSTGRES_PASSWORD", "stock_dev_password")
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, db)
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("postgres open failed: %v", err)
		return &PostgresRepository{}
	}
	if err := database.Ping(); err != nil {
		log.Printf("postgres ping failed: %v", err)
	}
	return &PostgresRepository{db: database}
}

func (r *PostgresRepository) Initialize() error {
	sql := `
	create table if not exists ingestion_task_configs (
	  task_id varchar(64) primary key,
	  symbol varchar(32) not null,
	  name varchar(128) not null,
	  market varchar(32) not null,
	  interval varchar(16) not null,
	  fields jsonb not null default '[]'::jsonb,
	  enabled boolean not null default true,
	  status varchar(32) not null default 'pending',
	  source varchar(32) not null default 'akshare',
	  last_message text not null default '',
	  last_run_at timestamptz,
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	alter table ingestion_task_configs add column if not exists last_message text not null default '';
	alter table ingestion_task_configs add column if not exists last_run_at timestamptz;
	alter table ingestion_task_configs add column if not exists fields jsonb not null default '[]'::jsonb;
	create table if not exists instrument_configs (
	  symbol varchar(32) primary key,
	  name varchar(128) not null,
	  market varchar(32) not null,
	  enabled boolean not null default true,
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create table if not exists stock_tasks (
	  task_id varchar(64) primary key,
	  task_type varchar(64) not null,
	  status varchar(32) not null default 'pending',
	  params jsonb not null default '{}'::jsonb,
	  result_ref text not null default '',
	  progress int not null default 0,
	  attempts int not null default 0,
	  last_error text not null default '',
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  started_at timestamptz,
	  finished_at timestamptz,
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_tasks_type_status on stock_tasks(task_type, status, created_at desc);
	create table if not exists stock_task_logs (
	  log_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  level varchar(16) not null default 'info',
	  message text not null default '',
	  context jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	create index if not exists idx_stock_task_logs_task on stock_task_logs(task_id, created_at asc);
	create table if not exists stock_screening_templates (
	  template_id varchar(64) primary key,
	  name varchar(128) not null,
	  description text not null default '',
	  conditions jsonb not null default '{}'::jsonb,
	  enabled boolean not null default true,
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_screening_templates_enabled on stock_screening_templates(enabled, updated_at desc);
	create table if not exists stock_strategy_templates (
	  strategy_id varchar(64) primary key,
	  name varchar(128) not null,
	  description text not null default '',
	  screening_template_id varchar(64) not null default '',
	  conditions jsonb not null default '{}'::jsonb,
	  backtest_params jsonb not null default '{}'::jsonb,
	  risk_params jsonb not null default '{}'::jsonb,
	  enabled boolean not null default true,
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_strategy_templates_enabled on stock_strategy_templates(enabled, updated_at desc);
	create table if not exists stock_strategy_runs (
	  run_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  task_type varchar(64) not null,
	  strategy_id varchar(64) not null,
	  snapshot_id varchar(64) not null default '',
	  strategy_name varchar(128) not null default '',
	  status varchar(32) not null default 'pending',
	  result_ref text not null default '',
	  summary_ref text not null default '',
	  created_by varchar(64) not null default 'system',
	  created_at timestamptz not null default now(),
	  updated_at timestamptz not null default now()
	);
	create index if not exists idx_stock_strategy_runs_strategy on stock_strategy_runs(strategy_id, created_at desc);
	create index if not exists idx_stock_strategy_runs_task on stock_strategy_runs(task_id, created_at desc);
	create table if not exists stock_screening_results (
	  result_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  template_id varchar(64) not null default '',
	  symbol varchar(32) not null,
	  score double precision not null default 0,
	  matched_conditions jsonb not null default '[]'::jsonb,
	  snapshot jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	create index if not exists idx_stock_screening_results_task on stock_screening_results(task_id, score desc, created_at desc);
	create index if not exists idx_stock_screening_results_template on stock_screening_results(template_id, created_at desc);
	create table if not exists stock_backtest_results (
	  result_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  symbol varchar(32) not null,
	  period varchar(16) not null default '1d',
	  entry_time timestamptz not null,
	  exit_time timestamptz not null,
	  entry_price double precision not null default 0,
	  exit_price double precision not null default 0,
	  return_pct double precision not null default 0,
	  benchmark_symbol varchar(32) not null default '',
	  benchmark_return_pct double precision not null default 0,
	  excess_return_pct double precision not null default 0,
	  meta jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	alter table stock_backtest_results add column if not exists benchmark_symbol varchar(32) not null default '';
	alter table stock_backtest_results add column if not exists benchmark_return_pct double precision not null default 0;
	alter table stock_backtest_results add column if not exists excess_return_pct double precision not null default 0;
	create index if not exists idx_stock_backtest_results_task on stock_backtest_results(task_id, return_pct desc, created_at desc);
	create index if not exists idx_stock_backtest_results_symbol on stock_backtest_results(symbol, created_at desc);
	create table if not exists stock_backtest_summaries (
	  summary_id varchar(64) primary key,
	  task_id varchar(64) not null references stock_tasks(task_id) on delete cascade,
	  total_trades int not null default 0,
	  win_rate double precision not null default 0,
	  avg_return_pct double precision not null default 0,
	  total_return_pct double precision not null default 0,
	  max_drawdown_pct double precision not null default 0,
	  best_return_pct double precision not null default 0,
	  worst_return_pct double precision not null default 0,
	  benchmark_symbol varchar(32) not null default '',
	  benchmark_return_pct double precision not null default 0,
	  avg_excess_return_pct double precision not null default 0,
	  return_distribution jsonb not null default '{}'::jsonb,
	  meta jsonb not null default '{}'::jsonb,
	  created_at timestamptz not null default now()
	);
	create index if not exists idx_stock_backtest_summaries_task on stock_backtest_summaries(task_id, created_at desc);
	`
	return r.exec(sql)
}

func (r *PostgresRepository) ListRunnableTasks(limit int) ([]model.IngestionTask, error) {
	sql := fmt.Sprintf(`
	select row_to_json(t)
	from (
	  select
	    task_id,
	    symbol,
	    name,
	    market,
	    interval,
	    fields,
	    enabled,
	    status,
	    source,
	    created_at,
	    updated_at,
	    coalesce(last_run_at, 'epoch'::timestamptz) as last_run_at,
	    last_message
	  from ingestion_task_configs
	  where enabled = true
	    and (
	      status in ('pending', 'retry')
	      or (
	        status = 'completed'
	        and coalesce(last_run_at, 'epoch'::timestamptz) <= now() - case interval
	          when '秒级' then interval '1 second'
	          when '5s' then interval '5 seconds'
	          when '10s' then interval '10 seconds'
	          when '30s' then interval '30 seconds'
	          when '1m' then interval '1 minute'
	          when '5m' then interval '5 minutes'
	          when '10m' then interval '10 minutes'
	          when '30m' then interval '30 minutes'
	          when '1h' then interval '1 hour'
	          when '1d' then interval '1 day'
	          else interval '1 minute'
	        end
	      )
	    )
	  order by created_at asc
	  limit %d
	) t;
	`, limit)
	return r.queryTasks(sql)
}

func (r *PostgresRepository) RecoverStaleRunningTasks(timeoutSeconds int) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("postgres 未连接")
	}
	var count int
	err := r.db.QueryRow(fmt.Sprintf(`
	update ingestion_task_configs
	set status = 'retry',
	    last_message = 'recovered stale running task after %d seconds',
	    updated_at = now()
	where enabled = true
	  and status = 'running'
	  and coalesce(last_run_at, 'epoch'::timestamptz) <= now() - interval '%d seconds'
	returning 1;`, timeoutSeconds, timeoutSeconds)).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return count, nil
}

func (r *PostgresRepository) UpdateTaskStatus(taskID, status, message string) error {
	return r.exec(`
	update ingestion_task_configs
	set status = $1,
	    last_message = $2,
	    last_run_at = now(),
	    updated_at = now()
	where task_id = $3;`, status, message, taskID)
}

func (r *PostgresRepository) ListRunnableStockTasks(limit int) ([]model.StockTask, error) {
	sql := `
	select row_to_json(t)
	from (
	  select task_id, task_type, status, params::text as params, result_ref, progress, attempts,
	         last_error, created_by, created_at,
	         coalesce(started_at, 'epoch'::timestamptz) as started_at,
	         coalesce(finished_at, 'epoch'::timestamptz) as finished_at,
	         updated_at
	  from stock_tasks
	  where status in ('pending', 'retrying')
	  order by created_at asc
	  limit $1
	) t;`
	lines, err := r.queryLines(sql, limit)
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
	    attempts = attempts + case when $1::text in ('retrying', 'failed') then 1 else 0 end,
	    started_at = %s,
	    finished_at = %s,
	    updated_at = now()
	where task_id = $5;`, startExpr, finishExpr),
		status, resultRef, lastError, progress, taskID); err != nil {
		return err
	}
	return r.updateStrategyRunStatus(taskID, status, resultRef)
}

func (r *PostgresRepository) AppendStockTaskLog(taskID, level, message string, context map[string]any) error {
	rawContext, err := json.Marshal(context)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return r.exec(`
	insert into stock_task_logs (log_id, task_id, level, message, context, created_at)
	values ($1, $2, $3, $4, $5::jsonb, $6);`,
		fmt.Sprintf("task_log_%d", now.UnixNano()),
		taskID, level, message, string(rawContext), now)
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
	where task_id = $4;`, status, resultRef, summaryRef, taskID)
}

func (r *PostgresRepository) GetScreeningTemplate(templateID string) (*model.ScreeningTemplate, error) {
	sql := `
	select row_to_json(t)
	from (
	  select template_id, name, description, conditions::text as conditions_json,
	         enabled, created_by, created_at, updated_at
	  from stock_screening_templates
	  where template_id = $1 and enabled = true
	  limit 1
	) t;`
	lines, err := r.queryLines(sql, templateID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("screening template not found or disabled: %s", templateID)
	}
	var template model.ScreeningTemplate
	if err := json.Unmarshal([]byte(lines[0]), &template); err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *PostgresRepository) GetStrategyTemplate(strategyID string) (*model.StrategyTemplate, error) {
	sql := `
	select row_to_json(t)
	from (
	  select strategy_id, name, description, screening_template_id,
	         conditions::text as conditions_json,
	         backtest_params::text as backtest_params_json,
	         risk_params::text as risk_params_json,
	         enabled, created_by, created_at, updated_at
	  from stock_strategy_templates
	  where strategy_id = $1 and enabled = true
	  limit 1
	) t;`
	lines, err := r.queryLines(sql, strategyID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("strategy template not found or disabled: %s", strategyID)
	}
	var template model.StrategyTemplate
	if err := json.Unmarshal([]byte(lines[0]), &template); err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *PostgresRepository) ReplaceScreeningResults(taskID string, results []ScreeningCandidate, templateID string) error {
	if err := r.exec(`delete from stock_screening_results where task_id = $1;`, taskID); err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}
	now := time.Now().UTC()
	stmts := make([]sqlStmt, 0, len(results))
	for idx, result := range results {
		rawMatched, err := json.Marshal(result.MatchedConditions)
		if err != nil {
			return err
		}
		rawSnapshot, err := json.Marshal(result.Snapshot)
		if err != nil {
			return err
		}
		resultID := fmt.Sprintf("%s_%04d", taskID, idx+1)
		stmts = append(stmts, sqlStmt{
			query: `
			insert into stock_screening_results
			  (result_id, task_id, template_id, symbol, score, matched_conditions, snapshot, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (result_id) do update set
			  score = excluded.score,
			  matched_conditions = excluded.matched_conditions,
			  snapshot = excluded.snapshot,
			  created_at = excluded.created_at;`,
			args: []any{
				resultID,
				taskID,
				templateID,
				result.Symbol,
				formatFloat(result.Score),
				string(rawMatched),
				string(rawSnapshot),
				now,
			},
		})
	}
	return r.execTx(stmts)
}

func (r *PostgresRepository) ListScreeningResultSymbols(taskID string, limit int) ([]string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sql := `
	select symbol
	from stock_screening_results
	where task_id = $1
	order by score desc, created_at desc
	limit $2;`
	lines, err := r.queryLines(sql, taskID, limit)
	if err != nil {
		return nil, err
	}
	symbols := make([]string, 0, len(lines))
	for _, line := range lines {
		symbol := strings.TrimSpace(line)
		if symbol == "" {
			continue
		}
		symbols = append(symbols, symbol)
	}
	return symbols, nil
}

func (r *PostgresRepository) ReplaceBacktestResults(taskID string, trades []BacktestTrade) error {
	if err := r.exec(`delete from stock_backtest_results where task_id = $1;`, taskID); err != nil {
		return err
	}
	if len(trades) == 0 {
		return nil
	}
	now := time.Now().UTC()
	stmts := make([]sqlStmt, 0, len(trades))
	for idx, trade := range trades {
		rawMeta, err := json.Marshal(trade.Meta)
		if err != nil {
			return err
		}
		resultID := fmt.Sprintf("%s_%04d", taskID, idx+1)
		stmts = append(stmts, sqlStmt{
			query: `
			insert into stock_backtest_results
			  (result_id, task_id, symbol, period, entry_time, exit_time, entry_price, exit_price, return_pct, benchmark_symbol, benchmark_return_pct, excess_return_pct, meta, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			on conflict (result_id) do update set
			  entry_time = excluded.entry_time,
			  exit_time = excluded.exit_time,
			  entry_price = excluded.entry_price,
			  exit_price = excluded.exit_price,
			  return_pct = excluded.return_pct,
			  benchmark_symbol = excluded.benchmark_symbol,
			  benchmark_return_pct = excluded.benchmark_return_pct,
			  excess_return_pct = excluded.excess_return_pct,
			  meta = excluded.meta,
			  created_at = excluded.created_at;`,
			args: []any{
				resultID,
				taskID,
				trade.Symbol,
				trade.Period,
				trade.EntryTime,
				trade.ExitTime,
				trade.EntryPrice,
				trade.ExitPrice,
				trade.ReturnPct,
				trade.BenchmarkSymbol,
				trade.BenchmarkReturnPct,
				trade.ExcessReturnPct,
				string(rawMeta),
				now,
			},
		})
	}
	return r.execTx(stmts)
}

func (r *PostgresRepository) ReplaceBacktestSummary(summary BacktestSummary) error {
	rawDistribution, err := json.Marshal(summary.ReturnDistribution)
	if err != nil {
		return err
	}
	rawMeta, err := json.Marshal(summary.Meta)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return r.exec(`
	insert into stock_backtest_summaries
	  (summary_id, task_id, total_trades, win_rate, avg_return_pct, total_return_pct, max_drawdown_pct,
	   best_return_pct, worst_return_pct, benchmark_symbol, benchmark_return_pct, avg_excess_return_pct,
	   return_distribution, meta, created_at)
	values
	  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14::jsonb, $15)
	on conflict (summary_id) do update set
	  total_trades = excluded.total_trades,
	  win_rate = excluded.win_rate,
	  avg_return_pct = excluded.avg_return_pct,
	  total_return_pct = excluded.total_return_pct,
	  max_drawdown_pct = excluded.max_drawdown_pct,
	  best_return_pct = excluded.best_return_pct,
	  worst_return_pct = excluded.worst_return_pct,
	  benchmark_symbol = excluded.benchmark_symbol,
	  benchmark_return_pct = excluded.benchmark_return_pct,
	  avg_excess_return_pct = excluded.avg_excess_return_pct,
	  return_distribution = excluded.return_distribution,
	  meta = excluded.meta,
	  created_at = excluded.created_at;`,
		"summary_"+summary.TaskID,
		summary.TaskID,
		summary.TotalTrades,
		formatFloat(summary.WinRate),
		formatFloat(summary.AvgReturnPct),
		formatFloat(summary.TotalReturnPct),
		formatFloat(summary.MaxDrawdownPct),
		formatFloat(summary.BestReturnPct),
		formatFloat(summary.WorstReturnPct),
		summary.BenchmarkSymbol,
		formatFloat(summary.BenchmarkReturnPct),
		formatFloat(summary.AvgExcessReturnPct),
		string(rawDistribution),
		string(rawMeta),
		now,
	)
}

// exec 执行一条写入语句，参数通过 $N 占位符绑定，杜绝注入。
func (r *PostgresRepository) exec(query string, args ...any) error {
	if r.db == nil {
		return fmt.Errorf("postgres 未连接")
	}
	if _, err := r.db.Exec(query, args...); err != nil {
		return fmt.Errorf("postgres exec failed: %w", err)
	}
	return nil
}

// execTx 在单个事务里按顺序执行多条语句（每条可带自己的参数）。
// lib/pq 扩展协议不支持一次 Exec 跑多条带占位符的语句，故拆成事务。
func (r *PostgresRepository) execTx(stmts []sqlStmt) error {
	if r.db == nil {
		return fmt.Errorf("postgres 未连接")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("postgres begin failed: %w", err)
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s.query, s.args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("postgres tx exec failed: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres commit failed: %w", err)
	}
	return nil
}

// execRaw 执行不带参数的多语句脚本（建表/初始化），走简单协议。
func (r *PostgresRepository) execRaw(script string) error {
	if r.db == nil {
		return fmt.Errorf("postgres 未连接")
	}
	if _, err := r.db.Exec(script); err != nil {
		return fmt.Errorf("postgres exec failed: %w", err)
	}
	return nil
}

// sqlStmt 是事务内的一条带参数语句。
type sqlStmt struct {
	query string
	args  []any
}

// queryLines 执行 select row_to_json(t) 查询，把单列 JSON 文本逐行收集为 []string，
// 调用方按既有逻辑 json.Unmarshal 每行即可，输出契约与原 psql -t -A 一致。
func (r *PostgresRepository) queryLines(query string, args ...any) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("postgres 未连接")
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query failed: %w", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line sql.NullString
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("postgres scan failed: %w", err)
		}
		if line.Valid && strings.TrimSpace(line.String) != "" {
			lines = append(lines, line.String)
		}
	}
	return lines, nil
}

func (r *PostgresRepository) queryTasks(sql string, args ...any) ([]model.IngestionTask, error) {
	lines, err := r.queryLines(sql, args...)
	if err != nil {
		return nil, err
	}
	tasks := make([]model.IngestionTask, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var task model.IngestionTask
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			return nil, err
		}
		task.Fields = normalizeFields(task.Fields)
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func NewTdengineRepository() *TdengineRepository {
	host := env("TDENGINE_HOST", "tdengine")
	port := env("TDENGINE_PORT", "6041")
	user := env("TDENGINE_USER", "root")
	password := env("TDENGINE_PASSWORD", "taosdata")
	return &TdengineRepository{
		url:      fmt.Sprintf("http://%s:%s/rest/sql", host, port),
		auth:     "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password)),
		database: env("TDENGINE_DATABASE", "stock_etf_ts"),
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (r *TdengineRepository) Initialize() error {
	if _, err := r.Exec("create database if not exists " + r.database); err != nil {
		return err
	}
	if _, err := r.Exec(fmt.Sprintf(`
create stable if not exists %s.market_bars (
  ts timestamp,
  open double,
  close double,
  high double,
  low double,
  volume double,
  amount double,
  turnover_rate double,
  turnover_amount double,
  volume_ratio double,
  premium_ratio double,
  big_order_volume double,
  medium_order_volume double,
  small_order_volume double,
  bid_1_price double,
  bid_1_volume double,
  bid_2_price double,
  bid_2_volume double,
  bid_3_price double,
  bid_3_volume double,
  bid_4_price double,
  bid_4_volume double,
  bid_5_price double,
  bid_5_volume double,
  ask_1_price double,
  ask_1_volume double,
  ask_2_price double,
  ask_2_volume double,
  ask_3_price double,
  ask_3_volume double,
  ask_4_price double,
  ask_4_volume double,
  ask_5_price double,
  ask_5_volume double,
  requested_period binary(16),
  source_period binary(16)
) tags (
  symbol binary(16),
  market binary(16),
  period binary(16)
)`, r.database)); err != nil {
		return err
	}
	if _, err := r.Exec(fmt.Sprintf(`
create stable if not exists %s.indicator_values (
  ts timestamp,
  indicator_value double,
  values_json nchar(2048)
) tags (
  symbol binary(16),
  period binary(16),
  indicator_code binary(64)
)`, r.database)); err != nil {
		return err
	}
	if _, err := r.Exec(fmt.Sprintf(`
create stable if not exists %s.pattern_hits (
  ts timestamp,
  pattern_value int,
  direction binary(16),
  extra_json nchar(2048),
  algorithm_version binary(64)
) tags (
  symbol binary(16),
  period binary(16),
  pattern_code binary(96)
)`, r.database)); err != nil {
		return err
	}
	// 兼容老库：超级表已存在但没有 period tag 的情况，尝试 ALTER ADD TAG（已存在则忽略）
	if _, err := r.Exec(fmt.Sprintf(`alter stable %s.market_bars add tag period binary(16)`, r.database)); err != nil {
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "already") && !strings.Contains(msg, "duplicate") && !strings.Contains(msg, "exist") {
			return err
		}
	}
	return nil
}

// periodSuffix 把任务周期映射成 TDengine 子表名安全的后缀。
// 「秒级」无法作为标识符，统一映射成 tick；其余周期天然合法直接复用。
func periodSuffix(interval string) string {
	switch interval {
	case "秒级", "tick", "":
		return "tick"
	case "5s", "10s", "30s", "1m", "5m", "10m", "30m", "1h", "1d":
		return interval
	default:
		return sanitizeIdentifier(interval)
	}
}

func (r *TdengineRepository) InsertBars(symbol, market, interval string, fields []string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	selected := makeFieldSet(fields)
	suffix := periodSuffix(interval)
	table := fmt.Sprintf("%s.bars_%s_%s", r.database, sanitizeIdentifier(symbol), suffix)
	if _, err := r.Exec(fmt.Sprintf(`create table if not exists %s using %s.market_bars tags ("%s", "%s", "%s")`,
		table, r.database, escapeDoubleQuote(symbol), escapeDoubleQuote(market), escapeDoubleQuote(suffix))); err != nil {
		return err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, fmt.Sprintf(
			`(%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
			tdTimestampLiteral(row["ts"]),
			numericValue(row, selected, "price", "open"),
			numericValue(row, selected, "price", "close"),
			numericValue(row, selected, "price", "high"),
			numericValue(row, selected, "price", "low"),
			numericValue(row, selected, "volume", "volume"),
			numericValue(row, selected, "amount", "amount"),
			numericValue(row, selected, "turnover_rate", "turnover_rate"),
			numericValue(row, selected, "turnover_amount", "turnover_amount"),
			numericValue(row, selected, "volume_ratio", "volume_ratio"),
			numericValue(row, selected, "premium_ratio", "premium_ratio"),
			numericValue(row, selected, "order_flow", "big_order_volume"),
			numericValue(row, selected, "order_flow", "medium_order_volume"),
			numericValue(row, selected, "order_flow", "small_order_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_1_price"),
			numericValue(row, selected, "buy_sell_5", "bid_1_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_2_price"),
			numericValue(row, selected, "buy_sell_5", "bid_2_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_3_price"),
			numericValue(row, selected, "buy_sell_5", "bid_3_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_4_price"),
			numericValue(row, selected, "buy_sell_5", "bid_4_volume"),
			numericValue(row, selected, "buy_sell_5", "bid_5_price"),
			numericValue(row, selected, "buy_sell_5", "bid_5_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_1_price"),
			numericValue(row, selected, "buy_sell_5", "ask_1_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_2_price"),
			numericValue(row, selected, "buy_sell_5", "ask_2_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_3_price"),
			numericValue(row, selected, "buy_sell_5", "ask_3_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_4_price"),
			numericValue(row, selected, "buy_sell_5", "ask_4_volume"),
			numericValue(row, selected, "buy_sell_5", "ask_5_price"),
			numericValue(row, selected, "buy_sell_5", "ask_5_volume"),
			sqlStringLiteral(fmt.Sprintf("%v", row["requested_period"])),
			sqlStringLiteral(fmt.Sprintf("%v", row["source_period"])),
		))
	}
	_, err := r.Exec("insert into " + table + " values " + strings.Join(values, ","))
	return err
}

func (r *TdengineRepository) QueryIndicatorBars(symbol, period string, limit int) ([]indicator.Bar, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	suffix := periodSuffix(period)
	table := fmt.Sprintf("%s.bars_%s_%s", r.database, sanitizeIdentifier(symbol), suffix)
	sql := fmt.Sprintf(`
select ts, open, close, high, low, volume
from %s
order by ts desc
limit %d`, table, limit)
	payload, err := r.Exec(sql)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "not exist") ||
			strings.Contains(msg, "table does not exist") || strings.Contains(msg, "invalid") {
			return []indicator.Bar{}, nil
		}
		return nil, err
	}
	columnMeta, ok := payload["column_meta"].([]any)
	if !ok {
		return []indicator.Bar{}, nil
	}
	dataRows, ok := payload["data"].([]any)
	if !ok {
		return []indicator.Bar{}, nil
	}
	columns := make([]string, 0, len(columnMeta))
	for _, item := range columnMeta {
		meta, ok := item.([]any)
		if !ok || len(meta) == 0 {
			continue
		}
		columns = append(columns, fmt.Sprintf("%v", meta[0]))
	}
	bars := make([]indicator.Bar, 0, len(dataRows))
	for i := len(dataRows) - 1; i >= 0; i-- {
		row, ok := dataRows[i].([]any)
		if !ok {
			continue
		}
		record := map[string]any{}
		for idx, column := range columns {
			if idx < len(row) {
				record[column] = row[idx]
			}
		}
		ts, ok := parseAnyTime(record["ts"])
		if !ok {
			continue
		}
		bars = append(bars, indicator.Bar{
			TS:     ts,
			Open:   anyFloat(record["open"]),
			Close:  anyFloat(record["close"]),
			High:   anyFloat(record["high"]),
			Low:    anyFloat(record["low"]),
			Volume: anyFloat(record["volume"]),
		})
	}
	return bars, nil
}

func (r *TdengineRepository) QueryBacktestBars(symbol, period string, limit int) ([]BacktestBar, error) {
	if limit <= 1 || limit > 5000 {
		limit = 260
	}
	suffix := periodSuffix(period)
	table := fmt.Sprintf("%s.bars_%s_%s", r.database, sanitizeIdentifier(symbol), suffix)
	sql := fmt.Sprintf(`
select ts, close
from %s
order by ts desc
limit %d`, table, limit)
	rows, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []BacktestBar{}, nil
		}
		return nil, err
	}
	bars := make([]BacktestBar, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		ts, ok := parseAnyTime(rows[i]["ts"])
		if !ok {
			continue
		}
		closeValue := anyFloat(rows[i]["close"])
		if closeValue <= 0 {
			continue
		}
		bars = append(bars, BacktestBar{TS: ts, Close: closeValue})
	}
	return bars, nil
}

func (r *TdengineRepository) InsertIndicatorValues(symbol, period, code string, points []indicator.Point) error {
	if len(points) == 0 {
		return nil
	}
	suffix := periodSuffix(period)
	table := fmt.Sprintf("%s.ind_%s_%s_%s", r.database, sanitizeIdentifier(symbol), suffix, sanitizeIdentifier(code))
	if _, err := r.Exec(fmt.Sprintf(`create table if not exists %s using %s.indicator_values tags ("%s", "%s", "%s")`,
		table, r.database, escapeDoubleQuote(symbol), escapeDoubleQuote(suffix), escapeDoubleQuote(strings.ToUpper(code)))); err != nil {
		return err
	}
	values := make([]string, 0, len(points))
	for _, point := range points {
		rawValues, err := json.Marshal(point.Values)
		if err != nil {
			return err
		}
		values = append(values, fmt.Sprintf(`(%s, %s, '%s')`,
			tdTimestampLiteral(point.TS.Format(time.RFC3339Nano)),
			formatFloat(point.Value),
			escape(string(rawValues)),
		))
	}
	_, err := r.Exec("insert into " + table + " values " + strings.Join(values, ","))
	return err
}

func (r *TdengineRepository) InsertPatternHits(symbol, period string, hits []pattern.Hit) error {
	if len(hits) == 0 {
		return nil
	}
	grouped := map[string][]pattern.Hit{}
	for _, hit := range hits {
		grouped[hit.PatternCode] = append(grouped[hit.PatternCode], hit)
	}
	suffix := periodSuffix(period)
	for code, items := range grouped {
		table := fmt.Sprintf("%s.pattern_%s_%s_%s", r.database, sanitizeIdentifier(symbol), suffix, sanitizeIdentifier(code))
		if _, err := r.Exec(fmt.Sprintf(`create table if not exists %s using %s.pattern_hits tags ("%s", "%s", "%s")`,
			table, r.database, escapeDoubleQuote(symbol), escapeDoubleQuote(suffix), escapeDoubleQuote(code))); err != nil {
			return err
		}
		values := make([]string, 0, len(items))
		for _, hit := range items {
			rawExtra, err := json.Marshal(hit.Extra)
			if err != nil {
				return err
			}
			values = append(values, fmt.Sprintf(`(%s, %d, '%s', '%s', '%s')`,
				tdTimestampLiteral(hit.TS.Format(time.RFC3339Nano)),
				hit.Value,
				escape(hit.Direction),
				escape(string(rawExtra)),
				pattern.AlgorithmVersion,
			))
		}
		if _, err := r.Exec("insert into " + table + " values " + strings.Join(values, ",")); err != nil {
			return err
		}
	}
	return nil
}

func (r *TdengineRepository) ScreenByIndicator(condition IndicatorCondition, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	period := periodSuffix(condition.Period)
	code := strings.ToUpper(strings.TrimSpace(condition.IndicatorCode))
	sql := fmt.Sprintf(`
select ts, indicator_value, values_json, symbol, period, indicator_code
from %s.indicator_values
where period = '%s' and indicator_code = '%s'
order by ts desc
limit %d`, r.database, escape(period), escape(code), limit*20)
	rows, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	items := make([]map[string]any, 0, limit)
	seen := map[string]struct{}{}
	for _, row := range rows {
		if _, ok := row["value"]; !ok {
			row["value"] = row["indicator_value"]
		}
		symbol := strings.TrimSpace(fmt.Sprintf("%v", row["symbol"]))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		value, ok := indicatorFieldValue(row, condition.Field)
		if !ok || !compareFloat(value, normalizeCompareOp(condition.Op), condition.Threshold) {
			continue
		}
		row["condition_type"] = "indicator"
		row["field"] = condition.Field
		row["field_value"] = value
		row["threshold"] = condition.Threshold
		seen[symbol] = struct{}{}
		items = append(items, row)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *TdengineRepository) ScreenByPattern(condition PatternCondition, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	period := periodSuffix(condition.Period)
	code := strings.TrimSpace(condition.PatternCode)
	sql := fmt.Sprintf(`
select ts, pattern_value, direction, extra_json, algorithm_version, symbol, period, pattern_code
from %s.pattern_hits
where period = '%s' and pattern_code = '%s'
order by ts desc
limit %d`, r.database, escape(period), escape(code), limit*20)
	rows, err := r.queryRows(sql)
	if err != nil {
		if isMissingTDTable(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	items := make([]map[string]any, 0, limit)
	seen := map[string]struct{}{}
	wantDirection := strings.ToLower(strings.TrimSpace(condition.Direction))
	for _, row := range rows {
		symbol := strings.TrimSpace(fmt.Sprintf("%v", row["symbol"]))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		gotDirection := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", row["direction"])))
		if wantDirection != "" && gotDirection != wantDirection {
			continue
		}
		row["condition_type"] = "pattern"
		seen[symbol] = struct{}{}
		items = append(items, row)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *TdengineRepository) queryRows(sql string) ([]map[string]any, error) {
	payload, err := r.Exec(sql)
	if err != nil {
		return nil, err
	}
	columnMeta, ok := payload["column_meta"].([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	dataRows, ok := payload["data"].([]any)
	if !ok {
		return []map[string]any{}, nil
	}
	columns := make([]string, 0, len(columnMeta))
	for _, item := range columnMeta {
		meta, ok := item.([]any)
		if !ok || len(meta) == 0 {
			continue
		}
		columns = append(columns, fmt.Sprintf("%v", meta[0]))
	}
	items := make([]map[string]any, 0, len(dataRows))
	for _, dataRow := range dataRows {
		row, ok := dataRow.([]any)
		if !ok {
			continue
		}
		record := map[string]any{}
		for idx, column := range columns {
			if idx < len(row) {
				record[column] = row[idx]
			}
		}
		items = append(items, record)
	}
	return items, nil
}

func (r *TdengineRepository) Exec(sql string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.url, bytes.NewBufferString(sql))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", r.auth)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tdengine error: %s", string(body))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if code, ok := payload["code"].(float64); ok && code != 0 {
		return nil, fmt.Errorf("tdengine error: %s", string(body))
	}
	return payload, nil
}

func NewMarketDataService() *MarketDataService {
	return &MarketDataService{
		cn: NewMarketAPIClient(),
		python: &PythonProvider{
			pythonBin: env("AKSHARE_PYTHON_BIN", "python3.11"),
			script:    env("AKSHARE_SCRIPT_PATH", "/app/network/akshare_adapter.py"),
		},
		tdx: &PythonProvider{
			pythonBin: env("MOOTDX_PYTHON_BIN", env("AKSHARE_PYTHON_BIN", "python3.11")),
			script:    env("MOOTDX_SCRIPT_PATH", "/app/network/mootdx_adapter.py"),
		},
	}
}

func (s *MarketDataService) Snapshot(symbol string) (map[string]any, error) {
	if isCNSymbol(symbol) && s.cn.Enabled() {
		if item, err := s.cn.Snapshot(context.Background(), symbol); err == nil {
			return item, nil
		}
	}
	return s.python.Snapshot(symbol)
}

func (s *MarketDataService) Minute(symbol, period string) ([]map[string]any, error) {
	if isCNSymbol(symbol) {
		if items, err := s.tdx.Minute(symbol, period); err == nil && len(items) > 0 {
			return items, nil
		}
	}
	if isCNSymbol(symbol) && s.cn.Enabled() {
		if items, err := s.cn.Minute(context.Background(), symbol, period); err == nil {
			return items, nil
		}
	}
	return s.python.Minute(symbol, period)
}

func (s *MarketDataService) Daily(symbol string, limit int) ([]map[string]any, error) {
	if isCNSymbol(symbol) {
		if items, err := s.tdx.Daily(symbol, limit); err == nil && len(items) > 0 {
			return items, nil
		}
	}
	if isCNSymbol(symbol) && s.cn.Enabled() {
		if data, err := s.cn.Daily(context.Background(), symbol, limit); err == nil {
			if items, ok := data["items"].([]map[string]any); ok {
				return items, nil
			}
		}
	}
	return s.python.Daily(symbol, limit)
}

func (a *PythonProvider) Snapshot(symbol string) (map[string]any, error) {
	var response SnapshotResponse
	if err := a.run(&response, "snapshot", symbol); err != nil {
		return nil, err
	}
	return response.Item, nil
}

func (a *PythonProvider) Minute(symbol, period string) ([]map[string]any, error) {
	var response BarsResponse
	if err := a.run(&response, "minute", symbol, period); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (a *PythonProvider) Daily(symbol string, limit int) ([]map[string]any, error) {
	var response struct {
		Items []map[string]any `json:"items"`
	}
	if err := a.run(&response, "daily", symbol, "", "", fmt.Sprintf("%d", limit)); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (a *PythonProvider) run(target any, args ...string) error {
	cmd := exec.Command(a.pythonBin, append([]string{a.script}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if len(detail) > 1500 {
			detail = detail[:1500] + "...(truncated)"
		}
		return fmt.Errorf("akshare adapter failed: %v: %s", err, detail)
	}
	return json.Unmarshal(stdout.Bytes(), target)
}

func (r *TaskRunner) RunPending() error {
	recovered, err := r.repo.RecoverStaleRunningTasks(r.runningTimeoutSeconds)
	if err != nil {
		return err
	}
	if recovered > 0 {
		log.Printf("[backend-worker] recovered %d stale running tasks", recovered)
	}
	tasks, err := r.repo.ListRunnableTasks(32)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		log.Println("[backend-worker] no pending tasks")
	} else {
		for _, task := range tasks {
			if err := r.runTask(task); err != nil {
				log.Printf("[backend-worker] task %s failed: %v", task.TaskID, err)
			}
		}
	}
	stockTasks, err := r.repo.ListRunnableStockTasks(16)
	if err != nil {
		return err
	}
	for _, task := range stockTasks {
		if err := r.runStockTask(task); err != nil {
			log.Printf("[backend-worker] stock task %s failed: %v", task.TaskID, err)
		}
	}
	return nil
}

const maxRetry = 5

// failOrRetry: 在失败后决定是 继续 retry 还是 转 failed
// 由 last_message 中提取现有 retry 计数，超过上限则锁定 failed。
func (r *TaskRunner) failOrRetry(task model.IngestionTask, err error) {
	prevAttempts := parseRetryCount(task.LastMessage)
	next := prevAttempts + 1
	msg := fmt.Sprintf("retry %d/%d · %s", next, maxRetry, err.Error())
	status := "retry"
	if next >= maxRetry {
		status = "failed"
		msg = fmt.Sprintf("failed after %d retries · %s", next, err.Error())
	}
	if len(msg) > 3500 {
		msg = msg[:3500] + "...(truncated)"
	}
	_ = r.repo.UpdateTaskStatus(task.TaskID, status, msg)
}

func parseRetryCount(msg string) int {
	if msg == "" {
		return 0
	}
	lower := strings.ToLower(msg)
	idx := strings.Index(lower, "retry ")
	if idx < 0 {
		return 0
	}
	rest := msg[idx+6:]
	slash := strings.Index(rest, "/")
	if slash <= 0 {
		return 0
	}
	num, err := strconv.Atoi(strings.TrimSpace(rest[:slash]))
	if err != nil {
		return 0
	}
	return num
}

func (r *TaskRunner) runTask(task model.IngestionTask) error {
	if err := r.repo.UpdateTaskStatus(task.TaskID, "running", "started by worker"); err != nil {
		return err
	}
	switch task.Interval {
	case "1d", "daily":
		rows, err := r.adapter.Daily(task.Symbol, 240)
		if err != nil {
			r.failOrRetry(task, err)
			return err
		}
		if err := r.td.InsertBars(task.Symbol, task.Market, task.Interval, task.Fields, rows); err != nil {
			r.failOrRetry(task, err)
			return err
		}
		return r.repo.UpdateTaskStatus(task.TaskID, "completed", fmt.Sprintf("inserted %d daily bars", len(rows)))
	case "1m", "5m", "10m", "30m", "1h":
		rows, err := r.adapter.Minute(task.Symbol, task.Interval)
		if err != nil {
			r.failOrRetry(task, err)
			return err
		}
		if err := r.td.InsertBars(task.Symbol, task.Market, task.Interval, task.Fields, rows); err != nil {
			r.failOrRetry(task, err)
			return err
		}
		return r.repo.UpdateTaskStatus(task.TaskID, "completed", fmt.Sprintf("inserted %d bars", len(rows)))
	default:
		snapshot, err := r.adapter.Snapshot(task.Symbol)
		if err != nil {
			r.failOrRetry(task, err)
			return err
		}
		if err := r.td.InsertBars(task.Symbol, task.Market, task.Interval, task.Fields, []map[string]any{snapshotRow(task.Interval, snapshot)}); err != nil {
			r.failOrRetry(task, err)
			return err
		}
		return r.repo.UpdateTaskStatus(task.TaskID, "completed", "inserted snapshot")
	}
}

func (r *TaskRunner) runStockTask(task model.StockTask) error {
	_ = r.repo.AppendStockTaskLog(task.TaskID, "info", "task picked by worker", map[string]any{"task_type": task.TaskType})
	if err := r.repo.UpdateStockTaskStatus(task.TaskID, "running", "", "", 5); err != nil {
		return err
	}
	switch task.TaskType {
	case "indicator_compute":
		return r.runIndicatorCompute(task)
	case "pattern_scan":
		return r.runPatternScan(task)
	case "screening":
		return r.runScreening(task)
	case "backtest":
		return r.runBacktest(task)
	default:
		err := fmt.Errorf("stock task type unsupported by worker: %s", task.TaskType)
		_ = r.repo.UpdateStockTaskStatus(task.TaskID, "failed", "", err.Error(), 100)
		_ = r.repo.AppendStockTaskLog(task.TaskID, "error", err.Error(), map[string]any{"task_type": task.TaskType})
		return err
	}
}

func (r *TaskRunner) runIndicatorCompute(task model.StockTask) error {
	var params map[string]any
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return r.failStockTask(task, err)
	}
	symbol := strings.TrimSpace(fmt.Sprintf("%v", params["symbol"]))
	if symbol == "" || symbol == "<nil>" {
		return r.failStockTask(task, fmt.Errorf("symbol is required"))
	}
	period := strings.TrimSpace(fmt.Sprintf("%v", params["period"]))
	if period == "" || period == "<nil>" {
		period = "1d"
	}
	code := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", params["indicator_code"])))
	if code == "" || code == "<NIL>" {
		code = strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", params["indicator"])))
	}
	if code == "" || code == "<NIL>" {
		return r.failStockTask(task, fmt.Errorf("indicator_code is required"))
	}
	limit := intFromAny(params["limit"], 500)
	bars, err := r.td.QueryIndicatorBars(symbol, period, limit)
	if err != nil {
		return r.failStockTask(task, err)
	}
	if len(bars) == 0 {
		return r.failStockTask(task, fmt.Errorf("no bars found for %s %s", symbol, period))
	}
	indicatorParams := map[string]any{}
	if raw, ok := params["params"].(map[string]any); ok {
		indicatorParams = raw
	}
	points, err := indicator.Compute(code, bars, indicatorParams)
	if err != nil {
		return r.failStockTask(task, err)
	}
	if err := r.td.InsertIndicatorValues(symbol, period, code, points); err != nil {
		return r.failStockTask(task, err)
	}
	resultRef := fmt.Sprintf("tdengine:%s.ind_%s_%s_%s", r.td.database, sanitizeIdentifier(symbol), periodSuffix(period), sanitizeIdentifier(code))
	_ = r.repo.AppendStockTaskLog(task.TaskID, "info", "indicator computed", map[string]any{
		"symbol":         symbol,
		"period":         period,
		"indicator_code": code,
		"points":         len(points),
		"result_ref":     resultRef,
	})
	return r.repo.UpdateStockTaskStatus(task.TaskID, "succeeded", resultRef, "", 100)
}

func (r *TaskRunner) runPatternScan(task model.StockTask) error {
	var params map[string]any
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return r.failStockTask(task, err)
	}
	symbol := strings.TrimSpace(fmt.Sprintf("%v", params["symbol"]))
	if symbol == "" || symbol == "<nil>" {
		return r.failStockTask(task, fmt.Errorf("symbol is required"))
	}
	period := strings.TrimSpace(fmt.Sprintf("%v", params["period"]))
	if period == "" || period == "<nil>" {
		period = "1d"
	}
	limit := intFromAny(params["limit"], 500)
	bars, err := r.td.QueryIndicatorBars(symbol, period, limit)
	if err != nil {
		return r.failStockTask(task, err)
	}
	if len(bars) == 0 {
		return r.failStockTask(task, fmt.Errorf("no bars found for %s %s", symbol, period))
	}
	patterns := stringListFromAny(params["patterns"])
	hits, err := pattern.Scan(patterns, bars)
	if err != nil {
		return r.failStockTask(task, err)
	}
	if err := r.td.InsertPatternHits(symbol, period, hits); err != nil {
		return r.failStockTask(task, err)
	}
	resultRef := fmt.Sprintf("tdengine:%s.pattern_%s_%s_*", r.td.database, sanitizeIdentifier(symbol), periodSuffix(period))
	_ = r.repo.AppendStockTaskLog(task.TaskID, "info", "patterns scanned", map[string]any{
		"symbol":             symbol,
		"period":             period,
		"patterns_requested": len(patterns),
		"hits":               len(hits),
		"algorithm_version":  pattern.AlgorithmVersion,
		"result_ref":         resultRef,
	})
	return r.repo.UpdateStockTaskStatus(task.TaskID, "succeeded", resultRef, "", 100)
}

func (r *TaskRunner) runScreening(task model.StockTask) error {
	var params map[string]any
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return r.failStockTask(task, err)
	}
	if err := r.applyStrategyTemplateParams(params); err != nil {
		return r.failStockTask(task, err)
	}
	templateID := strings.TrimSpace(fmt.Sprintf("%v", params["template_id"]))
	if templateID == "<nil>" {
		templateID = ""
	}
	conditionsRaw := params["conditions"]
	if templateID != "" {
		template, err := r.repo.GetScreeningTemplate(templateID)
		if err != nil {
			return r.failStockTask(task, err)
		}
		var conditions map[string]any
		if err := json.Unmarshal([]byte(template.ConditionsJSON), &conditions); err != nil {
			return r.failStockTask(task, err)
		}
		conditionsRaw = conditions
	}
	conditions, err := parseScreeningConditions(conditionsRaw)
	if err != nil {
		return r.failStockTask(task, err)
	}
	limit := intFromAny(params["limit"], 200)
	results, err := r.evaluateScreeningConditions(conditions, limit)
	if err != nil {
		return r.failStockTask(task, err)
	}
	if err := r.repo.ReplaceScreeningResults(task.TaskID, results, templateID); err != nil {
		return r.failStockTask(task, err)
	}
	resultRef := fmt.Sprintf("postgres:stock_screening_results:%s", task.TaskID)
	_ = r.repo.AppendStockTaskLog(task.TaskID, "info", "screening completed", map[string]any{
		"template_id": templateID,
		"strategy_id": optionalStringParam(params["strategy_id"]),
		"logic":       conditions.Logic,
		"matches":     len(results),
		"result_ref":  resultRef,
	})
	return r.repo.UpdateStockTaskStatus(task.TaskID, "succeeded", resultRef, "", 100)
}

func (r *TaskRunner) runBacktest(task model.StockTask) error {
	var params map[string]any
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return r.failStockTask(task, err)
	}
	if err := r.applyStrategyTemplateParams(params); err != nil {
		return r.failStockTask(task, err)
	}
	period := strings.TrimSpace(fmt.Sprintf("%v", params["period"]))
	if period == "" || period == "<nil>" {
		period = "1d"
	}
	limit := intFromAny(params["limit"], 200)
	lookback := intFromAny(params["lookback"], 260)
	holdBars := intFromAny(params["hold_bars"], 20)
	if holdBars <= 0 {
		holdBars = 20
	}
	feeRate := floatFromAny(params["fee_rate"], 0)
	slippageRate := floatFromAny(params["slippage_rate"], 0)
	stopLoss := floatFromAny(params["stop_loss"], 0)
	takeProfit := floatFromAny(params["take_profit"], 0)
	benchmarkSymbol := strings.TrimSpace(fmt.Sprintf("%v", params["benchmark_symbol"]))
	if benchmarkSymbol == "<nil>" {
		benchmarkSymbol = ""
	}
	symbols, source, err := r.resolveBacktestSymbols(params, limit)
	if err != nil {
		return r.failStockTask(task, err)
	}
	benchmarkReturn := 0.0
	if benchmarkSymbol != "" {
		benchmarkBars, err := r.td.QueryBacktestBars(benchmarkSymbol, period, lookback)
		if err != nil {
			return r.failStockTask(task, err)
		}
		if len(benchmarkBars) >= 2 {
			benchmarkTrade := evaluateBacktestTrade(benchmarkSymbol, period, benchmarkBars, holdBars, 0, 0, 0, 0, "", 0)
			benchmarkReturn = benchmarkTrade.ReturnPct
		}
	}
	trades := make([]BacktestTrade, 0, len(symbols))
	for _, symbol := range symbols {
		bars, err := r.td.QueryBacktestBars(symbol, period, lookback)
		if err != nil {
			return r.failStockTask(task, err)
		}
		if len(bars) < 2 {
			continue
		}
		trade := evaluateBacktestTrade(symbol, period, bars, holdBars, feeRate, slippageRate, stopLoss, takeProfit, benchmarkSymbol, benchmarkReturn)
		if trade.EntryPrice <= 0 {
			continue
		}
		trade.Meta["source"] = source
		trade.Meta["lookback"] = lookback
		trades = append(trades, trade)
	}
	if err := r.repo.ReplaceBacktestResults(task.TaskID, trades); err != nil {
		return r.failStockTask(task, err)
	}
	summary := summarizeBacktestTrades(task.TaskID, trades, map[string]any{
		"source":        source,
		"strategy_id":   optionalStringParam(params["strategy_id"]),
		"strategy_name": optionalStringParam(params["strategy_name"]),
		"period":        period,
		"symbols":       len(symbols),
		"lookback":      lookback,
		"hold_bars":     holdBars,
		"fee_rate":      feeRate,
		"slippage_rate": slippageRate,
		"stop_loss":     stopLoss,
		"take_profit":   takeProfit,
	})
	if err := r.repo.ReplaceBacktestSummary(summary); err != nil {
		return r.failStockTask(task, err)
	}
	resultRef := fmt.Sprintf("postgres:stock_backtest_results:%s", task.TaskID)
	_ = r.repo.AppendStockTaskLog(task.TaskID, "info", "backtest completed", map[string]any{
		"source":      source,
		"strategy_id": optionalStringParam(params["strategy_id"]),
		"period":      period,
		"symbols":     len(symbols),
		"trades":      len(trades),
		"win_rate":    summary.WinRate,
		"fee_rate":    feeRate,
		"slippage":    slippageRate,
		"result_ref":  resultRef,
	})
	return r.repo.UpdateStockTaskStatus(task.TaskID, "succeeded", resultRef, "", 100)
}

func (r *TaskRunner) failStockTask(task model.StockTask, err error) error {
	_ = r.repo.UpdateStockTaskStatus(task.TaskID, "failed", "", err.Error(), 100)
	_ = r.repo.AppendStockTaskLog(task.TaskID, "error", err.Error(), map[string]any{"task_type": task.TaskType})
	return err
}

func (r *TaskRunner) evaluateScreeningConditions(conditions ScreeningConditionSet, limit int) ([]ScreeningCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	totalConditions := len(conditions.IndicatorConditions) + len(conditions.PatternConditions)
	if totalConditions == 0 {
		return nil, fmt.Errorf("screening conditions are required")
	}
	conditionMatches := make([]map[string]map[string]any, 0, totalConditions)
	for _, condition := range conditions.IndicatorConditions {
		items, err := r.td.ScreenByIndicator(condition, limit)
		if err != nil {
			return nil, err
		}
		conditionMatches = append(conditionMatches, rowsBySymbol(items))
	}
	for _, condition := range conditions.PatternConditions {
		items, err := r.td.ScreenByPattern(condition, limit)
		if err != nil {
			return nil, err
		}
		conditionMatches = append(conditionMatches, rowsBySymbol(items))
	}
	logic := strings.ToLower(strings.TrimSpace(conditions.Logic))
	if logic == "" {
		logic = "and"
	}
	symbols := map[string][]map[string]any{}
	for idx, matches := range conditionMatches {
		for symbol, row := range matches {
			if logic == "and" {
				if idx == 0 {
					symbols[symbol] = append(symbols[symbol], row)
					continue
				}
				if _, ok := symbols[symbol]; ok {
					symbols[symbol] = append(symbols[symbol], row)
				}
				continue
			}
			symbols[symbol] = append(symbols[symbol], row)
		}
		if logic == "and" && idx > 0 {
			for symbol, matched := range symbols {
				if len(matched) < idx+1 {
					delete(symbols, symbol)
				}
			}
		}
	}
	results := make([]ScreeningCandidate, 0, len(symbols))
	for symbol, matched := range symbols {
		if logic == "and" && len(matched) != totalConditions {
			continue
		}
		score := float64(len(matched)) / float64(totalConditions)
		results = append(results, ScreeningCandidate{
			Symbol:            symbol,
			Score:             score,
			MatchedConditions: matched,
			Snapshot:          latestConditionSnapshot(matched),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Symbol < results[j].Symbol
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (r *TaskRunner) resolveBacktestSymbols(params map[string]any, limit int) ([]string, string, error) {
	if screeningTaskID := strings.TrimSpace(fmt.Sprintf("%v", params["screening_task_id"])); screeningTaskID != "" && screeningTaskID != "<nil>" {
		symbols, err := r.repo.ListScreeningResultSymbols(screeningTaskID, limit)
		if err != nil {
			return nil, "", err
		}
		if len(symbols) == 0 {
			return nil, "", fmt.Errorf("no screening results found for task_id=%s", screeningTaskID)
		}
		return symbols, "screening_task_id:" + screeningTaskID, nil
	}
	if rawSymbols, ok := params["symbols"]; ok {
		symbols := stringListFromAny(rawSymbols)
		if len(symbols) > 0 {
			if len(symbols) > limit {
				symbols = symbols[:limit]
			}
			return symbols, "symbols", nil
		}
	}
	conditionsRaw := params["conditions"]
	templateID := strings.TrimSpace(fmt.Sprintf("%v", params["template_id"]))
	if templateID != "" && templateID != "<nil>" {
		template, err := r.repo.GetScreeningTemplate(templateID)
		if err != nil {
			return nil, "", err
		}
		var conditions map[string]any
		if err := json.Unmarshal([]byte(template.ConditionsJSON), &conditions); err != nil {
			return nil, "", err
		}
		conditionsRaw = conditions
	}
	if conditionsRaw != nil {
		conditions, err := parseScreeningConditions(conditionsRaw)
		if err != nil {
			return nil, "", err
		}
		screens, err := r.evaluateScreeningConditions(conditions, limit)
		if err != nil {
			return nil, "", err
		}
		symbols := make([]string, 0, len(screens))
		for _, item := range screens {
			symbols = append(symbols, item.Symbol)
		}
		if len(symbols) == 0 {
			return nil, "", fmt.Errorf("no symbols matched screening conditions")
		}
		return symbols, "conditions", nil
	}
	return nil, "", fmt.Errorf("screening_task_id, template_id, conditions or symbols are required")
}

func (r *TaskRunner) applyStrategyTemplateParams(params map[string]any) error {
	if snapshotRaw, ok := params["strategy_snapshot"]; ok {
		snapshot, err := parseStrategySnapshot(snapshotRaw)
		if err != nil {
			return err
		}
		if snapshot != nil {
			return applyStrategySnapshotParams(params, snapshot)
		}
	}
	strategyID := strings.TrimSpace(fmt.Sprintf("%v", params["strategy_id"]))
	if strategyID == "" || strategyID == "<nil>" {
		return nil
	}
	template, err := repoGetStrategyTemplate(r.repo, strategyID)
	if err != nil {
		return err
	}
	if template.ScreeningTemplateID != "" {
		setDefaultParam(params, "template_id", template.ScreeningTemplateID)
	}
	conditions, err := jsonMapFromText(template.ConditionsJSON)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		setDefaultParam(params, "conditions", conditions)
	}
	backtestParams, err := jsonMapFromText(template.BacktestParamsJSON)
	if err != nil {
		return err
	}
	for key, value := range backtestParams {
		setDefaultParam(params, key, value)
	}
	riskParams, err := jsonMapFromText(template.RiskParamsJSON)
	if err != nil {
		return err
	}
	for key, value := range riskParams {
		setDefaultParam(params, key, value)
	}
	params["strategy_name"] = template.Name
	return nil
}

func parseStrategySnapshot(value any) (*model.StrategySnapshot, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var snapshot model.StrategySnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.StrategyID == "" {
		return nil, nil
	}
	return &snapshot, nil
}

func applyStrategySnapshotParams(params map[string]any, snapshot *model.StrategySnapshot) error {
	if snapshot.ScreeningTemplateID != "" {
		setDefaultParam(params, "template_id", snapshot.ScreeningTemplateID)
	}
	conditions, err := jsonMapFromText(snapshot.ConditionsJSON)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		setDefaultParam(params, "conditions", conditions)
	}
	backtestParams, err := jsonMapFromText(snapshot.BacktestParamsJSON)
	if err != nil {
		return err
	}
	for key, value := range backtestParams {
		setDefaultParam(params, key, value)
	}
	riskParams, err := jsonMapFromText(snapshot.RiskParamsJSON)
	if err != nil {
		return err
	}
	for key, value := range riskParams {
		setDefaultParam(params, key, value)
	}
	params["strategy_name"] = snapshot.Name
	return nil
}

func setDefaultParam(params map[string]any, key string, value any) {
	if _, ok := params[key]; ok {
		return
	}
	params[key] = value
}

func optionalStringParam(value any) string {
	out := strings.TrimSpace(fmt.Sprintf("%v", value))
	if out == "<nil>" {
		return ""
	}
	return out
}

func jsonMapFromText(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

func evaluateBacktestTrade(symbol, period string, bars []BacktestBar, holdBars int, feeRate, slippageRate, stopLoss, takeProfit float64, benchmarkSymbol string, benchmarkReturn float64) BacktestTrade {
	entryIdx := len(bars) - 1 - holdBars
	if entryIdx < 0 {
		entryIdx = 0
	}
	entry := bars[entryIdx]
	exitIdx := len(bars) - 1
	exitReason := "hold_bars"
	if entry.Close > 0 && (stopLoss > 0 || takeProfit > 0) {
		for idx := entryIdx + 1; idx < len(bars); idx++ {
			rawReturn := (bars[idx].Close - entry.Close) / entry.Close
			if takeProfit > 0 && rawReturn >= takeProfit {
				exitIdx = idx
				exitReason = "take_profit"
				break
			}
			if stopLoss > 0 && rawReturn <= -stopLoss {
				exitIdx = idx
				exitReason = "stop_loss"
				break
			}
		}
	}
	exit := bars[exitIdx]
	entryPrice := entry.Close * (1 + slippageRate)
	exitPrice := exit.Close * (1 - slippageRate)
	grossReturn := 0.0
	netReturn := 0.0
	if entryPrice > 0 {
		grossReturn = (exit.Close - entry.Close) / entry.Close
		netReturn = (exitPrice-entryPrice)/entryPrice - feeRate*2
	}
	return BacktestTrade{
		Symbol:             symbol,
		Period:             periodSuffix(period),
		EntryTime:          entry.TS,
		ExitTime:           exit.TS,
		EntryPrice:         entryPrice,
		ExitPrice:          exitPrice,
		ReturnPct:          netReturn,
		BenchmarkSymbol:    benchmarkSymbol,
		BenchmarkReturnPct: benchmarkReturn,
		ExcessReturnPct:    netReturn - benchmarkReturn,
		Meta: map[string]any{
			"gross_return_pct": grossReturn,
			"fee_rate":         feeRate,
			"slippage_rate":    slippageRate,
			"stop_loss":        stopLoss,
			"take_profit":      takeProfit,
			"hold_bars":        holdBars,
			"exit_reason":      exitReason,
			"bars_held":        exitIdx - entryIdx,
		},
	}
}

func summarizeBacktestTrades(taskID string, trades []BacktestTrade, meta map[string]any) BacktestSummary {
	summary := BacktestSummary{
		TaskID:             taskID,
		TotalTrades:        len(trades),
		ReturnDistribution: map[string]int{},
		Meta:               meta,
	}
	if len(trades) == 0 {
		return summary
	}
	sumReturn := 0.0
	sumExcess := 0.0
	wins := 0
	equity := 1.0
	peak := 1.0
	maxDrawdown := 0.0
	best := trades[0].ReturnPct
	worst := trades[0].ReturnPct
	summary.BenchmarkSymbol = trades[0].BenchmarkSymbol
	summary.BenchmarkReturnPct = trades[0].BenchmarkReturnPct
	for _, trade := range trades {
		sumReturn += trade.ReturnPct
		sumExcess += trade.ExcessReturnPct
		if trade.ReturnPct > 0 {
			wins++
		}
		if trade.ReturnPct > best {
			best = trade.ReturnPct
		}
		if trade.ReturnPct < worst {
			worst = trade.ReturnPct
		}
		summary.ReturnDistribution[returnBucket(trade.ReturnPct)]++
		equity *= 1 + trade.ReturnPct
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			drawdown := (peak - equity) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}
	summary.WinRate = float64(wins) / float64(len(trades))
	summary.AvgReturnPct = sumReturn / float64(len(trades))
	summary.TotalReturnPct = equity - 1
	summary.MaxDrawdownPct = maxDrawdown
	summary.BestReturnPct = best
	summary.WorstReturnPct = worst
	summary.AvgExcessReturnPct = sumExcess / float64(len(trades))
	return summary
}

func returnBucket(value float64) string {
	switch {
	case value < -0.10:
		return "loss_gt_10"
	case value < -0.05:
		return "loss_5_10"
	case value < 0:
		return "loss_0_5"
	case value < 0.05:
		return "gain_0_5"
	case value < 0.10:
		return "gain_5_10"
	default:
		return "gain_gt_10"
	}
}

func snapshotRow(interval string, snapshot map[string]any) map[string]any {
	return map[string]any{
		"ts":                  time.Now().Format(time.RFC3339),
		"open":                snapshot["price"],
		"close":               snapshot["price"],
		"high":                snapshot["price"],
		"low":                 snapshot["price"],
		"volume":              snapshot["volume"],
		"amount":              snapshot["amount"],
		"turnover_rate":       snapshot["turnover_rate"],
		"turnover_amount":     snapshot["turnover_amount"],
		"volume_ratio":        snapshot["volume_ratio"],
		"premium_ratio":       snapshot["premium_ratio"],
		"big_order_volume":    snapshot["big_order_volume"],
		"medium_order_volume": snapshot["medium_order_volume"],
		"small_order_volume":  snapshot["small_order_volume"],
		"bid_1_price":         snapshot["bid_1_price"],
		"bid_1_volume":        snapshot["bid_1_volume"],
		"bid_2_price":         snapshot["bid_2_price"],
		"bid_2_volume":        snapshot["bid_2_volume"],
		"bid_3_price":         snapshot["bid_3_price"],
		"bid_3_volume":        snapshot["bid_3_volume"],
		"bid_4_price":         snapshot["bid_4_price"],
		"bid_4_volume":        snapshot["bid_4_volume"],
		"bid_5_price":         snapshot["bid_5_price"],
		"bid_5_volume":        snapshot["bid_5_volume"],
		"ask_1_price":         snapshot["ask_1_price"],
		"ask_1_volume":        snapshot["ask_1_volume"],
		"ask_2_price":         snapshot["ask_2_price"],
		"ask_2_volume":        snapshot["ask_2_volume"],
		"ask_3_price":         snapshot["ask_3_price"],
		"ask_3_volume":        snapshot["ask_3_volume"],
		"ask_4_price":         snapshot["ask_4_price"],
		"ask_4_volume":        snapshot["ask_4_volume"],
		"ask_5_price":         snapshot["ask_5_price"],
		"ask_5_volume":        snapshot["ask_5_volume"],
		"requested_period":    interval,
		"source_period":       interval,
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func escape(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func escapeDoubleQuote(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func sqlStringLiteral(value string) string {
	return "'" + escape(value) + "'"
}

func tdTimestampLiteral(value any) string {
	raw := strings.TrimSpace(fmt.Sprintf("%v", value))
	if raw == "" || raw == "<nil>" {
		return sqlStringLiteral(time.Now().Format("2006-01-02 15:04:05"))
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05.000"))
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05"))
	}
	if ts, err := time.Parse("2006-01-02 15:04", raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05"))
	}
	if ts, err := time.Parse("2006-01-02", raw); err == nil {
		return sqlStringLiteral(ts.Format("2006-01-02 15:04:05"))
	}
	return sqlStringLiteral(raw)
}

func normalizeFields(fields []string) []string {
	if len(fields) == 0 {
		return append([]string{}, defaultFields...)
	}
	set := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		set[field] = struct{}{}
	}
	items := make([]string, 0, len(set))
	for field := range set {
		items = append(items, field)
	}
	sort.Strings(items)
	return items
}

func makeFieldSet(fields []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, field := range normalizeFields(fields) {
		set[field] = struct{}{}
	}
	return set
}

func numericValue(row map[string]any, selected map[string]struct{}, feature, key string) string {
	if _, ok := selected[feature]; !ok {
		return "0"
	}
	value, ok := row[key]
	if !ok || value == nil {
		return "0"
	}
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%v", v)
	case float32:
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return v.String()
	case string:
		if strings.TrimSpace(v) == "" {
			return "0"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func intFromAny(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case json.Number:
		parsed, err := strconv.Atoi(v.String())
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func floatFromAny(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		parsed, err := strconv.ParseFloat(v.String(), 64)
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func stringListFromAny(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		value := strings.TrimSpace(fmt.Sprintf("%v", item))
		if value == "" || value == "<nil>" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func parseScreeningConditions(value any) (ScreeningConditionSet, error) {
	if value == nil {
		return ScreeningConditionSet{}, fmt.Errorf("conditions are required")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ScreeningConditionSet{}, err
	}
	var conditions ScreeningConditionSet
	if err := json.Unmarshal(raw, &conditions); err != nil {
		return ScreeningConditionSet{}, err
	}
	conditions.Logic = strings.ToLower(strings.TrimSpace(conditions.Logic))
	if conditions.Logic == "" {
		conditions.Logic = "and"
	}
	if conditions.Logic != "and" && conditions.Logic != "or" {
		return ScreeningConditionSet{}, fmt.Errorf("conditions.logic must be and or or")
	}
	for idx := range conditions.IndicatorConditions {
		condition := &conditions.IndicatorConditions[idx]
		condition.Period = strings.TrimSpace(condition.Period)
		if condition.Period == "" {
			condition.Period = "1d"
		}
		condition.IndicatorCode = strings.ToUpper(strings.TrimSpace(condition.IndicatorCode))
		condition.Field = strings.TrimSpace(condition.Field)
		condition.Op = normalizeCompareOp(condition.Op)
		if condition.IndicatorCode == "" || condition.Field == "" || condition.Op == "" {
			return ScreeningConditionSet{}, fmt.Errorf("indicator condition requires indicator_code, field and valid op")
		}
	}
	for idx := range conditions.PatternConditions {
		condition := &conditions.PatternConditions[idx]
		condition.Period = strings.TrimSpace(condition.Period)
		if condition.Period == "" {
			condition.Period = "1d"
		}
		condition.PatternCode = strings.TrimSpace(condition.PatternCode)
		condition.Direction = strings.ToLower(strings.TrimSpace(condition.Direction))
		if condition.PatternCode == "" {
			return ScreeningConditionSet{}, fmt.Errorf("pattern condition requires pattern_code")
		}
		if condition.Direction != "" && condition.Direction != "bullish" && condition.Direction != "bearish" && condition.Direction != "neutral" {
			return ScreeningConditionSet{}, fmt.Errorf("pattern condition direction is invalid")
		}
	}
	return conditions, nil
}

func rowsBySymbol(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		symbol := strings.TrimSpace(fmt.Sprintf("%v", row["symbol"]))
		if symbol == "" || symbol == "<nil>" {
			continue
		}
		if _, ok := out[symbol]; ok {
			continue
		}
		out[symbol] = row
	}
	return out
}

func latestConditionSnapshot(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	snapshot := map[string]any{}
	for key, value := range rows[0] {
		snapshot[key] = value
	}
	snapshot["matched_count"] = len(rows)
	return snapshot
}

func indicatorFieldValue(row map[string]any, field string) (float64, bool) {
	field = strings.TrimSpace(field)
	if field == "" || field == "value" {
		return anyFloat(row["value"]), true
	}
	raw := strings.TrimSpace(fmt.Sprintf("%v", row["values_json"]))
	if raw == "" || raw == "<nil>" {
		return 0, false
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return 0, false
	}
	value, ok := values[field]
	if !ok {
		return 0, false
	}
	return anyFloat(value), true
}

func normalizeCompareOp(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "gt", "gte", "lt", "lte", "eq":
		return strings.ToLower(strings.TrimSpace(op))
	default:
		return ""
	}
}

func compareFloat(value float64, op string, threshold float64) bool {
	switch op {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

func isMissingTDTable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "table does not exist") || strings.Contains(msg, "invalid")
}

func anyFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		parsed, err := strconv.ParseFloat(v.String(), 64)
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func parseAnyTime(value any) (time.Time, bool) {
	raw := strings.TrimSpace(fmt.Sprintf("%v", value))
	if raw == "" || raw == "<nil>" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func sanitizeIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			b.WriteRune(ch)
			continue
		}
		b.WriteRune('_')
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "unknown"
	}
	return result
}

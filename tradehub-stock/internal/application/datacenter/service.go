package datacenter

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// ServiceImpl 数据中心服务实现
type ServiceImpl struct {
	db     *sql.DB
	client *EastMoneyClient
	config *Config
}

// NewService 创建数据中心服务
func NewService(db *sql.DB, cfg *Config) *ServiceImpl {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	return &ServiceImpl{
		db:     db,
		client: NewEastMoneyClient(30 * time.Second),
		config: cfg,
	}
}

// InitSchema 初始化数据库表结构
func (s *ServiceImpl) InitSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	schemas := []string{
		// 每日行情快照表
		`CREATE TABLE IF NOT EXISTS stock_daily_spot (
			date VARCHAR(8) NOT NULL,
			code VARCHAR(16) NOT NULL,
			name VARCHAR(128) NOT NULL DEFAULT '',
			last_price DOUBLE PRECISION NOT NULL DEFAULT 0,
			change_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			change_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
			volume BIGINT NOT NULL DEFAULT 0,
			turnover DOUBLE PRECISION NOT NULL DEFAULT 0,
			amplitude DOUBLE PRECISION NOT NULL DEFAULT 0,
			high DOUBLE PRECISION NOT NULL DEFAULT 0,
			low DOUBLE PRECISION NOT NULL DEFAULT 0,
			open_price DOUBLE PRECISION NOT NULL DEFAULT 0,
			closed DOUBLE PRECISION NOT NULL DEFAULT 0,
			volume_ratio DOUBLE PRECISION NOT NULL DEFAULT 0,
			turnover_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			pe_ratio DOUBLE PRECISION NOT NULL DEFAULT 0,
			pb_ratio DOUBLE PRECISION NOT NULL DEFAULT 0,
			market_cap DOUBLE PRECISION NOT NULL DEFAULT 0,
			circulating_market_cap DOUBLE PRECISION NOT NULL DEFAULT 0,
			rise_speed DOUBLE PRECISION NOT NULL DEFAULT 0,
			change_5min DOUBLE PRECISION NOT NULL DEFAULT 0,
			change_60day DOUBLE PRECISION NOT NULL DEFAULT 0,
			ytd_change_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			is_st BOOLEAN NOT NULL DEFAULT FALSE,
			is_suspended BOOLEAN NOT NULL DEFAULT FALSE,
			source VARCHAR(32) NOT NULL DEFAULT 'eastmoney',
			fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (date, code)
		)`,

		// 龙虎榜个股上榜表
		`CREATE TABLE IF NOT EXISTS stock_lhb_gg (
			date VARCHAR(8) NOT NULL,
			code VARCHAR(16) NOT NULL,
			name VARCHAR(128) NOT NULL DEFAULT '',
			ranking_times INT NOT NULL DEFAULT 0,
			sum_buy DOUBLE PRECISION NOT NULL DEFAULT 0,
			sum_sell DOUBLE PRECISION NOT NULL DEFAULT 0,
			net_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
			buy_seat INT NOT NULL DEFAULT 0,
			sell_seat INT NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			source VARCHAR(32) NOT NULL DEFAULT 'eastmoney',
			fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (date, code)
		)`,

		// 大宗交易表
		`CREATE TABLE IF NOT EXISTS stock_dzjy (
			date VARCHAR(8) NOT NULL,
			code VARCHAR(16) NOT NULL,
			name VARCHAR(128) NOT NULL DEFAULT '',
			quote_change DOUBLE PRECISION NOT NULL DEFAULT 0,
			close_price DOUBLE PRECISION NOT NULL DEFAULT 0,
			average_price DOUBLE PRECISION NOT NULL DEFAULT 0,
			overflow_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			trade_number INT NOT NULL DEFAULT 0,
			sum_volume BIGINT NOT NULL DEFAULT 0,
			sum_turnover DOUBLE PRECISION NOT NULL DEFAULT 0,
			turnover_market_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
			source VARCHAR(32) NOT NULL DEFAULT 'eastmoney',
			fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (date, code)
		)`,

		// 采集任务表
		`CREATE TABLE IF NOT EXISTS stock_collection_tasks (
			task_id VARCHAR(64) PRIMARY KEY,
			task_type VARCHAR(32) NOT NULL,
			target_date VARCHAR(8) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			total_count INT NOT NULL DEFAULT 0,
			success_count INT NOT NULL DEFAULT 0,
			fail_count INT NOT NULL DEFAULT 0,
			error_msg TEXT NOT NULL DEFAULT '',
			started_at TIMESTAMPTZ,
			finished_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// 采集日志表
		`CREATE TABLE IF NOT EXISTS stock_collection_logs (
			log_id VARCHAR(64) PRIMARY KEY,
			task_id VARCHAR(64) NOT NULL,
			level VARCHAR(8) NOT NULL,
			message TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	// 创建索引
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_daily_spot_date ON stock_daily_spot(date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_spot_code ON stock_daily_spot(code)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_spot_name ON stock_daily_spot(name)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_spot_change ON stock_daily_spot(change_percent DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_spot_turnover ON stock_daily_spot(turnover DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_lhb_date ON stock_lhb_gg(date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_lhb_code ON stock_lhb_gg(code)`,

		`CREATE INDEX IF NOT EXISTS idx_dzjy_date ON stock_dzjy(date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_dzjy_code ON stock_dzjy(code)`,

		`CREATE INDEX IF NOT EXISTS idx_tasks_type ON stock_collection_tasks(task_type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_date ON stock_collection_tasks(target_date)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_task ON stock_collection_logs(task_id, created_at)`,
	}

	for _, sql := range schemas {
		if _, err := s.db.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}

	for _, sql := range indexes {
		if _, err := s.db.ExecContext(ctx, sql); err != nil {
			log.Printf("建索引失败 (忽略): %v", err)
		}
	}

	return nil
}

// QueryDailySpot 查询每日行情
func (s *ServiceImpl) QueryDailySpot(ctx context.Context, q *DailySpotQuery) (*DailySpotResult, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}
	if q.PageSize > 500 {
		q.PageSize = 500
	}

	// 构建 WHERE 条件
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	if q.Date != "" {
		conditions = append(conditions, fmt.Sprintf("date = $%d", argIdx))
		args = append(args, q.Date)
		argIdx++
	}
	if q.Code != "" {
		conditions = append(conditions, fmt.Sprintf("code = $%d", argIdx))
		args = append(args, q.Code)
		argIdx++
	}
	if q.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name LIKE $%d", argIdx))
		args = append(args, "%"+q.Name+"%")
		argIdx++
	}
	if q.IsST != nil {
		conditions = append(conditions, fmt.Sprintf("is_st = $%d", argIdx))
		args = append(args, *q.IsST)
		argIdx++
	}
	if q.ChangeMin > 0 {
		conditions = append(conditions, fmt.Sprintf("change_percent >= $%d", argIdx))
		args = append(args, q.ChangeMin)
		argIdx++
	}
	if q.ChangeMax > 0 {
		conditions = append(conditions, fmt.Sprintf("change_percent <= $%d", argIdx))
		args = append(args, q.ChangeMax)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 排序
	orderBy := "change_percent DESC"
	if q.SortField != "" {
		dir := "DESC"
		if q.SortOrder == "asc" {
			dir = "ASC"
		}
		validFields := map[string]bool{
			"change_percent": true, "turnover": true, "volume": true,
			"last_price": true, "market_cap": true, "pe_ratio": true,
		}
		if validFields[q.SortField] {
			orderBy = q.SortField + " " + dir
		}
	}

	// 查询总数
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM stock_daily_spot %s", where)
	var total int64
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("查询总数失败: %w", err)
	}

	// 查询数据
	offset := (q.Page - 1) * q.PageSize
	dataSQL := fmt.Sprintf(`
		SELECT date, code, name, last_price, change_percent, change_amount,
			volume, turnover, amplitude, high, low, open_price, closed,
			volume_ratio, turnover_rate, pe_ratio, pb_ratio,
			market_cap, circulating_market_cap, rise_speed, change_5min,
			change_60day, ytd_change_percent, is_st, is_suspended, source, fetched_at
		FROM stock_daily_spot %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argIdx, argIdx+1)
	args = append(args, q.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	data := []DailySpot{}
	for rows.Next() {
		var spot DailySpot
		err := rows.Scan(
			&spot.Date, &spot.Code, &spot.Name, &spot.LastPrice, &spot.ChangePercent,
			&spot.ChangeAmount, &spot.Volume, &spot.Turnover, &spot.Amplitude,
			&spot.High, &spot.Low, &spot.Open, &spot.Closed, &spot.VolumeRatio,
			&spot.TurnoverRate, &spot.PERatio, &spot.PBRatio, &spot.MarketCap,
			&spot.CirculatingMarketCap, &spot.RiseSpeed, &spot.Change5Min,
			&spot.Change60Day, &spot.YTDChangePercent, &spot.IsST, &spot.IsSuspended,
			&spot.Source, &spot.FetchedAt,
		)
		if err != nil {
			continue
		}
		data = append(data, spot)
	}

	totalPages := int(total) / q.PageSize
	if int(total)%q.PageSize > 0 {
		totalPages++
	}

	return &DailySpotResult{
		Data:       data,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetDailySpotByDate 获取指定日期的每日行情
func (s *ServiceImpl) GetDailySpotByDate(ctx context.Context, date string) ([]DailySpot, error) {
	result, err := s.QueryDailySpot(ctx, &DailySpotQuery{Date: date, PageSize: 10000})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// QueryLHBGG 查询龙虎榜
func (s *ServiceImpl) QueryLHBGG(ctx context.Context, q *LHBGGQuery) (*LHBGGResult, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}

	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	if q.Date != "" {
		conditions = append(conditions, fmt.Sprintf("date = $%d", argIdx))
		args = append(args, q.Date)
		argIdx++
	}
	if q.DateStart != "" {
		conditions = append(conditions, fmt.Sprintf("date >= $%d", argIdx))
		args = append(args, q.DateStart)
		argIdx++
	}
	if q.DateEnd != "" {
		conditions = append(conditions, fmt.Sprintf("date <= $%d", argIdx))
		args = append(args, q.DateEnd)
		argIdx++
	}
	if q.Code != "" {
		conditions = append(conditions, fmt.Sprintf("code = $%d", argIdx))
		args = append(args, q.Code)
		argIdx++
	}
	if q.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name LIKE $%d", argIdx))
		args = append(args, "%"+q.Name+"%")
		argIdx++
	}
	if q.NetMin != 0 {
		conditions = append(conditions, fmt.Sprintf("net_amount >= $%d", argIdx))
		args = append(args, q.NetMin)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "date DESC, net_amount DESC"
	if q.SortField != "" {
		dir := "DESC"
		if q.SortOrder == "asc" {
			dir = "ASC"
		}
		validFields := map[string]bool{
			"net_amount": true, "sum_buy": true, "sum_sell": true, "ranking_times": true,
		}
		if validFields[q.SortField] {
			orderBy = q.SortField + " " + dir
		}
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM stock_lhb_gg %s", where)
	var total int64
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (q.Page - 1) * q.PageSize
	dataSQL := fmt.Sprintf(`
		SELECT date, code, name, ranking_times, sum_buy, sum_sell, net_amount,
			buy_seat, sell_seat, reason, source, fetched_at
		FROM stock_lhb_gg %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argIdx, argIdx+1)
	args = append(args, q.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []LHBGG{}
	for rows.Next() {
		var item LHBGG
		if err := rows.Scan(&item.Date, &item.Code, &item.Name, &item.RankingTimes,
			&item.SumBuy, &item.SumSell, &item.NetAmount, &item.BuySeat,
			&item.SellSeat, &item.Reason, &item.Source, &item.FetchedAt); err != nil {
			continue
		}
		data = append(data, item)
	}

	totalPages := int(total) / q.PageSize
	if int(total)%q.PageSize > 0 {
		totalPages++
	}

	return &LHBGGResult{
		Data:       data,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetLHBGGByDate 获取指定日期的龙虎榜
func (s *ServiceImpl) GetLHBGGByDate(ctx context.Context, date string) ([]LHBGG, error) {
	result, err := s.QueryLHBGG(ctx, &LHBGGQuery{Date: date, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// QueryDZJY 查询大宗交易
func (s *ServiceImpl) QueryDZJY(ctx context.Context, q *DZJYQuery) (*DZJYResult, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}

	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	if q.Date != "" {
		conditions = append(conditions, fmt.Sprintf("date = $%d", argIdx))
		args = append(args, q.Date)
		argIdx++
	}
	if q.DateStart != "" {
		conditions = append(conditions, fmt.Sprintf("date >= $%d", argIdx))
		args = append(args, q.DateStart)
		argIdx++
	}
	if q.DateEnd != "" {
		conditions = append(conditions, fmt.Sprintf("date <= $%d", argIdx))
		args = append(args, q.DateEnd)
		argIdx++
	}
	if q.Code != "" {
		conditions = append(conditions, fmt.Sprintf("code = $%d", argIdx))
		args = append(args, q.Code)
		argIdx++
	}
	if q.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name LIKE $%d", argIdx))
		args = append(args, "%"+q.Name+"%")
		argIdx++
	}
	if q.OverflowMin != 0 {
		conditions = append(conditions, fmt.Sprintf("overflow_rate >= $%d", argIdx))
		args = append(args, q.OverflowMin)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "date DESC, sum_turnover DESC"
	if q.SortField != "" {
		dir := "DESC"
		if q.SortOrder == "asc" {
			dir = "ASC"
		}
		validFields := map[string]bool{
			"sum_turnover": true, "overflow_rate": true, "trade_number": true,
		}
		if validFields[q.SortField] {
			orderBy = q.SortField + " " + dir
		}
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM stock_dzjy %s", where)
	var total int64
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (q.Page - 1) * q.PageSize
	dataSQL := fmt.Sprintf(`
		SELECT date, code, name, quote_change, close_price, average_price,
			overflow_rate, trade_number, sum_volume, sum_turnover,
			turnover_market_rate, source, fetched_at
		FROM stock_dzjy %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argIdx, argIdx+1)
	args = append(args, q.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []DZJY{}
	for rows.Next() {
		var item DZJY
		if err := rows.Scan(&item.Date, &item.Code, &item.Name, &item.QuoteChange,
			&item.ClosePrice, &item.AveragePrice, &item.OverflowRate, &item.TradeNumber,
			&item.SumVolume, &item.SumTurnover, &item.TurnoverMarketRate,
			&item.Source, &item.FetchedAt); err != nil {
			continue
		}
		data = append(data, item)
	}

	totalPages := int(total) / q.PageSize
	if int(total)%q.PageSize > 0 {
		totalPages++
	}

	return &DZJYResult{
		Data:       data,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetDZJYByDate 获取指定日期的大宗交易
func (s *ServiceImpl) GetDZJYByDate(ctx context.Context, date string) ([]DZJY, error) {
	result, err := s.QueryDZJY(ctx, &DZJYQuery{Date: date, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// Collect 执行数据采集
func (s *ServiceImpl) Collect(ctx context.Context, req *DataCollectionRequest) (*DataCollectionResponse, error) {
	taskID := uuid.New().String()
	now := time.Now()

	// 创建任务记录
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO stock_collection_tasks (task_id, task_type, target_date, status, created_at)
		VALUES ($1, $2, $3, 'running', $4)
	`, taskID, req.TaskType, req.TargetDate, now)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// 异步执行采集
	go s.runCollection(context.Background(), taskID, req)

	return &DataCollectionResponse{
		TaskID:  taskID,
		Status:  "pending",
		Message: "采集任务已创建",
	}, nil
}

// runCollection 执行采集任务
func (s *ServiceImpl) runCollection(ctx context.Context, taskID string, req *DataCollectionRequest) {
	now := time.Now()
	defer func() {
		if r := recover(); r != nil {
			s.db.ExecContext(ctx, `
				UPDATE stock_collection_tasks
				SET status = 'failed', error_msg = $1, finished_at = $2
				WHERE task_id = $3
			`, fmt.Sprintf("panic: %v", r), time.Now(), taskID)
		}
	}()

	// 更新任务状态
	s.db.ExecContext(ctx, `
		UPDATE stock_collection_tasks SET status = 'running', started_at = $1 WHERE task_id = $2
	`, now, taskID)

	var totalCount, successCount int
	var errMsg string

	switch req.TaskType {
	case "daily_spot":
		totalCount, successCount, errMsg = s.collectDailySpot(ctx, req.TargetDate)
	case "lhb":
		totalCount, successCount, errMsg = s.collectLHB(ctx, req.TargetDate)
	case "dzjy":
		totalCount, successCount, errMsg = s.collectDZJY(ctx, req.TargetDate)
	default:
		errMsg = fmt.Sprintf("未知的任务类型: %s", req.TaskType)
	}

	// 更新任务结果
	status := "success"
	if errMsg != "" {
		status = "failed"
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE stock_collection_tasks
		SET status = $1, total_count = $2, success_count = $3, fail_count = $4,
			error_msg = $5, finished_at = $6
		WHERE task_id = $7
	`, status, totalCount, successCount, totalCount-successCount, errMsg, time.Now(), taskID)
	if err != nil {
		log.Printf("更新任务状态失败: %v", err)
	}
}

// collectDailySpot 采集每日行情
func (s *ServiceImpl) collectDailySpot(ctx context.Context, date string) (total, success int, errMsg string) {
	// 调用东方财富接口获取行情
	spots, err := s.client.FetchAStockSpot(ctx)
	if err != nil {
		return 0, 0, fmt.Sprintf("获取行情失败: %v", err)
	}

	date = time.Now().Format("20060102") // 使用当前日期
	total = len(spots)

	// 批量插入/更新
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Sprintf("开启事务失败: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO stock_daily_spot (
			date, code, name, last_price, change_percent, change_amount,
			volume, turnover, amplitude, high, low, open_price, closed,
			volume_ratio, turnover_rate, pe_ratio, pb_ratio,
			market_cap, circulating_market_cap, rise_speed, change_5min,
			change_60day, ytd_change_percent, is_st, is_suspended, source, fetched_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)
		ON CONFLICT (date, code) DO UPDATE SET
			last_price = EXCLUDED.last_price,
			change_percent = EXCLUDED.change_percent,
			change_amount = EXCLUDED.change_amount,
			volume = EXCLUDED.volume,
			turnover = EXCLUDED.turnover,
			fetched_at = EXCLUDED.fetched_at
	`)

	// 东方财富数据可能没有 60 日涨跌幅，这里简化处理
	for _, spot := range spots {
		spot.Date = date
		spot.FetchedAt = time.Now()
		_, err := stmt.ExecContext(ctx,
			spot.Date, spot.Code, spot.Name, spot.LastPrice, spot.ChangePercent,
			spot.ChangeAmount, spot.Volume, spot.Turnover, spot.Amplitude,
			spot.High, spot.Low, spot.Open, spot.Closed, spot.VolumeRatio,
			spot.TurnoverRate, spot.PERatio, spot.PBRatio, spot.MarketCap,
			spot.CirculatingMarketCap, spot.RiseSpeed, spot.Change5Min,
			spot.Change60Day, spot.YTDChangePercent, spot.IsST, spot.IsSuspended,
			spot.Source, spot.FetchedAt,
		)
		if err == nil {
			success++
		}
	}
	stmt.Close()

	if err := tx.Commit(); err != nil {
		return total, success, fmt.Sprintf("提交失败: %v", err)
	}

	return total, success, ""
}

// collectLHB 采集龙虎榜
func (s *ServiceImpl) collectLHB(ctx context.Context, date string) (total, success int, errMsg string) {
	lhbList, err := s.client.FetchLHBGG(ctx, date)
	if err != nil {
		return 0, 0, fmt.Sprintf("获取龙虎榜失败: %v", err)
	}

	total = len(lhbList)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Sprintf("开启事务失败: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO stock_lhb_gg (
			date, code, name, quote_change, close_price, average_price,
			sum_buy, sum_sell, net_amount, buy_seat, sell_seat, reason, source, fetched_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (date, code) DO UPDATE SET
			quote_change = EXCLUDED.quote_change,
			close_price = EXCLUDED.close_price,
			average_price = EXCLUDED.average_price,
			sum_buy = EXCLUDED.sum_buy,
			sum_sell = EXCLUDED.sum_sell,
			net_amount = EXCLUDED.net_amount,
			reason = EXCLUDED.reason,
			fetched_at = EXCLUDED.fetched_at
	`)
	if err != nil {
		return 0, 0, fmt.Sprintf("准备语句失败: %v", err)
	}
	defer stmt.Close()

	for _, item := range lhbList {
		_, err := stmt.ExecContext(ctx,
			item.Date, item.Code, item.Name, item.QuoteChange, item.ClosePrice,
			item.AveragePrice, item.SumBuy, item.SumSell, item.NetAmount,
			item.BuySeat, item.SellSeat, item.Reason, item.Source, item.FetchedAt,
		)
		if err == nil {
			success++
		}
	}

	if err := tx.Commit(); err != nil {
		return total, success, fmt.Sprintf("提交失败: %v", err)
	}

	return total, success, ""
}

// collectDZJY 采集大宗交易
func (s *ServiceImpl) collectDZJY(ctx context.Context, date string) (total, success int, errMsg string) {
	dzjyList, err := s.client.FetchDZJY(ctx, date, date)
	if err != nil {
		return 0, 0, fmt.Sprintf("获取大宗交易失败: %v", err)
	}

	total = len(dzjyList)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Sprintf("开启事务失败: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO stock_dzjy (
			date, code, name, quote_change, close_price, average_price,
			overflow_rate, trade_number, sum_volume, sum_turnover,
			turnover_market_rate, source, fetched_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (date, code) DO UPDATE SET
			sum_turnover = EXCLUDED.sum_turnover,
			overflow_rate = EXCLUDED.overflow_rate,
			fetched_at = EXCLUDED.fetched_at
	`)
	if err != nil {
		return 0, 0, fmt.Sprintf("准备语句失败: %v", err)
	}
	defer stmt.Close()

	for _, item := range dzjyList {
		_, err := stmt.ExecContext(ctx,
			item.Date, item.Code, item.Name, item.QuoteChange, item.ClosePrice,
			item.AveragePrice, item.OverflowRate, item.TradeNumber,
			item.SumVolume, item.SumTurnover, item.TurnoverMarketRate,
			item.Source, item.FetchedAt,
		)
		if err == nil {
			success++
		}
	}

	if err := tx.Commit(); err != nil {
		return total, success, fmt.Sprintf("提交失败: %v", err)
	}

	return total, success, ""
}

// GetCollectionTask 获取采集任务
func (s *ServiceImpl) GetCollectionTask(ctx context.Context, taskID string) (*CollectionTask, error) {
	var task CollectionTask
	err := s.db.QueryRowContext(ctx, `
		SELECT task_id, task_type, target_date, status, total_count, success_count,
			fail_count, error_msg, started_at, finished_at, created_at
		FROM stock_collection_tasks WHERE task_id = $1
	`, taskID).Scan(
		&task.TaskID, &task.TaskType, &task.TargetDate, &task.Status,
		&task.TotalCount, &task.SuccessCount, &task.FailCount, &task.ErrorMsg,
		&task.StartedAt, &task.FinishedAt, &task.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetCollectionLogs 获取采集日志
func (s *ServiceImpl) GetCollectionLogs(ctx context.Context, taskID string) ([]CollectionLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT log_id, task_id, level, message, detail, created_at
		FROM stock_collection_logs WHERE task_id = $1
		ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []CollectionLog{}
	for rows.Next() {
		var log CollectionLog
		if err := rows.Scan(&log.LogID, &log.TaskID, &log.Level,
			&log.Message, &log.Detail, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// ListCollectionTasks 获取采集任务列表
func (s *ServiceImpl) ListCollectionTasks(ctx context.Context, taskType string, limit int) ([]CollectionTask, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT task_id, task_type, target_date, status, total_count, success_count,
			fail_count, error_msg, started_at, finished_at, created_at
		FROM stock_collection_tasks
	`
	args := []interface{}{}
	if taskType != "" {
		query += " WHERE task_type = $1 ORDER BY created_at DESC LIMIT $2"
		args = append(args, taskType, limit)
	} else {
		query += " ORDER BY created_at DESC LIMIT $1"
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []CollectionTask{}
	for rows.Next() {
		var task CollectionTask
		if err := rows.Scan(&task.TaskID, &task.TaskType, &task.TargetDate, &task.Status,
			&task.TotalCount, &task.SuccessCount, &task.FailCount, &task.ErrorMsg,
			&task.StartedAt, &task.FinishedAt, &task.CreatedAt); err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetHealthStatus 获取健康状态
func (s *ServiceImpl) GetHealthStatus(ctx context.Context) (*HealthStatus, error) {
	// 获取今日采集统计
	today := time.Now().Format("20060102")
	var todayCount, errorCount int
	s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(success_count), 0), COALESCE(SUM(fail_count), 0)
		FROM stock_collection_tasks
		WHERE target_date = $1 AND status = 'success'
	`, today).Scan(&todayCount, &errorCount)

	// 获取最后采集时间
	var lastCollect time.Time
	s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(finished_at), '1970-01-01') FROM stock_collection_tasks
		WHERE status = 'success'
	`).Scan(&lastCollect)

	// 检测数据源延迟
	latency, err := s.client.Ping(ctx)
	sourceStatus := "ok"
	if err != nil || latency > 10*time.Second {
		sourceStatus = "slow"
	}

	return &HealthStatus{
		Service:     "datacenter",
		Status:      "ok",
		LastCollect: lastCollect,
		TodayCount:  todayCount,
		ErrorCount:  errorCount,
		Sources: []SourceStatus{
			{Name: "eastmoney", Status: sourceStatus, Latency: latency.String()},
		},
	}, nil
}

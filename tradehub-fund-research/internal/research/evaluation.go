package research

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

type FundEvaluation struct {
	FundID         string     `json:"fund_id,omitempty"`
	FundCode       string     `json:"fund_code"`
	FundName       string     `json:"fund_name,omitempty"`
	EvaluationDate time.Time  `json:"evaluation_date"`
	WindowDays     int        `json:"window_days"`
	NavCount       int        `json:"nav_count"`
	StartDate      *time.Time `json:"start_date,omitempty"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	Return1M       *float64   `json:"return_1m,omitempty"`
	Return3M       *float64   `json:"return_3m,omitempty"`
	Return6M       *float64   `json:"return_6m,omitempty"`
	Return1Y       *float64   `json:"return_1y,omitempty"`
	MaxDrawdown    *float64   `json:"max_drawdown,omitempty"`
	Volatility     *float64   `json:"volatility,omitempty"`
	Sharpe         *float64   `json:"sharpe,omitempty"`
	Score          int        `json:"score"`
	Level          string     `json:"level"`
	Reasons        []string   `json:"reasons"`
}

type fundRow struct {
	ID   string
	Code string
	Name string
}

type navPoint struct {
	Date time.Time
	Nav  float64
}

type localRankRow struct {
	Growth *float64
	Rank   int
	Total  int
}

func (s *Server) syncEvaluationSnapshots(ctx context.Context, limit, windowDays int, codes []string) ([]FundEvaluation, error) {
	if s.db == nil {
		return nil, errors.New("postgres unavailable")
	}
	if windowDays <= 0 {
		windowDays = 370
	}
	if limit <= 0 {
		limit = 500
	}
	funds, err := s.evaluationFunds(ctx, limit, codes)
	if err != nil {
		return nil, err
	}
	if len(funds) == 0 {
		return []FundEvaluation{}, nil
	}
	fundIDs := make([]string, 0, len(funds))
	for _, fund := range funds {
		fundIDs = append(fundIDs, fund.ID)
	}
	navs, err := s.navPointsByFund(ctx, fundIDs, windowDays)
	if err != nil {
		return nil, err
	}
	ranks, err := s.latestLocalRanks(ctx, fundIDs)
	if err != nil {
		return nil, err
	}
	today := dateOnly(time.Now())
	items := make([]FundEvaluation, 0, len(funds))
	for _, fund := range funds {
		eval := calculateFundEvaluation(fund, navs[fund.ID], ranks[fund.ID], today, windowDays)
		if eval.NavCount == 0 {
			continue
		}
		items = append(items, eval)
	}
	if err := s.upsertEvaluationSnapshots(ctx, items); err != nil {
		return items, err
	}
	return items, nil
}

func (s *Server) evaluationFunds(ctx context.Context, limit int, codes []string) ([]fundRow, error) {
	args := []any{}
	where := []string{}
	if normalized := normalizeCodes(codes); len(normalized) > 0 {
		args = append(args, pq.Array(normalized))
		where = append(where, "fund_code = any($1)")
	}
	query := `
		select id::text, fund_code, fund_name
		from fund
	`
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by latest_nav_date desc nulls last, fund_code"
	if limit > 0 {
		args = append(args, limit)
		query += " limit $" + intString(len(args))
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	funds := []fundRow{}
	for rows.Next() {
		var fund fundRow
		if err := rows.Scan(&fund.ID, &fund.Code, &fund.Name); err != nil {
			return nil, err
		}
		funds = append(funds, fund)
	}
	return funds, rows.Err()
}

func (s *Server) navPointsByFund(ctx context.Context, fundIDs []string, windowDays int) (map[string][]navPoint, error) {
	result := map[string][]navPoint{}
	if len(fundIDs) == 0 {
		return result, nil
	}
	cutoff := dateOnly(time.Now()).AddDate(0, 0, -windowDays)
	rows, err := s.db.QueryContext(ctx, `
		select fund_id::text, nav_date, unit_nav::float8
		from fund_nav_history
		where fund_id::text = any($1) and nav_date >= $2 and unit_nav is not null and unit_nav > 0
		order by fund_id, nav_date
	`, pq.Array(fundIDs), cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var fundID string
		var point navPoint
		if err := rows.Scan(&fundID, &point.Date, &point.Nav); err != nil {
			return nil, err
		}
		result[fundID] = append(result[fundID], point)
	}
	return result, rows.Err()
}

func (s *Server) latestLocalRanks(ctx context.Context, fundIDs []string) (map[string]localRankRow, error) {
	result := map[string]localRankRow{}
	if len(fundIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		select distinct on (fund_id) fund_id::text, growth::float8, rank, total
		from fund_performance_rank_snapshot
		where fund_id::text = any($1) and rank_type = 'performance'
		order by fund_id, rank_date desc, period = 'year' desc, period = 'day' desc, rank nulls last
	`, pq.Array(fundIDs))
	if err != nil {
		if isUndefinedTable(err) {
			return result, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var fundID string
		var row localRankRow
		var growth sql.NullFloat64
		var rank sql.NullInt64
		var total sql.NullInt64
		if err := rows.Scan(&fundID, &growth, &rank, &total); err != nil {
			return nil, err
		}
		if growth.Valid {
			value := growth.Float64
			row.Growth = &value
		}
		if rank.Valid {
			row.Rank = int(rank.Int64)
		}
		if total.Valid {
			row.Total = int(total.Int64)
		}
		result[fundID] = row
	}
	return result, rows.Err()
}

func calculateFundEvaluation(fund fundRow, points []navPoint, rank localRankRow, today time.Time, windowDays int) FundEvaluation {
	sort.Slice(points, func(i, j int) bool { return points[i].Date.Before(points[j].Date) })
	eval := FundEvaluation{
		FundID:         fund.ID,
		FundCode:       fund.Code,
		FundName:       fund.Name,
		EvaluationDate: today,
		WindowDays:     windowDays,
		NavCount:       len(points),
		Level:          "谨慎",
	}
	if len(points) == 0 {
		return eval
	}
	eval.StartDate = &points[0].Date
	eval.EndDate = &points[len(points)-1].Date
	eval.Return1M = returnSince(points, today.AddDate(0, 0, -30))
	eval.Return3M = returnSince(points, today.AddDate(0, 0, -90))
	eval.Return6M = returnSince(points, today.AddDate(0, 0, -180))
	eval.Return1Y = returnSince(points, today.AddDate(0, 0, -365))
	eval.MaxDrawdown, eval.Volatility, eval.Sharpe = riskMetrics(points)
	eval.Score, eval.Level, eval.Reasons = scoreEvaluation(eval, rank)
	return eval
}

func returnSince(points []navPoint, cutoff time.Time) *float64 {
	if len(points) == 0 {
		return nil
	}
	var start *navPoint
	for i := range points {
		if !points[i].Date.After(cutoff) {
			start = &points[i]
		}
	}
	if start == nil || start.Nav <= 0 {
		return nil
	}
	end := points[len(points)-1]
	value := (end.Nav - start.Nav) / start.Nav * 100
	return roundPtr(value, 4)
}

func riskMetrics(points []navPoint) (*float64, *float64, *float64) {
	if len(points) < 60 {
		return nil, nil, nil
	}
	peak := points[0].Nav
	maxDrawdown := 0.0
	returns := make([]float64, 0, len(points)-1)
	for i, point := range points {
		if point.Nav > peak {
			peak = point.Nav
		}
		if peak > 0 {
			drawdown := (peak - point.Nav) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
		if i > 0 && points[i-1].Nav > 0 {
			returns = append(returns, (point.Nav-points[i-1].Nav)/points[i-1].Nav)
		}
	}
	dd := roundPtr(-maxDrawdown*100, 4)
	if len(returns) < 2 {
		return dd, nil, nil
	}
	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, value := range returns {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)
	annualVol := math.Sqrt(variance) * math.Sqrt(252)
	annualReturn := mean * 252
	vol := roundPtr(annualVol*100, 4)
	var sharpe *float64
	if annualVol > 0 {
		sharpe = roundPtr((annualReturn-0.02)/annualVol, 4)
	}
	return dd, vol, sharpe
}

func scoreEvaluation(eval FundEvaluation, rank localRankRow) (int, string, []string) {
	score := 0
	reasons := []string{}
	if rank.Rank > 0 && rank.Total > 0 {
		ratio := float64(rank.Rank) / float64(rank.Total)
		if ratio <= 0.25 {
			score += 30
			reasons = append(reasons, "同类排名前25%")
		} else if ratio <= 0.5 {
			score += 18
			reasons = append(reasons, "同类排名前50%")
		}
	}
	if positivePtr(eval.Return1Y) || positivePtr(eval.Return6M) || (rank.Growth != nil && *rank.Growth > 0) {
		score += 20
		reasons = append(reasons, "周期收益为正")
	}
	if eval.Sharpe != nil {
		if *eval.Sharpe >= 1 {
			score += 25
			reasons = append(reasons, "夏普率优秀")
		} else if *eval.Sharpe >= 0.5 {
			score += 15
			reasons = append(reasons, "夏普率尚可")
		}
	}
	if eval.MaxDrawdown != nil {
		dd := math.Abs(*eval.MaxDrawdown)
		if dd <= 15 {
			score += 20
			reasons = append(reasons, "回撤控制较好")
		} else if dd <= 30 {
			score += 10
			reasons = append(reasons, "回撤中等")
		}
	}
	level := "谨慎"
	if score >= 75 {
		level = "优选"
	} else if score >= 50 {
		level = "观察"
	}
	return score, level, reasons
}

func (s *Server) upsertEvaluationSnapshots(ctx context.Context, items []FundEvaluation) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		reasons, _ := json.Marshal(item.Reasons)
		raw, _ := json.Marshal(item)
		if _, err := tx.ExecContext(ctx, `
			insert into fund_evaluation_snapshot(
				id, fund_id, evaluation_date, window_days, nav_count, start_date, end_date,
				return_1m, return_3m, return_6m, return_1y,
				max_drawdown, volatility, sharpe, score, level, reasons, source, raw_data,
				created_at, updated_at
			)
			values(
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11,
				$12, $13, $14, $15, $16, $17::jsonb, 'go_fund_research', $18::jsonb,
				now(), now()
			)
			on conflict(fund_id, evaluation_date, window_days, source) do update set
				nav_count = excluded.nav_count,
				start_date = excluded.start_date,
				end_date = excluded.end_date,
				return_1m = excluded.return_1m,
				return_3m = excluded.return_3m,
				return_6m = excluded.return_6m,
				return_1y = excluded.return_1y,
				max_drawdown = excluded.max_drawdown,
				volatility = excluded.volatility,
				sharpe = excluded.sharpe,
				score = excluded.score,
				level = excluded.level,
				reasons = excluded.reasons,
				raw_data = excluded.raw_data,
				updated_at = now()
		`, newUUIDString(), item.FundID, item.EvaluationDate, item.WindowDays, item.NavCount, item.StartDate, item.EndDate,
			item.Return1M, item.Return3M, item.Return6M, item.Return1Y, item.MaxDrawdown, item.Volatility, item.Sharpe,
			item.Score, item.Level, string(reasons), string(raw)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func dateOnly(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func roundPtr(value float64, digits int) *float64 {
	scale := math.Pow10(digits)
	rounded := math.Round(value*scale) / scale
	return &rounded
}

func positivePtr(value *float64) bool {
	return value != nil && *value > 0
}

func newUUIDString() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return dateOnly(time.Now()).Format("20060102") + "-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return strings.ToLower(
		intHex(b[0:4]) + "-" +
			intHex(b[4:6]) + "-" +
			intHex(b[6:8]) + "-" +
			intHex(b[8:10]) + "-" +
			intHex(b[10:16]),
	)
}

func intHex(bytes []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(bytes)*2)
	for i, value := range bytes {
		out[i*2] = hex[value>>4]
		out[i*2+1] = hex[value&0x0f]
	}
	return string(out)
}

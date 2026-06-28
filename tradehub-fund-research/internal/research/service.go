package research

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Server struct {
	db *sql.DB
	em *EastMoneyClient
}

func NewServer(db *sql.DB, em *EastMoneyClient) *Server {
	return &Server{db: db, em: em}
}

func (s *Server) buildRankUniverse(ctx context.Context, limit int) ([]Fund, error) {
	periods := []string{"month_3", "month_6", "year_1", "year_2", "year_3", "year_5", "this_year"}
	funds := map[string]*Fund{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, len(periods))

	for _, period := range periods {
		wg.Add(1)
		go func(period string) {
			defer wg.Done()
			rows, err := s.em.Ranking(ctx, period, limit)
			if err != nil {
				errs <- fmt.Errorf("%s: %w", period, err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, row := range rows {
				fund := funds[row.Code]
				if fund == nil {
					fund = &Fund{Code: row.Code, Name: row.Name, Type: row.FundType}
					funds[row.Code] = fund
				}
				periodPerf := PeriodPerformance{
					Growth:    row.Growth,
					Rank:      row.Rank,
					Total:     row.Total,
					RankRatio: rankRatio(row.Rank, row.Total),
				}
				switch period {
				case "month_3":
					fund.Performance.Month3 = periodPerf
				case "month_6":
					fund.Performance.Month6 = periodPerf
				case "year_1":
					fund.Performance.Year1 = periodPerf
				case "year_2":
					fund.Performance.Year2 = periodPerf
				case "year_3":
					fund.Performance.Year3 = periodPerf
				case "year_5":
					fund.Performance.Year5 = periodPerf
				case "this_year":
					fund.Performance.ThisYear = periodPerf
				}
			}
		}(period)
	}
	wg.Wait()
	close(errs)
	if len(funds) == 0 {
		var messages []string
		for err := range errs {
			messages = append(messages, err.Error())
		}
		if len(messages) > 0 {
			return nil, errors.New(strings.Join(messages, "; "))
		}
		return nil, errors.New("empty fund ranking universe")
	}
	out := make([]Fund, 0, len(funds))
	for _, fund := range funds {
		fund.Is4433 = Is4433(*fund, true)
		out = append(out, *fund)
	}
	return out, nil
}

func (s *Server) enrichFunds(ctx context.Context, funds []Fund, limit int) []Fund {
	if limit <= 0 || limit > len(funds) {
		limit = len(funds)
	}
	out := append([]Fund(nil), funds...)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := 0; i < limit; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail, err := s.em.FundInfo(ctx, out[i].Code)
			if err != nil {
				return
			}
			mergeFundDetail(&out[i], detail)
		}()
	}
	wg.Wait()
	return out
}

func mergeFundDetail(base *Fund, detail Fund) {
	if detail.Name != "" {
		base.Name = detail.Name
	}
	if detail.Type != "" {
		base.Type = detail.Type
	}
	if detail.EstablishedDate != "" {
		base.EstablishedDate = detail.EstablishedDate
	}
	if detail.NetAssetsScale > 0 {
		base.NetAssetsScale = detail.NetAssetsScale
		base.NetAssetsScaleYi = detail.NetAssetsScaleYi
	}
	if detail.IndexCode != "" {
		base.IndexCode = detail.IndexCode
	}
	if detail.IndexName != "" {
		base.IndexName = detail.IndexName
	}
	if detail.Rate != "" {
		base.Rate = detail.Rate
	}
	if detail.FixedInvestmentStatus != "" {
		base.FixedInvestmentStatus = detail.FixedInvestmentStatus
	}
	if detail.Stddev.Avg135 > 0 {
		base.Stddev = detail.Stddev
	}
	if detail.MaxRetracement.Avg135 > 0 {
		base.MaxRetracement = detail.MaxRetracement
	}
	if detail.Sharp.Avg135 > 0 {
		base.Sharp = detail.Sharp
	}
	if len(detail.Stocks) > 0 {
		base.Stocks = detail.Stocks
	}
	if detail.Manager.Name != "" {
		base.Manager = detail.Manager
	}
	if detail.AssetsProportion.PubDate != "" {
		base.AssetsProportion = detail.AssetsProportion
	}
	if len(detail.IndustryProportions) > 0 {
		base.IndustryProportions = detail.IndustryProportions
	}
	base.Is4433 = Is4433(*base, true)
}

func Is4433(fund Fund, requireFiveYear bool) bool {
	quarter := 25.0
	third := float64(1) / float64(3) * 100
	if requireFiveYear && (fund.Performance.Year5.Rank == 0 || fund.Performance.Year5.Growth == 0) {
		return false
	}
	checks := []PeriodPerformance{
		fund.Performance.Year1,
		fund.Performance.Year2,
		fund.Performance.Year3,
		fund.Performance.ThisYear,
	}
	if requireFiveYear {
		checks = append(checks, fund.Performance.Year5)
	}
	for _, perf := range checks {
		if perf.Rank == 0 || perf.RankRatio == 0 || perf.RankRatio > quarter {
			return false
		}
	}
	for _, perf := range []PeriodPerformance{fund.Performance.Month6, fund.Performance.Month3} {
		if perf.Rank == 0 || perf.RankRatio == 0 || perf.RankRatio > third {
			return false
		}
	}
	return true
}

func ApplyFilter(funds []Fund, p FundFilterParams) []Fund {
	out := make([]Fund, 0, len(funds))
	for _, fund := range funds {
		if len(p.Types) > 0 && !containsString(p.Types, fund.Type) {
			continue
		}
		if p.MinScale > 0 && fund.NetAssetsScaleYi > 0 && fund.NetAssetsScaleYi < p.MinScale {
			continue
		}
		if p.MaxScale > 0 && fund.NetAssetsScaleYi > 0 && fund.NetAssetsScaleYi > p.MaxScale {
			continue
		}
		if p.MinEstabYears > 0 && yearsSince(fund.EstablishedDate) > 0 && yearsSince(fund.EstablishedDate) < p.MinEstabYears {
			continue
		}
		if p.MinManagerYears > 0 && fund.Manager.ManageDays > 0 && fund.Manager.ManageDays/365 < p.MinManagerYears {
			continue
		}
		if p.Year1RankRatio > 0 && !rankRatioPass(fund.Performance.Year1, p.Year1RankRatio) {
			continue
		}
		if p.ThisYear235RankRatio > 0 {
			perfs := []PeriodPerformance{fund.Performance.Year2, fund.Performance.Year3, fund.Performance.ThisYear}
			if p.RequireFiveYear {
				perfs = append(perfs, fund.Performance.Year5)
			}
			if !allRankRatioPass(perfs, p.ThisYear235RankRatio) {
				continue
			}
		}
		if p.Month6RankRatio > 0 && !rankRatioPass(fund.Performance.Month6, p.Month6RankRatio) {
			continue
		}
		if p.Month3RankRatio > 0 && !rankRatioPass(fund.Performance.Month3, p.Month3RankRatio) {
			continue
		}
		if p.Max135AvgStddev > 0 && fund.Stddev.Avg135 > 0 && fund.Stddev.Avg135 > p.Max135AvgStddev {
			continue
		}
		if p.Max135AvgRetr > 0 && fund.MaxRetracement.Avg135 > 0 && fund.MaxRetracement.Avg135 > p.Max135AvgRetr {
			continue
		}
		if p.Min135AvgSharp > 0 && fund.Sharp.Avg135 > 0 && fund.Sharp.Avg135 < p.Min135AvgSharp {
			continue
		}
		out = append(out, fund)
	}
	sortFunds(out, p.Sort)
	if p.Limit > 0 && len(out) > p.Limit {
		return out[:p.Limit]
	}
	return out
}

func rankRatioPass(perf PeriodPerformance, threshold float64) bool {
	return perf.Rank > 0 && perf.RankRatio > 0 && perf.RankRatio <= threshold
}

func allRankRatioPass(perfs []PeriodPerformance, threshold float64) bool {
	for _, perf := range perfs {
		if !rankRatioPass(perf, threshold) {
			return false
		}
	}
	return true
}

func sortFunds(funds []Fund, sortKey string) {
	switch sortKey {
	case "sharp":
		sort.Slice(funds, func(i, j int) bool { return funds[i].Sharp.Avg135 > funds[j].Sharp.Avg135 })
	case "stddev":
		sort.Slice(funds, func(i, j int) bool { return funds[i].Stddev.Avg135 < funds[j].Stddev.Avg135 })
	case "retracement":
		sort.Slice(funds, func(i, j int) bool { return funds[i].MaxRetracement.Avg135 < funds[j].MaxRetracement.Avg135 })
	case "year_1":
		sort.Slice(funds, func(i, j int) bool { return funds[i].Performance.Year1.Growth > funds[j].Performance.Year1.Growth })
	case "year_3":
		sort.Slice(funds, func(i, j int) bool { return funds[i].Performance.Year3.Growth > funds[j].Performance.Year3.Growth })
	default:
		sort.Slice(funds, func(i, j int) bool { return funds[i].Performance.Month3.Growth > funds[j].Performance.Month3.Growth })
	}
}

func DiagnoseFund(fund Fund, p FundFilterParams) Fund {
	check := func(key, label string, passed bool, value, expected, level string) {
		fund.Diagnostics = append(fund.Diagnostics, DiagnosticItem{
			Key: key, Label: label, Passed: passed, Value: value, Expected: expected, Level: level,
		})
	}
	check("4433", "4433 法则", Is4433(fund, p.RequireFiveYear), boolText(Is4433(fund, p.RequireFiveYear)), "长中短周期排名达标", "core")
	check("scale", "基金规模", p.MinScale <= 0 || fund.NetAssetsScaleYi == 0 || fund.NetAssetsScaleYi >= p.MinScale, fmt.Sprintf("%.2f 亿", fund.NetAssetsScaleYi), fmt.Sprintf("不低于 %.2f 亿", p.MinScale), "risk")
	check("manager", "基金经理任期", p.MinManagerYears <= 0 || fund.Manager.ManageDays == 0 || fund.Manager.ManageDays/365 >= p.MinManagerYears, fmt.Sprintf("%.1f 年", fund.Manager.ManageDays/365), fmt.Sprintf("不低于 %.1f 年", p.MinManagerYears), "manager")
	check("stddev", "波动率", p.Max135AvgStddev <= 0 || fund.Stddev.Avg135 == 0 || fund.Stddev.Avg135 <= p.Max135AvgStddev, fmt.Sprintf("%.2f", fund.Stddev.Avg135), fmt.Sprintf("不高于 %.2f", p.Max135AvgStddev), "risk")
	check("retracement", "最大回撤", p.Max135AvgRetr <= 0 || fund.MaxRetracement.Avg135 == 0 || fund.MaxRetracement.Avg135 <= p.Max135AvgRetr, fmt.Sprintf("%.2f", fund.MaxRetracement.Avg135), fmt.Sprintf("不高于 %.2f", p.Max135AvgRetr), "risk")
	check("sharp", "夏普比率", p.Min135AvgSharp <= 0 || fund.Sharp.Avg135 == 0 || fund.Sharp.Avg135 >= p.Min135AvgSharp, fmt.Sprintf("%.2f", fund.Sharp.Avg135), fmt.Sprintf("不低于 %.2f", p.Min135AvgSharp), "risk")
	check("holdings", "重仓披露", len(fund.Stocks) > 0, fmt.Sprintf("%d 只", len(fund.Stocks)), "能读取前十大持仓", "data")
	return fund
}

func boolText(value bool) string {
	if value {
		return "通过"
	}
	return "未通过"
}

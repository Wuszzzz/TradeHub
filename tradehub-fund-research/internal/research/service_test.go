package research

import "testing"

func TestIs4433(t *testing.T) {
	fund := Fund{Performance: PerformanceMetrics{
		Month3:   PeriodPerformance{Rank: 30, Total: 100, RankRatio: 30},
		Month6:   PeriodPerformance{Rank: 30, Total: 100, RankRatio: 30},
		Year1:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
		Year2:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
		Year3:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
		Year5:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20, Growth: 15},
		ThisYear: PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
	}}

	if !Is4433(fund, true) {
		t.Fatal("expected fund to pass 4433")
	}

	fund.Performance.Month3.RankRatio = 40
	if Is4433(fund, true) {
		t.Fatal("expected fund to fail short-term ranking threshold")
	}
}

func TestApplyFilter(t *testing.T) {
	funds := []Fund{
		{
			Code:             "000001",
			Type:             "混合型",
			EstablishedDate:  "2010-01-01",
			NetAssetsScaleYi: 10,
			Manager:          FundManager{ManageDays: 365 * 8},
			Performance: PerformanceMetrics{
				Month3:   PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20, Growth: 8},
				Month6:   PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
				Year1:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
				Year2:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
				Year3:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
				Year5:    PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
				ThisYear: PeriodPerformance{Rank: 20, Total: 100, RankRatio: 20},
			},
			Stddev:         RiskMetrics{Avg135: 18},
			MaxRetracement: RiskMetrics{Avg135: 12},
			Sharp:          RiskMetrics{Avg135: 1.4},
		},
		{
			Code:             "000002",
			Type:             "混合型",
			NetAssetsScaleYi: 80,
			Performance:      PerformanceMetrics{Month3: PeriodPerformance{Growth: 20}},
		},
	}

	result := ApplyFilter(funds, FundFilterParams{
		Types:                []string{"混合型"},
		MinScale:             2,
		MaxScale:             50,
		MinManagerYears:      5,
		MinEstabYears:        5,
		Year1RankRatio:       25,
		ThisYear235RankRatio: 25,
		Month6RankRatio:      33.33,
		Month3RankRatio:      33.33,
		Max135AvgStddev:      25,
		Min135AvgSharp:       1,
		Max135AvgRetr:        25,
		RequireFiveYear:      true,
	})
	if len(result) != 1 || result[0].Code != "000001" {
		t.Fatalf("unexpected filter result: %#v", result)
	}
}

func TestSimilarity(t *testing.T) {
	funds := []Fund{
		{Code: "A", Stocks: []FundStock{{Name: "贵州茅台"}, {Name: "宁德时代"}}},
		{Code: "B", Stocks: []FundStock{{Name: "贵州茅台"}, {Name: "招商银行"}}},
	}
	result := Similarity(funds)
	if len(result) != 2 {
		t.Fatalf("expected two similarity rows, got %d", len(result))
	}
	if result[0].SimilarityValue <= 0 {
		t.Fatalf("expected positive similarity, got %#v", result[0])
	}
	if len(result[0].SameStocks) != 1 || result[0].SameStocks[0] != "贵州茅台" {
		t.Fatalf("unexpected same stocks: %#v", result[0].SameStocks)
	}
}

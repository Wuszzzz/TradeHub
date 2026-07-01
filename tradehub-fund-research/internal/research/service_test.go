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

func TestAnalyzePortfolioHealth(t *testing.T) {
	result := AnalyzePortfolioHealth(PortfolioHealthRequest{
		AccountID:   "acc-1",
		AccountName: "测试账户",
		Positions: []PortfolioPosition{
			{
				FundCode:       "017811",
				FundName:       "东方人工智能主题混合C",
				FundType:       "混合型-偏股",
				HoldingCost:    10000,
				MarketValue:    18000,
				PnL:            8000,
				PnLRate:        80,
				Return30D:      60,
				Return1M:       29,
				Return3M:       120,
				Return1Y:       255,
				ReturnThisYear: 166,
				AssetAllocation: []PortfolioAllocationItem{
					{Name: "股票", Ratio: 92},
					{Name: "银行存款", Ratio: 4},
				},
				Industry: []PortfolioAllocationItem{
					{Name: "制造业", Ratio: 80},
				},
				TopHoldings: []PortfolioHoldingItem{
					{Code: "002850", Name: "科达利", Weight: 6},
				},
			},
			{
				FundCode:       "000001",
				FundName:       "测试债券",
				FundType:       "债券型",
				HoldingCost:    10000,
				MarketValue:    10200,
				PnL:            200,
				PnLRate:        2,
				Return30D:      0.5,
				Return1Y:       4,
				ReturnThisYear: 2,
				AssetAllocation: []PortfolioAllocationItem{
					{Name: "债券", Ratio: 90},
					{Name: "现金", Ratio: 5},
				},
			},
		},
	})
	if result.Score <= 0 {
		t.Fatalf("expected positive score, got %d", result.Score)
	}
	if result.Overview.PositionCount != 2 {
		t.Fatalf("expected 2 positions, got %d", result.Overview.PositionCount)
	}
	if len(result.FundTypeBreakdown) == 0 || len(result.AssetBreakdown) == 0 {
		t.Fatalf("expected breakdowns, got %#v %#v", result.FundTypeBreakdown, result.AssetBreakdown)
	}
	if result.AIPrompt == "" {
		t.Fatal("expected ai prompt")
	}
}

func TestRecommendTagsFromSeedSector(t *testing.T) {
	server := NewServer(nil, nil)
	tags, err := server.recommendTagsForFund(t.Context(), "000055")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) == 0 {
		t.Fatal("expected tags for seeded fund")
	}
	foundGlobalTech := false
	for _, tag := range tags {
		if tag.Name == "海外科技" {
			foundGlobalTech = true
		}
	}
	if !foundGlobalTech {
		t.Fatalf("expected overseas tech tag, got %#v", tags)
	}
}

func TestRelatedSectorMapUsesSeed(t *testing.T) {
	server := NewServer(nil, nil)
	rows, err := server.relatedSectorMap(t.Context(), []string{"260104", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	row, ok := rows["260104"]
	if !ok {
		t.Fatalf("expected seed sector row, got %#v", rows)
	}
	if row.Sector != "沪深300指数" || row.SecID != "1.000300" || row.Source != "seed" {
		t.Fatalf("unexpected seed sector row: %#v", row)
	}
}

package research

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func AnalyzePortfolioHealth(input PortfolioHealthRequest) PortfolioHealthResult {
	positions := normalizePortfolioPositions(input.Positions)
	result := PortfolioHealthResult{
		Tags:              []string{},
		Dimensions:        []PortfolioDimension{},
		FundTypeBreakdown: []PortfolioBreakdownItem{},
		AssetBreakdown:    []PortfolioBreakdownItem{},
		IndustryBreakdown: []PortfolioBreakdownItem{},
		PositionAnalysis:  []PortfolioPositionAnalysis{},
		Findings:          []PortfolioFinding{},
		Suggestions:        []string{},
		Overlap:            PortfolioOverlap{RepeatedHoldings: []PortfolioOverlapItem{}},
	}
	if len(positions) == 0 {
		result.Score = 0
		result.Level = "N/A"
		result.Summary = "当前账户没有可分析的基金持仓。"
		result.DataQuality = PortfolioDataQuality{Score: 0, Warnings: []string{}}
		result.AIPrompt = portfolioHealthPrompt(result)
		return result
	}

	totalMarketValue := sumPositionMarketValue(positions)
	totalCost := 0.0
	totalPnL := 0.0
	for _, p := range positions {
		totalCost += p.HoldingCost
		totalPnL += p.PnL
	}

	result.Overview = PortfolioOverview{
		AccountID:        input.AccountID,
		AccountName:      input.AccountName,
		PositionCount:    len(positions),
		TotalCost:        round2(totalCost),
		TotalMarketValue: round2(totalMarketValue),
		TotalPnL:         round2(totalPnL),
		TotalPnLRate:     round4(safeDiv(totalPnL, totalCost) * 100),
	}

	weightedReturns := weightedPortfolioReturns(positions, totalMarketValue)
	result.Overview.WeightedReturn30D = weightedReturns["30d"]
	result.Overview.WeightedReturn1M = weightedReturns["1m"]
	result.Overview.WeightedReturn3M = weightedReturns["3m"]
	result.Overview.WeightedReturn1Y = weightedReturns["1y"]
	result.Overview.WeightedReturnYTD = weightedReturns["ytd"]
	result.Overview.MaxPositionWeight, result.Overview.Top3PositionWeight = positionConcentration(positions, totalMarketValue)

	result.FundTypeBreakdown = buildFundTypeBreakdown(positions, totalMarketValue)
	result.AssetBreakdown = buildAllocationBreakdown(positions, totalMarketValue, "asset")
	result.IndustryBreakdown = buildAllocationBreakdown(positions, totalMarketValue, "industry")
	result.PositionAnalysis = buildPositionAnalysis(positions, totalMarketValue)
	result.Overlap = buildPortfolioOverlap(positions, totalMarketValue)
	result.DataQuality = buildPortfolioDataQuality(positions)
	result.Dimensions = scorePortfolio(result)
	result.Score = 0
	for _, dim := range result.Dimensions {
		result.Score += dim.Score
	}
	result.Level = portfolioLevel(result.Score)
	result.Tags = portfolioTags(result)
	result.Findings = buildPortfolioFindings(result)
	result.Suggestions = buildPortfolioSuggestions(result)
	result.Summary = buildPortfolioSummary(result)
	result.AIPrompt = portfolioHealthPrompt(result)
	return result
}

func normalizePortfolioPositions(input []PortfolioPosition) []PortfolioPosition {
	out := make([]PortfolioPosition, 0, len(input))
	for _, p := range input {
		if strings.TrimSpace(p.FundCode) == "" {
			continue
		}
		if p.MarketValue <= 0 && p.LatestNav > 0 && p.HoldingShare > 0 {
			p.MarketValue = p.LatestNav * p.HoldingShare
		}
		if p.PnL == 0 && p.MarketValue > 0 && p.HoldingCost > 0 {
			p.PnL = p.MarketValue - p.HoldingCost
		}
		if p.PnLRate == 0 && p.HoldingCost > 0 {
			p.PnLRate = p.PnL / p.HoldingCost * 100
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MarketValue == out[j].MarketValue {
			return out[i].FundCode < out[j].FundCode
		}
		return out[i].MarketValue > out[j].MarketValue
	})
	return out
}

func sumPositionMarketValue(positions []PortfolioPosition) float64 {
	total := 0.0
	for _, p := range positions {
		total += p.MarketValue
	}
	return total
}

func weightedPortfolioReturns(positions []PortfolioPosition, total float64) map[string]float64 {
	sums := map[string]float64{}
	weights := map[string]float64{}
	for _, p := range positions {
		weight := safeDiv(p.MarketValue, total)
		addWeighted := func(key string, value float64) {
			if value == 0 {
				return
			}
			sums[key] += weight * value
			weights[key] += weight
		}
		addWeighted("30d", p.Return30D)
		addWeighted("1m", p.Return1M)
		addWeighted("3m", p.Return3M)
		addWeighted("1y", p.Return1Y)
		addWeighted("ytd", p.ReturnThisYear)
	}
	return map[string]float64{
		"30d": round4(safeDiv(sums["30d"], weights["30d"])),
		"1m":  round4(safeDiv(sums["1m"], weights["1m"])),
		"3m":  round4(safeDiv(sums["3m"], weights["3m"])),
		"1y":  round4(safeDiv(sums["1y"], weights["1y"])),
		"ytd": round4(safeDiv(sums["ytd"], weights["ytd"])),
	}
}

func positionConcentration(positions []PortfolioPosition, total float64) (float64, float64) {
	top3 := 0.0
	maxWeight := 0.0
	for i, p := range positions {
		weight := safeDiv(p.MarketValue, total) * 100
		if weight > maxWeight {
			maxWeight = weight
		}
		if i < 3 {
			top3 += weight
		}
	}
	return round4(maxWeight), round4(top3)
}

func buildFundTypeBreakdown(positions []PortfolioPosition, total float64) []PortfolioBreakdownItem {
	buckets := map[string]*PortfolioBreakdownItem{}
	for _, p := range positions {
		name := normalizeFundType(p.FundType)
		item := ensureBreakdownItem(buckets, name)
		item.MarketValue += p.MarketValue
		item.PnL += p.PnL
		item.Count++
		item.FundCodes = append(item.FundCodes, p.FundCode)
		item.Return30D += p.MarketValue * p.Return30D
		item.ReturnYTD += p.MarketValue * p.ReturnThisYear
	}
	return finishBreakdown(buckets, total)
}

func buildAllocationBreakdown(positions []PortfolioPosition, total float64, kind string) []PortfolioBreakdownItem {
	buckets := map[string]*PortfolioBreakdownItem{}
	for _, p := range positions {
		allocation := p.AssetAllocation
		if kind == "industry" {
			allocation = p.Industry
		}
		if len(allocation) == 0 {
			continue
		}
		for _, alloc := range allocation {
			name := strings.TrimSpace(alloc.Name)
			if name == "" || alloc.Ratio <= 0 {
				continue
			}
			name = normalizeAllocationName(name, kind)
			exposure := p.MarketValue * alloc.Ratio / 100
			item := ensureBreakdownItem(buckets, name)
			item.MarketValue += exposure
			item.PnL += p.PnL * safeDiv(exposure, p.MarketValue)
			item.Count++
			item.FundCodes = appendUnique(item.FundCodes, p.FundCode)
			item.Return30D += exposure * p.Return30D
			item.ReturnYTD += exposure * p.ReturnThisYear
		}
	}
	return finishBreakdown(buckets, total)
}

func ensureBreakdownItem(buckets map[string]*PortfolioBreakdownItem, name string) *PortfolioBreakdownItem {
	if name == "" {
		name = "未分类"
	}
	if buckets[name] == nil {
		buckets[name] = &PortfolioBreakdownItem{Name: name, FundCodes: []string{}}
	}
	return buckets[name]
}

func finishBreakdown(buckets map[string]*PortfolioBreakdownItem, total float64) []PortfolioBreakdownItem {
	items := make([]PortfolioBreakdownItem, 0, len(buckets))
	for _, item := range buckets {
		item.MarketValue = round2(item.MarketValue)
		item.Weight = round4(safeDiv(item.MarketValue, total) * 100)
		item.PnL = round2(item.PnL)
		item.PnLRate = round4(safeDiv(item.PnL, item.MarketValue-item.PnL) * 100)
		item.Return30D = round4(safeDiv(item.Return30D, item.MarketValue))
		item.ReturnYTD = round4(safeDiv(item.ReturnYTD, item.MarketValue))
		sort.Strings(item.FundCodes)
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Weight == items[j].Weight {
			return items[i].Name < items[j].Name
		}
		return items[i].Weight > items[j].Weight
	})
	return items
}

func buildPositionAnalysis(positions []PortfolioPosition, total float64) []PortfolioPositionAnalysis {
	items := make([]PortfolioPositionAnalysis, 0, len(positions))
	for _, p := range positions {
		weight := safeDiv(p.MarketValue, total) * 100
		tags := positionProblemTags(p, weight)
		items = append(items, PortfolioPositionAnalysis{
			FundCode:     p.FundCode,
			FundName:     p.FundName,
			FundType:     p.FundType,
			Weight:       round4(weight),
			MarketValue:  round2(p.MarketValue),
			PnL:          round2(p.PnL),
			PnLRate:      round4(p.PnLRate),
			Return30D:    round4(p.Return30D),
			Return3M:     round4(p.Return3M),
			Return1Y:     round4(p.Return1Y),
			ReturnYTD:    round4(p.ReturnThisYear),
			Role:         inferPositionRole(p, weight),
			Contribution: round4(safeDiv(p.PnL, total) * 100),
			RiskLevel:    inferRiskLevel(p, weight),
			ProblemTags:  tags,
			DataFlags:    p.DataFlags,
		})
	}
	return items
}

func buildPortfolioOverlap(positions []PortfolioPosition, total float64) PortfolioOverlap {
	type exposureAgg struct {
		code      string
		name      string
		exposure  float64
		fundCodes []string
	}
	agg := map[string]*exposureAgg{}
	for _, p := range positions {
		for _, h := range p.TopHoldings {
			if strings.TrimSpace(h.Name) == "" || h.Weight <= 0 {
				continue
			}
			key := h.Code
			if key == "" {
				key = h.Name
			}
			if agg[key] == nil {
				agg[key] = &exposureAgg{code: h.Code, name: h.Name, fundCodes: []string{}}
			}
			agg[key].exposure += p.MarketValue * h.Weight / 100
			agg[key].fundCodes = appendUnique(agg[key].fundCodes, p.FundCode)
		}
	}
	items := []PortfolioOverlapItem{}
	maxName := ""
	maxExposure := 0.0
	duplication := 0.0
	for _, row := range agg {
		exposurePct := safeDiv(row.exposure, total) * 100
		if exposurePct > maxExposure {
			maxExposure = exposurePct
			maxName = row.name
		}
		if len(row.fundCodes) >= 2 {
			duplication += exposurePct
			items = append(items, PortfolioOverlapItem{
				Name:      row.name,
				Code:      row.code,
				Exposure:  round4(exposurePct),
				FundCodes: row.fundCodes,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Exposure > items[j].Exposure
	})
	if len(items) > 10 {
		items = items[:10]
	}
	return PortfolioOverlap{
		MaxHoldingName:       maxName,
		MaxHoldingExposure:   round4(maxExposure),
		RepeatedHoldings:     items,
		EstimatedDuplication: round4(duplication),
	}
}

func buildPortfolioDataQuality(positions []PortfolioPosition) PortfolioDataQuality {
	q := PortfolioDataQuality{Score: 10, Warnings: []string{}}
	for _, p := range positions {
		if p.LatestNav <= 0 {
			q.MissingNavCount++
		}
		if p.Return30D == 0 && p.Return1Y == 0 && p.ReturnThisYear == 0 {
			q.MissingReturnCount++
		}
		if len(p.AssetAllocation) == 0 && len(p.Industry) == 0 {
			q.MissingAllocationCount++
		}
		if len(p.TopHoldings) == 0 {
			q.MissingHoldingCount++
		}
	}
	q.Score -= minInt(q.MissingNavCount*2, 4)
	q.Score -= minInt(q.MissingReturnCount, 3)
	q.Score -= minInt(q.MissingAllocationCount, 3)
	if q.MissingHoldingCount > 0 {
		q.Score -= minInt((q.MissingHoldingCount+1)/2, 2)
	}
	if q.Score < 0 {
		q.Score = 0
	}
	if q.MissingAllocationCount > 0 {
		q.Warnings = append(q.Warnings, fmt.Sprintf("%d 只基金缺少资产/行业配置，穿透分析会偏保守。", q.MissingAllocationCount))
	}
	if q.MissingHoldingCount > 0 {
		q.Warnings = append(q.Warnings, fmt.Sprintf("%d 只基金缺少前十大持仓，重合度只代表可读取部分。", q.MissingHoldingCount))
	}
	return q
}

func scorePortfolio(result PortfolioHealthResult) []PortfolioDimension {
	dims := []PortfolioDimension{}
	assetScore, assetReasons := scoreAssetStructure(result)
	returnScore, returnReasons := scoreReturnQuality(result)
	riskScore, riskReasons := scoreRiskControl(result)
	divScore, divReasons := scoreDiversification(result)
	disciplineScore, disciplineReasons := scoreHoldingDiscipline(result)
	dims = append(dims,
		PortfolioDimension{Key: "asset_structure", Label: "资产结构", Score: assetScore, MaxScore: 25, Reasons: assetReasons},
		PortfolioDimension{Key: "return_quality", Label: "收益质量", Score: returnScore, MaxScore: 20, Reasons: returnReasons},
		PortfolioDimension{Key: "risk_control", Label: "风险控制", Score: riskScore, MaxScore: 20, Reasons: riskReasons},
		PortfolioDimension{Key: "diversification", Label: "分散程度", Score: divScore, MaxScore: 20, Reasons: divReasons},
		PortfolioDimension{Key: "data_quality", Label: "数据完整性", Score: result.DataQuality.Score, MaxScore: 10, Reasons: result.DataQuality.Warnings},
		PortfolioDimension{Key: "holding_discipline", Label: "持仓纪律", Score: disciplineScore, MaxScore: 5, Reasons: disciplineReasons},
	)
	return dims
}

func scoreAssetStructure(result PortfolioHealthResult) (int, []string) {
	score := 25
	reasons := []string{}
	equity := breakdownWeight(result.AssetBreakdown, "股票")
	bond := breakdownWeight(result.AssetBreakdown, "债券")
	cash := breakdownWeight(result.AssetBreakdown, "现金") + breakdownWeight(result.AssetBreakdown, "银行存款")
	if equity == 0 {
		equity = typeEquityWeight(result.FundTypeBreakdown)
	}
	if equity >= 85 {
		score -= 8
		reasons = append(reasons, fmt.Sprintf("股票/偏股暴露约 %.1f%%，组合明显偏进攻。", equity))
	} else if equity >= 70 {
		score -= 4
		reasons = append(reasons, fmt.Sprintf("股票/偏股暴露约 %.1f%%，波动承受要求较高。", equity))
	}
	if bond+cash < 10 {
		score -= 5
		reasons = append(reasons, "债券/现金类防守仓不足。")
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "资产结构没有明显极端暴露。")
	}
	return score, reasons
}

func scoreReturnQuality(result PortfolioHealthResult) (int, []string) {
	score := 20
	reasons := []string{}
	if result.Overview.WeightedReturnYTD < 0 {
		score -= 5
		reasons = append(reasons, fmt.Sprintf("组合今年以来加权收益 %.2f%%，处于亏损状态。", result.Overview.WeightedReturnYTD))
	}
	if result.Overview.WeightedReturn30D < -8 {
		score -= 4
		reasons = append(reasons, fmt.Sprintf("近30天加权收益 %.2f%%，短期承压明显。", result.Overview.WeightedReturn30D))
	}
	positiveWeight := 0.0
	for _, p := range result.PositionAnalysis {
		if p.PnL > 0 {
			positiveWeight += p.Weight
		}
	}
	if positiveWeight < 40 {
		score -= 5
		reasons = append(reasons, "盈利持仓权重偏低，收益来源不够健康。")
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "组合收益没有明显单点异常，收益质量可接受。")
	}
	return score, reasons
}

func scoreRiskControl(result PortfolioHealthResult) (int, []string) {
	score := 20
	reasons := []string{}
	if result.Overview.MaxPositionWeight > 35 {
		score -= 6
		reasons = append(reasons, fmt.Sprintf("最大单只仓位 %.1f%%，单点波动影响较大。", result.Overview.MaxPositionWeight))
	}
	if result.Overview.Top3PositionWeight > 75 {
		score -= 6
		reasons = append(reasons, fmt.Sprintf("前三大仓位 %.1f%%，组合集中度偏高。", result.Overview.Top3PositionWeight))
	}
	highRiskWeight := 0.0
	for _, p := range result.PositionAnalysis {
		if p.RiskLevel == "高" {
			highRiskWeight += p.Weight
		}
	}
	if highRiskWeight > 60 {
		score -= 5
		reasons = append(reasons, fmt.Sprintf("高波动/进攻型持仓约 %.1f%%。", highRiskWeight))
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "仓位集中度和高风险暴露处于可控区间。")
	}
	return score, reasons
}

func scoreDiversification(result PortfolioHealthResult) (int, []string) {
	score := 20
	reasons := []string{}
	if len(result.PositionAnalysis) < 3 {
		score -= 6
		reasons = append(reasons, "持仓基金数量较少，分散基础不足。")
	}
	if len(result.FundTypeBreakdown) <= 1 {
		score -= 5
		reasons = append(reasons, "基金类型较单一。")
	}
	if len(result.IndustryBreakdown) > 0 && result.IndustryBreakdown[0].Weight > 55 {
		score -= 5
		reasons = append(reasons, fmt.Sprintf("第一大行业/主题 %s 暴露 %.1f%%，主题集中。", result.IndustryBreakdown[0].Name, result.IndustryBreakdown[0].Weight))
	}
	if result.Overlap.EstimatedDuplication > 20 {
		score -= 4
		reasons = append(reasons, fmt.Sprintf("可识别重仓股重复暴露约 %.1f%%。", result.Overlap.EstimatedDuplication))
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "基金类型、行业和重仓股层面的分散度较好。")
	}
	return score, reasons
}

func scoreHoldingDiscipline(result PortfolioHealthResult) (int, []string) {
	score := 5
	reasons := []string{}
	oversized := 0
	for _, p := range result.PositionAnalysis {
		if p.Weight > 40 {
			oversized++
		}
	}
	if oversized > 0 {
		score -= 2
		reasons = append(reasons, "存在超大单只仓位，需要明确持仓纪律。")
	}
	if result.Overview.TotalPnLRate < -20 {
		score -= 2
		reasons = append(reasons, "组合整体亏损较深，需要复盘加减仓纪律。")
	}
	if score < 0 {
		score = 0
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "当前仓位纪律没有明显异常。")
	}
	return score, reasons
}

func buildPortfolioFindings(result PortfolioHealthResult) []PortfolioFinding {
	findings := []PortfolioFinding{}
	if result.Overview.MaxPositionWeight > 35 {
		findings = append(findings, PortfolioFinding{Level: "warning", Title: "单只仓位偏高", Detail: fmt.Sprintf("最大单只基金仓位 %.1f%%。", result.Overview.MaxPositionWeight), Section: "concentration"})
	}
	if result.Overview.Top3PositionWeight > 70 {
		findings = append(findings, PortfolioFinding{Level: "warning", Title: "前三大仓位集中", Detail: fmt.Sprintf("前三大仓位合计 %.1f%%。", result.Overview.Top3PositionWeight), Section: "concentration"})
	}
	if len(result.IndustryBreakdown) > 0 && result.IndustryBreakdown[0].Weight > 50 {
		findings = append(findings, PortfolioFinding{Level: "warning", Title: "主题暴露集中", Detail: fmt.Sprintf("%s 暴露约 %.1f%%。", result.IndustryBreakdown[0].Name, result.IndustryBreakdown[0].Weight), Section: "industry"})
	}
	if result.DataQuality.Score < 8 {
		findings = append(findings, PortfolioFinding{Level: "info", Title: "数据完整性不足", Detail: strings.Join(result.DataQuality.Warnings, " "), Section: "data"})
	}
	if len(findings) == 0 {
		findings = append(findings, PortfolioFinding{Level: "success", Title: "没有明显结构性硬伤", Detail: "当前组合在集中度、分散度和数据完整性上没有触发严重预警。", Section: "summary"})
	}
	return findings
}

func buildPortfolioSuggestions(result PortfolioHealthResult) []string {
	suggestions := []string{}
	if result.Overview.MaxPositionWeight > 35 {
		suggestions = append(suggestions, "复核最大仓位基金是否承担核心仓角色；如果只是主题进攻仓，建议降低单点暴露。")
	}
	if len(result.AssetBreakdown) > 0 {
		equity := breakdownWeight(result.AssetBreakdown, "股票")
		bondCash := breakdownWeight(result.AssetBreakdown, "债券") + breakdownWeight(result.AssetBreakdown, "现金") + breakdownWeight(result.AssetBreakdown, "银行存款")
		if equity > 75 && bondCash < 10 {
			suggestions = append(suggestions, "组合偏进攻且防守仓不足，可考虑补充债券/现金类低波资产来降低回撤。")
		}
	}
	if len(result.IndustryBreakdown) > 0 && result.IndustryBreakdown[0].Weight > 50 {
		suggestions = append(suggestions, fmt.Sprintf("降低对 %s 的单一依赖，补充相关性较低的行业或资产。", result.IndustryBreakdown[0].Name))
	}
	if result.Overlap.EstimatedDuplication > 20 {
		suggestions = append(suggestions, "检查重复重仓股对应基金，保留风格更清晰、费率/回撤/长期表现更好的品种。")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前组合没有明显需要立刻调整的结构性问题，建议持续跟踪仓位和主题暴露变化。")
	}
	return suggestions
}

func buildPortfolioSummary(result PortfolioHealthResult) string {
	tags := strings.Join(result.Tags, " / ")
	if tags == "" {
		tags = "未分类"
	}
	return fmt.Sprintf("当前组合评分 %d（%s），属于%s组合；最大单只仓位 %.1f%%，前三大仓位 %.1f%%，今年以来加权收益 %.2f%%。",
		result.Score, result.Level, tags, result.Overview.MaxPositionWeight, result.Overview.Top3PositionWeight, result.Overview.WeightedReturnYTD)
}

func portfolioHealthPrompt(result PortfolioHealthResult) string {
	lines := []string{
		"你是一位专业的基金组合体检分析师。请只基于下列结构化体检结果解释现状，不要预测短期涨跌，不要直接给买卖指令。",
		"输出结构：组合定位、主要优点、主要问题、结构拆解、可执行观察项。",
		fmt.Sprintf("总分：%d，等级：%s，标签：%s。", result.Score, result.Level, strings.Join(result.Tags, "、")),
		fmt.Sprintf("总市值：%.2f，总盈亏：%.2f，收益率：%.2f%%。", result.Overview.TotalMarketValue, result.Overview.TotalPnL, result.Overview.TotalPnLRate),
		fmt.Sprintf("最大单只仓位：%.2f%%，前三大仓位：%.2f%%。", result.Overview.MaxPositionWeight, result.Overview.Top3PositionWeight),
		fmt.Sprintf("基金类型拆解：%s。", compactBreakdownText(result.FundTypeBreakdown, 5)),
		fmt.Sprintf("底层资产拆解：%s。", compactBreakdownText(result.AssetBreakdown, 5)),
		fmt.Sprintf("行业主题拆解：%s。", compactBreakdownText(result.IndustryBreakdown, 5)),
		fmt.Sprintf("主要发现：%s。", compactFindingsText(result.Findings)),
		fmt.Sprintf("建议项：%s。", strings.Join(result.Suggestions, "；")),
	}
	return strings.Join(lines, "\n")
}

func portfolioTags(result PortfolioHealthResult) []string {
	tags := []string{}
	equity := breakdownWeight(result.AssetBreakdown, "股票")
	if equity == 0 {
		equity = typeEquityWeight(result.FundTypeBreakdown)
	}
	if equity >= 75 {
		tags = append(tags, "高进攻")
	} else if equity >= 45 {
		tags = append(tags, "均衡偏股")
	} else {
		tags = append(tags, "偏防守")
	}
	if result.Overview.Top3PositionWeight > 70 {
		tags = append(tags, "持仓集中")
	} else {
		tags = append(tags, "分散适中")
	}
	if len(result.IndustryBreakdown) > 0 && result.IndustryBreakdown[0].Weight > 50 {
		tags = append(tags, "主题集中")
	}
	if result.DataQuality.Score < 8 {
		tags = append(tags, "数据待补齐")
	}
	return tags
}

func portfolioLevel(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B+"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	default:
		return "D"
	}
}

func inferPositionRole(p PortfolioPosition, weight float64) string {
	fundType := normalizeFundType(p.FundType)
	if strings.Contains(fundType, "债券") || strings.Contains(fundType, "货币") {
		return "防守仓"
	}
	if weight >= 25 && p.Return1Y >= 0 && p.ReturnThisYear >= 0 {
		return "核心仓"
	}
	if p.Return30D > 15 || p.ReturnThisYear > 40 || strings.Contains(fundType, "股票") {
		return "进攻仓"
	}
	if weight <= 8 {
		return "卫星仓"
	}
	if p.PnLRate < -15 && p.Return1Y < 0 {
		return "观察仓"
	}
	return "均衡仓"
}

func inferRiskLevel(p PortfolioPosition, weight float64) string {
	if weight > 35 || p.Return30D > 20 || p.Return30D < -15 || strings.Contains(normalizeFundType(p.FundType), "股票") {
		return "高"
	}
	if strings.Contains(normalizeFundType(p.FundType), "债券") || strings.Contains(normalizeFundType(p.FundType), "货币") {
		return "低"
	}
	return "中"
}

func positionProblemTags(p PortfolioPosition, weight float64) []string {
	tags := []string{}
	if weight > 35 {
		tags = append(tags, "仓位过高")
	}
	if p.Return30D < -10 {
		tags = append(tags, "短期承压")
	}
	if p.Return30D > 20 {
		tags = append(tags, "短期波动放大")
	}
	if p.PnLRate < -15 {
		tags = append(tags, "持仓亏损较深")
	}
	if len(p.AssetAllocation) == 0 && len(p.Industry) == 0 {
		tags = append(tags, "缺少穿透数据")
	}
	if len(tags) == 0 {
		tags = append(tags, "暂无明显异常")
	}
	return tags
}

func normalizeFundType(value string) string {
	text := strings.TrimSpace(value)
	switch {
	case text == "":
		return "未分类"
	case strings.Contains(text, "债"):
		return "债券型"
	case strings.Contains(text, "货币"):
		return "货币型"
	case strings.Contains(text, "QDII") || strings.Contains(strings.ToLower(text), "qdii"):
		return "QDII"
	case strings.Contains(text, "指数"):
		return "指数型"
	case strings.Contains(text, "股票"):
		return "股票型"
	case strings.Contains(text, "混合"):
		return "混合型"
	case strings.Contains(text, "FOF") || strings.Contains(strings.ToLower(text), "fof"):
		return "FOF"
	default:
		return text
	}
}

func normalizeAllocationName(name, kind string) string {
	name = strings.TrimSpace(name)
	if kind == "asset" {
		if strings.Contains(name, "股票") {
			return "股票"
		}
		if strings.Contains(name, "债") {
			return "债券"
		}
		if strings.Contains(name, "银行") || strings.Contains(name, "现金") {
			return "现金/银行存款"
		}
	}
	return name
}

func breakdownWeight(items []PortfolioBreakdownItem, name string) float64 {
	total := 0.0
	for _, item := range items {
		if strings.Contains(item.Name, name) {
			total += item.Weight
		}
	}
	return total
}

func typeEquityWeight(items []PortfolioBreakdownItem) float64 {
	total := 0.0
	for _, item := range items {
		if strings.Contains(item.Name, "股票") || strings.Contains(item.Name, "混合") || strings.Contains(item.Name, "指数") {
			total += item.Weight
		}
	}
	return total
}

func compactBreakdownText(items []PortfolioBreakdownItem, limit int) string {
	if len(items) == 0 {
		return "暂无数据"
	}
	parts := []string{}
	for i, item := range items {
		if i >= limit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.1f%%", item.Name, item.Weight))
	}
	return strings.Join(parts, "、")
}

func compactFindingsText(items []PortfolioFinding) string {
	parts := []string{}
	for _, item := range items {
		parts = append(parts, item.Title+"："+item.Detail)
	}
	return strings.Join(parts, "；")
}

func appendUnique(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

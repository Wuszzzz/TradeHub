package research

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"service": "fund-research"}})
}

func (s *Server) Summary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{
		"features": []string{"4433", "filter", "check", "similarity", "portfolio-health", "by-stock", "managers", "related-sector", "tag-recommend", "sync"},
		"source":   "eastmoney",
		"language": "go",
	}})
}

func (s *Server) Fund4433(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 500)
	enrich := intQuery(r, "enrich", 30)
	requireFiveYear := boolQuery(r, "require_five_year", true)
	universe, err := s.buildRankUniverse(r.Context(), limit)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	results := make([]Fund, 0)
	for _, fund := range universe {
		fund.Is4433 = Is4433(fund, requireFiveYear)
		if fund.Is4433 {
			results = append(results, fund)
		}
	}
	sortFunds(results, r.URL.Query().Get("sort"))
	results = s.enrichFunds(r.Context(), results, enrich)
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{
		"count": len(results),
		"items": results,
		"meta":  map[string]any{"universe_count": len(universe), "require_five_year": requireFiveYear},
	}})
}

func (s *Server) FundFilter(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 500)
	enrich := intQuery(r, "enrich", 80)
	params := filterParamsFromQuery(r)
	params.Limit = intQuery(r, "result_limit", 100)
	universe, err := s.buildRankUniverse(r.Context(), limit)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	enriched := s.enrichFunds(r.Context(), universe, enrich)
	results := ApplyFilter(enriched, params)
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{
		"count":  len(results),
		"items":  results,
		"params": params,
	}})
}

func (s *Server) FundCheck(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Codes []string `json:"codes"`
		Code  string   `json:"code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	codes := payload.Codes
	if payload.Code != "" {
		codes = append(codes, splitFields(payload.Code)...)
	}
	if len(codes) == 0 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("codes required"))
		return
	}
	params := defaultStrictParams()
	results := make([]Fund, 0, len(codes))
	for _, code := range codes {
		fund, err := s.em.FundInfo(r.Context(), strings.TrimSpace(code))
		if err != nil {
			results = append(results, Fund{Code: code, Diagnostics: []DiagnosticItem{{
				Key: "fetch", Label: "数据读取", Passed: false, Value: err.Error(), Expected: "东方财富基金详情可用", Level: "data",
			}}})
			continue
		}
		results = append(results, DiagnoseFund(fund, params))
	}
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(results), "items": results}})
}

func (s *Server) FundSimilarity(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Codes []string `json:"codes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if len(payload.Codes) < 2 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("at least two fund codes required"))
		return
	}
	funds := make([]Fund, 0, len(payload.Codes))
	for _, code := range payload.Codes {
		fund, err := s.em.FundInfo(r.Context(), code)
		if err == nil {
			funds = append(funds, fund)
		}
	}
	results := Similarity(funds)
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(results), "items": results}})
}

func (s *Server) PortfolioHealth(w http.ResponseWriter, r *http.Request) {
	var payload PortfolioHealthRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("invalid portfolio payload: %w", err))
		return
	}
	result := AnalyzePortfolioHealth(payload)
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: result})
}

func (s *Server) FundByStock(w http.ResponseWriter, r *http.Request) {
	keywords := splitFields(r.URL.Query().Get("keywords"))
	if len(keywords) == 0 {
		keywords = splitFields(r.URL.Query().Get("keyword"))
	}
	if len(keywords) == 0 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("keywords required"))
		return
	}
	countMap := map[string]int{}
	fundMap := map[string]HoldStockFund{}
	for _, keyword := range keywords {
		stockCode, stockName, err := s.em.SearchStock(r.Context(), keyword)
		if err != nil {
			continue
		}
		rows, err := s.em.FundByStock(r.Context(), stockCode, stockName)
		if err != nil {
			continue
		}
		for _, row := range rows {
			countMap[row.Code]++
			row.Count = countMap[row.Code]
			fundMap[row.Code] = row
		}
	}
	results := make([]HoldStockFund, 0)
	for code, count := range countMap {
		if count == len(keywords) {
			row := fundMap[code]
			row.Count = count
			results = append(results, row)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Count == results[j].Count {
			return results[i].Code < results[j].Code
		}
		return results[i].Count > results[j].Count
	})
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(results), "items": results}})
}

func (s *Server) Managers(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 200)
	minWorkingYears := floatQuery(r, "min_working_years", 8)
	minYield := floatQuery(r, "min_yieldse", 15)
	maxFundCount := intQuery(r, "max_current_fund_count", 10)
	minScale := floatQuery(r, "min_scale", 60)
	rows, err := s.em.FundManagers(r.Context(), limit)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	results := make([]ManagerResult, 0, len(rows))
	for _, row := range rows {
		if minWorkingYears > 0 && row.Manager.WorkingDays/365 < minWorkingYears {
			continue
		}
		if minYield > 0 && row.Manager.YearsAvgRepay < minYield {
			continue
		}
		if maxFundCount > 0 && row.Manager.CurrentFundNum > maxFundCount {
			continue
		}
		if minScale > 0 && row.Manager.ScaleYi < minScale {
			continue
		}
		results = append(results, row)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Manager.YearsAvgRepay > results[j].Manager.YearsAvgRepay })
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(results), "items": results}})
}

func (s *Server) RelatedSectors(w http.ResponseWriter, r *http.Request) {
	codes := splitFields(r.URL.Query().Get("codes"))
	if len(codes) == 0 {
		codes = splitFields(r.URL.Query().Get("code"))
	}
	if len(codes) == 0 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("codes required"))
		return
	}
	rows, err := s.relatedSectorMap(r.Context(), codes)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	if boolQuery(r, "quote", false) {
		secids := make([]string, 0, len(rows))
		for _, row := range rows {
			if row.SecID != "" {
				secids = append(secids, row.SecID)
			}
		}
		quotes, _ := s.em.SectorQuotes(r.Context(), secids)
		for code, row := range rows {
			if quote, ok := quotes[row.SecID]; ok {
				q := quote
				row.Quote = &q
				rows[code] = row
			}
		}
	}
	items := make([]RelatedSector, 0, len(rows))
	for _, row := range rows {
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].FundCode < items[j].FundCode })
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(items), "items": items}})
}

func (s *Server) SectorQuotes(w http.ResponseWriter, r *http.Request) {
	secids := splitFields(r.URL.Query().Get("secids"))
	if len(secids) == 0 {
		labels := splitFields(r.URL.Query().Get("sectors"))
		secidMap, err := s.sectorSecIDMap(r.Context(), labels)
		if err != nil {
			errorJSON(w, http.StatusBadGateway, err)
			return
		}
		for _, secid := range secidMap {
			secids = append(secids, secid)
		}
	}
	if len(secids) == 0 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("secids or sectors required"))
		return
	}
	quotes, err := s.em.SectorQuotes(r.Context(), secids)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	items := make([]SectorQuote, 0, len(quotes))
	for _, quote := range quotes {
		items = append(items, quote)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SecID < items[j].SecID })
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(items), "items": items}})
}

func (s *Server) RecommendTags(w http.ResponseWriter, r *http.Request) {
	codes := splitFields(r.URL.Query().Get("codes"))
	if len(codes) == 0 {
		codes = splitFields(r.URL.Query().Get("code"))
	}
	if len(codes) == 0 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("codes required"))
		return
	}
	items := make([]FundTag, 0)
	for _, code := range codes {
		tags, err := s.recommendTagsForFund(r.Context(), strings.TrimSpace(code))
		if err != nil {
			errorJSON(w, http.StatusBadGateway, err)
			return
		}
		items = append(items, tags...)
	}
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"count": len(items), "items": items}})
}

func (s *Server) SyncStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: s.metadataSyncStatus(r.Context())})
}

func (s *Server) SyncSectorMap(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Items []RelatedSector `json:"items"`
		Seed  bool            `json:"seed"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	rows := payload.Items
	if payload.Seed {
		for code, sector := range seedFundRelated {
			rows = append(rows, RelatedSector{FundCode: code, Sector: sector, SecID: seedSectorSecIDs[sector]})
		}
	}
	if len(rows) == 0 {
		errorJSON(w, http.StatusBadRequest, fmt.Errorf("items required"))
		return
	}
	count, err := s.syncSectorRows(r.Context(), rows)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{"synced": count}})
}

func (s *Server) SyncEvaluations(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Codes      []string `json:"codes"`
		Code       string   `json:"code"`
		Limit      int      `json:"limit"`
		WindowDays int      `json:"window_days"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	codes := payload.Codes
	if payload.Code != "" {
		codes = append(codes, splitFields(payload.Code)...)
	}
	if raw := r.URL.Query().Get("codes"); raw != "" {
		codes = append(codes, splitFields(raw)...)
	}
	if payload.Limit <= 0 {
		payload.Limit = intQuery(r, "limit", 500)
	}
	if payload.WindowDays <= 0 {
		payload.WindowDays = intQuery(r, "window_days", 370)
	}
	items, err := s.syncEvaluationSnapshots(r.Context(), payload.Limit, payload.WindowDays, codes)
	if err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: map[string]any{
		"synced":      len(items),
		"window_days": payload.WindowDays,
		"items":       items,
	}})
}

func filterParamsFromQuery(r *http.Request) FundFilterParams {
	params := defaultStrictParams()
	if raw := r.URL.Query().Get("types"); raw != "" {
		params.Types = splitFields(raw)
	}
	params.MinScale = floatQuery(r, "min_scale", params.MinScale)
	params.MaxScale = floatQuery(r, "max_scale", params.MaxScale)
	params.MinManagerYears = floatQuery(r, "min_manager_years", params.MinManagerYears)
	params.MinEstabYears = floatQuery(r, "min_estab_years", params.MinEstabYears)
	params.Year1RankRatio = floatQuery(r, "year_1_rank_ratio", params.Year1RankRatio)
	params.ThisYear235RankRatio = floatQuery(r, "this_year_235_rank_ratio", params.ThisYear235RankRatio)
	params.Month6RankRatio = floatQuery(r, "month_6_rank_ratio", params.Month6RankRatio)
	params.Month3RankRatio = floatQuery(r, "month_3_rank_ratio", params.Month3RankRatio)
	params.Max135AvgStddev = floatQuery(r, "max_135_avg_stddev", params.Max135AvgStddev)
	params.Min135AvgSharp = floatQuery(r, "min_135_avg_sharp", params.Min135AvgSharp)
	params.Max135AvgRetr = floatQuery(r, "max_135_avg_retr", params.Max135AvgRetr)
	params.RequireFiveYear = boolQuery(r, "require_five_year", params.RequireFiveYear)
	params.Sort = r.URL.Query().Get("sort")
	return params
}

func defaultStrictParams() FundFilterParams {
	return FundFilterParams{
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
		Sort:                 "month_3",
	}
}

func Similarity(funds []Fund) []SimilarityResult {
	results := make([]SimilarityResult, 0, len(funds))
	for i, fund := range funds {
		setA := stockNameSet(fund.Stocks)
		setB := map[string]struct{}{}
		for j, other := range funds {
			if i == j {
				continue
			}
			for name := range stockNameSet(other.Stocks) {
				setB[name] = struct{}{}
			}
		}
		same := make([]string, 0)
		union := map[string]struct{}{}
		for name := range setA {
			union[name] = struct{}{}
			if _, ok := setB[name]; ok {
				same = append(same, name)
			}
		}
		for name := range setB {
			union[name] = struct{}{}
		}
		value := 0.0
		if len(union) > 0 {
			value = float64(len(same)) / float64(len(union))
		}
		results = append(results, SimilarityResult{Fund: fund, SimilarityValue: value, SameStocks: same})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].SimilarityValue > results[j].SimilarityValue })
	return results
}

func stockNameSet(stocks []FundStock) map[string]struct{} {
	out := map[string]struct{}{}
	for _, stock := range stocks {
		name := strings.TrimSpace(stock.Name)
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

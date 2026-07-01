package research

import "time"

type APIResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

type Fund struct {
	Code                  string             `json:"code"`
	Name                  string             `json:"name"`
	Type                  string             `json:"type"`
	EstablishedDate       string             `json:"established_date,omitempty"`
	NetAssetsScale        float64            `json:"net_assets_scale"`
	NetAssetsScaleYi      float64            `json:"net_assets_scale_yi"`
	IndexCode             string             `json:"index_code,omitempty"`
	IndexName             string             `json:"index_name,omitempty"`
	Rate                  string             `json:"rate,omitempty"`
	FixedInvestmentStatus string             `json:"fixed_investment_status,omitempty"`
	Stddev                RiskMetrics        `json:"stddev"`
	MaxRetracement        RiskMetrics        `json:"max_retracement"`
	Sharp                 RiskMetrics        `json:"sharp"`
	Performance           PerformanceMetrics `json:"performance"`
	Stocks                []FundStock        `json:"stocks"`
	Manager               FundManager        `json:"manager"`
	AssetsProportion      AssetProportion    `json:"assets_proportion"`
	IndustryProportions   []IndustryPosition `json:"industry_proportions"`
	Diagnostics           []DiagnosticItem   `json:"diagnostics,omitempty"`
	Is4433                bool               `json:"is_4433"`
}

type RiskMetrics struct {
	Year1  float64 `json:"year_1"`
	Year3  float64 `json:"year_3"`
	Year5  float64 `json:"year_5"`
	Avg135 float64 `json:"avg_135"`
}

type PerformanceMetrics struct {
	Month3   PeriodPerformance `json:"month_3"`
	Month6   PeriodPerformance `json:"month_6"`
	Year1    PeriodPerformance `json:"year_1"`
	Year2    PeriodPerformance `json:"year_2"`
	Year3    PeriodPerformance `json:"year_3"`
	Year5    PeriodPerformance `json:"year_5"`
	ThisYear PeriodPerformance `json:"this_year"`
}

type PeriodPerformance struct {
	Growth    float64 `json:"growth"`
	Rank      int     `json:"rank"`
	Total     int     `json:"total"`
	RankRatio float64 `json:"rank_ratio"`
}

type FundStock struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Exchange    string  `json:"exchange,omitempty"`
	Industry    string  `json:"industry,omitempty"`
	HoldRatio   float64 `json:"hold_ratio"`
	AdjustRatio float64 `json:"adjust_ratio"`
}

type FundManager struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	WorkingDays    float64 `json:"working_days"`
	ManageDays     float64 `json:"manage_days"`
	ManageRepay    float64 `json:"manage_repay"`
	YearsAvgRepay  float64 `json:"years_avg_repay"`
	CurrentFundNum int     `json:"current_fund_count,omitempty"`
	ScaleYi        float64 `json:"scale_yi,omitempty"`
}

type AssetProportion struct {
	PubDate   string `json:"pub_date,omitempty"`
	Stock     string `json:"stock,omitempty"`
	Bond      string `json:"bond,omitempty"`
	Cash      string `json:"cash,omitempty"`
	Other     string `json:"other,omitempty"`
	NetAssets string `json:"net_assets,omitempty"`
}

type IndustryPosition struct {
	PubDate  string `json:"pub_date"`
	Industry string `json:"industry"`
	Prop     string `json:"prop"`
}

type DiagnosticItem struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Passed   bool   `json:"passed"`
	Value    string `json:"value"`
	Expected string `json:"expected"`
	Level    string `json:"level"`
}

type FundFilterParams struct {
	Types                []string
	MinScale             float64
	MaxScale             float64
	MinManagerYears      float64
	MinEstabYears        float64
	Year1RankRatio       float64
	ThisYear235RankRatio float64
	Month6RankRatio      float64
	Month3RankRatio      float64
	Max135AvgStddev      float64
	Min135AvgSharp       float64
	Max135AvgRetr        float64
	RequireFiveYear      bool
	Limit                int
	Sort                 string
}

type SimilarityResult struct {
	Fund            Fund     `json:"fund"`
	SimilarityValue float64  `json:"similarity_value"`
	SameStocks      []string `json:"same_stocks"`
}

type HoldStockFund struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Type      string  `json:"type,omitempty"`
	StockName string  `json:"stock_name,omitempty"`
	StockCode string  `json:"stock_code,omitempty"`
	HoldRatio float64 `json:"hold_ratio,omitempty"`
	Count     int     `json:"matched_stock_count,omitempty"`
}

type ManagerResult struct {
	Manager        FundManager `json:"manager"`
	Company        string      `json:"company,omitempty"`
	BestFundCode   string      `json:"best_fund_code,omitempty"`
	BestFundName   string      `json:"best_fund_name,omitempty"`
	BestFundIs4433 bool        `json:"best_fund_is_4433"`
}

type RelatedSector struct {
	FundCode string       `json:"fund_code"`
	Sector   string       `json:"sector"`
	SecID    string       `json:"secid,omitempty"`
	Quote    *SectorQuote `json:"quote,omitempty"`
	Source   string       `json:"source"`
}

type SectorQuote struct {
	SecID     string  `json:"secid"`
	Code      string  `json:"code,omitempty"`
	Name      string  `json:"name"`
	ChangePct float64 `json:"change_pct"`
}

type FundTag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Theme    string `json:"theme"`
	Reason   string `json:"reason,omitempty"`
	FundCode string `json:"fund_code,omitempty"`
}

type SyncStatus struct {
	SeedRelatedCount int      `json:"seed_related_count"`
	SeedSecIDCount   int      `json:"seed_secid_count"`
	DBAvailable      bool     `json:"db_available"`
	DBTables         []string `json:"db_tables"`
	Mode             string   `json:"mode"`
}

type LocalRank struct {
	Period   string
	Growth   float64
	Rank     int
	Total    int
	RankDate time.Time
}

type PortfolioHealthRequest struct {
	AccountID   string              `json:"account_id,omitempty"`
	AccountName string              `json:"account_name,omitempty"`
	Positions   []PortfolioPosition `json:"positions"`
}

type PortfolioPosition struct {
	FundCode        string                    `json:"fund_code"`
	FundName        string                    `json:"fund_name"`
	FundType        string                    `json:"fund_type,omitempty"`
	Company         string                    `json:"company,omitempty"`
	HoldingShare    float64                   `json:"holding_share"`
	HoldingCost     float64                   `json:"holding_cost"`
	HoldingNav      float64                   `json:"holding_nav"`
	LatestNav       float64                   `json:"latest_nav"`
	EstimateNav     float64                   `json:"estimate_nav,omitempty"`
	MarketValue     float64                   `json:"market_value"`
	PnL             float64                   `json:"pnl"`
	PnLRate         float64                   `json:"pnl_rate"`
	Return30D       float64                   `json:"return_30d,omitempty"`
	Return1M        float64                   `json:"return_1m,omitempty"`
	Return3M        float64                   `json:"return_3m,omitempty"`
	Return1Y        float64                   `json:"return_1y,omitempty"`
	ReturnThisYear  float64                   `json:"return_this_year,omitempty"`
	MaxDrawdown     float64                   `json:"max_drawdown,omitempty"`
	Volatility      float64                   `json:"volatility,omitempty"`
	AssetAllocation []PortfolioAllocationItem `json:"asset_allocation,omitempty"`
	Industry        []PortfolioAllocationItem `json:"industry,omitempty"`
	TopHoldings     []PortfolioHoldingItem    `json:"top_holdings,omitempty"`
	DataFlags       []string                  `json:"data_flags,omitempty"`
}

type PortfolioAllocationItem struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
}

type PortfolioHoldingItem struct {
	Code     string  `json:"code,omitempty"`
	Name     string  `json:"name"`
	Weight   float64 `json:"weight"`
	Industry string  `json:"industry,omitempty"`
}

type PortfolioHealthResult struct {
	Score             int                         `json:"score"`
	Level             string                      `json:"level"`
	Tags              []string                    `json:"tags"`
	Summary           string                      `json:"summary"`
	Overview          PortfolioOverview           `json:"overview"`
	Dimensions        []PortfolioDimension        `json:"dimensions"`
	FundTypeBreakdown []PortfolioBreakdownItem    `json:"fund_type_breakdown"`
	AssetBreakdown    []PortfolioBreakdownItem    `json:"asset_breakdown"`
	IndustryBreakdown []PortfolioBreakdownItem    `json:"industry_breakdown"`
	PositionAnalysis  []PortfolioPositionAnalysis `json:"position_analysis"`
	Overlap           PortfolioOverlap            `json:"overlap"`
	Findings          []PortfolioFinding          `json:"findings"`
	Suggestions        []string                    `json:"suggestions"`
	DataQuality        PortfolioDataQuality        `json:"data_quality"`
	AIPrompt           string                      `json:"ai_prompt"`
}

type PortfolioOverview struct {
	AccountID          string  `json:"account_id,omitempty"`
	AccountName        string  `json:"account_name,omitempty"`
	PositionCount      int     `json:"position_count"`
	TotalCost          float64 `json:"total_cost"`
	TotalMarketValue   float64 `json:"total_market_value"`
	TotalPnL           float64 `json:"total_pnl"`
	TotalPnLRate       float64 `json:"total_pnl_rate"`
	WeightedReturn30D  float64 `json:"weighted_return_30d"`
	WeightedReturn1M   float64 `json:"weighted_return_1m"`
	WeightedReturn3M   float64 `json:"weighted_return_3m"`
	WeightedReturn1Y   float64 `json:"weighted_return_1y"`
	WeightedReturnYTD  float64 `json:"weighted_return_ytd"`
	MaxPositionWeight  float64 `json:"max_position_weight"`
	Top3PositionWeight float64 `json:"top3_position_weight"`
}

type PortfolioDimension struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Score     int      `json:"score"`
	MaxScore  int      `json:"max_score"`
	Reasons   []string `json:"reasons"`
}

type PortfolioBreakdownItem struct {
	Name        string   `json:"name"`
	Weight      float64  `json:"weight"`
	MarketValue float64  `json:"market_value"`
	Count       int      `json:"count"`
	PnL         float64  `json:"pnl"`
	PnLRate     float64  `json:"pnl_rate"`
	Return30D   float64  `json:"return_30d,omitempty"`
	ReturnYTD   float64  `json:"return_ytd,omitempty"`
	FundCodes   []string `json:"fund_codes,omitempty"`
}

type PortfolioPositionAnalysis struct {
	FundCode      string   `json:"fund_code"`
	FundName      string   `json:"fund_name"`
	FundType      string   `json:"fund_type,omitempty"`
	Weight        float64  `json:"weight"`
	MarketValue   float64  `json:"market_value"`
	PnL           float64  `json:"pnl"`
	PnLRate       float64  `json:"pnl_rate"`
	Return30D     float64  `json:"return_30d,omitempty"`
	Return3M      float64  `json:"return_3m,omitempty"`
	Return1Y      float64  `json:"return_1y,omitempty"`
	ReturnYTD     float64  `json:"return_ytd,omitempty"`
	Role          string   `json:"role"`
	Contribution  float64  `json:"contribution"`
	RiskLevel     string   `json:"risk_level"`
	ProblemTags   []string `json:"problem_tags"`
	DataFlags     []string `json:"data_flags,omitempty"`
}

type PortfolioOverlap struct {
	MaxHoldingName       string                 `json:"max_holding_name,omitempty"`
	MaxHoldingExposure   float64                `json:"max_holding_exposure,omitempty"`
	RepeatedHoldings     []PortfolioOverlapItem `json:"repeated_holdings"`
	EstimatedDuplication float64                `json:"estimated_duplication"`
}

type PortfolioOverlapItem struct {
	Name      string   `json:"name"`
	Code      string   `json:"code,omitempty"`
	Exposure  float64  `json:"exposure"`
	FundCodes []string `json:"fund_codes"`
}

type PortfolioFinding struct {
	Level   string `json:"level"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	Metric  string `json:"metric,omitempty"`
	Section string `json:"section"`
}

type PortfolioDataQuality struct {
	Score                  int      `json:"score"`
	MissingNavCount        int      `json:"missing_nav_count"`
	MissingReturnCount     int      `json:"missing_return_count"`
	MissingAllocationCount int      `json:"missing_allocation_count"`
	MissingHoldingCount    int      `json:"missing_holding_count"`
	Warnings               []string `json:"warnings"`
}

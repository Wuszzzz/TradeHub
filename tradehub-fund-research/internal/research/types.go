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

type LocalRank struct {
	Period   string
	Growth   float64
	Rank     int
	Total    int
	RankDate time.Time
}

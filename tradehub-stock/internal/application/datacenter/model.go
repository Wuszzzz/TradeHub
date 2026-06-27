package datacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DataSource 数据源类型
type DataSource string

const (
	DataSourceEastMoney  DataSource = "eastmoney"
	DataSourceSina       DataSource = "sina"
	DataSourceTencent    DataSource = "tencent"
	DataSourceSohu       DataSource = "sohu"
)

// DailySpot A股每日行情快照 (兼容 ak.stock_zh_a_spot_em)
type DailySpot struct {
	Date                  string  `json:"date" db:"date"`                                     // 日期 YYYYMMDD
	Code                  string  `json:"code" db:"code"`                                     // 股票代码
	Name                  string  `json:"name" db:"name"`                                     // 股票名称
	LastPrice             float64 `json:"last_price" db:"last_price"`                         // 最新价
	ChangePercent         float64 `json:"change_percent" db:"change_percent"`                 // 涨跌幅 %
	ChangeAmount          float64 `json:"change_amount" db:"change_amount"`                    // 涨跌额
	Volume                int64   `json:"volume" db:"volume"`                                 // 成交量 (手)
	Turnover              float64 `json:"turnover" db:"turnover"`                             // 成交额 (元)
	Amplitude             float64 `json:"amplitude" db:"amplitude"`                            // 振幅 %
	High                  float64 `json:"high" db:"high"`                                      // 最高价
	Low                   float64 `json:"low" db:"low"`                                       // 最低价
	Open                  float64 `json:"open" db:"open"`                                     // 今开
	Closed                float64 `json:"closed" db:"closed"`                                 // 昨收
	VolumeRatio           float64 `json:"volume_ratio" db:"volume_ratio"`                      // 量比
	TurnoverRate          float64 `json:"turnover_rate" db:"turnover_rate"`                    // 换手率 %
	PERatio               float64 `json:"pe_ratio" db:"pe_ratio"`                             // 动态市盈率
	PBRatio               float64 `json:"pb_ratio" db:"pb_ratio"`                              // 市净率
	MarketCap             float64 `json:"market_cap" db:"market_cap"`                         // 总市值 (元)
	CirculatingMarketCap  float64 `json:"circulating_market_cap" db:"circulating_market_cap"` // 流通市值 (元)
	RiseSpeed             float64 `json:"rise_speed" db:"rise_speed"`                          // 涨速
	Change5Min            float64 `json:"change_5min" db:"change_5min"`                      // 5分钟涨跌 %
	Change60Day           float64 `json:"change_60day" db:"change_60day"`                     // 60日涨跌幅 (原字段拼写兼容)
	YTDChangePercent      float64 `json:"ytd_change_percent" db:"ytd_change_percent"`         // 年初至今涨跌幅 %
	IsST                  bool    `json:"is_st" db:"is_st"`                                   // 是否ST
	IsSuspended           bool    `json:"is_suspended" db:"is_suspended"`                     // 是否停牌
	Source                string  `json:"source" db:"source"`                                  // 数据源
	FetchedAt             time.Time `json:"fetched_at" db:"fetched_at"`                       // 采集时间
}

// LHBGG 龍虎榜个股上榜统计 (兼容 ak.stock_lhb_ggtj_sina)
type LHBGG struct {
	Date          string    `json:"date" db:"date"`                   // 日期 YYYYMMDD
	Code          string    `json:"code" db:"code"`                   // 股票代码
	Name          string    `json:"name" db:"name"`                   // 股票名称
	QuoteChange   float64   `json:"quote_change" db:"quote_change"`   // 涨跌幅 %
	ClosePrice    float64   `json:"close_price" db:"close_price"`     // 收盘价
	AveragePrice  float64   `json:"average_price" db:"average_price"` // 平均价
	RankingTimes  int       `json:"ranking_times" db:"ranking_times"` // 上榜次数
	SumBuy        float64   `json:"sum_buy" db:"sum_buy"`             // 累积购买额 (元)
	SumSell       float64   `json:"sum_sell" db:"sum_sell"`          // 累积卖出额 (元)
	NetAmount     float64   `json:"net_amount" db:"net_amount"`     // 净额 (元)
	BuySeat       int       `json:"buy_seat" db:"buy_seat"`          // 买入席位数
	SellSeat      int       `json:"sell_seat" db:"sell_seat"`        // 卖出席位数
	Reason        string    `json:"reason" db:"reason"`               // 上榜原因
	Source        string    `json:"source" db:"source"`               // 数据源
	FetchedAt     time.Time `json:"fetched_at" db:"fetched_at"`       // 采集时间
}

// DZJY 大宗交易每日统计 (兼容 ak.stock_dzjy_mrtj)
type DZJY struct {
	Date                string    `json:"date" db:"date"`                         // 日期 YYYYMMDD
	Code                string    `json:"code" db:"code"`                         // 股票代码
	Name                string    `json:"name" db:"name"`                         // 股票名称
	QuoteChange         float64   `json:"quote_change" db:"quote_change"`         // 涨跌幅 %
	ClosePrice          float64   `json:"close_price" db:"close_price"`           // 收盘价
	AveragePrice        float64   `json:"average_price" db:"average_price"`       // 成交均价
	OverflowRate        float64   `json:"overflow_rate" db:"overflow_rate"`       // 折溢率 %
	TradeNumber         int       `json:"trade_number" db:"trade_number"`          // 成交笔数
	SumVolume           int64     `json:"sum_volume" db:"sum_volume"`               // 成交总量 (手)
	SumTurnover         float64   `json:"sum_turnover" db:"sum_turnover"`         // 成交总额 (元)
	TurnoverMarketRate  float64   `json:"turnover_market_rate" db:"turnover_market_rate"` // 成交总额/流通市值 %
	Source              string    `json:"source" db:"source"`                       // 数据源
	FetchedAt           time.Time `json:"fetched_at" db:"fetched_at"`               // 采集时间
}

// CollectionTask 采集任务记录
type CollectionTask struct {
	TaskID      string     `json:"task_id" db:"task_id"`
	TaskType    string     `json:"task_type" db:"task_type"` // daily_spot | lhb | dzjy
	TargetDate  string     `json:"target_date" db:"target_date"`
	Status      string     `json:"status" db:"status"`       // pending | running | success | failed | skipped
	TotalCount  int        `json:"total_count" db:"total_count"`
	SuccessCount int       `json:"success_count" db:"success_count"`
	FailCount   int        `json:"fail_count" db:"fail_count"`
	ErrorMsg    string     `json:"error_msg" db:"error_msg"`
	StartedAt   *time.Time `json:"started_at" db:"started_at"`
	FinishedAt  *time.Time `json:"finished_at" db:"finished_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// CollectionLog 采集日志
type CollectionLog struct {
	LogID     string    `json:"log_id" db:"log_id"`
	TaskID    string    `json:"task_id" db:"task_id"`
	Level     string    `json:"level" db:"level"`         // info | warn | error
	Message   string    `json:"message" db:"message"`
	Detail    string    `json:"detail" db:"detail"`       // JSON 详情
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// DailySpotQuery 每日行情查询参数
type DailySpotQuery struct {
	Date       string  `json:"date"`        // YYYYMMDD
	Code       string  `json:"code"`        // 精确匹配
	Name       string  `json:"name"`        // 模糊匹配
	Market     string  `json:"market"`      // sh | sz | bj
	IsST       *bool   `json:"is_st"`       // 是否ST
	ChangeMin  float64 `json:"change_min"`  // 涨跌幅下限
	ChangeMax  float64 `json:"change_max"` // 涨跌幅上限
	TurnoverMin float64 `json:"turnover_min"` // 成交额下限
	SortField  string  `json:"sort_field"`  // 排序字段
	SortOrder  string  `json:"sort_order"`  // asc | desc
	Page       int     `json:"page"`        // 页码
	PageSize   int     `json:"page_size"`   // 每页数量
}

// DailySpotResult 每日行情查询结果
type DailySpotResult struct {
	Data       []DailySpot `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// LHBGGQuery 龙虎榜查询参数
type LHBGGQuery struct {
	Date      string   `json:"date"`
	DateStart string   `json:"date_start"`
	DateEnd   string   `json:"date_end"`
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	NetMin    float64  `json:"net_min"`
	NetMax    float64  `json:"net_max"`
	SortField string   `json:"sort_field"`
	SortOrder string   `json:"sort_order"`
	Page      int      `json:"page"`
	PageSize  int      `json:"page_size"`
}

// LHBGGResult 龙虎榜查询结果
type LHBGGResult struct {
	Data       []LHBGG `json:"data"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}

// DZJYQuery 大宗交易查询参数
type DZJYQuery struct {
	Date      string   `json:"date"`
	DateStart string   `json:"date_start"`
	DateEnd   string   `json:"date_end"`
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	OverflowMin float64 `json:"overflow_min"`
	OverflowMax float64 `json:"overflow_max"`
	SortField string   `json:"sort_field"`
	SortOrder string   `json:"sort_order"`
	Page      int      `json:"page"`
	PageSize  int      `json:"page_size"`
}

// DZJYResult 大宗交易查询结果
type DZJYResult struct {
	Data       []DZJY `json:"data"`
	Total      int64  `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}

// DataCollectionRequest 数据采集请求
type DataCollectionRequest struct {
	TaskType   string `json:"task_type"`  // daily_spot | lhb | dzjy
	TargetDate string `json:"target_date"` // YYYYMMDD
	Force      bool   `json:"force"`      // 是否强制重新采集
}

// DataCollectionResponse 数据采集响应
type DataCollectionResponse struct {
	TaskID     string    `json:"task_id"`
	Status     string    `json:"status"`
	TotalCount int       `json:"total_count"`
	Message    string    `json:"message"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Service     string    `json:"service"`
	Status      string    `json:"status"`
	LastCollect time.Time `json:"last_collect"`
	TodayCount  int       `json:"today_count"`
	ErrorCount  int       `json:"error_count"`
	Sources     []SourceStatus `json:"sources"`
}

// SourceStatus 数据源状态
type SourceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Latency string `json:"latency"`
}

// Service 数据中心服务接口
type Service interface {
	// 每日行情
	QueryDailySpot(ctx context.Context, q *DailySpotQuery) (*DailySpotResult, error)
	GetDailySpotByDate(ctx context.Context, date string) ([]DailySpot, error)

	// 龙虎榜
	QueryLHBGG(ctx context.Context, q *LHBGGQuery) (*LHBGGResult, error)
	GetLHBGGByDate(ctx context.Context, date string) ([]LHBGG, error)

	// 大宗交易
	QueryDZJY(ctx context.Context, q *DZJYQuery) (*DZJYResult, error)
	GetDZJYByDate(ctx context.Context, date string) ([]DZJY, error)

	// 采集任务
	Collect(ctx context.Context, req *DataCollectionRequest) (*DataCollectionResponse, error)
	GetCollectionTask(ctx context.Context, taskID string) (*CollectionTask, error)
	GetCollectionLogs(ctx context.Context, taskID string) ([]CollectionLog, error)
	ListCollectionTasks(ctx context.Context, taskType string, limit int) ([]CollectionTask, error)

	// 健康检查
	GetHealthStatus(ctx context.Context) (*HealthStatus, error)
}

// Config 服务配置
type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// 采集配置
	EastMoneyURL string
	SinaURL      string

	// 缓存配置
	CacheTTL time.Duration
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.DBHost == "" {
		return fmt.Errorf("DBHost is required")
	}
	if c.DBName == "" {
		return fmt.Errorf("DBName is required")
	}
	return nil
}

// ToJSON 转换为JSON字符串
func ToJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

package datacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EastMoneyClient 东方财富数据采集客户端
type EastMoneyClient struct {
	httpClient *http.Client
	baseURL    string
}

const (
	// EastMoneyAStockSpotURL A股实时行情
	EastMoneyAStockSpotURL = "https://push2.eastmoney.com/api/qt/clist/get"
	// EastMoneyLHBURL 龙虎榜
	EastMoneyLHBURL = "https://datacenter-web.eastmoney.com/api/data/v1/get"
	// EastMoneyDZJYURL 大宗交易
	EastMoneyDZJYURL = "https://datacenter-web.eastmoney.com/api/data/v1/get"
)

// NewEastMoneyClient 创建东方财富客户端
func NewEastMoneyClient(timeout time.Duration) *EastMoneyClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &EastMoneyClient{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     60 * time.Second,
			},
		},
		baseURL: "https://push2.eastmoney.com",
	}
}

// FetchAStockSpot 获取A股实时行情 (兼容 ak.stock_zh_a_spot_em)
func (c *EastMoneyClient) FetchAStockSpot(ctx context.Context) ([]DailySpot, error) {
	// 东方财富 A股实时行情接口
	reqURL := EastMoneyAStockSpotURL + "?" + url.Values{
		"pn":   {"1"},
		"pz":   {"5000"}, // 全市场约5000只
		"po":   {"1"},
		"np":   {"1"},
		"ut":   {"bd1d9ddb04089700cf9c27f6f7426281"},
		"fltt": {"2"},
		"invt": {"2"},
		"fid":  {"f3"},
		"fs":   {"m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048"}, // 沪深A股
		"fields": {"f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f12,f13,f14,f15,f16,f17,f18,f20,f21,f23,f24,f25,f22,f11,f62,f128,f136,f115,f152"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Data struct {
		 Diff []json.RawMessage `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	spots := make([]DailySpot, 0, len(result.Data.Diff))
	for _, raw := range result.Data.Diff {
		spot, err := c.parseAStockSpot(raw)
		if err != nil {
			continue // 跳过解析失败的记录
		}
		spots = append(spots, *spot)
	}

	return spots, nil
}

// FieldIndex 字段索引映射
// f1=?, f2=最新价, f3=涨跌幅, f4=涨跌额, f5=成交量, f6=成交额, f7=振幅, f8=换手率
// f9=市盈率(动态), f10=市净率, f12=代码, f14=名称
// f15=最高, f16=最低, f17=今开, f18=昨收
// f62=主力净流入, f128=60日涨跌幅, f136=年初至今涨跌幅
var fieldIndexRegex = regexp.MustCompile(`"f(\d+)":`)

// parseAStockSpot 解析单条行情记录
// 东方财富返回的是数组格式，按字段顺序对应
func (c *EastMoneyClient) parseAStockSpot(raw json.RawMessage) (*DailySpot, error) {
	var fields []interface{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		// 尝试对象格式
		var obj map[string]interface{}
		if err2 := json.Unmarshal(raw, &obj); err2 == nil {
			return c.parseAStockSpotObject(obj)
		}
		return nil, err
	}

	spot := &DailySpot{
		Source:    string(DataSourceEastMoney),
		FetchedAt: time.Now(),
	}

	// 根据字段顺序解析 (需要对照东方财富文档)
	// 通常返回: f2,f3,f4,f5,f6,f7,f8,f9,f10,f12,f13,f14,f15,f16,f17,f18,f19,f20...
	getFloat := func(idx int) float64 {
		if idx >= len(fields) {
			return 0
		}
		switch v := fields[idx].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
		return 0
	}
	getString := func(idx int) string {
		if idx >= len(fields) {
			return ""
		}
		if v, ok := fields[idx].(string); ok {
			return v
		}
		return fmt.Sprintf("%v", fields[idx])
	}
	getInt := func(idx int) int64 {
		if idx >= len(fields) {
			return 0
		}
		switch v := fields[idx].(type) {
		case float64:
			return int64(v)
		case int:
			return int64(v)
		case int64:
			return v
		case string:
			i, _ := strconv.ParseInt(v, 10, 64)
			return i
		}
		return 0
	}

	// 字段索引 (根据东方财富实际返回顺序)
	// f2=1, f3=2, f4=3, f5=4, f6=5, f7=6, f8=7, f9=8, f10=9, f12=10, f14=11, f15=12, f16=13, f17=14, f18=15
	spot.LastPrice = getFloat(1)    // f2
	spot.ChangePercent = getFloat(2) // f3
	spot.ChangeAmount = getFloat(3) // f4
	spot.Volume = getInt(4)        // f5 成交量(手)
	spot.Turnover = getFloat(5)    // f6 成交额(元)
	spot.Amplitude = getFloat(6)   // f7 振幅
	spot.TurnoverRate = getFloat(7) // f8 换手率
	spot.PERatio = getFloat(8)     // f9 市盈率
	spot.PBRatio = getFloat(9)     // f10 市净率
	spot.Code = getString(10)      // f12 代码
	spot.Name = getString(11)      // f14 名称
	spot.High = getFloat(12)       // f15 最高
	spot.Low = getFloat(13)        // f16 最低
	spot.Open = getFloat(14)      // f17 今开
	spot.Closed = getFloat(15)     // f18 昨收

	// 解析代码前缀判断市场
	spot.Code = strings.TrimPrefix(spot.Code, "1.") // 沪市
	spot.Code = strings.TrimPrefix(spot.Code, "0.") // 深市

	// 判断是否ST
	spot.IsST = strings.Contains(spot.Name, "ST") || strings.Contains(spot.Name, "*ST")

	// 判断是否停牌 (成交量为0)
	spot.IsSuspended = spot.Volume == 0

	// 涨速和5分钟涨跌需要额外计算或获取
	spot.RiseSpeed = 0
	spot.Change5Min = 0
	spot.Change60Day = 0
	spot.YTDChangePercent = 0

	// 市值相关
	spot.MarketCap = 0
	spot.CirculatingMarketCap = 0
	spot.VolumeRatio = 0

	return spot, nil
}

// parseAStockSpotObject 解析对象格式的行情记录
func (c *EastMoneyClient) parseAStockSpotObject(obj map[string]interface{}) (*DailySpot, error) {
	spot := &DailySpot{
		Source:    string(DataSourceEastMoney),
		FetchedAt: time.Now(),
	}

	// 东方财富字段映射
	getVal := func(key string) interface{} {
		if v, ok := obj[key]; ok {
			return v
		}
		return nil
	}
	getFloat := func(key string) float64 {
		switch v := getVal(key).(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
		return 0
	}
	getInt := func(key string) int64 {
		switch v := getVal(key).(type) {
		case float64:
			return int64(v)
		case int:
			return int64(v)
		case int64:
			return v
		case string:
			i, _ := strconv.ParseInt(v, 10, 64)
			return i
		}
		return 0
	}
	getString := func(key string) string {
		if v, ok := getVal(key).(string); ok {
			return v
		}
		return ""
	}

	// 标准字段
	spot.Code = getString("f12")
	spot.Name = getString("f14")
	spot.LastPrice = getFloat("f2")
	spot.ChangePercent = getFloat("f3")
	spot.ChangeAmount = getFloat("f4")
	spot.Volume = getInt("f5")
	spot.Turnover = getFloat("f6")
	spot.Amplitude = getFloat("f7")
	spot.TurnoverRate = getFloat("f8")
	spot.PERatio = getFloat("f9")
	spot.PBRatio = getFloat("f10")
	spot.High = getFloat("f15")
	spot.Low = getFloat("f16")
	spot.Open = getFloat("f17")
	spot.Closed = getFloat("f18")

	// 判断ST和停牌
	spot.IsST = strings.Contains(spot.Name, "ST") || strings.Contains(spot.Name, "*ST")
	spot.IsSuspended = spot.Volume == 0

	// 量比
	spot.VolumeRatio = getFloat("f10") // 注意：东方财富可能用其他字段

	return spot, nil
}

// FetchLHBGG 获取龙虎榜个股上榜统计 (兼容 ak.stock_lhb_ggtj_sina)
// symbol: 5=全部, 0=上海, 1=深圳, 2=主板, 3=创业板, 4=科创板
func (c *EastMoneyClient) FetchLHBGG(ctx context.Context, date string) ([]LHBGG, error) {
	// 东方财富龙虎榜接口
	// https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_DRAGON_LIST_DAILYSTATISTIC&columns=ALL&filter=(TRADE_DATE%3E%3D'2024-01-01')&pageNumber=1&pageSize=50&sortTypes=-1&sortColumns=TRADE_DATE&source=WEB&client=WEB
	reqURL := EastMoneyLHBURL + "?" + url.Values{
		"reportName": {"RPT_DRAGON_LIST_DAILYSTATISTIC"},
		"columns":    {"ALL"},
		"filter":     {fmt.Sprintf("(TRADE_DATE='%s')", date)},
		"pageNumber": {"1"},
		"pageSize":   {"500"},
		"sortTypes":  {"-1"},
		"sortColumns": {"TRADE_DATE"},
		"source":     {"WEB"},
		"client":     {"WEB"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Pages   int `json:"pages"`
			Data    []struct {
				TRADE_DATE        string  `json:"TRADE_DATE"`        // 交易日期
				SECURITY_CODE     string  `json:"SECURITY_CODE"`     // 证券代码
				SECURITY_NAME     string  `json:"SECURITY_NAME"`     // 证券名称
				CHANGE_RATE       float64 `json:"CHANGE_RATE"`       // 涨跌幅
				CLOSE_PRICE       float64 `json:"CLOSE_PRICE"`       // 收盘价
				AVG_PRICE         float64 `json:"AVG_PRICE"`         // 平均价
				SUM_ACCUMULATE_BUY  float64 `json:"SUM_ACCUMULATE_BUY"`  // 累计买入
				SUM_ACCUMULATE_SELL float64 `json:"SUM_ACCUMULATE_SELL"` // 累计卖出
				NET_AMOUNT        float64 `json:"NET_AMOUNT"`        // 净额
				BUY_SEAT_NUM      int     `json:"BUY_SEAT_NUM"`      // 买入席位
				SELL_SEAT_NUM     int     `json:"SELL_SEAT_NUM"`     // 卖出席位
				EXPLANATION       string  `json:"EXPLANATION"`       // 上榜原因
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	lhbList := make([]LHBGG, 0, len(result.Data.Data))
	for _, item := range result.Data.Data {
		lhb := LHBGG{
			Date:         date,
			Code:         item.SECURITY_CODE,
			Name:         item.SECURITY_NAME,
			QuoteChange:  item.CHANGE_RATE,
			ClosePrice:   item.CLOSE_PRICE,
			AveragePrice: item.AVG_PRICE,
			SumBuy:       item.SUM_ACCUMULATE_BUY * 10000, // 万元转元
			SumSell:      item.SUM_ACCUMULATE_SELL * 10000,
			NetAmount:    item.NET_AMOUNT * 10000,
			BuySeat:      item.BUY_SEAT_NUM,
			SellSeat:     item.SELL_SEAT_NUM,
			Reason:       item.EXPLANATION,
			Source:       string(DataSourceEastMoney),
			FetchedAt:    time.Now(),
		}
		lhbList = append(lhbList, lhb)
	}

	return lhbList, nil
}

// FetchDZJY 获取大宗交易每日统计 (兼容 ak.stock_dzjy_mrtj)
func (c *EastMoneyClient) FetchDZJY(ctx context.Context, dateStart, dateEnd string) ([]DZJY, error) {
	// 东方财富大宗交易接口
	// https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_BIGDEAL_DETAILS&columns=ALL&filter=(TRADE_DATE%3E%3D'2024-01-01')&pageNumber=1&pageSize=50
	reqURL := EastMoneyDZJYURL + "?" + url.Values{
		"reportName": {"RPT_BIGDEAL_DETAILS"},
		"columns":    {"ALL"},
		"filter":     {fmt.Sprintf("(TRADE_DATE>='%s')(TRADE_DATE<='%s')", dateStart, dateEnd)},
		"pageNumber": {"1"},
		"pageSize":   {"500"},
		"sortTypes":  {"-1"},
		"sortColumns": {"TRADE_DATE"},
		"source":     {"WEB"},
		"client":     {"WEB"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Pages int `json:"pages"`
			Data  []struct {
				TRADE_DATE       string  `json:"TRADE_DATE"`       // 交易日期
				SECURITY_CODE    string  `json:"SECURITY_CODE"`    // 证券代码
				SECURITY_NAME    string  `json:"SECURITY_NAME"`    // 证券名称
				CHANGE_RATE      float64 `json:"CHANGE_RATE"`     // 涨跌幅
				CLOSE_PRICE      float64 `json:"CLOSE_PRICE"`     // 收盘价
				DEAL_PRICE       float64 `json:"DEAL_PRICE"`       // 成交价格
				PREMIUM_DISCOUNT float64 `json:"PREMIUM_DISCOUNT"` // 溢价/折价率
				DEAL_NUMBER      int     `json:"DEAL_NUMBER"`     // 成交笔数
				DEAL_VOLUME      int64   `json:"DEAL_VOLUME"`     // 成交总量
				DEAL_AMOUNT      float64 `json:"DEAL_AMOUNT"`     // 成交总额
				FREE_MARKET_RATE float64 `json:"FREE_MARKET_RATE"` // 自由流通市值
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	dzjyList := make([]DZJY, 0, len(result.Data.Data))
	for _, item := range result.Data.Data {
		dzjy := DZJY{
			Date:         dateStart,
			Code:         item.SECURITY_CODE,
			Name:         item.SECURITY_NAME,
			QuoteChange:  item.CHANGE_RATE,
			ClosePrice:   item.CLOSE_PRICE,
			AveragePrice: item.DEAL_PRICE,
			OverflowRate: item.PREMIUM_DISCOUNT,
			TradeNumber:  item.DEAL_NUMBER,
			SumVolume:    item.DEAL_VOLUME,
			SumTurnover:  item.DEAL_AMOUNT * 10000, // 万元转元
			Source:       string(DataSourceEastMoney),
			FetchedAt:    time.Now(),
		}
		// 计算成交总额/流通市值
		if item.FREE_MARKET_RATE > 0 {
			dzjy.TurnoverMarketRate = (item.DEAL_AMOUNT * 10000) / (item.FREE_MARKET_RATE * 100000000) * 100
		}
		dzjyList = append(dzjyList, dzjy)
	}

	return dzjyList, nil
}

// Ping 检查东方财富接口可用性
func (c *EastMoneyClient) Ping(ctx context.Context) (latency time.Duration, err error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, EastMoneyAStockSpotURL+"?pn=1&pz=1&fs=m:0+t:6&fields=f2", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return time.Since(start), nil
}

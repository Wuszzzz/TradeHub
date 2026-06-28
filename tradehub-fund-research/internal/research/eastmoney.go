package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type EastMoneyClient struct {
	client *http.Client
}

func NewEastMoneyClient(timeout time.Duration) *EastMoneyClient {
	return &EastMoneyClient{client: &http.Client{Timeout: timeout}}
}

type rankRow struct {
	Code     string
	Name     string
	Date     string
	Growth   float64
	Rank     int
	Total    int
	Raw      string
	FundType string
}

var rankPeriods = map[string]struct {
	SortField  string
	FieldIndex int
}{
	"month_3":   {"3yzf", 9},
	"month_6":   {"6yzf", 10},
	"year_1":    {"1nzf", 11},
	"year_2":    {"2nzf", 12},
	"year_3":    {"3nzf", 13},
	"year_5":    {"5nzf", 13},
	"this_year": {"jnzf", 14},
}

func (c *EastMoneyClient) Ranking(ctx context.Context, period string, limit int) ([]rankRow, error) {
	cfg, ok := rankPeriods[period]
	if !ok {
		return nil, fmt.Errorf("unsupported period: %s", period)
	}
	if limit <= 0 {
		limit = 500
	}
	params := url.Values{
		"op": {"ph"}, "dt": {"kf"}, "ft": {"all"}, "rs": {""}, "gs": {"0"},
		"sc": {cfg.SortField}, "st": {"desc"}, "sd": {""}, "ed": {""},
		"qdii": {""}, "tabSubtype": {",,,,,"}, "pi": {"1"}, "pn": {fmt.Sprintf("%d", limit)},
		"dx": {"1"}, "v": {"0.1"},
	}
	body, err := c.getText(ctx, "https://fund.eastmoney.com/data/rankhandler.aspx?"+params.Encode(), "https://fund.eastmoney.com/data/fundranking.html")
	if err != nil {
		return nil, err
	}
	match := regexp.MustCompile(`var rankData = (.*);?$`).FindStringSubmatch(strings.TrimSpace(body))
	if len(match) < 2 {
		return nil, errors.New("eastmoney ranking payload not found")
	}
	payload := regexp.MustCompile(`(\w+):`).ReplaceAllString(strings.TrimSuffix(match[1], ";"), `"$1":`)
	var data struct {
		Datas      []string `json:"datas"`
		AllRecords int      `json:"allRecords"`
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, err
	}
	rows := make([]rankRow, 0, len(data.Datas))
	for index, raw := range data.Datas {
		parts := strings.Split(raw, ",")
		if len(parts) <= cfg.FieldIndex {
			continue
		}
		rows = append(rows, rankRow{
			Code:   parts[0],
			Name:   parts[1],
			Date:   parts[3],
			Growth: parseFloat(parts[cfg.FieldIndex]),
			Rank:   index + 1,
			Total:  data.AllRecords,
			Raw:    raw,
		})
	}
	return rows, nil
}

func (c *EastMoneyClient) FundInfo(ctx context.Context, code string) (Fund, error) {
	u := "https://j5.dfcfw.com/sc/tfs/qt/v2.0.1/" + url.PathEscape(code) + ".json"
	body, err := c.getText(ctx, u, "https://fundf10.eastmoney.com/")
	if err != nil {
		return Fund{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return Fund{}, err
	}
	fund := Fund{Code: code}
	fillFundBasics(&fund, payload)
	fillFundPerformance(&fund, payload)
	fillFundRisk(&fund, payload)
	fillFundHoldings(&fund, payload)
	fillFundManager(&fund, payload)
	fillFundAllocations(&fund, payload)
	fund.Is4433 = Is4433(fund, true)
	return fund, nil
}

func fillFundBasics(fund *Fund, payload map[string]any) {
	jjxq := mapAt(payload, "JJXQ", "Datas")
	fund.Code = stringAt(jjxq, "FCODE", fund.Code)
	fund.Name = stringAt(jjxq, "SHORTNAME", fund.Name)
	fund.Type = stringAt(jjxq, "FTYPE", fund.Type)
	fund.EstablishedDate = stringAt(jjxq, "ESTABDATE", fund.EstablishedDate)
	fund.IndexCode = stringAt(jjxq, "INDEXCODE", "")
	fund.IndexName = stringAt(jjxq, "INDEXNAME", "")
	fund.Rate = stringAt(jjxq, "RATE", "")
	if stringAt(jjxq, "DTZT", "") == "1" {
		fund.FixedInvestmentStatus = "可定投"
	}
	jjgm := sliceAt(mapAt(payload, "JJGM"), "Datas")
	if len(jjgm) > 0 {
		first, _ := jjgm[0].(map[string]any)
		fund.NetAssetsScale = parseFloat(first["NETNAV"])
		fund.NetAssetsScaleYi = fund.NetAssetsScale / 100000000
	}
}

func fillFundPerformance(fund *Fund, payload map[string]any) {
	for _, raw := range sliceAt(mapAt(payload, "JDZF"), "Datas") {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rank := parseInt(valueAt(item, "RANK"))
		total := parseInt(valueAt(item, "SC"))
		perf := PeriodPerformance{
			Growth:    parseFloat(valueAt(item, "SYL")),
			Rank:      rank,
			Total:     total,
			RankRatio: rankRatio(rank, total),
		}
		switch stringAt(item, "TITLE", "") {
		case "3Y":
			fund.Performance.Month3 = perf
		case "6Y":
			fund.Performance.Month6 = perf
		case "1N":
			fund.Performance.Year1 = perf
		case "2N":
			fund.Performance.Year2 = perf
		case "3N":
			fund.Performance.Year3 = perf
		case "5N":
			fund.Performance.Year5 = perf
		case "JN":
			fund.Performance.ThisYear = perf
		}
	}
}

func fillFundRisk(fund *Fund, payload map[string]any) {
	datas := mapAt(payload, "TSSJ", "Datas")
	fund.Stddev.Year1 = parseFloat(valueAt(datas, "STDDEV1"))
	fund.Stddev.Year3 = parseFloat(valueAt(datas, "STDDEV3"))
	fund.Stddev.Year5 = parseFloat(valueAt(datas, "STDDEV5"))
	fund.Stddev.Avg135 = avgNonZero(fund.Stddev.Year1, fund.Stddev.Year3, fund.Stddev.Year5)
	fund.MaxRetracement.Year1 = parseFloat(valueAt(datas, "MAXRETRA1"))
	fund.MaxRetracement.Year3 = parseFloat(valueAt(datas, "MAXRETRA3"))
	fund.MaxRetracement.Year5 = parseFloat(valueAt(datas, "MAXRETRA5"))
	fund.MaxRetracement.Avg135 = avgNonZero(fund.MaxRetracement.Year1, fund.MaxRetracement.Year3, fund.MaxRetracement.Year5)
	fund.Sharp.Year1 = parseFloat(valueAt(datas, "SHARP1"))
	fund.Sharp.Year3 = parseFloat(valueAt(datas, "SHARP3"))
	fund.Sharp.Year5 = parseFloat(valueAt(datas, "SHARP5"))
	fund.Sharp.Avg135 = avgNonZero(fund.Sharp.Year1, fund.Sharp.Year3, fund.Sharp.Year5)
}

func fillFundHoldings(fund *Fund, payload map[string]any) {
	position := mapAt(payload, "JJCC", "Datas", "InverstPosition")
	for _, raw := range sliceAt(position, "fundStocks") {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		fund.Stocks = append(fund.Stocks, FundStock{
			Code:        stringAt(item, "GPDM", ""),
			Name:        stringAt(item, "GPJC", ""),
			Exchange:    stringAt(item, "NEWTEXCH", ""),
			Industry:    stringAt(item, "INDEXNAME", ""),
			HoldRatio:   parseFloat(item["JZBL"]),
			AdjustRatio: parseFloat(item["PCTNVCHG"]),
		})
	}
	sort.Slice(fund.Stocks, func(i, j int) bool { return fund.Stocks[i].HoldRatio > fund.Stocks[j].HoldRatio })
}

func fillFundManager(fund *Fund, payload map[string]any) {
	rows := sliceAt(mapAt(payload, "JJJLNEW"), "Datas")
	if len(rows) == 0 {
		return
	}
	first, _ := rows[0].(map[string]any)
	managers := sliceAt(first, "MANGER")
	if len(managers) == 0 {
		managers = sliceAt(first, "manger")
	}
	if len(managers) == 0 {
		return
	}
	m, _ := managers[0].(map[string]any)
	fund.Manager = FundManager{
		ID:            stringAt(m, "MGRID", ""),
		Name:          stringAt(m, "MGRNAME", ""),
		WorkingDays:   parseFloat(m["TOTALDAYS"]),
		ManageDays:    parseFloat(m["DAYS"]),
		ManageRepay:   parseFloat(m["PENAVGROWTH"]),
		YearsAvgRepay: parseFloat(m["YIELDSE"]),
	}
}

func fillFundAllocations(fund *Fund, payload map[string]any) {
	jjcc := mapAt(payload, "JJCC", "Datas")
	for _, raw := range sliceAt(jjcc, "AssetAllocation") {
		row, ok := raw.([]any)
		if !ok || len(row) == 0 {
			continue
		}
		item, _ := row[0].(map[string]any)
		fund.AssetsProportion = AssetProportion{
			PubDate:   stringAt(item, "FSRQ", ""),
			Stock:     stringAt(item, "GP", "") + "%",
			Bond:      stringAt(item, "ZQ", "") + "%",
			Cash:      stringAt(item, "HB", "") + "%",
			Other:     stringAt(item, "QT", "") + "%",
			NetAssets: stringAt(item, "JZC", "") + "亿",
		}
	}
	sector := mapAt(jjcc, "SectorAllocation")
	for dateKey, raw := range sector {
		rows, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, itemRaw := range rows {
			item, ok := itemRaw.(map[string]any)
			if !ok {
				continue
			}
			prop := stringAt(item, "ZJZBL", "")
			if prop == "" || prop == "0" || prop == "--" {
				continue
			}
			fund.IndustryProportions = append(fund.IndustryProportions, IndustryPosition{
				PubDate:  dateKey,
				Industry: stringAt(item, "HYMC", ""),
				Prop:     prop,
			})
		}
	}
}

func (c *EastMoneyClient) SearchStock(ctx context.Context, keyword string) (string, string, error) {
	u := "https://searchapi.eastmoney.com/api/suggest/get?input=" + url.QueryEscape(keyword) + "&type=14&token=D43BF722C8E33BDC906FB84D85E326E8"
	body, err := c.getText(ctx, u, "https://www.eastmoney.com/")
	if err != nil {
		return "", "", err
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return "", "", err
	}
	items := sliceAt(mapAt(payload, "QuotationCodeTable"), "Data")
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		code := stringAt(item, "Code", "")
		name := stringAt(item, "Name", "")
		if code != "" && name != "" {
			return code, name, nil
		}
	}
	return "", "", fmt.Errorf("stock not found: %s", keyword)
}

func (c *EastMoneyClient) FundByStock(ctx context.Context, stockCode, stockName string) ([]HoldStockFund, error) {
	params := url.Values{
		"code": {stockCode},
		"name": {stockName},
		"pi":   {"1"},
		"pn":   {"200"},
	}
	body, err := c.getText(ctx, "https://fundf10.eastmoney.com/FundArchivesDatas.aspx?type=jjccgp&"+params.Encode(), "https://fundf10.eastmoney.com/")
	if err != nil {
		return nil, err
	}
	return parseHoldStockFunds(body, stockCode, stockName), nil
}

func parseHoldStockFunds(body, stockCode, stockName string) []HoldStockFund {
	matches := regexp.MustCompile(`\["([^"]+)"\s*,\s*"([^"]+)"\s*,\s*"([^"]*)"[^\]]*\]`).FindAllStringSubmatch(body, -1)
	results := make([]HoldStockFund, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		code := match[1]
		if _, ok := seen[code]; ok || len(code) != 6 {
			continue
		}
		seen[code] = struct{}{}
		results = append(results, HoldStockFund{
			Code:      code,
			Name:      match[2],
			Type:      match[3],
			StockCode: stockCode,
			StockName: stockName,
		})
	}
	return results
}

func (c *EastMoneyClient) FundManagers(ctx context.Context, limit int) ([]ManagerResult, error) {
	if limit <= 0 {
		limit = 200
	}
	params := url.Values{
		"dt": {"14"}, "mc": {"returnjson"}, "ft": {"all"}, "pn": {fmt.Sprintf("%d", limit)}, "pi": {"1"},
		"sc": {"penavgrowth"}, "st": {"desc"},
	}
	body, err := c.getText(ctx, "https://fund.eastmoney.com/Data/FundDataPortfolio_Interface.aspx?"+params.Encode(), "https://fund.eastmoney.com/manager/")
	if err != nil {
		return nil, err
	}
	match := regexp.MustCompile(`returnjson\s*=\s*(\{.*\})`).FindStringSubmatch(body)
	if len(match) < 2 {
		return nil, errors.New("fund managers payload not found")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
		return nil, err
	}
	rows := sliceAt(payload, "data")
	results := make([]ManagerResult, 0, len(rows))
	for _, raw := range rows {
		arr, ok := raw.([]any)
		if !ok || len(arr) < 8 {
			continue
		}
		results = append(results, ManagerResult{
			Manager: FundManager{
				ID:             fmt.Sprint(arr[0]),
				Name:           fmt.Sprint(arr[1]),
				WorkingDays:    parseFloat(arr[3]) * 365,
				YearsAvgRepay:  parseFloat(arr[4]),
				ScaleYi:        parseFloat(arr[5]),
				CurrentFundNum: parseInt(arr[6]),
			},
			Company: fmt.Sprint(arr[2]),
		})
	}
	return results, nil
}

func (c *EastMoneyClient) getText(ctx context.Context, rawURL, referer string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", referer)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("eastmoney status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func mapAt(root map[string]any, path ...string) map[string]any {
	current := root
	for _, key := range path {
		value, ok := valueAt(current, key).(map[string]any)
		if !ok {
			return map[string]any{}
		}
		current = value
	}
	return current
}

func sliceAt(root map[string]any, key string) []any {
	if root == nil {
		return nil
	}
	value := valueAt(root, key)
	if value == nil {
		return nil
	}
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	return rows
}

func stringAt(root map[string]any, key, fallback string) string {
	if root == nil {
		return fallback
	}
	value := valueAt(root, key)
	if value == nil {
		return fallback
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func valueAt(root map[string]any, key string) any {
	if root == nil {
		return nil
	}
	if value, ok := root[key]; ok {
		return value
	}
	lower := strings.ToLower(key)
	for k, value := range root {
		if strings.ToLower(k) == lower {
			return value
		}
	}
	return nil
}

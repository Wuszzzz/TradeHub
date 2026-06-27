package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const fundAssetURL = "https://zxg.txfund.com/ifzqgtimg/appstock/fund/baseInfo/asset?code=%s"

type FundAsset struct {
	Name  string `json:"name"`
	Ratio string `json:"ratio"`
}

type FundHolding struct {
	Name     string `json:"name"`
	Ratio    string `json:"ratio"`
	Code     string `json:"code"`
	JumpCode string `json:"jump_code,omitempty"`
	Rate     string `json:"rate,omitempty"`
	Change   any    `json:"change,omitempty"`
}

type FundAssetData struct {
	Selector   []string      `json:"selector"`
	Selected   int           `json:"selected"`
	ReportTime string        `json:"report_time"`
	TotalMoney string        `json:"total_money"`
	Asset      []FundAsset   `json:"asset"`
	Industry   []FundAsset   `json:"industry"`
	Stock      []FundHolding `json:"stock"`
	TotalStock string        `json:"total_stock"`
	Product    []FundHolding `json:"product"`
	TotalProd  string        `json:"total_product"`
}

type fundAssetEnvelope struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data FundAssetData `json:"data"`
}

type FundAssetResult struct {
	Dataset    string        `json:"dataset"`
	Source     string        `json:"source"`
	Symbol     string        `json:"symbol"`
	ReportTime string        `json:"report_time"`
	TotalMoney string        `json:"total_money"`
	Asset      []FundAsset   `json:"asset"`
	Industry   []FundAsset   `json:"industry"`
	Stock      []FundHolding `json:"stock"`
	TotalStock string        `json:"total_stock"`
	Product    []FundHolding `json:"product"`
	TotalProd  string        `json:"total_product"`
}

func NormalizeFundSymbol(input string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return "", fmt.Errorf("empty symbol")
	}
	if strings.HasPrefix(s, "sh") || strings.HasPrefix(s, "sz") {
		return s, nil
	}
	if len(s) == 6 {
		if strings.HasPrefix(s, "5") || strings.HasPrefix(s, "6") || strings.HasPrefix(s, "9") {
			return "sh" + s, nil
		}
		return "sz" + s, nil
	}
	return "", fmt.Errorf("invalid fund symbol: %s", input)
}

func (c *Client) FundAsset(ctx context.Context, symbol string) (FundAssetResult, error) {
	n, err := NormalizeFundSymbol(symbol)
	if err != nil {
		return FundAssetResult{}, err
	}
	text, err := c.get(ctx, fmt.Sprintf(fundAssetURL, n), false)
	if err != nil {
		return FundAssetResult{}, err
	}
	var env fundAssetEnvelope
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		return FundAssetResult{}, err
	}
	if env.Code != 0 {
		return FundAssetResult{}, fmt.Errorf("tencent fund asset: %s", env.Msg)
	}
	return FundAssetResult{
		Dataset:    "fund_asset",
		Source:     "tencent",
		Symbol:     n,
		ReportTime: env.Data.ReportTime,
		TotalMoney: env.Data.TotalMoney,
		Asset:      env.Data.Asset,
		Industry:   env.Data.Industry,
		Stock:      env.Data.Stock,
		TotalStock: env.Data.TotalStock,
		Product:    env.Data.Product,
		TotalProd:  env.Data.TotalProd,
	}, nil
}

// Package symbol provides cross-vendor security identifier normalization.
//
// Inputs accepted by Resolve:
//
//	"300308"         pure 6-digit code, market inferred by prefix
//	"sz300308"       tencent / sina style
//	"sh513310"       tencent / sina style
//	"bj430047"       tencent / sina style
//	"0.300308"       eastmoney secid (market_id.code)
//	"1.513310"       eastmoney secid
//	"cn_300308"      sohu / sina-like prefix
//	"zs_000001"      sohu index prefix
package symbol

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Spec holds a single security identified across vendors.
type Spec struct {
	// Code is the 6-digit numeric code.
	Code string
	// Market is one of "sh", "sz", "bj". Indexes are normalized to "sh"/"sz".
	Market string
	// IsIndex reports whether the raw input identified an index (e.g. zs_/sh000001).
	IsIndex bool
}

func (s Spec) Empty() bool { return s.Code == "" }

// Tencent returns "sh513310"/"sz300308"/"bj430047".
func (s Spec) Tencent() string { return s.Market + s.Code }

// Eastmoney returns "1.513310"/"0.300308". BJ falls back to "0.".
func (s Spec) Eastmoney() string {
	switch s.Market {
	case "sh":
		return "1." + s.Code
	default:
		return "0." + s.Code
	}
}

// Sohu returns "cn_xxxxxx" for stocks/ETFs and "zs_xxxxxx" for indexes.
func (s Spec) Sohu() string {
	if s.IsIndex {
		return "zs_" + s.Code
	}
	return "cn_" + s.Code
}

// Xueqiu returns "SH600519"/"SZ300308"/"BJ430047".
func (s Spec) Xueqiu() string {
	return strings.ToUpper(s.Market) + s.Code
}

// SohuLast3 returns the 3-character bucket sohu uses to shard hq URLs:
// e.g. 688008 -> "008", 300308 -> "308", 600519 -> "519".
func (s Spec) SohuLast3() string {
	if len(s.Code) < 3 {
		return s.Code
	}
	return s.Code[len(s.Code)-3:]
}

var (
	rePureCode = regexp.MustCompile(`^\d{6}$`)
	rePrefixed = regexp.MustCompile(`^(sh|sz|bj)(\d{6})$`)
	reEastmny  = regexp.MustCompile(`^([0-9]+)\.(\d{6})$`)
	reSohu     = regexp.MustCompile(`^(cn|zs)_(\d{6})$`)
)

// Resolve parses any of the supported input forms.
func Resolve(input string) (Spec, error) {
	raw := strings.ToLower(strings.TrimSpace(input))
	if raw == "" {
		return Spec{}, errors.New("empty symbol")
	}
	switch {
	case rePureCode.MatchString(raw):
		return Spec{Code: raw, Market: inferMarket(raw)}, nil
	case rePrefixed.MatchString(raw):
		m := rePrefixed.FindStringSubmatch(raw)
		return Spec{Code: m[2], Market: m[1]}, nil
	case reEastmny.MatchString(raw):
		m := reEastmny.FindStringSubmatch(raw)
		market := "sz"
		if m[1] == "1" {
			market = "sh"
		}
		return Spec{Code: m[2], Market: market}, nil
	case reSohu.MatchString(raw):
		m := reSohu.FindStringSubmatch(raw)
		isIdx := m[1] == "zs"
		market := inferMarket(m[2])
		return Spec{Code: m[2], Market: market, IsIndex: isIdx}, nil
	default:
		return Spec{}, fmt.Errorf("invalid symbol: %s", input)
	}
}

// inferMarket assigns market by first digit per Chinese A-share conventions.
// 5/6/9 -> sh, 8/4 -> bj, otherwise sz.
func inferMarket(code string) string {
	if len(code) == 0 {
		return "sz"
	}
	switch code[0] {
	case '5', '6', '9':
		return "sh"
	case '4', '8':
		return "bj"
	default:
		return "sz"
	}
}

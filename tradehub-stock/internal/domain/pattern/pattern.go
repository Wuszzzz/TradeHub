package pattern

import (
	"math"
	"sort"
	"strings"
	"time"

	"stock-etf-monitor/backend/internal/domain/indicator"
)

const AlgorithmVersion = "go-candlestick-v1"

type Hit struct {
	TS          time.Time
	PatternCode string
	Value       int
	Direction   string
	Extra       map[string]any
}

type candle struct {
	body       float64
	rangeSize  float64
	upper      float64
	lower      float64
	bodyRatio  float64
	upperRatio float64
	lowerRatio float64
	bullish    bool
	bearish    bool
}

var allCodes = []string{
	"tow_crows", "upside_gap_two_crows", "three_black_crows", "identical_three_crows", "three_line_strike",
	"dark_cloud_cover", "evening_doji_star", "doji_Star", "hanging_man", "hikkake_pattern",
	"modified_hikkake_pattern", "in_neck_pattern", "on_neck_pattern", "thrusting_pattern", "shooting_star",
	"stalled_pattern", "advance_block", "high_wave_candle", "engulfing_pattern", "abandoned_baby",
	"closing_marubozu", "doji", "up_down_gap", "long_legged_doji", "rickshaw_man", "marubozu",
	"three_inside_up_down", "three_outside_up_down", "three_stars_in_the_south", "three_white_soldiers",
	"belt_hold", "breakaway", "concealing_baby_swallow", "counterattack", "dragonfly_doji", "evening_star",
	"gravestone_doji", "hammer", "harami_pattern", "harami_cross_pattern", "homing_pigeon", "inverted_hammer",
	"kicking", "kicking_bull_bear", "ladder_bottom", "long_line_candle", "matching_low", "mat_hold",
	"morning_doji_star", "morning_star", "piercing_pattern", "rising_falling_three", "separating_lines",
	"short_line_candle", "spinning_top", "stick_sandwich", "takuri", "tasuki_gap", "tristar_pattern",
	"unique_3_river", "upside_downside_gap",
}

func AllCodes() []string {
	items := append([]string{}, allCodes...)
	sort.Strings(items)
	return items
}

func Scan(codes []string, bars []indicator.Bar) ([]Hit, error) {
	if len(bars) == 0 {
		return nil, nil
	}
	codes = normalizeCodes(codes)
	hits := make([]Hit, 0)
	for i := range bars {
		for _, code := range codes {
			value := evaluate(code, bars, i)
			if value == 0 {
				continue
			}
			hits = append(hits, Hit{
				TS:          bars[i].TS,
				PatternCode: code,
				Value:       value,
				Direction:   direction(value),
				Extra: map[string]any{
					"algorithm_version": AlgorithmVersion,
					"index":             i,
					"close":             bars[i].Close,
				},
			})
		}
	}
	return hits, nil
}

func normalizeCodes(codes []string) []string {
	if len(codes) == 0 {
		return AllCodes()
	}
	allowed := map[string]struct{}{}
	for _, code := range allCodes {
		allowed[code] = struct{}{}
	}
	out := make([]string, 0, len(codes))
	seen := map[string]struct{}{}
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := allowed[code]; !ok {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	if len(out) == 0 {
		return AllCodes()
	}
	sort.Strings(out)
	return out
}

func evaluate(code string, bars []indicator.Bar, i int) int {
	cur := analyze(bars[i])
	switch code {
	case "doji", "doji_Star":
		if isDoji(cur) {
			return 100
		}
	case "long_legged_doji", "rickshaw_man":
		if isDoji(cur) && cur.upperRatio > 0.25 && cur.lowerRatio > 0.25 {
			return 100
		}
	case "dragonfly_doji":
		if isDoji(cur) && cur.lowerRatio > 0.6 && cur.upperRatio < 0.12 {
			return 100
		}
	case "gravestone_doji":
		if isDoji(cur) && cur.upperRatio > 0.6 && cur.lowerRatio < 0.12 {
			return -100
		}
	case "hammer", "takuri":
		if cur.lowerRatio > 0.55 && cur.upperRatio < 0.15 && cur.bodyRatio < 0.35 {
			return 100
		}
	case "hanging_man":
		if i >= 3 && uptrend(bars, i-3, i-1) && cur.lowerRatio > 0.55 && cur.upperRatio < 0.15 {
			return -100
		}
	case "inverted_hammer":
		if cur.upperRatio > 0.55 && cur.lowerRatio < 0.15 && cur.bodyRatio < 0.35 {
			return 100
		}
	case "shooting_star":
		if i >= 3 && uptrend(bars, i-3, i-1) && cur.upperRatio > 0.55 && cur.lowerRatio < 0.15 {
			return -100
		}
	case "spinning_top":
		if cur.bodyRatio < 0.3 && cur.upperRatio > 0.2 && cur.lowerRatio > 0.2 {
			return signedByCandle(cur)
		}
	case "high_wave_candle":
		if cur.bodyRatio < 0.25 && cur.upperRatio > 0.3 && cur.lowerRatio > 0.3 {
			return signedByCandle(cur)
		}
	case "marubozu", "closing_marubozu":
		if cur.bodyRatio > 0.85 && cur.upperRatio < 0.08 && cur.lowerRatio < 0.08 {
			return signedByCandle(cur)
		}
	case "long_line_candle":
		if cur.bodyRatio > 0.7 {
			return signedByCandle(cur)
		}
	case "short_line_candle":
		if cur.bodyRatio < 0.18 {
			return signedByCandle(cur)
		}
	case "engulfing_pattern":
		if i >= 1 {
			return engulfing(analyze(bars[i-1]), cur, bars[i-1], bars[i])
		}
	case "harami_pattern", "harami_cross_pattern":
		if i >= 1 {
			return harami(analyze(bars[i-1]), cur, bars[i-1], bars[i], code == "harami_cross_pattern")
		}
	case "piercing_pattern":
		if i >= 1 && piercing(analyze(bars[i-1]), cur, bars[i-1], bars[i]) {
			return 100
		}
	case "dark_cloud_cover":
		if i >= 1 && darkCloud(analyze(bars[i-1]), cur, bars[i-1], bars[i]) {
			return -100
		}
	case "counterattack":
		if i >= 1 && near(bars[i-1].Close, bars[i].Close, avgRange(bars, i, 5)*0.08) {
			return signedByCandle(cur)
		}
	case "matching_low":
		if i >= 1 && analyze(bars[i-1]).bearish && cur.bearish && near(bars[i-1].Close, bars[i].Close, avgRange(bars, i, 5)*0.05) {
			return 100
		}
	case "homing_pigeon":
		if i >= 1 && analyze(bars[i-1]).bearish && cur.bearish && insideBody(bars[i], bars[i-1]) {
			return 100
		}
	case "three_white_soldiers":
		if i >= 2 && allBullish(bars[i-2:i+1]) && risingCloses(bars[i-2:i+1]) {
			return 100
		}
	case "three_black_crows", "identical_three_crows":
		if i >= 2 && allBearish(bars[i-2:i+1]) && fallingCloses(bars[i-2:i+1]) {
			return -100
		}
	case "three_inside_up_down":
		if i >= 2 {
			return threeInside(bars[i-2 : i+1])
		}
	case "three_outside_up_down":
		if i >= 2 {
			return threeOutside(bars[i-2 : i+1])
		}
	case "morning_star", "morning_doji_star":
		if i >= 2 && morningStar(bars[i-2:i+1], code == "morning_doji_star") {
			return 100
		}
	case "evening_star", "evening_doji_star":
		if i >= 2 && eveningStar(bars[i-2:i+1], code == "evening_doji_star") {
			return -100
		}
	case "belt_hold":
		if cur.bodyRatio > 0.65 && (cur.upperRatio < 0.12 || cur.lowerRatio < 0.12) {
			return signedByCandle(cur)
		}
	case "kicking", "kicking_bull_bear":
		if i >= 1 && isMarubozu(analyze(bars[i-1])) && isMarubozu(cur) && gapBetween(bars[i-1], bars[i]) {
			return signedByCandle(cur)
		}
	case "up_down_gap", "tasuki_gap", "upside_downside_gap":
		if i >= 1 && gapBetween(bars[i-1], bars[i]) {
			return signedByCandle(cur)
		}
	case "separating_lines":
		if i >= 1 && near(bars[i-1].Open, bars[i].Open, avgRange(bars, i, 5)*0.05) && analyze(bars[i-1]).bullish != cur.bullish {
			return signedByCandle(cur)
		}
	case "three_stars_in_the_south", "ladder_bottom", "stick_sandwich", "unique_3_river":
		if i >= 2 && downtrend(bars, max(0, i-5), i-1) && cur.bullish {
			return 100
		}
	case "advance_block", "stalled_pattern":
		if i >= 2 && allBullish(bars[i-2:i+1]) && risingCloses(bars[i-2:i+1]) && cur.bodyRatio < analyze(bars[i-1]).bodyRatio {
			return -100
		}
	case "tow_crows", "upside_gap_two_crows":
		if i >= 2 && uptrend(bars, max(0, i-5), i-2) && analyze(bars[i-1]).bearish && cur.bearish {
			return -100
		}
	case "in_neck_pattern", "on_neck_pattern", "thrusting_pattern":
		if i >= 1 && analyze(bars[i-1]).bearish && cur.bullish && bars[i].Close < midpoint(bars[i-1]) {
			return -100
		}
	case "abandoned_baby", "tristar_pattern":
		if i >= 2 && isDoji(analyze(bars[i-1])) && gapBetween(bars[i-2], bars[i-1]) && gapBetween(bars[i-1], bars[i]) {
			return signedByCandle(cur)
		}
	case "breakaway", "mat_hold", "rising_falling_three", "three_line_strike":
		if i >= 4 {
			return continuationFive(bars[i-4 : i+1])
		}
	case "hikkake_pattern", "modified_hikkake_pattern":
		if i >= 3 && insideBody(bars[i-2], bars[i-3]) && !insideBody(bars[i], bars[i-1]) {
			return signedByCandle(cur)
		}
	case "concealing_baby_swallow":
		if i >= 3 && allBearish(bars[i-3:i+1]) && bars[i].Close > bars[i-1].Close {
			return 100
		}
	}
	return 0
}

func analyze(bar indicator.Bar) candle {
	high := math.Max(bar.High, math.Max(bar.Open, bar.Close))
	low := math.Min(bar.Low, math.Min(bar.Open, bar.Close))
	rangeSize := math.Max(high-low, 0.000001)
	body := math.Abs(bar.Close - bar.Open)
	upper := high - math.Max(bar.Open, bar.Close)
	lower := math.Min(bar.Open, bar.Close) - low
	return candle{
		body:       body,
		rangeSize:  rangeSize,
		upper:      upper,
		lower:      lower,
		bodyRatio:  body / rangeSize,
		upperRatio: upper / rangeSize,
		lowerRatio: lower / rangeSize,
		bullish:    bar.Close > bar.Open,
		bearish:    bar.Close < bar.Open,
	}
}

func isDoji(c candle) bool { return c.bodyRatio <= 0.1 }

func isMarubozu(c candle) bool {
	return c.bodyRatio > 0.85 && c.upperRatio < 0.08 && c.lowerRatio < 0.08
}

func signedByCandle(c candle) int {
	if c.bullish {
		return 100
	}
	if c.bearish {
		return -100
	}
	return 0
}

func direction(value int) string {
	if value > 0 {
		return "bullish"
	}
	if value < 0 {
		return "bearish"
	}
	return "neutral"
}

func engulfing(prevC, curC candle, prev, cur indicator.Bar) int {
	if prevC.bearish && curC.bullish && cur.Open < prev.Close && cur.Close > prev.Open {
		return 100
	}
	if prevC.bullish && curC.bearish && cur.Open > prev.Close && cur.Close < prev.Open {
		return -100
	}
	return 0
}

func harami(prevC, curC candle, prev, cur indicator.Bar, requireDoji bool) int {
	if requireDoji && !isDoji(curC) {
		return 0
	}
	if insideBody(cur, prev) {
		if prevC.bearish && curC.bullish {
			return 100
		}
		if prevC.bullish && curC.bearish {
			return -100
		}
	}
	return 0
}

func piercing(prevC, curC candle, prev, cur indicator.Bar) bool {
	return prevC.bearish && curC.bullish && cur.Open < prev.Close && cur.Close > midpoint(prev) && cur.Close < prev.Open
}

func darkCloud(prevC, curC candle, prev, cur indicator.Bar) bool {
	return prevC.bullish && curC.bearish && cur.Open > prev.Close && cur.Close < midpoint(prev) && cur.Close > prev.Open
}

func threeInside(bars []indicator.Bar) int {
	first, second, third := analyze(bars[0]), analyze(bars[1]), analyze(bars[2])
	if first.bearish && insideBody(bars[1], bars[0]) && third.bullish && bars[2].Close > bars[0].Open {
		return 100
	}
	if first.bullish && insideBody(bars[1], bars[0]) && third.bearish && bars[2].Close < bars[0].Open {
		return -100
	}
	_ = second
	return 0
}

func threeOutside(bars []indicator.Bar) int {
	first, second, third := analyze(bars[0]), analyze(bars[1]), analyze(bars[2])
	v := engulfing(first, second, bars[0], bars[1])
	if v > 0 && third.bullish && bars[2].Close > bars[1].Close {
		return 100
	}
	if v < 0 && third.bearish && bars[2].Close < bars[1].Close {
		return -100
	}
	return 0
}

func morningStar(bars []indicator.Bar, middleDoji bool) bool {
	first, second, third := analyze(bars[0]), analyze(bars[1]), analyze(bars[2])
	return first.bearish && (!middleDoji || isDoji(second)) && second.bodyRatio < 0.35 && third.bullish && bars[2].Close > midpoint(bars[0])
}

func eveningStar(bars []indicator.Bar, middleDoji bool) bool {
	first, second, third := analyze(bars[0]), analyze(bars[1]), analyze(bars[2])
	return first.bullish && (!middleDoji || isDoji(second)) && second.bodyRatio < 0.35 && third.bearish && bars[2].Close < midpoint(bars[0])
}

func continuationFive(bars []indicator.Bar) int {
	first := analyze(bars[0])
	last := analyze(bars[4])
	if first.bullish && last.bullish && bars[4].Close > bars[0].Close {
		return 100
	}
	if first.bearish && last.bearish && bars[4].Close < bars[0].Close {
		return -100
	}
	return 0
}

func allBullish(bars []indicator.Bar) bool {
	for _, bar := range bars {
		if !analyze(bar).bullish {
			return false
		}
	}
	return true
}

func allBearish(bars []indicator.Bar) bool {
	for _, bar := range bars {
		if !analyze(bar).bearish {
			return false
		}
	}
	return true
}

func risingCloses(bars []indicator.Bar) bool {
	for i := 1; i < len(bars); i++ {
		if bars[i].Close <= bars[i-1].Close {
			return false
		}
	}
	return true
}

func fallingCloses(bars []indicator.Bar) bool {
	for i := 1; i < len(bars); i++ {
		if bars[i].Close >= bars[i-1].Close {
			return false
		}
	}
	return true
}

func uptrend(bars []indicator.Bar, start, end int) bool {
	if end <= start {
		return false
	}
	return bars[end].Close > bars[start].Close
}

func downtrend(bars []indicator.Bar, start, end int) bool {
	if end <= start {
		return false
	}
	return bars[end].Close < bars[start].Close
}

func insideBody(inner, outer indicator.Bar) bool {
	innerHigh := math.Max(inner.Open, inner.Close)
	innerLow := math.Min(inner.Open, inner.Close)
	outerHigh := math.Max(outer.Open, outer.Close)
	outerLow := math.Min(outer.Open, outer.Close)
	return innerHigh <= outerHigh && innerLow >= outerLow
}

func gapBetween(a, b indicator.Bar) bool {
	return b.Low > a.High || b.High < a.Low
}

func midpoint(bar indicator.Bar) float64 {
	return (bar.Open + bar.Close) / 2
}

func avgRange(bars []indicator.Bar, i, window int) float64 {
	start := max(0, i-window+1)
	var sum float64
	for j := start; j <= i; j++ {
		sum += math.Max(bars[j].High-bars[j].Low, 0)
	}
	return math.Max(sum/float64(i-start+1), 0.000001)
}

func near(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

package indicator

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Bar struct {
	TS     time.Time
	Open   float64
	Close  float64
	High   float64
	Low    float64
	Volume float64
}

type Point struct {
	TS     time.Time
	Value  float64
	Values map[string]float64
}

func Compute(code string, bars []Bar, params map[string]any) ([]Point, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(bars) == 0 {
		return nil, nil
	}
	switch code {
	case "MA":
		return computeMultiMA(bars, intList(params, "windows", []int{5, 10, 20, 30, 60, 120, 250}), "ma"), nil
	case "EMA":
		return computeMultiEMA(bars, intList(params, "windows", []int{5, 10, 12, 20, 26, 60}), "ema"), nil
	case "MACD":
		return computeMACD(bars, intParam(params, "fast", 12), intParam(params, "slow", 26), intParam(params, "signal", 9)), nil
	case "KDJ":
		return computeKDJ(bars, intParam(params, "n", 9), intParam(params, "m1", 3), intParam(params, "m2", 3)), nil
	case "BOLL":
		return computeBOLL(bars, intParam(params, "window", 20), floatParam(params, "std", 2)), nil
	case "RSI":
		return computeRSI(bars, intList(params, "windows", []int{6, 12, 24})), nil
	case "TRIX":
		return computeTRIX(bars, intParam(params, "window", 12), intParam(params, "ma", 20)), nil
	case "VR":
		return computeVR(bars, intParam(params, "window", 26)), nil
	case "WR":
		return computeWR(bars, intList(params, "windows", []int{10, 14})), nil
	default:
		return nil, fmt.Errorf("indicator unsupported: %s", code)
	}
}

func computeMultiMA(bars []Bar, windows []int, prefix string) []Point {
	closes := closes(bars)
	series := map[int][]float64{}
	for _, window := range windows {
		series[window] = sma(closes, window)
	}
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		values := map[string]float64{}
		for _, window := range windows {
			values[fmt.Sprintf("%s%d", prefix, window)] = series[window][i]
		}
		points = append(points, Point{TS: bar.TS, Value: firstValue(values, fmt.Sprintf("%s%d", prefix, windows[0])), Values: values})
	}
	return points
}

func computeMultiEMA(bars []Bar, windows []int, prefix string) []Point {
	closes := closes(bars)
	series := map[int][]float64{}
	for _, window := range windows {
		series[window] = ema(closes, window)
	}
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		values := map[string]float64{}
		for _, window := range windows {
			values[fmt.Sprintf("%s%d", prefix, window)] = series[window][i]
		}
		points = append(points, Point{TS: bar.TS, Value: firstValue(values, fmt.Sprintf("%s%d", prefix, windows[0])), Values: values})
	}
	return points
}

func computeMACD(bars []Bar, fast, slow, signal int) []Point {
	closes := closes(bars)
	fastEMA := ema(closes, fast)
	slowEMA := ema(closes, slow)
	dif := make([]float64, len(closes))
	for i := range closes {
		dif[i] = fastEMA[i] - slowEMA[i]
	}
	dea := ema(dif, signal)
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		macd := (dif[i] - dea[i]) * 2
		points = append(points, Point{
			TS:    bar.TS,
			Value: macd,
			Values: map[string]float64{
				"dif":  round4(dif[i]),
				"dea":  round4(dea[i]),
				"macd": round4(macd),
			},
		})
	}
	return points
}

func computeKDJ(bars []Bar, n, m1, m2 int) []Point {
	k := 50.0
	d := 50.0
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		start := max(0, i-n+1)
		highest := bars[start].High
		lowest := bars[start].Low
		for j := start; j <= i; j++ {
			highest = math.Max(highest, bars[j].High)
			lowest = math.Min(lowest, bars[j].Low)
		}
		rsv := 50.0
		if highest != lowest {
			rsv = (bar.Close - lowest) / (highest - lowest) * 100
		}
		k = (float64(m1-1)*k + rsv) / float64(m1)
		d = (float64(m2-1)*d + k) / float64(m2)
		j := 3*k - 2*d
		points = append(points, Point{
			TS:    bar.TS,
			Value: round4(j),
			Values: map[string]float64{
				"kdjk": round4(k),
				"kdjd": round4(d),
				"kdjj": round4(j),
			},
		})
	}
	return points
}

func computeBOLL(bars []Bar, window int, multiplier float64) []Point {
	closes := closes(bars)
	mid := sma(closes, window)
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		start := max(0, i-window+1)
		var sum float64
		for j := start; j <= i; j++ {
			diff := closes[j] - mid[i]
			sum += diff * diff
		}
		std := math.Sqrt(sum / float64(i-start+1))
		upper := mid[i] + multiplier*std
		lower := mid[i] - multiplier*std
		points = append(points, Point{
			TS:    bar.TS,
			Value: round4(mid[i]),
			Values: map[string]float64{
				"boll_mid":   round4(mid[i]),
				"boll_upper": round4(upper),
				"boll_lower": round4(lower),
			},
		})
	}
	return points
}

func computeRSI(bars []Bar, windows []int) []Point {
	closes := closes(bars)
	series := map[int][]float64{}
	for _, window := range windows {
		series[window] = rsi(closes, window)
	}
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		values := map[string]float64{}
		for _, window := range windows {
			values[fmt.Sprintf("rsi%d", window)] = series[window][i]
		}
		points = append(points, Point{TS: bar.TS, Value: firstValue(values, fmt.Sprintf("rsi%d", windows[0])), Values: values})
	}
	return points
}

func computeTRIX(bars []Bar, window, maWindow int) []Point {
	closes := closes(bars)
	e1 := ema(closes, window)
	e2 := ema(e1, window)
	e3 := ema(e2, window)
	trix := make([]float64, len(e3))
	for i := range e3 {
		if i == 0 || e3[i-1] == 0 {
			trix[i] = 0
			continue
		}
		trix[i] = (e3[i] - e3[i-1]) / e3[i-1] * 100
	}
	trixMA := sma(trix, maWindow)
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		points = append(points, Point{
			TS:    bar.TS,
			Value: round4(trix[i]),
			Values: map[string]float64{
				"trix":    round4(trix[i]),
				"trix_ma": round4(trixMA[i]),
			},
		})
	}
	return points
}

func computeVR(bars []Bar, window int) []Point {
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		start := max(1, i-window+1)
		var avs, bvs, cvs float64
		for j := start; j <= i; j++ {
			switch {
			case bars[j].Close > bars[j-1].Close:
				avs += bars[j].Volume
			case bars[j].Close < bars[j-1].Close:
				bvs += bars[j].Volume
			default:
				cvs += bars[j].Volume
			}
		}
		vr := 0.0
		denominator := bvs + cvs/2
		if denominator != 0 {
			vr = (avs + cvs/2) / denominator * 100
		}
		points = append(points, Point{
			TS:     bar.TS,
			Value:  round4(vr),
			Values: map[string]float64{"vr": round4(vr)},
		})
	}
	return points
}

func computeWR(bars []Bar, windows []int) []Point {
	points := make([]Point, 0, len(bars))
	for i, bar := range bars {
		values := map[string]float64{}
		for _, window := range windows {
			start := max(0, i-window+1)
			highest := bars[start].High
			lowest := bars[start].Low
			for j := start; j <= i; j++ {
				highest = math.Max(highest, bars[j].High)
				lowest = math.Min(lowest, bars[j].Low)
			}
			wr := 0.0
			if highest != lowest {
				wr = (highest - bar.Close) / (highest - lowest) * -100
			}
			values[fmt.Sprintf("wr%d", window)] = round4(wr)
		}
		points = append(points, Point{TS: bar.TS, Value: firstValue(values, fmt.Sprintf("wr%d", windows[0])), Values: values})
	}
	return points
}

func closes(bars []Bar) []float64 {
	values := make([]float64, len(bars))
	for i, bar := range bars {
		values[i] = bar.Close
	}
	return values
}

func sma(values []float64, window int) []float64 {
	if window <= 0 {
		window = 1
	}
	out := make([]float64, len(values))
	var sum float64
	for i, value := range values {
		sum += value
		if i >= window {
			sum -= values[i-window]
		}
		count := min(i+1, window)
		out[i] = round4(sum / float64(count))
	}
	return out
}

func ema(values []float64, window int) []float64 {
	if window <= 0 {
		window = 1
	}
	out := make([]float64, len(values))
	alpha := 2.0 / float64(window+1)
	for i, value := range values {
		if i == 0 {
			out[i] = value
			continue
		}
		out[i] = round4(alpha*value + (1-alpha)*out[i-1])
	}
	return out
}

func rsi(values []float64, window int) []float64 {
	if window <= 0 {
		window = 1
	}
	out := make([]float64, len(values))
	var avgGain, avgLoss float64
	for i := range values {
		if i == 0 {
			out[i] = 50
			continue
		}
		change := values[i] - values[i-1]
		gain := math.Max(change, 0)
		loss := math.Max(-change, 0)
		if i == 1 {
			avgGain = gain
			avgLoss = loss
		} else {
			avgGain = (avgGain*float64(window-1) + gain) / float64(window)
			avgLoss = (avgLoss*float64(window-1) + loss) / float64(window)
		}
		if avgLoss == 0 {
			out[i] = 100
			continue
		}
		rs := avgGain / avgLoss
		out[i] = round4(100 - 100/(1+rs))
	}
	return out
}

func firstValue(values map[string]float64, key string) float64 {
	return round4(values[key])
}

func intParam(params map[string]any, key string, fallback int) int {
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return fallback
	}
}

func floatParam(params map[string]any, key string, fallback float64) float64 {
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return fallback
	}
}

func intList(params map[string]any, key string, fallback []int) []int {
	raw, ok := params[key]
	if !ok {
		return fallback
	}
	items, ok := raw.([]any)
	if !ok {
		return fallback
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case float64:
			out = append(out, int(v))
		case int:
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

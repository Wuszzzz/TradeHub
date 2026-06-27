package indicator

import (
	"testing"
	"time"
)

func sampleBars() []Bar {
	base := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	bars := make([]Bar, 0, 40)
	for i := 0; i < 40; i++ {
		close := 100 + float64(i)
		bars = append(bars, Bar{
			TS:     base.Add(time.Duration(i) * time.Minute),
			Open:   close - 0.5,
			Close:  close,
			High:   close + 1,
			Low:    close - 1,
			Volume: 1000 + float64(i*10),
		})
	}
	return bars
}

func TestComputeMACDReturnsDIFDEAAndMACD(t *testing.T) {
	points, err := Compute("MACD", sampleBars(), nil)
	if err != nil {
		t.Fatalf("compute macd failed: %v", err)
	}
	if len(points) != 40 {
		t.Fatalf("expected 40 points, got %d", len(points))
	}
	last := points[len(points)-1]
	for _, key := range []string{"dif", "dea", "macd"} {
		if _, ok := last.Values[key]; !ok {
			t.Fatalf("missing %s in %+v", key, last.Values)
		}
	}
}

func TestComputeKDJReturnsKDJFields(t *testing.T) {
	points, err := Compute("KDJ", sampleBars(), nil)
	if err != nil {
		t.Fatalf("compute kdj failed: %v", err)
	}
	last := points[len(points)-1]
	for _, key := range []string{"kdjk", "kdjd", "kdjj"} {
		if _, ok := last.Values[key]; !ok {
			t.Fatalf("missing %s in %+v", key, last.Values)
		}
	}
}

func TestComputeBOLLReturnsBands(t *testing.T) {
	points, err := Compute("BOLL", sampleBars(), map[string]any{"window": float64(20), "std": float64(2)})
	if err != nil {
		t.Fatalf("compute boll failed: %v", err)
	}
	last := points[len(points)-1]
	if last.Values["boll_upper"] <= last.Values["boll_mid"] || last.Values["boll_lower"] >= last.Values["boll_mid"] {
		t.Fatalf("unexpected boll bands: %+v", last.Values)
	}
}

func TestComputeRejectsUnsupportedIndicator(t *testing.T) {
	if _, err := Compute("BAD", sampleBars(), nil); err == nil {
		t.Fatalf("expected unsupported indicator error")
	}
}

package pattern

import (
	"testing"
	"time"

	"stock-etf-monitor/backend/internal/domain/indicator"
)

func bar(i int, open, close, high, low float64) indicator.Bar {
	return indicator.Bar{
		TS:     time.Date(2026, 6, 1, 9, 30+i, 0, 0, time.UTC),
		Open:   open,
		Close:  close,
		High:   high,
		Low:    low,
		Volume: 1000,
	}
}

func TestAllCodesContainsSixtyOnePatterns(t *testing.T) {
	if len(AllCodes()) != 61 {
		t.Fatalf("expected 61 patterns, got %d", len(AllCodes()))
	}
}

func TestScanDetectsDoji(t *testing.T) {
	hits, err := Scan([]string{"doji"}, []indicator.Bar{
		bar(0, 10, 10.01, 10.8, 9.2),
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(hits) != 1 || hits[0].PatternCode != "doji" || hits[0].Value != 100 {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestScanDetectsBullishEngulfing(t *testing.T) {
	hits, err := Scan([]string{"engulfing_pattern"}, []indicator.Bar{
		bar(0, 11, 10, 11.2, 9.8),
		bar(1, 9.8, 11.3, 11.5, 9.7),
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(hits) != 1 || hits[0].Value != 100 {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestScanDetectsThreeWhiteSoldiers(t *testing.T) {
	hits, err := Scan([]string{"three_white_soldiers"}, []indicator.Bar{
		bar(0, 10, 11, 11.2, 9.8),
		bar(1, 11, 12, 12.2, 10.8),
		bar(2, 12, 13, 13.2, 11.8),
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(hits) != 1 || hits[0].Value != 100 {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

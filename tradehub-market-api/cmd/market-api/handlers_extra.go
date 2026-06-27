package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"stock/cmd/market-api/internal/symbol"
	"stock/cmd/market-api/internal/xueqiu"
)

// resolveSpec parses ?symbol= (or first of ?symbols=) into a normalized Spec.
// On failure it writes a 400 response and returns ok=false.
func (s *server) resolveSpec(w http.ResponseWriter, r *http.Request) (symbol.Spec, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if raw == "" {
		if list := strings.TrimSpace(r.URL.Query().Get("symbols")); list != "" {
			raw = strings.Split(list, ",")[0]
		}
	}
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "missing symbol"})
		return symbol.Spec{}, false
	}
	spec, err := symbol.Resolve(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
		return symbol.Spec{}, false
	}
	return spec, true
}

// resolveSpecs parses ?symbols=a,b,c (or single ?symbol=).
func (s *server) resolveSpecs(w http.ResponseWriter, r *http.Request) ([]symbol.Spec, bool) {
	q := r.URL.Query()
	var raws []string
	if list := strings.TrimSpace(q.Get("symbols")); list != "" {
		for _, item := range strings.Split(list, ",") {
			if t := strings.TrimSpace(item); t != "" {
				raws = append(raws, t)
			}
		}
	}
	if single := strings.TrimSpace(q.Get("symbol")); single != "" {
		raws = append(raws, single)
	}
	if len(raws) == 0 {
		writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "missing symbol"})
		return nil, false
	}
	out := make([]symbol.Spec, 0, len(raws))
	for _, r := range raws {
		spec, err := symbol.Resolve(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return nil, false
		}
		out = append(out, spec)
	}
	return out, true
}

// ---- Eastmoney handlers -------------------------------------------------

func (s *server) eastmoneySnapshot(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 800 * time.Millisecond
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.eastmoney.Snapshot(r.Context(), spec)
	})
}

func (s *server) eastmoneyKline(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "day"
	}
	adjust := strings.TrimSpace(r.URL.Query().Get("adjust"))
	if adjust == "" {
		adjust = "qfq"
	}
	limit := intQuery(r, "limit", 100)
	ttl := eastmoneyKlineTTL(period)
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.eastmoney.Kline(r.Context(), spec, period, adjust, limit)
	})
}

func (s *server) eastmoneyTrends(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	days := intQuery(r, "days", 1)
	ttl := 15 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.eastmoney.Trends(r.Context(), spec, days)
	})
}

func (s *server) eastmoneyFlowSnapshot(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 5 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.eastmoney.FlowSnapshot(r.Context(), spec)
	})
}

func (s *server) eastmoneyFlowIntraday(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	limit := intQuery(r, "limit", 0)
	ttl := 30 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.eastmoney.FlowIntraday(r.Context(), spec, limit)
	})
}

func (s *server) eastmoneyFlowDaily(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	limit := intQuery(r, "limit", 30)
	ttl := 30 * time.Minute
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.eastmoney.FlowDaily(r.Context(), spec, limit)
	})
}

func eastmoneyKlineTTL(period string) time.Duration {
	switch period {
	case "1m", "5m", "15m", "30m", "60m":
		return 30 * time.Second
	case "day":
		return 30 * time.Minute
	case "week", "month":
		return 1 * time.Hour
	default:
		return 30 * time.Second
	}
}

// ---- Sohu handlers ------------------------------------------------------

func (s *server) sohuSnapshot(w http.ResponseWriter, r *http.Request) {
	specs, ok := s.resolveSpecs(w, r)
	if !ok {
		return
	}
	ttl := 1 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		if len(specs) == 1 {
			snap, err := s.sohu.Snapshot(r.Context(), specs[0])
			if err != nil {
				return nil, err
			}
			return snap, nil
		}
		rows, err := s.sohu.SnapshotBatch(r.Context(), specs)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"dataset": "snapshot_batch",
			"source":  "sohu",
			"count":   len(rows),
			"rows":    rows,
		}, nil
	})
}

func (s *server) sohuKline(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "day"
	}
	begin := strings.TrimSpace(r.URL.Query().Get("begin"))
	end := strings.TrimSpace(r.URL.Query().Get("end"))
	limit := intQuery(r, "limit", 0)
	ttl := sohuKlineTTL(period)
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.Kline(r.Context(), spec, period, begin, end, limit)
	})
}

func sohuKlineTTL(period string) time.Duration {
	switch period {
	case "day":
		return 30 * time.Minute
	case "week", "month":
		return 1 * time.Hour
	default:
		return 30 * time.Minute
	}
}

func (s *server) sohuTicks(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 2 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.Ticks(r.Context(), spec)
	})
}

func (s *server) sohuMinute(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 8 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.Minute(r.Context(), spec)
	})
}

func (s *server) sohuPriceDistribution(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 5 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.PriceDistribution(r.Context(), spec)
	})
}

func (s *server) sohuFlow(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 5 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.FundFlow(r.Context(), spec)
	})
}

func (s *server) sohuFlowSeries(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 5 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.FundFlowSeries(r.Context(), spec)
	})
}

func (s *server) sohuOrderBook(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 1 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.OrderBook(r.Context(), spec)
	})
}

func (s *server) sohuAggregate(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 1 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.sohu.Aggregate(r.Context(), spec)
	})
}

// ---- THS (10jqka) handlers ---------------------------------------------

func (s *server) thsSnapshot(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 1 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.ths.Snapshot(r.Context(), spec)
	})
}

func (s *server) thsMinute(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	ttl := 8 * time.Second
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.ths.Minute(r.Context(), spec)
	})
}

func (s *server) thsKline(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "day"
	}
	adjust := strings.TrimSpace(r.URL.Query().Get("adjust"))
	if adjust == "" {
		adjust = "qfq"
	}
	limit := intQuery(r, "limit", 100)
	ttl := 30 * time.Minute
	s.serveWithCache(w, r, ttl, func() (any, error) {
		return s.ths.Kline(r.Context(), spec, period, adjust, limit)
	})
}

// ---- Xueqiu handlers ----------------------------------------------------

func xueqiuCookieFromRequest(r *http.Request) string {
	cookie := strings.TrimSpace(r.Header.Get("X-Xueqiu-Cookie"))
	if cookie != "" {
		return cookie
	}
	return strings.TrimSpace(r.Header.Get("X-XQ-Cookie"))
}

func (s *server) xueqiuSnapshot(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	cookie := xueqiuCookieFromRequest(r)
	if cookie == "" && !s.xueqiu.HasDefaultCookie() {
		writeJSON(w, http.StatusUnauthorized, response{OK: false, Error: "xueqiu cookie is required: set env XUEQIU_COOKIE or header X-Xueqiu-Cookie"})
		return
	}
	data, err := s.xueqiu.Snapshot(r.Context(), spec, cookie)
	if err != nil {
		status := http.StatusBadGateway
		if xueqiu.IsAuthError(err) {
			status = http.StatusUnauthorized
		}
		writeJSON(w, status, response{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{OK: true, Data: data})
}

func (s *server) xueqiuKline(w http.ResponseWriter, r *http.Request) {
	spec, ok := s.resolveSpec(w, r)
	if !ok {
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "day"
	}
	adjust := strings.TrimSpace(r.URL.Query().Get("adjust"))
	if adjust == "" {
		adjust = "qfq"
	}
	limit := intQuery(r, "limit", 100)
	cookie := xueqiuCookieFromRequest(r)
	if cookie == "" && !s.xueqiu.HasDefaultCookie() {
		writeJSON(w, http.StatusUnauthorized, response{OK: false, Error: "xueqiu cookie is required: set env XUEQIU_COOKIE or header X-Xueqiu-Cookie"})
		return
	}
	data, err := s.xueqiu.Kline(r.Context(), spec, period, adjust, limit, cookie)
	if err != nil {
		status := http.StatusBadGateway
		if xueqiu.IsAuthError(err) {
			status = http.StatusUnauthorized
		}
		writeJSON(w, status, response{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{OK: true, Data: data})
}

// silence "unused" lint when we briefly add diagnostic helpers; kept as
// package-internal escape hatch.
var _ = fmt.Sprint

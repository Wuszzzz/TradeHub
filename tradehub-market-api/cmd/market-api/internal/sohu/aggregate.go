// Sohu's hq.stock.sohu.com/cn/{last3}/cn_{code}-1.html aggregate endpoint.
//
// This single ~1.6KB GB18030 JSONP fragment carries 13 distinct blocks:
//
//   index         major indices (sh000001 / sz399001)
//   change        global daily gainers top-10
//   price_A1      stock identity + last price + change
//   price_A2      17 extension fields (avg, prev_close, open, vol_ratio, high,
//                 turnover, low, vol_hand, limit_up, amplitude_proxy, ?,
//                 amount_kyuan, last_price_dup, ?, ?, total_mcap_yi, ?)
//   price_A3      committee buy/sell hand counts + trade time + status
//   price_hk      paired Hong-Kong listing (if any)
//   perform       ⭐ committee-ratio + level-2 order book (5 ask + 5 bid)
//                 + inner/outer volume + status flag
//   dealdetail    most recent ticks (tail-first, ~13 rows)
//   pricedetail   per-price distribution near last price (~10 rows)
//   sector        belonging sectors / industry tags
//   quote_m_r     last few minutes of intraday minute bars
//   quote_k_r     today's daily K bar
//   quote_wk_r    this week's weekly K bar
//   quote_mk_r    this month's monthly K bar
//   time          server-side trade time
//
// This is the most information-dense single endpoint sohu exposes: a UI
// dashboard can be hydrated entirely from one call. We expose two HTTP
// endpoints over it:
//
//   /api/v1/sohu/order-book   compact view of the 5-level depth only
//   /api/v1/sohu/aggregate    full decoded payload of every block

package sohu

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"stock/cmd/market-api/internal/symbol"
)

// reBlock pulls a top-level block by name out of the fortune_hq({...}) body.
//
// The grammar is somewhat irregular: blocks may be string-quoted, single
// arrays, arrays of arrays, or stringified arrays. We capture greedily up to
// the next ",'<lower>':" or to "})" tail.
var reBlock = regexp.MustCompile(`'([a-z_A-Z0-9]+)'\s*:\s*((?:\[[^\[\]]*(?:\[[^\[\]]*\][^\[\]]*)*\])|(?:"[^"]*")|(?:\[\]))`)

// Aggregate is the decoded payload of cn_{code}-1.html. Every individual
// block is preserved both as a typed projection and as the raw `string` /
// `[][]string` so callers can access undocumented columns we did not yet name.
type Aggregate struct {
	Dataset    string         `json:"dataset"`
	Source     string         `json:"source"`
	Symbol     string         `json:"symbol"`
	SohuCode   string         `json:"sohu_code"`
	TradeTime  string         `json:"trade_time,omitempty"`
	CapturedAt int64          `json:"captured_at"`

	Indices    [][]string     `json:"indices,omitempty"`
	TopGainers [][]string     `json:"top_gainers,omitempty"`

	Identity   AggIdentity    `json:"identity"`
	Quote      AggQuote       `json:"quote"`
	OrderBook  AggOrderBook   `json:"order_book"`
	Sectors    []AggSector    `json:"sectors,omitempty"`
	HK         []string       `json:"hk_pair,omitempty"`

	RecentTicks    []Tick           `json:"recent_ticks,omitempty"`
	NearbyPrices   []PriceDistRow   `json:"nearby_prices,omitempty"`
	MinuteTail     [][]string       `json:"minute_tail,omitempty"`
	KlineDay       [][]string       `json:"kline_day,omitempty"`
	KlineWeek      [][]string       `json:"kline_week,omitempty"`
	KlineMonth     [][]string       `json:"kline_month,omitempty"`

	Raw map[string]string `json:"raw,omitempty"`
}

type AggIdentity struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	URLDesktop  string `json:"url_desktop,omitempty"`
}

type AggQuote struct {
	Latest               *float64 `json:"latest"`
	ChangeAmount         *float64 `json:"change_amount"`
	ChangePercent        *float64 `json:"change_percent"`
	Average              *float64 `json:"average"`
	PreviousClose        *float64 `json:"previous_close"`
	Open                 *float64 `json:"open"`
	High                 *float64 `json:"high"`
	Low                  *float64 `json:"low"`
	VolumeRatio          *float64 `json:"volume_ratio"`
	TurnoverRatePercent  *float64 `json:"turnover_rate_percent"`
	VolumeHand           *int64   `json:"volume_hand"`
	AmountYuan           *float64 `json:"amount_yuan"`
	LimitUp              *float64 `json:"limit_up"`
	TotalMarketCapYuan   *float64 `json:"total_market_cap_yuan"`
	BuyOrdersHand        *int64   `json:"buy_orders_hand"`   // 委买总手
	SellOrdersHand       *int64   `json:"sell_orders_hand"`  // 委卖总手
	StatusFlag           string   `json:"status_flag,omitempty"`
}

// AggOrderBook is the level-2 5+5 depth book extracted from `perform`.
type AggOrderBook struct {
	CommitteeRatioPercent *float64       `json:"committee_ratio_percent"` // 委比 (-100~+100)
	CommitteeDiffHand     *int64         `json:"committee_diff_hand"`     // 委差(手)
	Asks                  []AggOrderLvl  `json:"asks"`                    // ask1..ask5 (price ascending)
	Bids                  []AggOrderLvl  `json:"bids"`                    // bid1..bid5 (price descending)
	InnerVolumeHand       *int64         `json:"inner_volume_hand"`       // 内盘
	OuterVolumeHand       *int64         `json:"outer_volume_hand"`       // 外盘
	StatusFlag            string         `json:"status_flag,omitempty"`   // e.g. "Z"
}

type AggOrderLvl struct {
	Level      int      `json:"level"`
	Price      *float64 `json:"price"`
	VolumeHand *int64   `json:"volume_hand"`
}

type AggSector struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	ChangePercent *float64 `json:"change_percent"`
	URL            string  `json:"url,omitempty"`
}

func (c *Client) fetchAggregateBody(ctx context.Context, spec symbol.Spec) (string, error) {
	url := fmt.Sprintf(hqStockURL, spec.SohuLast3(), spec.Code, 1, time.Now().UnixMilli())
	text, err := c.fetchJSONP(ctx, url)
	if err != nil {
		return "", err
	}
	return stripCallback(text), nil
}

// Aggregate parses the full cn_{code}-1.html payload.
func (c *Client) Aggregate(ctx context.Context, spec symbol.Spec) (*Aggregate, error) {
	body, err := c.fetchAggregateBody(ctx, spec)
	if err != nil {
		return nil, err
	}
	blocks := splitAggregateBlocks(body)
	if len(blocks) == 0 {
		return nil, errors.New("sohu aggregate: no blocks parsed")
	}
	out := &Aggregate{
		Dataset:    "aggregate",
		Source:     "sohu",
		Symbol:     spec.Tencent(),
		SohuCode:   spec.Sohu(),
		CapturedAt: time.Now().Unix(),
		Raw:        blocks,
	}

	// indices / change
	out.Indices = parseTuples(blocks["index"])
	out.TopGainers = parseTuples(blocks["change"])

	// price_A1 -> identity + base quote
	if a1 := stringSliceFromBlock(blocks["price_A1"]); len(a1) >= 5 {
		out.Identity = AggIdentity{Code: a1[0], Name: a1[1]}
		out.Quote.Latest = parseFloatPtr(a1[2])
		out.Quote.ChangeAmount = parseFloatPtr(strings.TrimPrefix(a1[3], "+"))
		out.Quote.ChangePercent = percentToFloat(a1[4])
		if len(a1) >= 6 {
			out.Quote.StatusFlag = a1[5]
		}
	}
	// price_A2 -> extended quote
	if a2 := stringSliceFromBlock(blocks["price_A2"]); len(a2) >= 17 {
		out.Quote.Average = parseFloatPtr(a2[0])
		out.Quote.PreviousClose = parseFloatPtr(a2[1])
		out.Quote.Open = parseFloatPtr(a2[3])
		out.Quote.VolumeRatio = parseFloatPtr(a2[4])
		out.Quote.High = parseFloatPtr(a2[5])
		out.Quote.TurnoverRatePercent = percentToFloat(a2[6])
		out.Quote.Low = parseFloatPtr(a2[7])
		out.Quote.VolumeHand = parseInt64Ptr(a2[8])
		out.Quote.LimitUp = parseFloatPtr(a2[9])
		// a2[10] amplitude/something, a2[11] aux
		if v := parseFloatPtr(a2[12]); v != nil {
			amount := *v * 1000 // 千元 -> 元 (consistent with snapshot)
			out.Quote.AmountYuan = &amount
		}
		// a2[13],a2[14],a2[15] aux
		if v := parseChineseYi(a2[16]); v != nil {
			yuan := *v
			out.Quote.TotalMarketCapYuan = &yuan
		}
	}
	// price_A3 -> committee hand counts + trade time
	if a3 := stringSliceFromBlock(blocks["price_A3"]); len(a3) >= 7 {
		out.Quote.BuyOrdersHand = parseInt64Ptr(a3[2])
		out.Quote.SellOrdersHand = parseInt64Ptr(a3[3])
		out.TradeTime = a3[6]
	}
	out.HK = stringSliceFromBlock(blocks["price_hk"])

	// perform -> 5+5 order book
	if pf := stringSliceFromBlock(blocks["perform"]); len(pf) >= 22 {
		out.OrderBook = parsePerformOrderBook(pf)
	}

	// sectors
	for _, row := range parseTuples(blocks["sector"]) {
		if len(row) < 4 {
			continue
		}
		out.Sectors = append(out.Sectors, AggSector{
			Code:          row[0],
			Name:          row[1],
			ChangePercent: percentToFloat(row[2]),
			URL:           row[3],
		})
	}

	// dealdetail -> recent ticks (sohu lists newest first; flip to chronological)
	for _, row := range parseTuples(blocks["dealdetail"]) {
		if len(row) < 5 {
			continue
		}
		out.RecentTicks = append(out.RecentTicks, Tick{
			Time:          row[0],
			Price:         parseFloatPtr(row[1]),
			ChangePercent: percentToFloat(row[2]),
			VolumeHand:    parseInt64Ptr(row[3]),
			Count:         parseInt64Ptr(row[4]),
			Raw:           row,
		})
	}
	reverseTicks(out.RecentTicks)

	// pricedetail -> nearby price distribution
	for _, row := range parseTuples(blocks["pricedetail"]) {
		if len(row) < 4 {
			continue
		}
		out.NearbyPrices = append(out.NearbyPrices, PriceDistRow{
			Price:        parseFloatPtr(row[0]),
			Volume1Hand:  parseInt64Ptr(row[1]),
			Volume2Hand:  parseInt64Ptr(row[2]),
			RatioPercent: percentToFloat(row[3]),
			Raw:          row,
		})
	}

	// quote_m_r / quote_k_r / quote_wk_r / quote_mk_r are arrays of stringified rows.
	out.MinuteTail = parseQuoteArrayBlock(blocks["quote_m_r"])
	out.KlineDay = parseQuoteArrayBlock(blocks["quote_k_r"])
	out.KlineWeek = parseQuoteArrayBlock(blocks["quote_wk_r"])
	out.KlineMonth = parseQuoteArrayBlock(blocks["quote_mk_r"])

	// time block (already captured trade_time from price_A3 if present).
	if out.TradeTime == "" {
		if t := stringSliceFromBlock(blocks["time"]); len(t) >= 6 {
			out.TradeTime = strings.Join(t, "-")
		}
	}
	return out, nil
}

// OrderBook returns just the level-2 5+5 depth view, fast and small.
type OrderBook struct {
	Dataset    string        `json:"dataset"`
	Source     string        `json:"source"`
	Symbol     string        `json:"symbol"`
	SohuCode   string        `json:"sohu_code"`
	Name       string        `json:"name,omitempty"`
	Latest     *float64      `json:"latest,omitempty"`
	TradeTime  string        `json:"trade_time,omitempty"`
	CapturedAt int64         `json:"captured_at"`
	OrderBook  AggOrderBook  `json:"order_book"`
}

func (c *Client) OrderBook(ctx context.Context, spec symbol.Spec) (*OrderBook, error) {
	body, err := c.fetchAggregateBody(ctx, spec)
	if err != nil {
		return nil, err
	}
	blocks := splitAggregateBlocks(body)
	pf := stringSliceFromBlock(blocks["perform"])
	if len(pf) < 22 {
		return nil, errors.New("sohu order-book: perform block too short")
	}
	out := &OrderBook{
		Dataset:    "order_book",
		Source:     "sohu",
		Symbol:     spec.Tencent(),
		SohuCode:   spec.Sohu(),
		CapturedAt: time.Now().Unix(),
		OrderBook:  parsePerformOrderBook(pf),
	}
	if a1 := stringSliceFromBlock(blocks["price_A1"]); len(a1) >= 3 {
		out.Name = a1[1]
		out.Latest = parseFloatPtr(a1[2])
	}
	if a3 := stringSliceFromBlock(blocks["price_A3"]); len(a3) >= 7 {
		out.TradeTime = a3[6]
	}
	return out, nil
}

// ---- helpers ------------------------------------------------------------

// parsePerformOrderBook decodes the 27-element `perform` array.
//
// Layout (verified across 5 different stocks):
//
//	[0]  committee ratio %     (e.g. "-75.82%")
//	[1]  committee diff hands  (signed integer)
//	[2,3]   ask5 price, ask5 vol
//	[4,5]   ask4 ...
//	[6,7]   ask3
//	[8,9]   ask2
//	[10,11] ask1               (closest to last price from above)
//	[12,13] bid1               (closest to last price from below)
//	[14,15] bid2
//	[16,17] bid3
//	[18,19] bid4
//	[20,21] bid5
//	[22]    inner volume (内盘) hands
//	[23]    outer volume (外盘) hands
//	[24]    status flag (e.g. "Z")
//	[25]    aux
//	[26]    aux %
func parsePerformOrderBook(pf []string) AggOrderBook {
	ob := AggOrderBook{
		CommitteeRatioPercent: percentToFloat(pf[0]),
		CommitteeDiffHand:     parseInt64Ptr(pf[1]),
	}
	// asks: ask5..ask1 in raw -> reorder to ask1..ask5 (price ascending from last).
	// We expose ask1 first (level 1) up to ask5 (level 5) as is convention in
	// trading UIs (top-of-book first).
	rawAsks := []AggOrderLvl{
		{Level: 5, Price: parseFloatPtr(pf[2]),  VolumeHand: parseInt64Ptr(pf[3])},
		{Level: 4, Price: parseFloatPtr(pf[4]),  VolumeHand: parseInt64Ptr(pf[5])},
		{Level: 3, Price: parseFloatPtr(pf[6]),  VolumeHand: parseInt64Ptr(pf[7])},
		{Level: 2, Price: parseFloatPtr(pf[8]),  VolumeHand: parseInt64Ptr(pf[9])},
		{Level: 1, Price: parseFloatPtr(pf[10]), VolumeHand: parseInt64Ptr(pf[11])},
	}
	// reverse so level 1 is first
	ob.Asks = []AggOrderLvl{rawAsks[4], rawAsks[3], rawAsks[2], rawAsks[1], rawAsks[0]}
	ob.Bids = []AggOrderLvl{
		{Level: 1, Price: parseFloatPtr(pf[12]), VolumeHand: parseInt64Ptr(pf[13])},
		{Level: 2, Price: parseFloatPtr(pf[14]), VolumeHand: parseInt64Ptr(pf[15])},
		{Level: 3, Price: parseFloatPtr(pf[16]), VolumeHand: parseInt64Ptr(pf[17])},
		{Level: 4, Price: parseFloatPtr(pf[18]), VolumeHand: parseInt64Ptr(pf[19])},
		{Level: 5, Price: parseFloatPtr(pf[20]), VolumeHand: parseInt64Ptr(pf[21])},
	}
	if len(pf) >= 23 {
		ob.InnerVolumeHand = parseInt64Ptr(pf[22])
	}
	if len(pf) >= 24 {
		ob.OuterVolumeHand = parseInt64Ptr(pf[23])
	}
	if len(pf) >= 25 {
		ob.StatusFlag = pf[24]
	}
	return ob
}

// splitAggregateBlocks tokenizes the aggregate body into name->raw_block_text.
//
// We don't use a JSON parser because sohu emits unquoted single-quoted strings
// and stringified-array values like quote_k_r:["quote_k_r","['20260511',...]"].
func splitAggregateBlocks(body string) map[string]string {
	out := map[string]string{}
	matches := reBlock.FindAllStringSubmatchIndex(body, -1)
	for _, m := range matches {
		name := body[m[2]:m[3]]
		val := body[m[4]:m[5]]
		out[name] = val
	}
	return out
}

// stringSliceFromBlock parses ['a','b','c'] -> ["a","b","c"].
func stringSliceFromBlock(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return []string{strings.Trim(s, `"`)}
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		// Single flat array? Or array of arrays?
		inner := s[1 : len(s)-1]
		if strings.HasPrefix(strings.TrimSpace(inner), "[") {
			// array of arrays - take the first one.
			tuples := parseTuples(s)
			if len(tuples) == 0 {
				return nil
			}
			return tuples[0]
		}
		return splitCSVQuoted(inner)
	}
	return splitCSVQuoted(s)
}

// parseQuoteArrayBlock handles blocks shaped as:
//   ['quote_k_r',"['20260511','231.26','249.22',...]","['20260511',...]"]
// where every quote row is itself a stringified array. We unquote each row and
// split it into a string slice.
func parseQuoteArrayBlock(s string) [][]string {
	if s == "" {
		return nil
	}
	// Strip outer brackets.
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	body := s[1 : len(s)-1]
	// Split on top-level commas that separate string elements; the strings may
	// contain commas inside, so we walk balanced double-quoted segments.
	var elems []string
	var cur strings.Builder
	inStr := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		switch ch {
		case '"':
			inStr = !inStr
			cur.WriteByte(ch)
		case ',':
			if inStr {
				cur.WriteByte(ch)
			} else {
				elems = append(elems, strings.TrimSpace(cur.String()))
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		elems = append(elems, strings.TrimSpace(cur.String()))
	}
	out := make([][]string, 0, len(elems))
	for _, e := range elems {
		if !strings.HasPrefix(e, "\"") || !strings.HasSuffix(e, "\"") {
			continue
		}
		// Unwrap double-quote string then parse as a tuple expression like ['a','b'].
		unquoted := strings.Trim(e, "\"")
		// Also unescape any \' or \" if sohu emits them (defensive).
		unquoted = strings.ReplaceAll(unquoted, `\"`, `"`)
		row := stringSliceFromBlock(unquoted)
		if row == nil {
			continue
		}
		out = append(out, row)
	}
	return out
}

func reverseTicks(rows []Tick) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

// parseChineseYi handles "3045.97亿" -> 3045.97e8 yuan; "10亿" -> 10e8;
// returns nil if no recognizable suffix.
func parseChineseYi(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "--" {
		return nil
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(s, "亿"):
		s = strings.TrimSuffix(s, "亿")
		multiplier = 1e8
	case strings.HasSuffix(s, "万"):
		s = strings.TrimSuffix(s, "万")
		multiplier = 1e4
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	out := v * multiplier
	return &out
}

package model

import "time"

// IndicatorDefinition 技术指标定义
type IndicatorDefinition struct {
	IndicatorCode string    `json:"indicator_code"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	Description   string    `json:"description"`
	ParamsSchema  string    `json:"params_schema"`
	OutputFields  string    `json:"output_fields"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IndicatorValue 技术指标值
type IndicatorValue struct {
	Symbol         string                 `json:"symbol"`
	Period        string                 `json:"period"`
	TS            time.Time              `json:"ts"`
	IndicatorCode string                 `json:"indicator_code"`
	Values        map[string]float64     `json:"values"`
}

// PatternDefinition K线形态定义
type PatternDefinition struct {
	PatternCode   string    `json:"pattern_code"`
	Name          string    `json:"name"`
	NameCN        string    `json:"name_cn"`
	Category      string    `json:"category"`
	TALibFunction string    `json:"talib_function"`
	Direction     string    `json:"direction"` // bullish, bearish, neutral, both
	Description   string    `json:"description"`
	ParamsSchema  string    `json:"params_schema"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PatternHit K线形态命中
type PatternHit struct {
	Symbol           string                 `json:"symbol"`
	Period          string                 `json:"period"`
	TS              time.Time              `json:"ts"`
	PatternCode     string                 `json:"pattern_code"`
	PatternName     string                 `json:"pattern_name"`
	Direction       string                 `json:"direction"` // bullish, bearish, neutral
	PatternValue    float64                `json:"pattern_value"` // TA-Lib返回值: <0卖出信号, 0无信号, >0买入信号
	Confidence      float64                `json:"confidence"` // 置信度0-100
	Extra           map[string]any         `json:"extra"` // 额外数据(open/high/low/close等)
	AlgorithmVersion string                `json:"algorithm_version"`
}

// GetAllPatternDefinitions 返回61种K线形态定义
func GetAllPatternDefinitions() []PatternDefinition {
	return []PatternDefinition{
		{PatternCode: "two_crows", Name: "Two Crows", NameCN: "两只乌鸦", Category: "candlestick", TALibFunction: "CDL2CROWS", Direction: "bearish", Description: "在上升趋势中出现两天连续阳线后出现跳空阴线"},
		{PatternCode: "upside_gap_two_crows", Name: "Upside Gap Two Crows", NameCN: "向上跳空的两只乌鸦", Category: "candlestick", TALibFunction: "CDLUPSIDEGAP2CROWS", Direction: "bearish", Description: "向上跳空后出现两只乌鸦形态"},
		{PatternCode: "three_black_crows", Name: "Three Black Crows", NameCN: "三只乌鸦", Category: "candlestick", TALibFunction: "CDL3BLACKCROWS", Direction: "bearish", Description: "连续三天下跌的阴线形态"},
		{PatternCode: "identical_three_crows", Name: "Identical Three Crows", NameCN: "三胞胎乌鸦", Category: "candlestick", TALibFunction: "CDLIDENTICAL3CROWS", Direction: "bearish", Description: "三只完全相同的乌鸦形态"},
		{PatternCode: "three_line_strike", Name: "Three Line Strike", NameCN: "三线打击", Category: "candlestick", TALibFunction: "CDL3LINESTRIKE", Direction: "both", Description: "三根连续阴阳线后反向突破"},
		{PatternCode: "dark_cloud_cover", Name: "Dark Cloud Cover", NameCN: "乌云压顶", Category: "candlestick", TALibFunction: "CDLDARKCLOUDCOVER", Direction: "bearish", Description: "在上升趋势中出现乌云压顶形态"},
		{PatternCode: "evening_doji_star", Name: "Evening Doji Star", NameCN: "十字暮星", Category: "candlestick", TALibFunction: "CDLEVENINGDOJISTAR", Direction: "bearish", Description: "黄昏十字星形态"},
		{PatternCode: "doji_star", Name: "Doji Star", NameCN: "十字星", Category: "candlestick", TALibFunction: "CDLDOJISTAR", Direction: "neutral", Description: "十字星形态，出现在趋势顶部或底部"},
		{PatternCode: "hanging_man", Name: "Hanging Man", NameCN: "上吊线", Category: "candlestick", TALibFunction: "CDLHANGINGMAN", Direction: "bearish", Description: "出现在上升趋势顶部的上吊线形态"},
		{PatternCode: "hikkake_pattern", Name: "Hikkake Pattern", NameCN: "陷阱", Category: "candlestick", TALibFunction: "CDLHIKKAKE", Direction: "neutral", Description: "Hikkake陷阱形态"},
		{PatternCode: "modified_hikkake", Name: "Modified Hikkake", NameCN: "修正陷阱", Category: "candlestick", TALibFunction: "CDLHIKKAKEMOD", Direction: "neutral", Description: "修正版Hikkake陷阱形态"},
		{PatternCode: "in_neck_pattern", Name: "In-Neck Pattern", NameCN: "颈内线", Category: "candlestick", TALibFunction: "CDLINNECK", Direction: "bearish", Description: "在下跌趋势中出现颈内线形态"},
		{PatternCode: "on_neck_pattern", Name: "On-Neck Pattern", NameCN: "颈上线", Category: "candlestick", TALibFunction: "CDLONNECK", Direction: "bearish", Description: "在下跌趋势中出现颈上线形态"},
		{PatternCode: "thrusting_pattern", Name: "Thrusting Pattern", NameCN: "插入", Category: "candlestick", TALibFunction: "CDLTHRUSTING", Direction: "bearish", Description: "在下跌趋势中出现插入形态"},
		{PatternCode: "shooting_star", Name: "Shooting Star", NameCN: "射击之星", Category: "candlestick", TALibFunction: "CDLSHOOTINGSTAR", Direction: "bearish", Description: "射击之星形态，出现在上升趋势顶部"},
		{PatternCode: "stalled_pattern", Name: "Stalled Pattern", NameCN: "停顿形态", Category: "candlestick", TALibFunction: "CDLSTALLEDPATTERN", Direction: "bearish", Description: "停顿形态"},
		{PatternCode: "advance_block", Name: "Advance Block", NameCN: "大敌当前", Category: "candlestick", TALibFunction: "CDLADVANCEBLOCK", Direction: "bearish", Description: "大敌当前形态，上涨动能减弱"},
		{PatternCode: "high_wave_candle", Name: "High Wave Candle", NameCN: "风高浪大线", Category: "candlestick", TALibFunction: "CDLHIGHWAVE", Direction: "neutral", Description: "风高浪大线形态"},
		{PatternCode: "engulfing_pattern", Name: "Engulfing Pattern", NameCN: "吞噬模式", Category: "candlestick", TALibFunction: "CDLENGULFING", Direction: "both", Description: "吞噬形态，看涨或看跌"},
		{PatternCode: "abandoned_baby", Name: "Abandoned Baby", NameCN: "弃婴", Category: "candlestick", TALibFunction: "CDLABANDONEDBABY", Direction: "both", Description: "弃婴形态，底部或顶部反转"},
		{PatternCode: "closing_marubozu", Name: "Closing Marubozu", NameCN: "收盘缺影线", Category: "candlestick", TALibFunction: "CDLCLOSINGMARUBOZU", Direction: "both", Description: "收盘缺影线形态"},
		{PatternCode: "doji", Name: "Doji", NameCN: "十字", Category: "candlestick", TALibFunction: "CDLDOJI", Direction: "neutral", Description: "十字形态，开盘收盘价相近"},
		{PatternCode: "up_down_gap", Name: "Up/Down Gap", NameCN: "向上/下跳空并列阳线", Category: "candlestick", TALibFunction: "CDLGAPSIDESIDEWHITE", Direction: "neutral", Description: "向上/下跳空并列阳线"},
		{PatternCode: "long_legged_doji", Name: "Long Legged Doji", NameCN: "长脚十字", Category: "candlestick", TALibFunction: "CDLLONGLEGGEDDOJI", Direction: "neutral", Description: "长脚十字形态"},
		{PatternCode: "rickshaw_man", Name: "Rickshaw Man", NameCN: "黄包车夫", Category: "candlestick", TALibFunction: "CDLRICKSHAWMAN", Direction: "neutral", Description: "黄包车夫形态"},
		{PatternCode: "marubozu", Name: "Marubozu", NameCN: "光头光脚/缺影线", Category: "candlestick", TALibFunction: "CDLMARUBOZU", Direction: "both", Description: "光头光脚形态"},
		{PatternCode: "three_inside_up_down", Name: "Three Inside Up/Down", NameCN: "三内部上涨和下跌", Category: "candlestick", TALibFunction: "CDL3INSIDE", Direction: "both", Description: "三内部形态"},
		{PatternCode: "three_outside_up_down", Name: "Three Outside Up/Down", NameCN: "三外部上涨和下跌", Category: "candlestick", TALibFunction: "CDL3OUTSIDE", Direction: "both", Description: "三外部形态"},
		{PatternCode: "three_stars_in_the_south", Name: "Three Stars In The South", NameCN: "南方三星", Category: "candlestick", TALibFunction: "CDL3STARSINSOUTH", Direction: "bullish", Description: "南方三星形态，底部反转"},
		{PatternCode: "three_white_soldiers", Name: "Three White Soldiers", NameCN: "三个白兵", Category: "candlestick", TALibFunction: "CDL3WHITESOLDIERS", Direction: "bullish", Description: "三个白兵形态，底部连续上涨"},
		{PatternCode: "belt_hold", Name: "Belt Hold", NameCN: "捉腰带线", Category: "candlestick", TALibFunction: "CDLBELTHOLD", Direction: "both", Description: "捉腰带线形态"},
		{PatternCode: "breakaway", Name: "Breakaway", NameCN: "脱离", Category: "candlestick", TALibFunction: "CDLBREAKAWAY", Direction: "both", Description: "脱离形态"},
		{PatternCode: "concealing_baby_swallow", Name: "Concealing Baby Swallow", NameCN: "藏婴吞没", Category: "candlestick", TALibFunction: "CDLCONCEALBABYSWALL", Direction: "bullish", Description: "藏婴吞没形态"},
		{PatternCode: "counterattack", Name: "Counterattack", NameCN: "反击线", Category: "candlestick", TALibFunction: "CDLCOUNTERATTACK", Direction: "neutral", Description: "反击线形态"},
		{PatternCode: "dragonfly_doji", Name: "Dragonfly Doji", NameCN: "蜻蜓十字/T形十字", Category: "candlestick", TALibFunction: "CDLDRAGONFLYDOJI", Direction: "bullish", Description: "蜻蜓十字形态，底部反转信号"},
		{PatternCode: "evening_star", Name: "Evening Star", NameCN: "暮星", Category: "candlestick", TALibFunction: "CDLEVENINGSTAR", Direction: "bearish", Description: "黄昏之星形态，顶部反转"},
		{PatternCode: "gravestone_doji", Name: "Gravestone Doji", NameCN: "墓碑十字/倒T十字", Category: "candlestick", TALibFunction: "CDLGRAVESTONEDOJI", Direction: "bearish", Description: "墓碑十字形态，顶部反转"},
		{PatternCode: "hammer", Name: "Hammer", NameCN: "锤头", Category: "candlestick", TALibFunction: "CDLHAMMER", Direction: "bullish", Description: "锤头形态，底部反转信号"},
		{PatternCode: "harami_pattern", Name: "Harami Pattern", NameCN: "母子线", Category: "candlestick", TALibFunction: "CDLHARAMI", Direction: "both", Description: "母子线形态"},
		{PatternCode: "harami_cross_pattern", Name: "Harami Cross Pattern", NameCN: "十字孕线", Category: "candlestick", TALibFunction: "CDLHARAMICROSS", Direction: "both", Description: "十字孕线形态"},
		{PatternCode: "homing_pigeon", Name: "Homing Pigeon", NameCN: "家鸽", Category: "candlestick", TALibFunction: "CDLHOMINGPIGEON", Direction: "bullish", Description: "家鸽形态，底部形态"},
		{PatternCode: "inverted_hammer", Name: "Inverted Hammer", NameCN: "倒锤头", Category: "candlestick", TALibFunction: "CDLINVERTEDHAMMER", Direction: "bullish", Description: "倒锤头形态"},
		{PatternCode: "kicking", Name: "Kicking", NameCN: "反冲形态", Category: "candlestick", TALibFunction: "CDLKICKING", Direction: "both", Description: "反冲形态"},
		{PatternCode: "kicking_bull_bear", Name: "Kicking By Length", NameCN: "由较长缺影线决定的反冲形态", Category: "candlestick", TALibFunction: "CDLKICKINGBYLENGTH", Direction: "both", Description: "由较长缺影线决定的反冲形态"},
		{PatternCode: "ladder_bottom", Name: "Ladder Bottom", NameCN: "梯底", Category: "candlestick", TALibFunction: "CDLLADDERBOTTOM", Direction: "bullish", Description: "梯底形态"},
		{PatternCode: "long_line_candle", Name: "Long Line Candle", NameCN: "长蜡烛", Category: "candlestick", TALibFunction: "CDLLONGLINE", Direction: "both", Description: "长蜡烛形态"},
		{PatternCode: "matching_low", Name: "Matching Low", NameCN: "相同低价", Category: "candlestick", TALibFunction: "CDLMATCHINGLOW", Direction: "bullish", Description: "相同低价形态"},
		{PatternCode: "mat_hold", Name: "Mat Hold", NameCN: "铺垫", Category: "candlestick", TALibFunction: "CDLMATHOLD", Direction: "bullish", Description: "铺垫形态"},
		{PatternCode: "morning_doji_star", Name: "Morning Doji Star", NameCN: "十字晨星", Category: "candlestick", TALibFunction: "CDLMORNINGDOJISTAR", Direction: "bullish", Description: "早晨十字星形态"},
		{PatternCode: "morning_star", Name: "Morning Star", NameCN: "晨星", Category: "candlestick", TALibFunction: "CDLMORNINGSTAR", Direction: "bullish", Description: "早晨之星形态"},
		{PatternCode: "piercing_pattern", Name: "Piercing Pattern", NameCN: "刺透形态", Category: "candlestick", TALibFunction: "CDLPIERCING", Direction: "bullish", Description: "刺透形态，底部反转"},
		{PatternCode: "rising_falling_three", Name: "Rising/Falling Three", NameCN: "上升/下降三法", Category: "candlestick", TALibFunction: "CDLRISEFALL3METHODS", Direction: "both", Description: "上升/下降三法形态"},
		{PatternCode: "separating_lines", Name: "Separating Lines", NameCN: "分离线", Category: "candlestick", TALibFunction: "CDLSEPARATINGLINES", Direction: "neutral", Description: "分离线形态"},
		{PatternCode: "short_line_candle", Name: "Short Line Candle", NameCN: "短蜡烛", Category: "candlestick", TALibFunction: "CDLSHORTLINE", Direction: "neutral", Description: "短蜡烛形态"},
		{PatternCode: "spinning_top", Name: "Spinning Top", NameCN: "纺锤", Category: "candlestick", TALibFunction: "CDLSPINNINGTOP", Direction: "neutral", Description: "纺锤形态，趋势停顿信号"},
		{PatternCode: "stick_sandwich", Name: "Stick Sandwich", NameCN: "条形三明治", Category: "candlestick", TALibFunction: "CDLSTICKSANDWICH", Direction: "bullish", Description: "条形三明治形态"},
		{PatternCode: "takuri", Name: "Takuri", NameCN: "探水竿", Category: "candlestick", TALibFunction: "CDLTAKURI", Direction: "bullish", Description: "探水竿形态"},
		{PatternCode: "tasuki_gap", Name: "Tasuki Gap", NameCN: "跳空并列阴阳线", Category: "candlestick", TALibFunction: "CDLTASUKIGAP", Direction: "both", Description: "跳空并列阴阳线形态"},
		{PatternCode: "tristar_pattern", Name: "Tristar Pattern", NameCN: "三星", Category: "candlestick", TALibFunction: "CDLTRISTAR", Direction: "both", Description: "三星形态"},
		{PatternCode: "unique_3_river", Name: "Unique 3 River", NameCN: "奇特三河床", Category: "candlestick", TALibFunction: "CDLUNIQUE3RIVER", Direction: "bullish", Description: "奇特三河床形态"},
		{PatternCode: "upside_downside_gap", Name: "Upside/Downside Gap 3 Methods", NameCN: "上升/下降跳空三法", Category: "candlestick", TALibFunction: "CDLXSIDEGAP3METHODS", Direction: "neutral", Description: "上升/下降跳空三法形态"},
	}
}

"""美股 / 港股 / 全球 ticker 适配器，基于 yfinance。

接口契约（与 CN adapter 对齐）：
- snapshot(symbol) -> {"item": {symbol, name, market, price, pct_change, ...}}
- daily(symbol, start, end, limit) -> {"items": [{ts, open, close, high, low, volume, ...}]}
- minute(symbol, period) -> {"items": [...]}, period ∈ {1m, 5m, 30m, 1h}
- profile(symbol) -> {"item": {symbol, name, market, board, industry, market_cap, ...}}

注意：
- yfinance 在 free tier 上没有 SLA；本 adapter 内置 3 次重试 + 2s 间隔。
- 盘前/盘后数据通过 yfinance 默认行为合并，目前未单独区分（后续按需补充）。
"""
from __future__ import annotations

import math
import time
from typing import Any

# yfinance 是重依赖，延迟导入避免 module import 时副作用
def _yf():
    import yfinance as yf  # noqa: WPS433
    return yf


# yfinance period ↔ 项目内 interval 的映射
_PERIOD_MAP = {
    "1m": "1m",
    "5m": "5m",
    "15m": "15m",
    "30m": "30m",
    "1h": "60m",
    "1d": "1d",
}


def _retry(callable_, *args, **kwargs):
    last: Exception | None = None
    for i in range(3):
        try:
            return callable_(*args, **kwargs)
        except Exception as exc:  # noqa: BLE001
            last = exc
            if i < 2:
                time.sleep(2.0 * (i + 1))
    assert last is not None
    raise last


def _f(v: Any) -> float:
    """安全转 float：None/NaN/Inf 都视为 0.0。"""
    if v is None:
        return 0.0
    try:
        x = float(v)
    except (TypeError, ValueError):
        return 0.0
    if not math.isfinite(x):
        return 0.0
    return x


def _f_or_none(v: Any):
    """安全转 float，缺失/NaN 时返回 None（前端可识别"无数据"）。"""
    if v is None:
        return None
    try:
        x = float(v)
    except (TypeError, ValueError):
        return None
    if not math.isfinite(x):
        return None
    return x


def snapshot(symbol: str) -> dict[str, object]:
    yf = _yf()
    ticker = _retry(yf.Ticker, symbol)
    # fast_info 在 yfinance 新版本里是 O(1) 的浅快照接口，避免触发 info() 全量爬虫
    fi = getattr(ticker, "fast_info", None) or {}
    # 兼容 dict 和 FastInfo 对象
    def g(key: str, default=None):
        try:
            return fi[key]
        except (KeyError, TypeError):
            return getattr(fi, key, default)

    last_price = _f(g("last_price") or g("regular_market_price"))
    prev_close = _f(g("previous_close") or g("regular_market_previous_close"))
    pct = ((last_price - prev_close) / prev_close * 100) if prev_close else 0.0

    return {
        "item": {
            "symbol": symbol,
            "name": symbol,  # yfinance Ticker.info 会触发慢请求，先用 symbol 兜底
            "market": "US",
            "price": last_price,
            "pct_change": pct,
            "amount": _f(g("day_volume") or 0) * last_price,
            "volume": _f(g("day_volume")),
            "turnover_rate": 0.0,
            "turnover_amount": 0.0,
            "volume_ratio": 0.0,
            "iopv": None,           # 美股个股无 IOPV 概念
            "premium_ratio": None,
            "big_order_volume": None,
            "medium_order_volume": None,
            "small_order_volume": None,
            # yfinance fast_info 不返回完整盘口，五档统一为 None
            "bid_1_price": _f_or_none(g("bid")),
            "bid_1_volume": _f_or_none(g("bid_size")),
            "bid_2_price": None, "bid_2_volume": None,
            "bid_3_price": None, "bid_3_volume": None,
            "bid_4_price": None, "bid_4_volume": None,
            "bid_5_price": None, "bid_5_volume": None,
            "ask_1_price": _f_or_none(g("ask")),
            "ask_1_volume": _f_or_none(g("ask_size")),
            "ask_2_price": None, "ask_2_volume": None,
            "ask_3_price": None, "ask_3_volume": None,
            "ask_4_price": None, "ask_4_volume": None,
            "ask_5_price": None, "ask_5_volume": None,
        }
    }


def daily(symbol: str, start: str = "", end: str = "", limit: int = 240) -> dict[str, object]:
    yf = _yf()
    ticker = _retry(yf.Ticker, symbol)
    # 用 period 兜底；start/end 仅在显式给出时使用
    kwargs = {"period": "2y" if not (start or end) else "max"}
    if start:
        kwargs["start"] = start
    if end:
        kwargs["end"] = end
    df = _retry(ticker.history, interval="1d", **kwargs)
    items: list[dict[str, object]] = []
    if df is None or df.empty:
        return {"items": items, "market": "US"}
    df = df.tail(max(1, int(limit)))
    for ts, row in df.iterrows():
        items.append({
            "ts": str(ts.date()) if hasattr(ts, "date") else str(ts),
            "open": _f(row.get("Open")),
            "close": _f(row.get("Close")),
            "high": _f(row.get("High")),
            "low": _f(row.get("Low")),
            "volume": _f(row.get("Volume")),
            "amount": _f(row.get("Close")) * _f(row.get("Volume")),
            "turnover_rate": 0.0,
            "pct_change": 0.0,
            "requested_period": "1d",
            "source_period": "daily",
        })
    # 简易补算 pct_change（基于前一根 close）
    for i in range(1, len(items)):
        prev = items[i - 1]["close"]
        if prev:
            items[i]["pct_change"] = (items[i]["close"] - prev) / prev * 100
    return {"items": items, "market": "US"}


def minute(symbol: str, period: str) -> dict[str, object]:
    yf = _yf()
    ticker = _retry(yf.Ticker, symbol)
    yf_period = _PERIOD_MAP.get(period, "1m")
    # 1m 数据 yfinance 限制最多 7 天；5m/15m 60 天；1h 730 天
    days = {"1m": "5d", "5m": "60d", "15m": "60d", "30m": "60d", "60m": "60d"}.get(yf_period, "5d")
    df = _retry(ticker.history, period=days, interval=yf_period)
    items: list[dict[str, object]] = []
    if df is None or df.empty:
        return {"items": items, "market": "US"}
    for ts, row in df.iterrows():
        items.append({
            "ts": ts.isoformat() if hasattr(ts, "isoformat") else str(ts),
            "open": _f(row.get("Open")),
            "close": _f(row.get("Close")),
            "high": _f(row.get("High")),
            "low": _f(row.get("Low")),
            "volume": _f(row.get("Volume")),
            "amount": _f(row.get("Close")) * _f(row.get("Volume")),
            "turnover_rate": 0.0,
            "requested_period": period,
            "source_period": yf_period,
        })
    return {"items": items, "market": "US"}


def profile(symbol: str) -> dict[str, object]:
    # yfinance Ticker.info 是慢请求（爬整页），只在 profile 模式调用
    yf = _yf()
    info: dict[str, Any] = {}
    try:
        info = _retry(lambda: _yf().Ticker(symbol).info) or {}
    except Exception:  # noqa: BLE001
        info = {}
    return {
        "item": {
            "symbol": symbol,
            "name": str(info.get("longName") or info.get("shortName") or symbol),
            "market": "US",
            "board": str(info.get("quoteType", "EQUITY")),
            "industry": str(info.get("industry", "")),
            "listed_at": "",
            "market_cap": _f(info.get("marketCap")),
            "float_market_cap": _f(info.get("floatShares") or 0) * _f(info.get("currentPrice") or 0),
            "exchange": str(info.get("exchange", "")),
            "currency": str(info.get("currency", "USD")),
        }
    }

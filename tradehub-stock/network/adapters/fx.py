"""汇率适配器，基于 Frankfurter 开放 API（https://frankfurter.dev/）。

无需 API key；日频数据；适合作为研究基线汇率。
盘中实时汇率可后续接 yfinance "USDCNY=X" 等代码当 fallback。

symbol 约定：
- "USD/CNY"  -> base=USD, quote=CNY
- "USD/KRW"  -> base=USD, quote=KRW
- 也兼容 "USDCNY" / "USDKRW" 无分隔写法

接口契约同 us.py：snapshot / daily / minute / profile。
"""
from __future__ import annotations

import math
import time
from typing import Any

import requests

_BASE_URL = "https://api.frankfurter.dev/v1"
_TIMEOUT = 8


def _parse_pair(symbol: str) -> tuple[str, str]:
    s = symbol.upper().strip()
    if "/" in s:
        base, quote = s.split("/", 1)
    elif len(s) == 6:
        base, quote = s[:3], s[3:]
    else:
        raise ValueError(f"invalid fx symbol: {symbol!r}")
    return base, quote


def _f(v: Any) -> float:
    if v is None:
        return 0.0
    try:
        x = float(v)
    except (TypeError, ValueError):
        return 0.0
    return x if math.isfinite(x) else 0.0


def _request(path: str, **params) -> dict:
    last: Exception | None = None
    for i in range(3):
        try:
            resp = requests.get(f"{_BASE_URL}{path}", params=params, timeout=_TIMEOUT)
            resp.raise_for_status()
            return resp.json()
        except Exception as exc:  # noqa: BLE001
            last = exc
            if i < 2:
                time.sleep(1.0 * (i + 1))
    assert last is not None
    raise last


def snapshot(symbol: str) -> dict[str, object]:
    base, quote = _parse_pair(symbol)
    # 一次区间查询同时拿到最新与前一日，省一次 HTTP。
    # 用近 14 天兜底节假日窗口（汇率市场周末没有数据点）。
    from datetime import date, timedelta
    start_ = (date.today() - timedelta(days=14)).isoformat()
    data = _request(f"/{start_}..", base=base, symbols=quote)
    rates_map: dict[str, dict[str, float]] = data.get("rates", {}) if isinstance(data, dict) else {}
    dates_sorted = sorted(rates_map.keys())
    rate = _f(rates_map[dates_sorted[-1]].get(quote)) if dates_sorted else 0.0
    prev = _f(rates_map[dates_sorted[-2]].get(quote)) if len(dates_sorted) >= 2 else rate
    pct = ((rate - prev) / prev * 100) if prev else 0.0

    return {
        "item": {
            "symbol": symbol,
            "name": f"{base}/{quote}",
            "market": "FX",
            "price": rate,
            "pct_change": pct,
            "amount": 0.0,
            "volume": 0.0,
            "turnover_rate": 0.0,
            "turnover_amount": 0.0,
            "volume_ratio": 0.0,
            "iopv": None,
            "premium_ratio": None,
            "big_order_volume": None,
            "medium_order_volume": None,
            "small_order_volume": None,
            "bid_1_price": None, "bid_1_volume": None,
            "bid_2_price": None, "bid_2_volume": None,
            "bid_3_price": None, "bid_3_volume": None,
            "bid_4_price": None, "bid_4_volume": None,
            "bid_5_price": None, "bid_5_volume": None,
            "ask_1_price": None, "ask_1_volume": None,
            "ask_2_price": None, "ask_2_volume": None,
            "ask_3_price": None, "ask_3_volume": None,
            "ask_4_price": None, "ask_4_volume": None,
            "ask_5_price": None, "ask_5_volume": None,
        }
    }


def daily(symbol: str, start: str = "", end: str = "", limit: int = 240) -> dict[str, object]:
    base, quote = _parse_pair(symbol)
    # Frankfurter 区间查询：/<from>..<to>?base=&symbols=
    # 不给 start 时默认拿近 2 年
    from datetime import date, timedelta
    if not start:
        start = (date.today() - timedelta(days=2 * 365)).isoformat()
    if not end:
        end = date.today().isoformat()
    data = _request(f"/{start}..{end}", base=base, symbols=quote)
    rates_map: dict[str, dict[str, float]] = data.get("rates", {})
    items: list[dict[str, object]] = []
    for d in sorted(rates_map.keys()):
        rate = _f(rates_map[d].get(quote))
        items.append({
            "ts": d,
            "open": rate, "close": rate, "high": rate, "low": rate,
            "volume": 0.0, "amount": 0.0, "turnover_rate": 0.0,
            "pct_change": 0.0,
            "requested_period": "1d",
            "source_period": "daily",
        })
    # 补 pct_change（基于前一日）
    for i in range(1, len(items)):
        prev = items[i - 1]["close"]
        if prev:
            items[i]["pct_change"] = (items[i]["close"] - prev) / prev * 100
    items = items[-max(1, int(limit)):]
    return {"items": items, "market": "FX"}


def minute(symbol: str, period: str) -> dict[str, object]:
    # Frankfurter 不提供分钟级数据；返回空，让上层 fallback 或忽略
    return {"items": [], "market": "FX", "note": "Frankfurter 仅提供日频汇率"}


def profile(symbol: str) -> dict[str, object]:
    base, quote = _parse_pair(symbol)
    return {
        "item": {
            "symbol": symbol,
            "name": f"{base}/{quote} 即期汇率",
            "market": "FX",
            "board": "FX",
            "industry": "外汇",
            "listed_at": "",
            "market_cap": 0.0,
            "float_market_cap": 0.0,
            "exchange": "Frankfurter",
            "currency": quote,
        }
    }

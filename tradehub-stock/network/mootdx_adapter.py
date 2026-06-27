#!/usr/bin/env python3.11
from __future__ import annotations

import json
import math
import re
import sys
from datetime import datetime
from typing import Any


TDX_FREQUENCY_MAP = {
    "5m": 0,
    "15m": 1,
    "30m": 2,
    "1h": 3,
    "60m": 3,
    "1d": 9,
    "day": 9,
    "daily": 9,
    "week": 5,
    "1w": 5,
    "month": 6,
    "1mo": 6,
    # mootdx has both extension-market 1m(7) and standard-market 1m(8).
    "1m": 8,
}


def normalize_symbol(symbol: str) -> str:
    symbol = (symbol or "").strip()
    if symbol.lower().startswith(("sh", "sz")):
        return symbol[2:]
    return symbol


def classify_market(symbol: str) -> str:
    s = normalize_symbol(symbol)
    if s.startswith(("5", "15", "16")):
        return "CN-ETF"
    return "CN-A"


def _finite_float(value: Any) -> float:
    if value in ("", None):
        return 0.0
    if isinstance(value, (int, float)):
        x = float(value)
    else:
        x = float(str(value).replace(",", ""))
    if math.isnan(x) or math.isinf(x):
        return 0.0
    return x


def _optional_float(value: Any) -> float | None:
    try:
        return _finite_float(value)
    except Exception:
        return None


def _json_default(value: Any) -> str:
    if isinstance(value, (datetime,)):
        return value.isoformat()
    return str(value)


def _df_rows(df) -> list[dict[str, Any]]:
    if df is None or df.empty:
        return []
    frame = df.copy()
    if "datetime" in frame.columns:
        frame = frame.drop(columns=["datetime"])
    frame = frame.reset_index()
    return frame.fillna("").to_dict(orient="records")


def _client():
    from mootdx.quotes import Quotes

    return Quotes.factory(market="std")


def _frequency(period: str) -> int:
    key = (period or "1d").strip().lower()
    if key not in TDX_FREQUENCY_MAP:
        raise ValueError(f"unsupported mootdx period: {period}")
    return TDX_FREQUENCY_MAP[key]


def health(symbol: str = "600519") -> dict[str, Any]:
    symbol = normalize_symbol(symbol)
    client = _client()
    rows = _df_rows(client.bars(symbol=symbol, frequency=9, offset=1))
    return {
        "ok": bool(rows),
        "provider": "mootdx",
        "symbol": symbol,
        "server": getattr(client, "client", None).__dict__.get("ip", "") if getattr(client, "client", None) else "",
        "sample_count": len(rows),
        "sample": rows[-1] if rows else None,
    }


def daily(symbol: str, start: str = "", end: str = "", limit: int = 240) -> dict[str, Any]:
    symbol = normalize_symbol(symbol)
    limit = max(1, min(int(limit or 240), 800))
    rows = _df_rows(_client().bars(symbol=symbol, frequency=9, offset=limit))
    items = []
    for row in rows[-limit:]:
        ts = str(row.get("datetime") or row.get("date") or row.get("index") or "")
        items.append({
            "symbol": symbol,
            "market": classify_market(symbol),
            "ts": ts,
            "open": _finite_float(row.get("open")),
            "close": _finite_float(row.get("close")),
            "high": _finite_float(row.get("high")),
            "low": _finite_float(row.get("low")),
            "volume": _finite_float(row.get("vol") or row.get("volume")),
            "amount": _finite_float(row.get("amount")),
            "requested_period": "1d",
            "source_period": "mootdx:9",
            "source": "mootdx",
        })
    return {"items": items, "market": classify_market(symbol), "source": "mootdx"}


def minute(symbol: str, period: str = "5m", limit: int = 240) -> dict[str, Any]:
    symbol = normalize_symbol(symbol)
    limit = max(1, min(int(limit or 240), 800))
    frequency = _frequency(period)
    rows = _df_rows(_client().bars(symbol=symbol, frequency=frequency, offset=limit))
    items = []
    for row in rows[-limit:]:
        ts = str(row.get("datetime") or row.get("date") or row.get("index") or "")
        items.append({
            "symbol": symbol,
            "ts": ts,
            "open": _finite_float(row.get("open")),
            "close": _finite_float(row.get("close")),
            "high": _finite_float(row.get("high")),
            "low": _finite_float(row.get("low")),
            "volume": _finite_float(row.get("vol") or row.get("volume")),
            "amount": _finite_float(row.get("amount")),
            "requested_period": period,
            "source_period": f"mootdx:{frequency}",
            "source": "mootdx",
        })
    return {"items": items, "source": "mootdx"}


def snapshot(symbol: str) -> dict[str, Any]:
    symbol = normalize_symbol(symbol)
    rows = _df_rows(_client().quotes(symbol=[symbol]))
    if not rows:
        return {"item": {}, "source": "mootdx"}
    row = rows[0]
    item = {
        "symbol": symbol,
        "name": str(row.get("name") or symbol),
        "market": classify_market(symbol),
        "price": _finite_float(row.get("price") or row.get("last_close")),
        "pct_change": _finite_float(row.get("涨幅") or row.get("percent")),
        "open": _optional_float(row.get("open")),
        "high": _optional_float(row.get("high")),
        "low": _optional_float(row.get("low")),
        "amount": _finite_float(row.get("amount")),
        "volume": _finite_float(row.get("vol") or row.get("volume")),
        "turnover_amount": _finite_float(row.get("amount")),
        "source": "mootdx",
    }
    for i in range(1, 6):
        item[f"bid_{i}_price"] = _optional_float(row.get(f"bid{i}") or row.get(f"bid_{i}") or row.get(f"买{i}价"))
        item[f"bid_{i}_volume"] = _optional_float(row.get(f"bid_vol{i}") or row.get(f"bid{i}_vol") or row.get(f"买{i}量"))
        item[f"ask_{i}_price"] = _optional_float(row.get(f"ask{i}") or row.get(f"ask_{i}") or row.get(f"卖{i}价"))
        item[f"ask_{i}_volume"] = _optional_float(row.get(f"ask_vol{i}") or row.get(f"ask{i}_vol") or row.get(f"卖{i}量"))
    return {"item": item, "source": "mootdx"}


def transactions(symbol: str, start: int = 0, count: int = 200) -> dict[str, Any]:
    symbol = normalize_symbol(symbol)
    count = max(1, min(int(count or 200), 1800))
    rows = _df_rows(_client().transactions(symbol=symbol, start=int(start or 0), offset=count))
    items = []
    for row in rows:
        items.append({
            "symbol": symbol,
            "time": str(row.get("time") or row.get("datetime") or row.get("index") or ""),
            "price": _finite_float(row.get("price")),
            "volume": _finite_float(row.get("vol") or row.get("volume")),
            "amount": _optional_float(row.get("amount")),
            "buy_or_sell": row.get("buyorsell", row.get("buy_or_sell", "")),
            "source": "mootdx",
        })
    return {"items": items, "source": "mootdx"}


def main() -> None:
    if len(sys.argv) < 2:
        raise ValueError("usage: mootdx_adapter.py <health|daily|minute|snapshot|transactions> ...")
    mode = sys.argv[1]
    if mode == "health":
        payload = health(sys.argv[2] if len(sys.argv) > 2 else "600519")
    elif mode == "daily":
        symbol = sys.argv[2]
        start = sys.argv[3] if len(sys.argv) > 3 else ""
        end = sys.argv[4] if len(sys.argv) > 4 else ""
        limit = int(sys.argv[5]) if len(sys.argv) > 5 else 240
        payload = daily(symbol, start, end, limit)
    elif mode == "minute":
        symbol = sys.argv[2]
        period = sys.argv[3] if len(sys.argv) > 3 else "5m"
        limit = int(sys.argv[4]) if len(sys.argv) > 4 else 240
        payload = minute(symbol, period, limit)
    elif mode == "snapshot":
        payload = snapshot(sys.argv[2])
    elif mode == "transactions":
        symbol = sys.argv[2]
        start = int(sys.argv[3]) if len(sys.argv) > 3 else 0
        count = int(sys.argv[4]) if len(sys.argv) > 4 else 200
        payload = transactions(symbol, start, count)
    else:
        raise ValueError(f"unsupported mode: {mode}")
    print(json.dumps(payload, ensure_ascii=False, default=_json_default))


if __name__ == "__main__":
    main()

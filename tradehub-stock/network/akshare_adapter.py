#!/usr/bin/env python3.11
from __future__ import annotations

import json
import math
import os
import re
import sys
import time
import warnings
from urllib.parse import urlencode

import pandas as pd
import requests


SPOT_CACHE_PATH = "/tmp/akshare_stock_spot_cache.json"
SPOT_CACHE_TTL_SECONDS = 60
ETF_CACHE_PATH = "/tmp/akshare_etf_spot_cache.json"
ETF_CACHE_TTL_SECONDS = 60
PROFILE_CACHE_DIR = "/tmp/akshare_profile_cache"
PROFILE_CACHE_TTL_SECONDS = 60 * 60 * 6  # 板块/行业资料 6 小时
SECTOR_BOARD_CACHE_DIR = "/tmp/akshare_sector_board_cache"
SECTOR_BOARD_CACHE_TTL_SECONDS = 60


def classify_board(symbol: str, market_hint: str = "") -> str:
    """按代码前缀粗分板块（沪深主板/创业板/科创板/北交所/ETF/LOF）"""
    s = (symbol or "").strip()
    if not s:
        return "未知"
    if market_hint == "CN-ETF":
        return "ETF"
    head = s[:3] if len(s) >= 3 else s
    head1 = s[0]
    if s.startswith("688") or s.startswith("689"):
        return "科创板"
    if s.startswith("300") or s.startswith("301"):
        return "创业板"
    if s.startswith("60") or s.startswith("601") or s.startswith("603") or s.startswith("605"):
        return "沪市主板"
    if s.startswith("000") or s.startswith("001") or s.startswith("002") or s.startswith("003"):
        return "深市主板"
    if head1 in ("4", "8", "9") or s.startswith("83") or s.startswith("87"):
        return "北交所"
    if s.startswith("5") or s.startswith("15"):
        return "ETF"
    if s.startswith("16") or s.startswith("50") or s.startswith("51"):
        return "LOF/ETF"
    return "其他"

warnings.filterwarnings("ignore")


def normalize_symbol(symbol: str) -> str:
    symbol = symbol.strip()
    if symbol.startswith(("sh", "sz")):
        return symbol[2:]
    return symbol


def _to_float(value: object) -> float:
    if value in ("", None):
        return 0.0
    if isinstance(value, float):
        # pandas 缺失值会以 float('nan') 出现，这里视为无值；
        # JSON 不支持 NaN/Inf，序列化后 Go 端会解码失败。
        if not math.isfinite(value):
            raise ValueError("non-finite float (NaN/Inf)")
        return value
    if isinstance(value, int):
        return float(value)
    return float(str(value).replace(",", ""))


def _safe_get(item: dict[str, object], *keys: str) -> float:
    """按顺序尝试 keys，命中则返回 float；都缺失或无法转 float 时返回 0.0。"""
    for key in keys:
        if key in item:
            try:
                return _to_float(item.get(key))
            except Exception:
                continue
    return 0.0


def _safe_get_or_none(item: dict[str, object], *keys: str):
    """同 _safe_get 但缺失时返回 None。前端可据此区分『无此字段』与『真为 0』。"""
    for key in keys:
        if key in item:
            try:
                return _to_float(item.get(key))
            except Exception:
                continue
    return None


def _retry(func, *args, **kwargs):
    last_error: Exception | None = None
    for attempt in range(3):
        try:
            return func(*args, **kwargs)
        except Exception as exc:  # noqa: BLE001
            last_error = exc
            if attempt < 2:
                time.sleep(1.5 * (attempt + 1))
    if last_error is not None:
        raise last_error
    raise RuntimeError("unexpected empty retry state")


def _retry_quick(func, *args, **kwargs):
    last_error: Exception | None = None
    for attempt in range(2):
        try:
            return func(*args, **kwargs)
        except Exception as exc:  # noqa: BLE001
            last_error = exc
            if attempt < 1:
                time.sleep(0.8)
    if last_error is not None:
        raise last_error
    raise RuntimeError("unexpected empty retry state")


def _load_spot_cache() -> pd.DataFrame | None:
    if not os.path.exists(SPOT_CACHE_PATH):
        return None
    try:
        if time.time() - os.path.getmtime(SPOT_CACHE_PATH) > SPOT_CACHE_TTL_SECONDS:
            return None
        return pd.read_json(SPOT_CACHE_PATH)
    except Exception:  # noqa: BLE001
        return None


def _save_spot_cache(df: pd.DataFrame) -> None:
    try:
        df.to_json(SPOT_CACHE_PATH, orient="records", force_ascii=False, date_format="iso")
    except Exception:  # noqa: BLE001
        return


def _load_etf_cache() -> pd.DataFrame | None:
    if not os.path.exists(ETF_CACHE_PATH):
        return None
    try:
        if time.time() - os.path.getmtime(ETF_CACHE_PATH) > ETF_CACHE_TTL_SECONDS:
            return None
        return pd.read_json(ETF_CACHE_PATH)
    except Exception:  # noqa: BLE001
        return None


def _save_etf_cache(df: pd.DataFrame) -> None:
    try:
        df.to_json(ETF_CACHE_PATH, orient="records", force_ascii=False, date_format="iso")
    except Exception:  # noqa: BLE001
        return


def _get_spot_df(ak) -> pd.DataFrame:
    cached = _load_spot_cache()
    if cached is not None and not cached.empty:
        return cached
    df = _retry(ak.stock_zh_a_spot_em)
    _save_spot_cache(df)
    return df


def _get_etf_df(ak) -> pd.DataFrame:
    cached = _load_etf_cache()
    if cached is not None and not cached.empty:
        return cached
    df = _retry(ak.fund_etf_spot_em)
    _save_etf_cache(df)
    return df


def search(keyword: str) -> dict[str, object]:
    import akshare as ak

    exact_symbol = re.fullmatch(r"\d{6}", keyword.strip())

    try:
        etf_df = _get_etf_df(ak)
        if exact_symbol:
            row = etf_df[etf_df["代码"].astype(str) == keyword.strip()].head(1)
            if not row.empty:
                item = row.iloc[0].to_dict()
                return {
                    "items": [
                        {
                            "symbol": str(item.get("代码", keyword.strip())),
                            "name": str(item.get("名称", f"ETF {keyword.strip()}")),
                            "market": "CN-ETF",
                            "board": "ETF",
                        }
                    ]
                }

        df = _get_spot_df(ak)
        stock_matches = df[
            df["代码"].astype(str).str.contains(keyword, na=False)
            | df["名称"].astype(str).str.contains(keyword, na=False)
        ].head(10)
        etf_matches = etf_df[
            etf_df["代码"].astype(str).str.contains(keyword, na=False)
            | etf_df["名称"].astype(str).str.contains(keyword, na=False)
        ].head(10)
        items = []
        for _, row in stock_matches.iterrows():
            sym = str(row["代码"])
            items.append(
                {
                    "symbol": sym,
                    "name": str(row["名称"]),
                    "market": "CN-A",
                    "board": classify_board(sym, "CN-A"),
                }
            )
        for _, row in etf_matches.iterrows():
            sym = str(row["代码"])
            items.append(
                {
                    "symbol": sym,
                    "name": str(row["名称"]),
                    "market": "CN-ETF",
                    "board": "ETF",
                }
            )
        return {"items": items}
    except Exception:  # noqa: BLE001
        symbol_match = re.search(r"\d{6}", keyword)
        if symbol_match:
            symbol = symbol_match.group(0)
            return {
                "items": [
                    {
                        "symbol": symbol,
                        "name": f"待确认标的 {symbol}",
                        "market": "CN-A",
                        "board": classify_board(symbol, "CN-A"),
                    }
                ]
            }
        raise


def snapshot(symbol: str) -> dict[str, object]:
    import akshare as ak

    symbol = normalize_symbol(symbol)
    item: dict[str, object] = {}
    market = "CN-A"
    # 按代码前缀判断更可能是 ETF 还是 A股，优先查命中率高的，命中即早退
    likely_etf = symbol.startswith(("5", "15", "16"))
    try:
        if likely_etf:
            etf_df = _get_etf_df(ak)
            etf_row = etf_df[etf_df["代码"].astype(str) == symbol].head(1)
            if not etf_row.empty:
                item = etf_row.iloc[0].to_dict()
                market = "CN-ETF"
            else:
                df = _get_spot_df(ak)
                row = df[df["代码"].astype(str) == symbol].head(1)
                if not row.empty:
                    item = row.iloc[0].to_dict()
        else:
            df = _get_spot_df(ak)
            row = df[df["代码"].astype(str) == symbol].head(1)
            if not row.empty:
                item = row.iloc[0].to_dict()
            else:
                etf_df = _get_etf_df(ak)
                etf_row = etf_df[etf_df["代码"].astype(str) == symbol].head(1)
                if not etf_row.empty:
                    item = etf_row.iloc[0].to_dict()
                    market = "CN-ETF"
    except Exception:  # noqa: BLE001
        item = {}

    if not item:
        return {
            "item": {
                "symbol": symbol,
                "name": f"待确认标的 {symbol}",
                "market": market,
                "price": 0.0,
                "pct_change": 0.0,
                "amount": 0.0,
                "volume": 0.0,
                "turnover_rate": 0.0,
                "turnover_amount": 0.0,
                "volume_ratio": 0.0,
                "iopv": None,
                "premium_ratio": None,
                "big_order_volume": 0.0,
                "medium_order_volume": 0.0,
                "small_order_volume": 0.0,
                "bid_1_price": 0.0,
                "bid_1_volume": 0.0,
                "bid_2_price": 0.0,
                "bid_2_volume": 0.0,
                "bid_3_price": 0.0,
                "bid_3_volume": 0.0,
                "bid_4_price": 0.0,
                "bid_4_volume": 0.0,
                "bid_5_price": 0.0,
                "bid_5_volume": 0.0,
                "ask_1_price": 0.0,
                "ask_1_volume": 0.0,
                "ask_2_price": 0.0,
                "ask_2_volume": 0.0,
                "ask_3_price": 0.0,
                "ask_3_volume": 0.0,
                "ask_4_price": 0.0,
                "ask_4_volume": 0.0,
                "ask_5_price": 0.0,
                "ask_5_volume": 0.0,
            }
        }

    # IOPV 仅 ETF spot 接口返回；A 股个股置 None
    iopv = _safe_get_or_none(item, "IOPV实时估值", "IOPV", "iopv")
    # AKShare 字段名「基金折价率」语义是「负数=溢价」。统一项目口径为「正数=溢价」，因此取负。
    # 同时保留对旧字段名的兼容（个别历史源是『折价率』『溢价率』）。
    raw_discount = _safe_get_or_none(item, "基金折价率", "折价率")
    raw_premium = _safe_get_or_none(item, "溢价率")
    if raw_premium is not None:
        premium_ratio = raw_premium  # 已经是「正数=溢价」语义
    elif raw_discount is not None:
        premium_ratio = -raw_discount  # 折价率取负 → 正数=溢价
    else:
        premium_ratio = None  # 个股没有溢价率概念

    return {
        "item": {
            "symbol": symbol,
            "name": str(item.get("名称", "")),
            "market": market,
            "price": _safe_get(item, "最新价"),
            "pct_change": _safe_get(item, "涨跌幅"),
            "amount": _safe_get(item, "成交额"),
            "volume": _safe_get(item, "成交量"),
            "turnover_rate": _safe_get(item, "换手率"),
            "turnover_amount": _safe_get(item, "换手额", "成交额"),
            "volume_ratio": _safe_get(item, "量比"),
            # ETF 风控核心两字段：iopv + premium_ratio（正数=溢价）
            "iopv": iopv,
            "premium_ratio": premium_ratio,
            "big_order_volume": _safe_get_or_none(item, "大单净流入", "主力净流入", "大单净流入-净额", "主力净流入-净额"),
            "medium_order_volume": _safe_get_or_none(item, "中单净流入", "中单净流入-净额"),
            "small_order_volume": _safe_get_or_none(item, "小单净流入", "小单净流入-净额"),
            # 五档：AKShare ETF spot 仅返回买一/卖一；A 股 spot 也无完整五档。
            # 缺失字段一律返 None，前端据此显示『—』而非误导性 0。
            # 重要：bid_1_volume 不再 fallback『现手』（语义完全不同）。
            "bid_1_price": _safe_get_or_none(item, "买一"),
            "bid_1_volume": _safe_get_or_none(item, "买一量"),
            "bid_2_price": _safe_get_or_none(item, "买二"),
            "bid_2_volume": _safe_get_or_none(item, "买二量"),
            "bid_3_price": _safe_get_or_none(item, "买三"),
            "bid_3_volume": _safe_get_or_none(item, "买三量"),
            "bid_4_price": _safe_get_or_none(item, "买四"),
            "bid_4_volume": _safe_get_or_none(item, "买四量"),
            "bid_5_price": _safe_get_or_none(item, "买五"),
            "bid_5_volume": _safe_get_or_none(item, "买五量"),
            "ask_1_price": _safe_get_or_none(item, "卖一"),
            "ask_1_volume": _safe_get_or_none(item, "卖一量"),
            "ask_2_price": _safe_get_or_none(item, "卖二"),
            "ask_2_volume": _safe_get_or_none(item, "卖二量"),
            "ask_3_price": _safe_get_or_none(item, "卖三"),
            "ask_3_volume": _safe_get_or_none(item, "卖三量"),
            "ask_4_price": _safe_get_or_none(item, "卖四"),
            "ask_4_volume": _safe_get_or_none(item, "卖四量"),
            "ask_5_price": _safe_get_or_none(item, "卖五"),
            "ask_5_volume": _safe_get_or_none(item, "卖五量"),
        }
    }


def minute(symbol: str, period: str) -> dict[str, object]:
    import akshare as ak

    symbol = normalize_symbol(symbol)
    adjusted = period
    if period == "10m":
        adjusted = "5"
    elif period == "1h":
        adjusted = "60"
    elif period.endswith("m"):
        adjusted = period[:-1]

    # AKShare 的分钟接口按品种区分：股票走 stock_zh_a_hist_min_em，ETF/LOF 走 fund_etf_hist_min_em。
    # 错配时下游会 TypeError: 'NoneType' is not subscriptable（akshare 内部不做空值判断）。
    # 这里按 symbol 前缀粗判，再以另一条接口作 fallback；两条都失败才抛错。
    likely_etf = symbol.startswith(("5", "15", "16"))
    primary = ak.fund_etf_hist_min_em if likely_etf else ak.stock_zh_a_hist_min_em
    secondary = ak.stock_zh_a_hist_min_em if likely_etf else ak.fund_etf_hist_min_em

    df = None
    last_err: Exception | None = None
    for fn in (primary, secondary):
        try:
            df = _retry(fn, symbol=symbol, period=adjusted, adjust="")
            if df is not None and not df.empty:
                break
        except Exception as exc:  # noqa: BLE001
            last_err = exc
            df = None
    if df is None or df.empty:
        if last_err is not None:
            raise last_err
        return {"items": []}
    df = df.tail(240).copy().fillna("")
    items = []
    for _, row in df.iterrows():
        data = row.to_dict()
        items.append(
            {
                "symbol": symbol,
                "ts": str(data.get("时间", "")),
                "open": _to_float(data.get("开盘")),
                "close": _to_float(data.get("收盘")),
                "high": _to_float(data.get("最高")),
                "low": _to_float(data.get("最低")),
                "volume": _to_float(data.get("成交量")),
                "amount": _to_float(data.get("成交额")),
                "requested_period": period,
                "source_period": adjusted,
            }
        )
    return {"items": items}


def instruments(market: str = "all", keyword: str = "", board: str = "", limit: int = 200, offset: int = 0) -> dict[str, object]:
    """全市场浏览：按市场 / 关键字 / 板块过滤，分页返回。

    market: all | stock | etf
    board:  沪市主板 / 深市主板 / 创业板 / 科创板 / 北交所 / ETF / LOF/ETF / 其他
    """
    import akshare as ak

    items: list[dict[str, object]] = []
    if market in ("all", "stock"):
        try:
            df = _get_spot_df(ak)
            for _, row in df.iterrows():
                sym = str(row.get("代码", "")).strip()
                if not sym:
                    continue
                items.append({
                    "symbol": sym,
                    "name": str(row.get("名称", "")),
                    "market": "CN-A",
                    "board": classify_board(sym, "CN-A"),
                    "price": _safe_get(row, "最新价"),
                    "pct_change": _safe_get(row, "涨跌幅"),
                    "volume": _safe_get(row, "成交量"),
                    "amount": _safe_get(row, "成交额"),
                    "turnover_rate": _safe_get(row, "换手率"),
                    "market_cap": _safe_get(row, "总市值"),
                })
        except Exception:  # noqa: BLE001
            pass
    if market in ("all", "etf"):
        try:
            df = _get_etf_df(ak)
            for _, row in df.iterrows():
                sym = str(row.get("代码", "")).strip()
                if not sym:
                    continue
                items.append({
                    "symbol": sym,
                    "name": str(row.get("名称", "")),
                    "market": "CN-ETF",
                    "board": "ETF",
                    "price": _safe_get(row, "最新价"),
                    "pct_change": _safe_get(row, "涨跌幅"),
                    "volume": _safe_get(row, "成交量"),
                    "amount": _safe_get(row, "成交额"),
                    "turnover_rate": _safe_get(row, "换手率"),
                    "premium_ratio": _safe_get(row, "折价率", "溢价率", "基金折价率"),
                })
        except Exception:  # noqa: BLE001
            pass

    kw = keyword.strip().lower()
    if kw:
        items = [
            it for it in items
            if kw in str(it.get("symbol", "")).lower() or kw in str(it.get("name", "")).lower()
        ]
    if board.strip():
        items = [it for it in items if it.get("board") == board.strip()]

    total = len(items)
    if offset > 0:
        items = items[offset:]
    if limit > 0:
        items = items[:limit]
    return {"total": total, "items": items}


def _profile_cache_path(symbol: str) -> str:
    os.makedirs(PROFILE_CACHE_DIR, exist_ok=True)
    safe = re.sub(r"[^0-9a-zA-Z_]", "_", symbol)
    return os.path.join(PROFILE_CACHE_DIR, f"{safe}.json")


def _sector_board_cache_path(board_type: str) -> str:
    os.makedirs(SECTOR_BOARD_CACHE_DIR, exist_ok=True)
    safe = re.sub(r"[^0-9a-zA-Z_]", "_", board_type or "industry")
    return os.path.join(SECTOR_BOARD_CACHE_DIR, f"{safe}.json")


def profile(symbol: str) -> dict[str, object]:
    """标的板块/行业资料：A股调用 stock_individual_info_em；ETF 走简版分类。"""
    import akshare as ak

    symbol = normalize_symbol(symbol)
    cache_path = _profile_cache_path(symbol)
    if os.path.exists(cache_path):
        try:
            if time.time() - os.path.getmtime(cache_path) < PROFILE_CACHE_TTL_SECONDS:
                with open(cache_path, "r", encoding="utf-8") as f:
                    return json.load(f)
        except Exception:  # noqa: BLE001
            pass

    market = "CN-A"
    name = ""
    industry = ""
    listed_at = ""
    total_share = 0.0
    float_share = 0.0
    market_cap = 0.0
    float_cap = 0.0

    # ETF 优先尝试
    try:
        etf_df = _get_etf_df(ak)
        etf_row = etf_df[etf_df["代码"].astype(str) == symbol].head(1)
        if not etf_row.empty:
            item = etf_row.iloc[0].to_dict()
            market = "CN-ETF"
            name = str(item.get("名称", ""))
    except Exception:  # noqa: BLE001
        pass

    if market != "CN-ETF":
        try:
            df = _retry(ak.stock_individual_info_em, symbol=symbol)
            kv = {str(r["item"]): r["value"] for _, r in df.iterrows()}
            name = str(kv.get("股票简称", name))
            industry = str(kv.get("行业", ""))
            listed_at = str(kv.get("上市时间", ""))
            total_share = _to_float(kv.get("总股本", 0))
            float_share = _to_float(kv.get("流通股", 0))
            market_cap = _to_float(kv.get("总市值", 0))
            float_cap = _to_float(kv.get("流通市值", 0))
        except Exception:  # noqa: BLE001
            pass

    payload = {
        "item": {
            "symbol": symbol,
            "name": name,
            "market": market,
            "board": classify_board(symbol, market),
            "industry": industry,
            "listed_at": listed_at,
            "total_share": total_share,
            "float_share": float_share,
            "market_cap": market_cap,
            "float_market_cap": float_cap,
        }
    }
    try:
        with open(cache_path, "w", encoding="utf-8") as f:
            json.dump(payload, f, ensure_ascii=False)
    except Exception:  # noqa: BLE001
        pass
    return payload


def sector_boards(board_type: str = "industry", limit: int = 500) -> dict[str, object]:
    """市场板块列表：industry=行业板块，concept=概念板块。"""
    import akshare as ak

    board_type = (board_type or "industry").strip().lower()
    cache_path = _sector_board_cache_path(board_type)
    if os.path.exists(cache_path):
        try:
            if time.time() - os.path.getmtime(cache_path) < SECTOR_BOARD_CACHE_TTL_SECONDS:
                with open(cache_path, "r", encoding="utf-8") as f:
                    return json.load(f)
        except Exception:
            pass

    source = "akshare"
    fallback_error = ""
    if board_type == "concept":
        fetcher = ak.stock_board_concept_name_em
    else:
        fetcher = ak.stock_board_industry_name_em
    try:
        df = _retry_quick(fetcher)
    except Exception as exc:  # noqa: BLE001
        fallback_error = str(exc)
        try:
            df = _eastmoney_sector_boards(board_type, limit)
            source = "eastmoney_push2_fallback"
        except Exception as fallback_exc:  # noqa: BLE001
            return {
                "board_type": board_type,
                "source": "unavailable",
                "total": 0,
                "items": [],
                "error": str(fallback_exc),
                "fallback_error": fallback_error,
            }

    items: list[dict[str, object]] = []
    if df is not None and not df.empty:
        df = df.head(max(1, int(limit))).fillna("")
        for _, row in df.iterrows():
            item = row.to_dict()
            items.append({
                "board_type": board_type,
                "sector_code": str(item.get("板块代码", "")),
                "sector_name": str(item.get("板块名称", "")),
                "latest_price": _safe_get_or_none(item, "最新价"),
                "change_percent": _safe_get_or_none(item, "涨跌幅"),
                "change_amount": _safe_get_or_none(item, "涨跌额"),
                "turnover_rate": _safe_get_or_none(item, "换手率"),
                "amplitude": _safe_get_or_none(item, "振幅"),
                "leading_stock_name": str(item.get("领涨股票", "")),
                "leading_stock_change_percent": _safe_get_or_none(item, "领涨股票-涨跌幅"),
                "market_cap": _safe_get_or_none(item, "总市值"),
                "up_count": _safe_get_or_none(item, "上涨家数"),
                "down_count": _safe_get_or_none(item, "下跌家数"),
                "raw": item,
            })
    payload = {
        "board_type": board_type,
        "source": source,
        "total": len(items),
        "items": items,
    }
    if fallback_error:
        payload["fallback_error"] = fallback_error
    try:
        with open(cache_path, "w", encoding="utf-8") as f:
            json.dump(payload, f, ensure_ascii=False)
    except Exception:
        pass
    return payload


def _eastmoney_sector_boards(board_type: str, limit: int) -> pd.DataFrame:
    board_filter = "m:90+t:3" if board_type == "concept" else "m:90+t:2"
    params = {
        "pn": 1,
        "pz": max(1, int(limit)),
        "po": 1,
        "np": 1,
        "fltt": 2,
        "invt": 2,
        "fid": "f3",
        "fs": board_filter,
        "fields": "f12,f14,f2,f3,f4,f8,f9,f20,f104,f105,f128,f136",
    }
    url = "https://push2.eastmoney.com/api/qt/clist/get?" + urlencode(params)
    response = requests.get(url, headers={"User-Agent": "Mozilla/5.0"}, timeout=6)
    response.raise_for_status()
    rows = (response.json().get("data") or {}).get("diff") or []
    mapped = []
    for row in rows:
        mapped.append({
            "板块代码": row.get("f12"),
            "板块名称": row.get("f14"),
            "最新价": row.get("f2"),
            "涨跌幅": row.get("f3"),
            "涨跌额": row.get("f4"),
            "换手率": row.get("f8"),
            "振幅": row.get("f9"),
            "总市值": row.get("f20"),
            "上涨家数": row.get("f104"),
            "下跌家数": row.get("f105"),
            "领涨股票": row.get("f128"),
            "领涨股票-涨跌幅": row.get("f136"),
        })
    return pd.DataFrame(mapped)


def daily(symbol: str, start: str = "", end: str = "", limit: int = 240) -> dict[str, object]:
    """日线历史。A股: stock_zh_a_hist；ETF: fund_etf_hist_em。返回升序时间。"""
    import akshare as ak

    symbol = normalize_symbol(symbol)
    market = "CN-A"
    df = None

    # 判断是否 ETF
    try:
        etf_df = _get_etf_df(ak)
        if (etf_df["代码"].astype(str) == symbol).any():
            market = "CN-ETF"
    except Exception:  # noqa: BLE001
        pass

    start_date = (start or "19700101").replace("-", "")
    end_date = (end or time.strftime("%Y%m%d", time.localtime())).replace("-", "")

    try:
        if market == "CN-ETF":
            df = _retry(ak.fund_etf_hist_em, symbol=symbol, period="daily", start_date=start_date, end_date=end_date, adjust="")
        else:
            df = _retry(ak.stock_zh_a_hist, symbol=symbol, period="daily", start_date=start_date, end_date=end_date, adjust="")
    except Exception as exc:  # noqa: BLE001
        return {"items": [], "error": str(exc), "market": market}

    if df is None or df.empty:
        return {"items": [], "market": market}

    df = df.tail(max(1, int(limit))).copy().fillna("")
    items: list[dict[str, object]] = []
    for _, row in df.iterrows():
        data = row.to_dict()
        items.append(
            {
                "symbol": symbol,
                "market": market,
                "ts": str(data.get("日期", "")),
                "open": _to_float(data.get("开盘")),
                "close": _to_float(data.get("收盘")),
                "high": _to_float(data.get("最高")),
                "low": _to_float(data.get("最低")),
                "volume": _to_float(data.get("成交量")),
                "amount": _to_float(data.get("成交额")),
                "turnover_rate": _to_float(data.get("换手率")),
                "pct_change": _to_float(data.get("涨跌幅")),
                "requested_period": "1d",
                "source_period": "daily",
            }
        )
    return {"items": items, "market": market}


def etf_risk(symbol: str, limit: int = 120) -> dict[str, object]:
    """ETF 风控数据：实时 IOPV / 折溢价 + 日线收盘 + 折溢价简单分布。"""
    import akshare as ak

    symbol = normalize_symbol(symbol)
    realtime: dict[str, object] = {}
    try:
        etf_df = _get_etf_df(ak)
        row = etf_df[etf_df["代码"].astype(str) == symbol].head(1)
        if not row.empty:
            it = row.iloc[0].to_dict()
            # premium_ratio 口径：正数=溢价。AKShare『基金折价率』负数代表溢价，需取负还原。
            raw_discount = _safe_get_or_none(it, "基金折价率", "折价率")
            raw_premium = _safe_get_or_none(it, "溢价率")
            if raw_premium is not None:
                premium_ratio = raw_premium
            elif raw_discount is not None:
                premium_ratio = -raw_discount
            else:
                premium_ratio = None
            realtime = {
                "symbol": symbol,
                "name": str(it.get("名称", "")),
                "price": _safe_get(it, "最新价"),
                "iopv": _safe_get_or_none(it, "IOPV实时估值", "IOPV", "iopv"),
                "premium_ratio": premium_ratio,
                "pct_change": _safe_get(it, "涨跌幅"),
                "turnover_rate": _safe_get(it, "换手率"),
                "volume": _safe_get(it, "成交量"),
                "amount": _safe_get(it, "成交额"),
            }
    except Exception:  # noqa: BLE001
        realtime = {}

    # 日线数据
    history: list[dict[str, object]] = []
    try:
        start_date = "19700101"
        end_date = time.strftime("%Y%m%d", time.localtime())
        df = _retry(ak.fund_etf_hist_em, symbol=symbol, period="daily",
                    start_date=start_date, end_date=end_date, adjust="")
        if df is not None and not df.empty:
            df = df.tail(max(1, int(limit))).copy().fillna("")
            for _, r in df.iterrows():
                d = r.to_dict()
                history.append({
                    "ts": str(d.get("日期", "")),
                    "open": _to_float(d.get("开盘")),
                    "close": _to_float(d.get("收盘")),
                    "high": _to_float(d.get("最高")),
                    "low": _to_float(d.get("最低")),
                    "volume": _to_float(d.get("成交量")),
                    "amount": _to_float(d.get("成交额")),
                    "turnover_rate": _to_float(d.get("换手率")),
                    "pct_change": _to_float(d.get("涨跌幅")),
                })
    except Exception:  # noqa: BLE001
        history = []

    # 折溢价分布（简化：近 N 日收盘相对均线偏离，作为代理）
    closes = [h["close"] for h in history if isinstance(h.get("close"), (int, float)) and h["close"]]
    distribution: list[dict[str, object]] = []
    if closes:
        avg = sum(closes) / len(closes)
        buckets = [-3, -2, -1, 0, 1, 2, 3]
        counts = {b: 0 for b in buckets}
        for c in closes:
            dev_pct = (c - avg) / avg * 100 if avg else 0
            bucket = max(buckets[0], min(buckets[-1], round(dev_pct)))
            counts[bucket] = counts.get(bucket, 0) + 1
        for b in buckets:
            distribution.append({"bucket_pct": b, "count": counts.get(b, 0)})

    return {
        "realtime": realtime,
        "history": history,
        "distribution": distribution,
    }


_FX_RE = re.compile(r"^[A-Z]{3}(/[A-Z]{3}|[A-Z]{3})$")


def _detect_market(symbol: str) -> str:
    """根据 symbol 字面格式识别市场。

    规则（保守、覆盖常见情况）：
    - 6 位纯数字            -> CN-A / CN-ETF（由下游 spot 表 fallback 决定）
    - 6 位数字 + .KS / .KQ  -> KR
    - "USD/CNY" / "USDCNY"  -> FX
    - 其他纯字母（含 . - ） -> US（yfinance 支持 BRK.A、TSM、SOXX 等格式）
    """
    s = symbol.strip().upper()
    if not s:
        return "UNKNOWN"
    if s.endswith(".KS") or s.endswith(".KQ"):
        return "KR"
    if _FX_RE.match(s):
        return "FX"
    # 纯 6 位数字 -> 中国
    if re.fullmatch(r"\d{6}", s):
        return "CN"
    # 兜底：含字母即视为美股（yfinance 接 NVDA / SOXX / BRK.B / TSM 等）
    if re.fullmatch(r"[A-Z0-9.\-]+", s):
        return "US"
    return "UNKNOWN"


def _resolve_adapter(market: str):
    """根据市场返回 adapter module；CN 路径直接复用本文件内函数。"""
    if market == "CN":
        return sys.modules[__name__]
    if market == "US":
        from adapters import us  # noqa: WPS433
        return us
    if market == "KR":
        from adapters import kr  # noqa: WPS433
        return kr
    if market == "FX":
        from adapters import fx  # noqa: WPS433
        return fx
    raise ValueError(f"unsupported market: {market}")


def main() -> None:
    # 让 adapters 子包可以被 sibling import（CLI 直接 python 运行时 PYTHONPATH 需要含当前目录）
    here = os.path.dirname(os.path.abspath(__file__))
    if here not in sys.path:
        sys.path.insert(0, here)

    mode = sys.argv[1]
    if mode == "search":
        # search 跨市场暂只走 CN（akshare），后续可在此聚合 yfinance.Tickers
        payload = search(sys.argv[2])
    elif mode == "instruments":
        # instruments 仅 CN 全市场（保留原行为）
        market_arg = sys.argv[2] if len(sys.argv) > 2 else "all"
        keyword = sys.argv[3] if len(sys.argv) > 3 else ""
        board = sys.argv[4] if len(sys.argv) > 4 else ""
        try:
            limit = int(sys.argv[5]) if len(sys.argv) > 5 else 200
        except ValueError:
            limit = 200
        try:
            offset = int(sys.argv[6]) if len(sys.argv) > 6 else 0
        except ValueError:
            offset = 0
        payload = instruments(market_arg, keyword, board, limit, offset)
    elif mode == "sector_boards":
        board_type = sys.argv[2] if len(sys.argv) > 2 else "industry"
        try:
            limit = int(sys.argv[3]) if len(sys.argv) > 3 else 500
        except ValueError:
            limit = 500
        payload = sector_boards(board_type, limit)
    elif mode in ("snapshot", "profile", "minute", "daily", "etf_risk"):
        symbol = sys.argv[2]
        market = _detect_market(symbol)
        adapter = _resolve_adapter(market)
        func = getattr(adapter, mode, None)
        if func is None:
            payload = {"error": f"{market} adapter does not support {mode}"}
        elif mode == "snapshot" or mode == "profile":
            payload = func(symbol)
        elif mode == "minute":
            payload = func(symbol, sys.argv[3])
        elif mode == "daily":
            start = sys.argv[3] if len(sys.argv) > 3 else ""
            end = sys.argv[4] if len(sys.argv) > 4 else ""
            try:
                limit = int(sys.argv[5]) if len(sys.argv) > 5 else 240
            except ValueError:
                limit = 240
            payload = func(symbol, start, end, limit)
        elif mode == "etf_risk":
            # etf_risk 仅 CN ETF 有意义；其他市场返回空结构
            if market != "CN":
                payload = {"realtime": {}, "history": [], "distribution": [],
                           "note": f"etf_risk not applicable for market {market}"}
            else:
                try:
                    limit = int(sys.argv[3]) if len(sys.argv) > 3 else 120
                except ValueError:
                    limit = 120
                payload = etf_risk(symbol, limit)
    else:
        raise ValueError(f"unsupported mode: {mode}")
    print(json.dumps(payload, ensure_ascii=False))


if __name__ == "__main__":
    main()

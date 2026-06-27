"""韩国市场适配器（占位）。

pykrx 暂未启用：体积大、对韩国市场专用。如需启用：
1) requirements.txt 增加 `pykrx`
2) 这里实现 snapshot / daily / minute / profile
3) `_detect_market` 已能识别 ".KS" / ".KQ" 后缀

当前所有函数 raise NotImplementedError，避免误用。
"""
from __future__ import annotations


_MSG = "KR adapter not enabled; install pykrx and implement adapters/kr.py"


def snapshot(symbol: str) -> dict[str, object]:
    raise NotImplementedError(_MSG)


def daily(symbol: str, start: str = "", end: str = "", limit: int = 240) -> dict[str, object]:
    raise NotImplementedError(_MSG)


def minute(symbol: str, period: str) -> dict[str, object]:
    raise NotImplementedError(_MSG)


def profile(symbol: str) -> dict[str, object]:
    raise NotImplementedError(_MSG)

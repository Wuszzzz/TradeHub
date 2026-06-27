"""跨市场数据适配器集合。

按市场维度拆分，每个 module 暴露一组同名函数（snapshot / daily / minute / profile），
由 `akshare_adapter.py` 顶层 dispatcher 根据 symbol 自动路由。

当前包含：
- `us`  : 美股（yfinance）
- `fx`  : 汇率（Frankfurter HTTP API）
- `kr`  : 韩国（占位，pykrx 未启用）

CN 市场保留在 `network/akshare_adapter.py` 内（历史原因，避免大规模搬迁）。
"""

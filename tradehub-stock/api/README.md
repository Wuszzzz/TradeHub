# Backend API

Go 后端 HTTP 接口层。监听 `${API_HOST}:${API_PORT}`（默认 `0.0.0.0:8000`）。

## 路由清单

当前保留两套路由：

- 旧路由 `/api/v1/*`：兼容现有前端和脚本，不在第一阶段删除。
- 新路由 `/api/stock/v1/*`、`/api/system/v1/*`：使用统一响应结构 `{success, code, message, data, meta, error}`，后续新增功能优先使用新路由。

### 系统

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查 |
| GET | `/api/v1/overview` | 服务与架构概览 |
| GET | `/api/system/v1/health` | 统一响应健康检查 |
| GET | `/api/system/v1/overview` | 统一响应服务概览 |

### 行情与标的

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/instruments/search?keyword=` | 关键字搜索（含 board 字段） |
| GET | `/api/v1/instruments?market=&keyword=&board=&limit=&offset=` | 全市场浏览（行情快照字段） |
| GET | `/api/v1/instruments/profile?symbol=` | 板块/行业资料（6h 缓存） |
| GET | `/api/v1/market/realtime?symbol=` | 实时快照（五档、资金流） |
| GET | `/api/v1/market/history?symbol=&interval=&limit=` | TDengine 分钟 K 线 |
| GET | `/api/v1/market/daily?symbol=&start=&end=&limit=` | AKShare 日线 |
| GET | `/api/stock/v1/instruments/search?keyword=` | 统一响应关键字搜索 |
| GET | `/api/stock/v1/instruments/profile?symbol=` | 统一响应标的资料 |
| GET | `/api/stock/v1/market/snapshot?symbol=` | 统一响应实时快照 |
| GET | `/api/stock/v1/market/kline?symbol=&period=&limit=` | 统一响应 TDengine K 线 |

### ETF 风控

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/etf/risk?symbol=&limit=` | 实时 IOPV / 折溢价 + 日线 + 偏离分布 |
| GET | `/api/stock/v1/etf/risk?symbol=&limit=` | 统一响应 ETF 风控 |

### 数据采集

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST | `/api/v1/ingestion/tasks` | 任务列表 / 创建 |
| GET / POST / DELETE | `/api/stock/v1/tasks/ingestion` | 统一响应采集任务 |
| GET / POST | `/api/stock/v1/tasks` | 通用任务列表 / 详情 / 创建，支持 `indicator_compute`、`pattern_scan`、`screening`、`backtest`、`ai_report`、`data_ingest` |
| POST | `/api/stock/v1/tasks/status?task_id=` | 更新通用任务状态 |
| GET / POST | `/api/stock/v1/tasks/logs?task_id=` | 通用任务日志列表 / 追加 |

`indicator_compute` 已接入 worker，任务参数示例：

```json
{
  "task_type": "indicator_compute",
  "params": {
    "symbol": "600519",
    "period": "1d",
    "indicator_code": "MACD",
    "limit": 500,
    "params": {
      "fast": 12,
      "slow": 26,
      "signal": 9
    }
  }
}
```

`pattern_scan` 已接入 worker，任务参数示例：

```json
{
  "task_type": "pattern_scan",
  "params": {
    "symbol": "600519",
    "period": "1d",
    "limit": 500,
    "patterns": ["doji", "engulfing_pattern", "three_white_soldiers"]
  }
}
```

不传 `patterns` 时默认扫描 61 种 K 线形态；当前算法版本为 `go-candlestick-v1`，后续可替换为 TA-Lib 精确引擎。

`screening` 已接入 worker，支持按组合模板或内联条件执行选股，任务参数示例：

```json
{
  "task_type": "screening",
  "params": {
    "template_id": "screen_tpl_macd_engulfing",
    "limit": 200
  }
}
```

内联条件示例：

```json
{
  "task_type": "screening",
  "params": {
    "limit": 200,
    "conditions": {
      "logic": "and",
      "indicator_conditions": [
        {"period": "1d", "indicator_code": "MACD", "field": "macd", "op": "gt", "threshold": 0}
      ],
      "pattern_conditions": [
        {"period": "1d", "pattern_code": "engulfing_pattern", "direction": "bullish"}
      ]
    }
  }
}
```

`backtest` 已接入 worker，支持直接传 `symbols`、复用 `screening_task_id`，或按 `template_id/conditions` 现场筛选后回测。当前支持手续费、滑点、止盈止损、基准收益和超额收益，任务参数示例：

```json
{
  "task_type": "backtest",
  "params": {
    "screening_task_id": "stock_task_20260615_screening",
    "period": "1d",
    "hold_bars": 20,
    "lookback": 260,
    "fee_rate": 0.00025,
    "slippage_rate": 0.0005,
    "stop_loss": 0.08,
    "take_profit": 0.15,
    "benchmark_symbol": "000300",
    "limit": 200
  }
}
```

也可以用 `strategy_id` 复用策略模板，模板会展开 `screening_template_id`、`conditions`、`backtest_params`、`risk_params`；任务参数同名字段优先级更高。
创建 `screening/backtest` 任务时，后端会把 `strategy_id` 冻结成 `strategy_snapshot` 写入任务参数，避免模板后续修改影响历史任务复现。

### 量化定义

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST | `/api/stock/v1/quant/indicators?category=&enabled_only=` | 技术指标定义列表 / 创建，已内置 MA、EMA、MACD、KDJ、BOLL、RSI、TRIX、VR、WR |
| GET | `/api/stock/v1/quant/indicator-values?symbol=&period=&indicator_code=&limit=` | 技术指标计算结果查询，读取 TDengine `indicator_values` |
| GET / POST | `/api/stock/v1/quant/patterns?category=&enabled_only=` | K 线形态定义列表 / 创建，已内置 InStock / myhhub 61 种 TA-Lib 形态字段 |
| GET | `/api/stock/v1/quant/pattern-hits?symbol=&period=&pattern_code=&limit=` | K 线形态命中结果查询，读取 TDengine `pattern_hits`；`pattern_code` 为空时查全部形态命中 |

### 选股查询

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/stock/v1/screener/indicator?period=&indicator_code=&field=&op=&threshold=&limit=` | 按指标最新值选股，支持 `gt/gte/lt/lte/eq` |
| GET | `/api/stock/v1/screener/pattern?period=&pattern_code=&direction=&limit=` | 按 K 线形态最新命中选股，`direction` 可选 `bullish/bearish/neutral` |
| GET / POST / DELETE | `/api/stock/v1/screener/templates?enabled_only=&template_id=` | 组合选股模板 CRUD，模板条件以 JSON 保存，供后续选股任务、回测、页面筛选复用 |
| GET | `/api/stock/v1/screener/results?task_id=&template_id=&limit=` | 查询 `screening` 任务选股结果，读取 PostgreSQL `stock_screening_results` |

### 回测

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/stock/v1/backtest/results?task_id=&symbol=&limit=` | 查询 `backtest` 任务结果，读取 PostgreSQL `stock_backtest_results` |
| GET | `/api/stock/v1/backtest/summaries?task_id=&limit=` | 查询 `backtest` 任务组合级汇总，读取 PostgreSQL `stock_backtest_summaries` |

### 策略模板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST / DELETE | `/api/stock/v1/strategies/templates?enabled_only=&strategy_id=` | 策略模板 CRUD，保存选股条件、回测参数、风控参数 |
| GET | `/api/stock/v1/strategies/runs?strategy_id=&task_id=&status=&limit=` | 查询策略运行记录，串联模板、任务、结果和汇总 |

组合选股模板请求示例：

```json
{
  "name": "MACD红柱且吞噬形态",
  "description": "MACD macd > 0 且最近出现看涨吞噬形态",
  "conditions": {
    "logic": "and",
    "indicator_conditions": [
      {"period": "1d", "indicator_code": "MACD", "field": "macd", "op": "gt", "threshold": 0}
    ],
    "pattern_conditions": [
      {"period": "1d", "pattern_code": "engulfing_pattern", "direction": "bullish"}
    ]
  }
}
```

### 告警

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST / DELETE | `/api/v1/alerts/rules` | 规则 CRUD（DELETE 用 `?rule_id=`） |
| GET | `/api/v1/alerts/events?status=&limit=` | 事件流（`status=open/ack/空`） |
| POST | `/api/v1/alerts/events/ack?event_id=` | 标记事件已处理 |
| GET / POST / DELETE | `/api/stock/v1/alerts/rules` | 统一响应规则 CRUD |
| GET | `/api/stock/v1/alerts/events?status=&limit=` | 统一响应事件流 |
| POST | `/api/stock/v1/alerts/events/ack?event_id=` | 统一响应事件确认 |

### 模拟交易

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/paper/account` | 账户（现金、总资产、累计收益率） |
| GET | `/api/v1/paper/positions` | 持仓（含当前市值与浮动盈亏） |
| GET / POST | `/api/v1/paper/orders` | 成交记录 / 下单（`side=buy/sell, qty, price`） |
| POST | `/api/v1/paper/reset` | 清空持仓/订单并重置初始资金 |
| GET | `/api/stock/v1/paper/account` | 统一响应账户 |
| GET | `/api/stock/v1/paper/positions` | 统一响应持仓 |
| GET / POST | `/api/stock/v1/paper/orders` | 统一响应成交记录 / 下单 |
| POST | `/api/stock/v1/paper/reset` | 统一响应账户重置 |

### 券商对接

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/broker/status` | 当前 broker 适配状态（目前 noop） |
| GET | `/api/stock/v1/broker/status` | 统一响应 broker 适配状态 |

### 自选

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST / DELETE | `/api/v1/watchlist/groups` | 自选分组 CRUD |
| GET / POST / DELETE | `/api/v1/watchlist/items` | 自选标的 CRUD |
| GET | `/api/v1/watchlist/snapshot?group_id=` | 分组标的 + 最新快照 |
| GET / POST / DELETE | `/api/stock/v1/watchlist/groups` | 统一响应自选分组 CRUD |
| GET / POST / DELETE | `/api/stock/v1/watchlist/items` | 统一响应自选标的 CRUD |

## 板块分类口径

由 `akshare_adapter.classify_board(symbol)` 按代码前缀粗分：沪市主板 / 深市主板 / 创业板 / 科创板 / 北交所 / ETF / LOF/ETF。

精确「行业」字段需走 `/api/v1/instruments/profile`（AKShare `stock_individual_info_em`，本地落盘缓存 6h）。

## 告警规则字段

```json
{
  "symbol": "513310",
  "name": "中概互联",
  "market": "CN-ETF",
  "metric": "premium_ratio",         // price / pct_change / volume_ratio / premium_ratio / iopv / turnover_rate
  "op": "gt",                         // gt / lt / gte / lte / eq
  "threshold": 3.0,
  "cooldown_seconds": 300,
  "enabled": true
}
```

worker 每 `WORKER_ALERT_SECONDS`（默认 10s）扫一次所有启用规则，按 symbol 聚合调用 snapshot，命中即写 `alert_events` 并更新 rule.last_triggered_at。冷却内不会重复触发。

## 模拟下单规则

- 立即按传入价成交（不撮合不滑点）。
- 手续费默认 `成交额 × 0.025%`，最低 5 元。
- 买入校验现金，卖出校验持仓。
- 卖出时按当前 **加权平均成本** 结算已实现盈亏并累加到 `paper_account.realized_pl`。
- `equity = cash + Σ(last_price × qty)`，`total_return = (equity - initial) / initial`。

## 数据库表

| 表 | 说明 |
| --- | --- |
| `ingestion_task_configs` | 落库任务 |
| `instrument_configs` | 已知标的元数据 |
| `stock_tasks` | 指标、形态、选股、回测、AI 等通用股票任务 |
| `stock_task_logs` | 通用股票任务日志 |
| `stock_indicator_definitions` | 技术指标定义 |
| `stock_pattern_definitions` | K 线形态定义 |
| `stock_screening_templates` | 组合选股模板，保存指标条件、形态条件、逻辑关系 |
| `stock_strategy_templates` | 策略模板，保存选股来源、回测参数、风控参数 |
| `stock_strategy_runs` | 策略运行记录，保存模板、快照、任务、状态、结果引用 |
| `stock_screening_results` | 选股任务结果，保存命中标的、得分、命中条件快照 |
| `stock_backtest_results` | 回测任务结果，保存标的、进出场价格、持有收益 |
| `stock_backtest_summaries` | 回测任务汇总，保存胜率、平均收益、最大回撤、收益分布、超额收益 |
| `alert_rules` | 告警规则 |
| `alert_events` | 告警触发事件 |
| `paper_orders` | 模拟订单（仅 filled / canceled） |
| `paper_account` | 单行账户（`id=1`） |

K 线时序数据走 TDengine `stock_etf_ts.market_bars` 超级表；指标计算结果走 `stock_etf_ts.indicator_values` 超级表，子表命名为 `ind_<symbol>_<period>_<indicator>`；形态命中结果走 `stock_etf_ts.pattern_hits` 超级表，子表命名为 `pattern_<symbol>_<period>_<pattern>`。

## 依赖

- AKShare（通过 `backend/network/akshare_adapter.py` 子进程调用）
- PostgreSQL（任务/标的/告警/模拟交易，使用 `psql` 命令行）
- TDengine REST 接口（K 线时序）

## 环境变量

| 名称 | 默认值 | 说明 |
| --- | --- | --- |
| `API_HOST` / `API_PORT` | `0.0.0.0` / `8000` | 监听地址 |
| `POSTGRES_*` | `postgres / 5432 / stock_etf / stock / stock_dev_password` | 关系库 |
| `TDENGINE_*` | `tdengine / 6041 / root / taosdata / stock_etf_ts` | 时序库 |
| `AKSHARE_PYTHON_BIN` | `python3.11` | adapter 解释器 |
| `AKSHARE_SCRIPT_PATH` | `/app/network/akshare_adapter.py` | adapter 脚本路径 |

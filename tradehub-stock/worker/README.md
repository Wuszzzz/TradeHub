# Backend Worker

Go 后端的后台进程，**三路并行 / 串行调度**：
1. **采集主循环**：按 `WORKER_HEARTBEAT_SECONDS`（默认 15s）扫 `ingestion_task_configs`，按周期触发数据采集。
2. **告警扫描循环**：按 `WORKER_ALERT_SECONDS`（默认 10s）扫 `alert_rules`，命中即写 `alert_events`。
3. **通用股票任务**：同一主循环内扫 `stock_tasks`，第一阶段已支持 `indicator_compute`、`pattern_scan`、`screening`。

## 采集调度规则

`ListRunnableTasks` 选出 enabled 任务中：

1. `status in ('pending', 'retry')` —— 新任务或上次失败需要重跑
2. `status = 'completed'` 且距上次执行已超过该任务的 `interval`（秒级/5s/10s/30s/1m/5m/10m/30m/1h/1d）

## 周期 → 数据源

| interval | 走的 AKShare 接口 | 落库 |
| --- | --- | --- |
| `秒级 / 5s / 10s / 30s` | `snapshot` | 单条快照 |
| `1m / 5m / 10m / 30m / 1h` | `stock_zh_a_hist_min_em` | 分钟 K 线 |
| `1d` | `stock_zh_a_hist` / `fund_etf_hist_em` | 日线 K 线 |

数据写入 TDengine `stock_etf_ts.bars_<symbol>`（继承 `market_bars` 超级表）。

## 通用股票任务

`stock_tasks` 当前已接入 `indicator_compute`、`pattern_scan`、`screening` 和 `backtest`。

`indicator_compute` 示例：

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

执行流程：

1. worker 领取 `pending/retrying` 的 `indicator_compute`。
2. 从 TDengine `bars_<symbol>_<period>` 读取 K 线。
3. 计算 `MA/EMA/MACD/KDJ/BOLL/RSI/TRIX/VR/WR`。
4. 写入 TDengine `indicator_values` 超级表的子表 `ind_<symbol>_<period>_<indicator>`。
5. 更新 `stock_tasks.result_ref`，并写入 `stock_task_logs`。

`pattern_scan` 示例：

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

不传 `patterns` 时默认扫描 61 种形态。当前执行器为 `go-candlestick-v1`，已打通 61 个形态编码、任务执行和 TDengine 落库；后续可在同一入口替换为 TA-Lib 精确引擎。

执行流程：

1. worker 领取 `pending/retrying` 的 `pattern_scan`。
2. 从 TDengine `bars_<symbol>_<period>` 读取 K 线。
3. 扫描 61 种 K 线形态或指定形态列表。
4. 命中结果写入 TDengine `pattern_hits` 超级表的子表 `pattern_<symbol>_<period>_<pattern>`。
5. 更新 `stock_tasks.result_ref`，并写入 `stock_task_logs`。

`screening` 示例：

```json
{
  "task_type": "screening",
  "params": {
    "template_id": "screen_tpl_macd_engulfing",
    "limit": 200
  }
}
```

也可以不传 `template_id`，直接传 `params.conditions`：

```json
{
  "logic": "and",
  "indicator_conditions": [
    {"period": "1d", "indicator_code": "MACD", "field": "macd", "op": "gt", "threshold": 0}
  ],
  "pattern_conditions": [
    {"period": "1d", "pattern_code": "engulfing_pattern", "direction": "bullish"}
  ]
}
```

执行流程：

1. worker 领取 `pending/retrying` 的 `screening`。
2. 从 `stock_screening_templates` 读取模板条件，或直接解析 `params.conditions`。
3. 从 TDengine `indicator_values`、`pattern_hits` 读取每个条件的最新命中标的。
4. 按 `logic=and/or` 做交集或并集，计算 `score=命中条件数/总条件数`。
5. 写入 PostgreSQL `stock_screening_results`，并把 `stock_tasks.result_ref` 更新为 `postgres:stock_screening_results:<task_id>`。

如果传 `strategy_id`，`screening` 也会先从 `stock_strategy_templates` 展开 `screening_template_id` 和 `conditions`。

`backtest` 示例：

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

如果传 `strategy_id`，worker 会从 `stock_strategy_templates` 展开 `screening_template_id`、`conditions`、`backtest_params`、`risk_params`。任务参数同名字段优先级更高，便于临时覆盖模板默认值。

执行流程：

1. worker 领取 `pending/retrying` 的 `backtest`。
2. 如果 `task.params` 里带 `strategy_snapshot`，优先用它冻结当时的策略配置；否则再按 `strategy_id` 读取模板。
3. 优先读取 `screening_task_id` 对应的 `stock_screening_results`；也支持直接传 `symbols`，或传 `template_id/conditions` 现场筛选。
4. 从 TDengine `bars_<symbol>_<period>` 读取 K 线。
5. 按最近 `hold_bars` 根 K 线执行持有回测，支持 `fee_rate`、`slippage_rate`、`stop_loss`、`take_profit`、`benchmark_symbol`。
6. 写入 PostgreSQL `stock_backtest_results` 和 `stock_backtest_summaries`，并把 `stock_tasks.result_ref` 更新为 `postgres:stock_backtest_results:<task_id>`。

## 状态机与 retry 策略

```
pending ──► running ──► completed
              │
              └──► retry (n/5) ──► running ──► ...
                                       │
                                       └──► failed (n=5)
```

- 任务失败置 `retry`，`last_message` 形如 `retry 2/5 · akshare adapter failed: exit status 1: <python traceback>`
- 累计 5 次失败转 `failed` 锁定，不再被调度
- AKShare 子进程的 stderr 完整保留（截断 1500 字符）

## 告警扫描

按 symbol 聚合规则，单 symbol 只拉一次 snapshot，再逐条规则做比较。

```
for symbol, rules in groupBy(enabledRules):
  snapshot ← akshare.snapshot(symbol)
  for rule in rules:
    if now - rule.last_triggered_at < cooldown: continue
    value ← snapshot[rule.metric]
    if compare(rule.op, value, rule.threshold):
      insert into alert_events ...
      update alert_rules set last_triggered_at = now()
```

## 环境变量

| 名称 | 默认值 |
| --- | --- |
| `WORKER_HEARTBEAT_SECONDS` | `15` |
| `WORKER_ALERT_SECONDS` | `10` |
| 其余 | 与 `backend/api` 共用 PostgreSQL / TDengine / AKShare 配置 |

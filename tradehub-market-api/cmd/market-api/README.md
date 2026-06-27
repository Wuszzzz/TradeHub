# market-api

聚合腾讯财经 / 东方财富 / 搜狐财经 / 同花顺 / 雪球五家行情源的本地 HTTP 网关。

- 默认监听 `:18080`，可通过环境变量 `MARKET_API_ADDR` 覆盖。
- 所有响应统一格式：`{"ok": bool, "data": ..., "error": "..."}`。
- 所有 GET 路由自动经过：**内存 LRU 缓存** + **singleflight 防击穿** + **X-Cache 响应头**（`HIT` / `MISS` / `COALESCED`）。

启动：

```bash
go run ./cmd/market-api
# 或编译后运行
go build -o bin/market-api ./cmd/market-api && bin/market-api

# 若要启用雪球（需要真实 cookie）
XUEQIU_COOKIE='xq_a_token=...; xq_r_token=...; u=...; device_id=...' go run ./cmd/market-api
```

---

## 1. 数据源能力速查表

打钩表示**已通过 HTTP 接口提供**；打叉表示该源本身不提供（不是没接，是上游就没有）。

| 数据 | 腾讯 `tencent` | 东方财富 `eastmoney` | 搜狐 `sohu` | 同花顺 `ths` | 雪球 `xueqiu` |
|---|:---:|:---:|:---:|:---:|:---:|
| 实时快照（最新价、涨跌、今开、昨收、最高、最低） | ✅ | ✅ | ✅ | ✅ | ✅** |
| **五档买卖盘口（实时挂单 ask/bid 1-5）** | ✅ | ✅ | ✅（从聚合接口 cn_{code}-1.html 的 `perform` 块提取） | ❌ | ❌ |
| 估值（PE/PB/市值/股本/EPS） | 部分 | ✅ | 仅总市值 | 部分 | ✅** |
| 资金流（主力/超大/大/中/小净额） | ❌ | ✅ | ✅ | ❌ | ❌ |
| 资金流：主动 vs 被动 / 行业归属 | ❌ | ❌ | ✅（独有） | ❌ | ❌ |
| 资金流当日 1min | ❌ | ✅ | ❌ | ❌ | ❌ |
| 资金流日线 | ❌ | ✅ | ❌ | ❌ | ❌ |
| 逐笔成交明细 | ✅ | ✅* | ✅（最近若干笔） | ❌ | ❌ |
| **大单筛选** | ✅ | — | ❌ | ❌ | ❌ |
| 分时走势（每分钟） | ✅ | ✅* | ✅ | ✅ | ❌ |
| **逐价位成交分布**（≠ 五档） | ❌ | ❌ | ✅（独有） | ❌ | ❌ |
| 日 / 周 / 月 K 线 | ✅ | ✅* | ✅ | ⚠️（路由已开，但日线解码尚未启用） | ✅** |
| 分钟 K 线（1/5/15/30/60m） | ✅ | ✅* | ❌（搜狐已下线） | ❌（未找到稳定公开口） | ❌ |
| 批量快照（一次多个标的） | ✅ | ❌（逐个调） | ✅ | ❌ | ❌ |

\* 东方财富的「\*」标记接口对 IP 风控比较严格，**单 host 直连成功率较低**，本服务已在内部实现 **30 子域名池随机轮换 + 失败立切**，但被深度限流时仍可能全部失败。这类情况下应回退到腾讯或搜狐。
\
** 雪球需要真实登录态 cookie；默认不属于匿名公网源。

### 推荐选源策略

| 场景 | 主源 | 备 1 | 备 2 |
|---|---|---|---|
| 实时快照 + 五档 | `tencent/snapshot` | `eastmoney/snapshot`（含日 K fallback） | `sohu/order-book`（仅五档）或 `sohu/aggregate`（五档+快照+近期 K 一打包） |
| 大单筛选 | `tencent/large-trades` | — | — |
| 逐笔明细 | `tencent/ticks`（多页历史） | `sohu/ticks`（仅最近若干笔） | — |
| 日 / 周 / 月 K | `tencent/kline` | `eastmoney/kline` | `sohu/kline` |
| 分钟 K | `tencent/kline` | `eastmoney/kline` | — |
| 分时（每分钟） | `tencent/minute` | `sohu/minute` | `eastmoney/trends` |
| 资金流（主力 4 档净额） | `eastmoney/flow/snapshot` + `flow/intraday` | `sohu/flow/series` | — |
| 资金流（主动 vs 被动 / 含行业） | `sohu/flow`（独有） | — | — |
| 资金流日线 | `eastmoney/flow/daily` | — | — |
| 逐价位成交分布（成交分布柱状图） | `sohu/price-distribution`（独有） | — | — |

**搜狐独有能力**：主动买/被动买/主动卖/被动卖四象限拆分（含占比），以及逐价位的内外盘量与主动占比。腾讯和东财都不提供这两组数据。

---

## 2. 通用参数 & 标识

### symbol 输入格式（任意支持）

| 写法 | 示例 | 说明 |
|---|---|---|
| 6 位纯代码 | `300308` | 自动按首位判 `sh`/`sz`/`bj` |
| 腾讯前缀式 | `sh513310` `sz300308` `bj430047` | — |
| 东财 secid | `0.300308` `1.513310` | `0` 深+北，`1` 沪 |
| 搜狐前缀式 | `cn_300308` `zs_000001` | `zs_` 表示指数 |

三家源接口都接受任意一种写法，服务端自动归一化。

### 多标的

部分接口支持 `?symbols=300308,600519,688008` 批量。腾讯/搜狐 snapshot 原生批量；东财 snapshot 当前仅支持单只。

### 缓存与并发

- 每个端点有独立 **TTL**（见下文「TTL 速查」）。
- 同一 URL 同一时刻的多个并发请求由 singleflight 合并为一次上游调用，等待的请求返回 `X-Cache: COALESCED`。
- `GET /api/v1/cache/stats` 查看缓存命中统计。

### 通用响应字段

```json
{
  "ok": true,
  "data": {
    "dataset": "snapshot",      // 数据类型
    "source":  "tencent",       // 数据源
    "symbol":  "sh688008",      // 归一化后的腾讯式 symbol
    "captured_at": 1778517194,  // unix 秒，server 端时间
    "degraded": false,          // 仅东财 snapshot 有，true 表示走了 fallback
    "fallback_from": "...",     // 仅 degraded=true 时存在
    "quote": { ... },
    ...
  }
}
```

失败：

```json
{ "ok": false, "error": "..." }
```

---

## 3. 端点详表

### 3.1 通用

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| GET | `/api/v1/cache/stats` | 缓存命中/容量/淘汰统计 |

### 3.2 腾讯（tencent）

| 路径 | 用途 | 关键参数 | TTL |
|---|---|---|---|
| `GET /api/v1/tencent/snapshot` | 实时快照（支持批量） | `symbol=` 或 `symbols=a,b,c` | 800ms |
| `GET /api/v1/tencent/ticks` | 逐笔成交明细 | `symbol`、`pages`(默认 1)、`limit`、`large_threshold`、`super_threshold` | 2s |
| `GET /api/v1/tencent/large-trades` | 大单筛选 | `symbol`、`pages`(默认 5)、`min_amount`(默认 100 万元)、`limit` | 3s |
| `GET /api/v1/tencent/minute` | 分时走势（当日） | `symbol`、`limit` | 8s |
| `GET /api/v1/tencent/kline` | K 线 | `symbol`、`period=1m/5m/15m/30m/60m/day/week/month`、`adjust=none/qfq/hfq`、`limit` | 30s（分钟） / 30min（日） / 1h（周月） |

大单参数说明：
- `min_amount`：成交额过滤阈值（**元**，不是万元）。
- `large_threshold`：归类为「大单」的阈值，影响返回里 `category` 字段。
- `super_threshold`：归类为「超大单」的阈值。

### 3.3 东方财富（eastmoney）

| 路径 | 用途 | 关键参数 | TTL |
|---|---|---|---|
| `GET /api/v1/eastmoney/snapshot` | 快照 + 五档 + 估值（80+ 字段） | `symbol` | 800ms |
| `GET /api/v1/eastmoney/kline` | K 线 | `symbol`、`period`、`adjust=qfq/hfq/none`、`limit`(默认 100) | 30s/30min/1h |
| `GET /api/v1/eastmoney/trends` | 分时走势 | `symbol`、`days`(默认 1) | 15s |
| `GET /api/v1/eastmoney/flow/snapshot` | 资金流瞬时 | `symbol` | 5s |
| `GET /api/v1/eastmoney/flow/intraday` | 资金流当日 1min | `symbol`、`limit` | 30s |
| `GET /api/v1/eastmoney/flow/daily` | 资金流日线 | `symbol`、`limit`(默认 30) | 30min |

**重要：东财 IP 风控提示**

- 东财对 `push2his.eastmoney.com`（kline / trends / 资金流日线）和 `push2.eastmoney.com`（流快照 / 流分钟 / 逐笔 / ulist.np）有 **基于路径 × IP × 流量特征的多级 WAF**。
- 本服务每次调用自动在 **30 个子域名池**中随机选 8 个尝试，单个失败立切。
- 即便如此，IP 被深度灰名单时**所有子域名都会返回 `EOF` / `Empty reply from server`**。这种情况下要么等几小时冷却，要么通过 fallback：
  - `snapshot` 内置 fallback：上游全部失败时回退到 push2his 日 K 最后一根，返回带 `"degraded": true, "fallback_from": "push2his_kline_day"`。
  - 其它接口在客户端层面回退到腾讯/搜狐。

### 3.4 搜狐（sohu）

| 路径 | 用途 | 关键参数 | TTL |
|---|---|---|---|
| `GET /api/v1/sohu/snapshot` | 实时快照（**支持批量**） | `symbol` 或 `symbols=` | 1s |
| `GET /api/v1/sohu/kline` | 日 / 周 / 月 K 线 | `symbol`、`period=day/week/month`、`begin`、`end`、`limit` | 30min / 1h / 1h |
| `GET /api/v1/sohu/ticks` | 逐笔成交明细（最近若干笔） | `symbol` | 2s |
| `GET /api/v1/sohu/minute` | 当日分时（每分钟一行，含均价） | `symbol` | 8s |
| `GET /api/v1/sohu/price-distribution` | 逐价位成交分布（**不是五档**，详见下） | `symbol` | 5s |
| `GET /api/v1/sohu/flow` | 资金流详细（主动/被动 + 4 档买卖 + 行业 + 净流入时序） | `symbol` | 5s |
| `GET /api/v1/sohu/flow/series` | 资金流简版（4 档累计 + 3 组净入时序） | `symbol` | 5s |
| `GET /api/v1/sohu/order-book` | **五档买卖盘口** + 委比委差 + 内外盘 | `symbol` | 1s |
| `GET /api/v1/sohu/aggregate` | **一次拿 13 类数据**：快照 + 五档 + 逐笔 + 价位分布 + 板块 + H 股 + 近期 K 线 | `symbol` | 1s |

### 3.5 同花顺（ths / 10jqka）

| 路径 | 用途 | 关键参数 | TTL |
|---|---|---|---|
| `GET /api/v1/ths/snapshot` | 实时快照 | `symbol` | 1s |
| `GET /api/v1/ths/minute` | 当日分时（每分钟一行，含均价） | `symbol` | 8s |
| `GET /api/v1/ths/kline` | 日 K 线路由占位 | `symbol`、`period=day`、`adjust=qfq/hfq/none`、`limit` | 30min |

**同花顺能力边界（2026-05-12 首轮接入）**：

- ✅ **实时快照**：`d.10jqka.com.cn/v6/realhead/hs_{code}/last.js`，JSONP，可直连。
- ✅ **当日分时**：`d.10jqka.com.cn/v6/time/hs_{code}/last.js`，返回分钟串，可稳定解析。
- ⚠️ **日线历史原始口可拿到**：`d.10jqka.com.cn/v6/line/hs_{code}/{fq}/all.js` + `defer/today.js` 存在，但 `all.js` 是压缩格式，这版代码还没启用正式 decoder，所以 `/ths/kline` 当前会明确返回未启用错误，而不是给伪数据。
- ⚠️ **当前仅支持 A 股/ETF 的 `sh/sz` 标的**；指数、北交所、港美股未接入。
- ⚠️ **当前未接五档、逐笔、资金流、分钟 K**：不是声明它们绝对不存在，而是这轮未验证到稳定公开口。

### 3.6 雪球（xueqiu）

| 路径 | 用途 | 关键参数 | 说明 |
|---|---|---|---|
| `GET /api/v1/xueqiu/snapshot` | 实时快照 + 估值 | `symbol` | 需要 cookie |
| `GET /api/v1/xueqiu/kline` | 日 / 周 / 月 K 线 | `symbol`、`period=day/week/month`、`adjust=qfq/hfq/none`、`limit` | 需要 cookie |

**启用方式**：

- 服务级默认 cookie：启动前设置 `XUEQIU_COOKIE='xq_a_token=...; xq_r_token=...; u=...; device_id=...'`
- 单请求覆盖：HTTP Header `X-Xueqiu-Cookie: xq_a_token=...; ...`
- 未提供 cookie 时，`xueqiu/*` 直接返回 `401`，不会假装成匿名可用源。

**雪球能力边界（2026-05-12 实测）**：

- `https://xueqiu.com/S/SH600519` 首页可打开，但会下发阿里云 WAF 挑战页。
- `https://stock.xueqiu.com/v5/stock/quote.json?symbol=SH600519&extend=detail` 在无真实登录态时直接返回 `400016`。
- 单靠 `curl` 拿到的 `acw_tc` 不足以通过行情接口，因此这不是一个“裸 HTTP 免登录”数据源。
- 这版 Go API 已把雪球接成**显式会话态源**：接口形态先统一，后续只要补进真实 cookie，就能直接走同一套 HTTP 路由。

**五档盘口 vs 价位分布（两者均可用，但含义不同）**

搜狐 K 线右侧的彩色「价格 / 量」柱状条对应的就是 `sohu/price-distribution`，它表示**当日每个成交价位的累计成交量与主动占比**（成交分布直方图），不是实时五档挂单（即买一价/卖一价及对应挂单量）。区别：

| 维度 | 五档买卖盘口（level-2 depth） | 价位成交分布（volume-by-price） |
|---|---|---|
| 反映 | **未来**会成交什么（当前挂单） | **过去**已经怎么成交了（累计成交） |
| 行/档数 | 5 档 ask + 5 档 bid 共 10 行 | 数百到上千个价位 |
| 时效 | 实时挂单，秒级变化 | 自开盘累计 |
| API 来源 | `tencent/snapshot.order_book` 或 `eastmoney/snapshot.order_book` | `sohu/price-distribution` |

**搜狐能力边界（2026-05-12 复测）**：

- ✅ **实时快照** `hqm.stock.sohu.com/getqjson`：支持批量。当前出口 IP 实测串行 40 次 / 并发 60 次均 100% 成功，单次约 55ms，经公网直连到上游约 100-140ms。
- ✅ **日/周/月 K 线** `q.stock.sohu.com/hisHq`：JSONP，可用，但不同标的耗时差异较大；本次 5 个样本在 ~128-950ms。
- ✅ **逐笔/分时/价位分布** `hq.stock.sohu.com/cn/{后3位}/cn_{code}-{N}.html`：N=3/4/5 三个 JSONP（callback 名分别是 `deal_data`/`time_data`/`div_price_data`），样本股大多 ~45-80ms。
- ✅ **五档买卖盘口 + 聚合仪表盘** `hq.stock.sohu.com/cn/{后3位}/cn_{code}-1.html`：可用；当前出口 IP 实测串行 40 次 / 并发 60 次均 100% 成功。
- ✅ **资金流详细** `ushq.stock.sohu.com/AFundFlow/STOCKS/{code}.html`：A 股可用，当前出口 IP 实测串行 40 次 / 并发 60 次均 100% 成功。
- ⚠️ **资金流并非全品类覆盖**：ETF 样本 `513310` 的 `sohu/flow` 与 `sohu/flow/series` 直接返回 `404`，不能假设所有证券类型都有资金流。
- ⚠️ **跨端点字段口径不完全一致**：同一标的在 `snapshot` 与 `aggregate` 的 `total_market_cap_yuan` 可能不同；`trade_time` 格式也不统一（`YYYY-MM-DD HH:MM:SS` / `YYYYMMDDHHMMSS` / `YYYY-MM-DD-HH-MM-SS` 都出现过）。
- ⚠️ **当前环境下未观测到明确限流，不等于绝对无风控**：本次对快照上游做了最高 100 并发 / 300 请求的阶梯压测，未出现 `429/5xx`，但延迟会从 ~120ms 抬升到 ~800ms；结论仅对本机当前出口 IP 和本次时段成立。
- ❌ **分钟 K 线**：5/15/30/60m 周期被搜狐禁用，统一返回 500/503。
- ❌ **盘后大单 / 资金流日线 / 历史逐笔**：上游不提供。

搜狐定位（修正版）：**可作为强实时备选主源，但不能把所有数据类型都视作全市场、全品类一致可用**。它的独特价值仍然是「主动 vs 被动资金流」「逐价位成交分布」「13 区块一打包聚合仪表盘」三项差异化能力；若业务强依赖资金流，需先按证券类型验证覆盖率。

---

## 4. TTL 速查

| 数据类型 | TTL |
|---|---|
| snapshot（腾讯 / 东财） | 800ms |
| snapshot（搜狐） | 1s |
| ticks 逐笔（腾讯） / sohu/ticks | 2s |
| large-trades 大单 | 3s |
| flow/snapshot 资金流快照 / sohu/flow / sohu/flow/series / sohu/price-distribution | 5s |
| minute 分时（腾讯） / sohu/minute | 8s |
| trends 分时 | 15s |
| kline 分钟级 / flow/intraday | 30s |
| kline 日线 / flow/daily | 30min |
| kline 周月线 | 1h |

短 TTL 配合 singleflight 即可应对**几百 QPS** 的同 key 击穿，不需要外部缓存。

---

## 5. 字段示例

### 5.1 腾讯 snapshot

```bash
curl -s "http://127.0.0.1:18080/api/v1/tencent/snapshot?symbol=688008"
```

```json
{
  "ok": true,
  "data": {
    "dataset": "snapshot",
    "source": "tencent",
    "symbol": "sh688008",
    "name": "澜起科技",
    "quote": { "latest": 249.22, "change_percent": 18.52, ... },
    "order_book": { "asks": [...5档], "bids": [...5档] },
    "volume_amount": { "volume_hand": 1102142, "amount_yuan": 26886060000, "turnover_rate_percent": 9.61, "volume_ratio": 1.48 },
    "valuation": { "pe_ttm": ..., "pb": ..., "total_market_cap": ... }
  }
}
```

### 5.2 东财 snapshot（含 fallback 标识）

```json
{
  "ok": true,
  "data": {
    "dataset": "snapshot",
    "source": "eastmoney",
    "secid": "0.300308",
    "symbol": "sz300308",
    "quote": { "latest": 940.13, "change_percent": 6.11, ... },
    "order_book": { "asks": [...], "bids": [...] },
    "valuation": { "pe_ttm": ..., "total_market_cap": ..., "float_market_cap": ... },
    "degraded": false
  }
}
```

降级时（push2 全部失败 → 日 K fallback）：

```json
{
  "ok": true,
  "data": {
    "dataset": "snapshot",
    "source": "eastmoney",
    "quote": { "latest": ... (来自日 K close), ... },
    "order_book": { "asks": [], "bids": [] },
    "degraded": true,
    "degraded_reason": "snap=N.push2.eastmoney.com: ... EOF",
    "fallback_from": "push2his_kline_day"
  }
}
```

### 5.3 搜狐 snapshot

```bash
# 单只
curl -s "http://127.0.0.1:18080/api/v1/sohu/snapshot?symbol=688008"
# 批量
curl -s "http://127.0.0.1:18080/api/v1/sohu/snapshot?symbols=600519,300308,688008,513310"
```

```json
{
  "ok": true,
  "data": {
    "dataset": "snapshot",
    "source": "sohu",
    "symbol": "sh688008",
    "sohu_code": "cn_688008",
    "name": "澜起科技",
    "trade_time": "2026-05-11 15:30:56",
    "quote": { "latest": 249.22, "change_percent": 18.52, "change_amount": 38.95, "open": 231.26, "previous_close": 210.27, "high": 252.32, "low": 230 },
    "volume_amount": { "volume_hand": 1102142, "amount_yuan": 2688606000, "turnover_rate_percent": 9.61, "volume_ratio": 1.48 },
    "valuation": { "total_market_cap_yuan": 285712000000 }
  }
}
```

### 5.4 搜狐 K 线

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/kline?symbol=600519&period=day&limit=5"
```

字段（每行）：

| 字段 | 说明 |
|---|---|
| `time` | 日期 `YYYY-MM-DD` |
| `open` `close` `high` `low` | OHLC，元 |
| `change_amount` `change_percent` | 涨跌额（元）、涨跌幅（%） |
| `volume_hand` | 成交量（手） |
| `amount_yuan` | 成交额（**元**，已从搜狐原始的万元换算） |
| `turnover_rate_percent` | 换手率（%） |

返回顺序为**时间升序**（最早 → 最新）。

### 5.5 东财 K 线

字段（每行）：

| 字段 | 说明 |
|---|---|
| `time` | 日期或时间 |
| `open` `close` `high` `low` | OHLC |
| `volume_hand` | 成交量（手） |
| `amount_yuan` | 成交额（元） |
| `amplitude_percent` `change_percent` `change_amount` | 振幅 / 涨跌幅 / 涨跌额 |
| `turnover_rate_percent` | 换手率（%） |
| `raw` | 原始 CSV 拆分数组，便于调试 |

### 5.6 搜狐逐笔 ticks

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/ticks?symbol=688008"
```

```json
{
  "ok": true,
  "data": {
    "dataset": "ticks",
    "source": "sohu",
    "symbol": "sh688008",
    "period": "15:15-15:30",
    "count": 12,
    "rows": [
      { "time": "15:15:55", "price": 249.22, "change_percent": 18.52, "volume_hand": 2,  "count": 5  },
      { "time": "15:16:50", "price": 249.22, "change_percent": 18.52, "volume_hand": 5,  "count": 11 },
      { "time": "15:18:08", "price": 249.22, "change_percent": 18.52, "volume_hand": 24, "count": 59 }
    ]
  }
}
```

搜狐只返回**最近一个半小时窗口的重要成交**（十几条），不是完整逐笔。要全量逐笔请用 `tencent/ticks?pages=N`。

### 5.7 搜狐分时 minute

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/minute?symbol=688008"
```

```json
{
  "data": {
    "dataset": "minute",
    "summary": {
      "previous_close": 210.27,
      "open": 231.26,
      "high": 252.32,
      "low": 230.0,
      "amount_yuan": 11464265.21
    },
    "count": 272,
    "rows": [
      { "time": "09:30", "price": 234.11, "average_price": 232.80, "volume_hand_delta": 43767,  "volume_hand_total": 102463 },
      { "time": "09:31", "price": 234.95, "average_price": 233.60, "volume_hand_delta": 39279,  "volume_hand_total": 92287  }
    ]
  }
}
```

- `summary.amount_yuan` 是搜狐原始上传的「总成交额 / 万元」表示，未做转换（保留原值）。
- `volume_hand_delta` = 当分钟增量，`volume_hand_total` = 自开盘累计。

### 5.8 搜狐逐价位成交分布 price-distribution

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/price-distribution?symbol=688008"
```

```json
{
  "data": {
    "dataset": "price_distribution",
    "note": "volume-by-price histogram, not level-2 depth",
    "count": 872,
    "rows": [
      { "price": 231.99, "volume1_hand": 5618,  "volume2_hand": 13017, "ratio_percent": 100.0 },
      { "price": 232.00, "volume1_hand": 11660, "volume2_hand": 83713, "ratio_percent": 61.20 },
      { "price": 252.32, "volume1_hand": 85126, "volume2_hand": 214789, "ratio_percent": 100.0 }
    ]
  }
}
```

字段说明：

| 字段 | 含义 |
|---|---|
| `price` | 成交价位（元） |
| `volume1_hand` | 该价位内盘类累计量（手） |
| `volume2_hand` | 该价位外盘类累计量（手） |
| `ratio_percent` | 主动占比（%），100% 表示该价位所有成交都是主动方向 |
| `raw` | 原始数组，便于调试 |

搜狐原未对 `volume1`/`volume2` 标明到底哪个是主买、哪个是主卖，字段名保留中性、依靠 `raw` 验证。常见经验是「列 1 为内盘/主卖、列 2 为外盘/主买」，但上游未官方确认。

### 5.9 搜狐资金流详细 flow

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/flow?symbol=688008"
```

```json
{
  "data": {
    "dataset": "fund_flow",
    "stock_name": "澜起科技",
    "industry": "电子",
    "in_value": 16077571471,
    "out_value": 10808488472,
    "net_value": 5269082999,
    "active_buy": 13402965966,  "active_buy_ratio":  0.4985,
    "passive_buy": 2674605504,  "passive_buy_ratio": 0.0995,
    "active_sell": 8040766878,  "active_sell_ratio": 0.2991,
    "passive_sell": 2767721594, "passive_sell_ratio":0.1029,
    "tier": {
      "super_buy":  14751282554, "super_sell":  9188680438, "super_net":   5562602116,
      "big_buy":     1235616266, "big_sell":    1473130248, "big_net":     -237513982,
      "medium_buy":    89190772, "medium_sell":  145165016, "medium_net":   -55974244,
      "small_buy":      1481878, "small_sell":    1512769,  "small_net":       -30891
    },
    "big_order_active_buy": 13342198709,
    "big_order_passive_buy": 2645864440,
    "big_order_active_sell": 7943125118,
    "big_order_passive_sell": 2721187083,
    "net_series":  [["1","5269082999","108013504"], ...],
    "corp_series": [["1","5323750949","115854092"], ...],
    "quick_series":[["362","0","0"], ...],
    "time": "20260511150004"
  }
}
```

金额全部是**元**（未乘 1e4）。「主动买」是买方向主动成交额，「被动买」是卖方主动击买价成交的金额。`big_order_*` 以「大单」口径重复拆分主动/被动。`net_series` / `corp_series` / `quick_series` 是搜狐三种口径的净流入时序（原未提供表头，raw 保留字符串，判定请结合他源交叉验证）。

### 5.10 搜狐五档买卖盘口 order-book

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/order-book?symbol=000001"
```

```json
{
  "data": {
    "dataset": "order_book",
    "source": "sohu",
    "symbol": "sz000001",
    "name": "平安银行",
    "latest": 11.28,
    "trade_time": "20260511153056",
    "order_book": {
      "committee_ratio_percent": -32.59,
      "committee_diff_hand": -10488,
      "asks": [
        { "level": 1, "price": 11.29, "volume_hand": 1420 },
        { "level": 2, "price": 11.30, "volume_hand": 7962 },
        { "level": 3, "price": 11.31, "volume_hand": 3492 },
        { "level": 4, "price": 11.32, "volume_hand": 4384 },
        { "level": 5, "price": 11.33, "volume_hand": 4078 }
      ],
      "bids": [
        { "level": 1, "price": 11.28, "volume_hand": 1313 },
        { "level": 2, "price": 11.27, "volume_hand": 2538 },
        { "level": 3, "price": 11.26, "volume_hand": 2728 },
        { "level": 4, "price": 11.25, "volume_hand": 2886 },
        { "level": 5, "price": 11.24, "volume_hand": 1384 }
      ],
      "inner_volume_hand": 413468,
      "outer_volume_hand": 509651,
      "status_flag": "Z"
    }
  }
}
```

字段说明：

| 字段 | 含义 |
|---|---|
| `committee_ratio_percent` | 委比 (-100% ~ +100%)，>0 多头占优 |
| `committee_diff_hand` | 委差(手) = 委买 - 委卖 |
| `asks` / `bids` | 五档挂单，level 1 最贴近现价（最优挂单） |
| `inner_volume_hand` | 内盘累计（手）：主动卖压 |
| `outer_volume_hand` | 外盘累计（手）：主动买力 |
| `status_flag` | 行情状态标志（如 Z=正常） |

数据源是 `hq.stock.sohu.com/cn/{后3位}/cn_{code}-1.html` 的 `perform` 块，响应 50ms 以内。

### 5.11 搜狐聚合仪表盘 aggregate

一次请求拿到 **13 个数据块**（11 KB），适合页面初次渲染时一次性获取所有需要的数据：

```bash
curl -s "http://127.0.0.1:18080/api/v1/sohu/aggregate?symbol=688008"
```

返回结构：

```json
{
  "data": {
    "dataset": "aggregate",
    "source": "sohu",
    "symbol": "sh688008",
    "trade_time": "20260511153056",
    "captured_at": 1778517194,

    "indices":     [["zs_000001","上证指数","4225.02","1.08%", ...]],
    "top_gainers": [["cn_301531","N春光集","633.08%", "..."]],

    "identity": { "code": "cn_688008", "name": "澜起科技" },

    "quote": {
      "latest": 249.22, "change_amount": 38.95, "change_percent": 18.52,
      "average": 243.94, "previous_close": 210.27,
      "open": 231.26, "high": 252.32, "low": 230,
      "volume_ratio": 1.48, "turnover_rate_percent": 9.61,
      "volume_hand": 1102142, "amount_yuan": 2688606000,
      "limit_up": 252.32, "total_market_cap_yuan": 304597000000,
      "buy_orders_hand": 111, "sell_orders_hand": 276
    },

    "order_book": { "asks": [...], "bids": [...], "committee_ratio_percent": -75.82, ... },

    "sectors": [
      { "code": "3123", "name": "电子",      "change_percent": 4.51 },
      { "code": "7591", "name": "半导体",    "change_percent": 4.92 },
      { "code": "6578", "name": "AI芯片",    "change_percent": 3.25 },
      { "code": "7711", "name": "集成电路",  "change_percent": 5.44 }
    ],

    "hk_pair": ["06809", "澜起科技", "418.600", "43.000", "11.45%", "46.91%"],

    "recent_ticks":  [...],
    "nearby_prices": [...],
    "minute_tail":   [...],
    "kline_day":     [["20260511","231.26","249.22","252.32","230.00","1102142","2688606","9.61%","38.95","18.52%","111","276"]],
    "kline_week":    [...],
    "kline_month":   [...],

    "raw": { ...各 block 的原始字符串... }
  }
}
```

亮点：

- **`sectors`**：标的所属的所有板块/概念（含板块当日涨跌幅），其它两源都不直接给。腾讯 / 东财要单独多接口拼。
- **`hk_pair`**：A+H 两地上市标的的港股关联代码与 A/H 溢价（澜起 H 股 06809，溢价 +46.91%）。
- **`top_gainers`**：当日全市场涨幅榜 Top10，渲染首页热点完美。
- **`raw`**：每个 block 的原始字符串保留下来，便于调试搜狐返回的未文档化字段。

`aggregate` 的 TTL 是 1s，热缓存命中 0.5ms 返回。在做盘中页面时它能省掉同标的的多次 round-trip。

### 5.12 东财资金流（intraday / daily）

每行包含主力 / 超大 / 大 / 中 / 小净额（元）与净占比（%）：

```json
{
  "rows": [
    {
      "time": "2026-05-11",
      "main_net_amount": 123456789,
      "main_net_ratio_percent": 5.12,
      "super_big_net_amount": ..., "super_big_net_ratio_percent": ...,
      "big_net_amount": ...,       "big_net_ratio_percent": ...,
      "medium_net_amount": ...,    "medium_net_ratio_percent": ...,
      "small_net_amount": ...,     "small_net_ratio_percent": ...,
      "close": 940.13,
      "change_percent": 6.11
    }
  ]
}
```

### 5.7 腾讯大单

```bash
curl -s "http://127.0.0.1:18080/api/v1/tencent/large-trades?symbol=688008&pages=5&min_amount=2000000"
```

返回 `rows` 仅保留 `amount_yuan >= min_amount` 的逐笔；每行有 `category` 字段（`super` / `large` / 普通）。

---

## 6. 已知问题与排查

| 现象 | 原因 | 处理 |
|---|---|---|
| `eastmoney/*` 全部返回 `EOF` 或 `Empty reply from server` | 测试 IP 被东财 WAF 灰名单 | 等数小时冷却 / 换出口 IP / 临时切换到 tencent + sohu |
| `eastmoney/snapshot` 返回 `degraded=true` | 上游 `push2 stock/get` 失败，已自动 fallback 到日 K | 业务侧根据 `degraded` 字段决定是否提示「数据延迟」 |
| `sohu/kline?period=5m` 报 `period unsupported` | 搜狐侧已禁分钟级 K | 改用腾讯或东财的分钟 K |
| `sohu/flow` / `sohu/flow/series` 对个别 ETF 返回 `http 404` | 搜狐资金流并非覆盖所有证券类型 | 先按证券类型做覆盖探测；ETF/指数资金流优先回退到东财或放弃该维度 |
| `sohu/snapshot` 返回 `name` 是 unicode 转义 | JSON 标准编码，前端 `JSON.parse` 自动还原；终端可加 `| python3 -m json.tool` | — |
| 同标的 `sohu/snapshot`、`sohu/order-book`、`sohu/aggregate` 的 `trade_time` 格式不一致 | 上游多个端点各自拼接时间字符串 | 下游统一做时间标准化，不要直接按单一格式解析 |
| 同 URL 高并发只看到一次上游访问 | singleflight 合并 | 这是预期行为，看 `X-Cache: COALESCED` 头确认 |

排查思路：

1. **先看响应头** `X-Cache`：`HIT` 表示缓存命中没打上游；`COALESCED` 表示被其它请求带飞；`MISS` 表示实际打了上游。
2. **看错误中的子域名编号**：东财错误会带出 host（如 `23.push2.eastmoney.com`），说明轮换策略在工作。
3. **看 `degraded` 字段**：东财 snapshot 唯一会自动降级的端点，其它都直接报错而非静默降级。
4. **`cache/stats`**：观察 `hits / misses / evicts / expires` 比例。

---

## 7. 内部结构（仅供二次开发参考）

```
cmd/market-api/
├── main.go              # 路由注册 + 启动
├── handlers_extra.go    # 东财 / 搜狐 / 同花顺 handler + symbol 解析助手
├── cache.go             # LRU 缓存
├── http.go              # 中间件、CORS、JSON 响应辅助
├── internal/
│   ├── symbol/          # 跨源 symbol 归一化
│   ├── tencent/         # 腾讯客户端（已有，未改）
│   ├── eastmoney/       # 东财客户端 + 30 host 池 + fallback
│   ├── sohu/            # 搜狐客户端 + GB18030 解码
│   └── ths/             # 同花顺客户端 + JSONP 解析
```

新增数据源遵循同一模式：

1. 在 `internal/<vendor>/client.go` 实现 vendor 客户端。
2. 在 `handlers_extra.go` 写薄薄的 handler 包装。
3. 在 `main.go` 注册路由 + TTL。
4. 复用 `s.serveWithCache(...)` 自动接入缓存 + singleflight。

---

## 8. 相关文档

- 东方财富接口细节、字段语义、采集脚本说明：`@/Users/Apple/Documents/stock/需求/东方财富/接口数据爬取分析.md`
- 腾讯 Python 采集器：`@/Users/Apple/Documents/stock/scripts/tencent_realtime_collector.py`
- 东财 Python 采集器（含 IP 限流时的浏览器兜底）：`@/Users/Apple/Documents/stock/scripts/eastmoney_collector.py` / `eastmoney_browser_tap.py`

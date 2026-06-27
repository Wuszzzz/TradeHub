# Network

网络层。Go 后端通过子进程方式调用 Python adapter 接入外部行情/数据源。

当前主 adapter 是 `akshare_adapter.py`。`mootdx_adapter.py` 是通达信 TCP 行情源的独立 adapter，用于 Linux 部署后的连通性测试、K 线补源和后续多源比对。

## adapter 模式

### AKShare adapter

| 命令 | 参数 | 用途 |
| --- | --- | --- |
| `search <keyword>` | 关键字 | 关键字搜索（A股 + ETF），返回 `{symbol, name, market, board}` |
| `instruments <market> <keyword> <board> <limit> <offset>` | 市场/关键字/板块/分页 | 全市场浏览，附带最新价、涨跌幅等 |
| `snapshot <symbol>` | 标的 | 实时快照（含五档、资金流） |
| `profile <symbol>` | 标的 | 板块/行业资料，本地缓存 6 小时（`/tmp/akshare_profile_cache/`） |
| `minute <symbol> <period>` | 标的 + 周期 | 分钟级 K 线（`stock_zh_a_hist_min_em`） |
| `daily <symbol> <start> <end> <limit>` | 标的 + 区间 + 上限 | 日线 K 线（A股 / ETF 自动判断） |

### mootdx adapter

`mootdx_adapter.py` 使用通达信 TCP 行情服务器，适合国内 Linux 服务器作为 A 股行情补源。它不需要 API Key，但依赖服务器网络能直连通达信行情节点。

| 命令 | 参数 | 用途 |
| --- | --- | --- |
| `health [symbol]` | 标的，默认 `600519` | 测试 mootdx 连通性并返回一条日线样本 |
| `daily <symbol> <start> <end> <limit>` | 标的 + 区间 + 上限 | 日线 K 线，最多 800 根/次 |
| `minute <symbol> <period> <limit>` | 标的 + 周期 + 上限 | 分钟 K 线，支持 `1m/5m/15m/30m/60m` |
| `snapshot <symbol>` | 标的 | 实时行情快照，含可解析到的五档字段 |
| `transactions <symbol> <start> <count>` | 标的 + 起点 + 条数 | 逐笔成交，含买卖方向字段 |

本机测试：

```bash
python3.11 tradehub-stock/network/mootdx_adapter.py health 600519
python3.11 tradehub-stock/network/mootdx_adapter.py daily 600519 "" "" 20
python3.11 tradehub-stock/network/mootdx_adapter.py minute 600519 5m 20
python3.11 tradehub-stock/network/mootdx_adapter.py snapshot 600519
```

Docker 测试：

```bash
docker compose exec stock-api python /app/network/mootdx_adapter.py health 600519
```

注意：

- 首次运行会测速通达信服务器并生成 `~/.mootdx/config.json`。
- `docker-compose.yml` 已把 stock-api / stock-worker 的 `/root/.mootdx` 挂到 `stock_mootdx_config` volume，避免容器重建后反复测速。
- 海外 VPS 直连通达信服务器可能超时；国内服务器更稳定。
- `block()` 不作为板块主源，TradeHub 板块仍优先使用 AkShare/东财/腾讯等来源。

## 缓存

- A 股 spot：`/tmp/akshare_stock_spot_cache.json`，TTL 30s
- ETF spot：`/tmp/akshare_etf_spot_cache.json`，TTL 30s
- 行业资料：`/tmp/akshare_profile_cache/<symbol>.json`，TTL 6h
- mootdx 节点测速配置：`~/.mootdx/config.json`

## 错误处理

- 三次自动重试（`_retry`），每次间隔 1.5s 递增
- 顶层异常时根据模式回退到「待确认标的」占位

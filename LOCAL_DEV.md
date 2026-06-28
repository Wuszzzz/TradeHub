# TradeHub 本地开发启动文档

## 1. 本地开发模式

当前本地开发模式是“Docker 跑基础设施，业务服务跑宿主机”：

| 类型 | 服务 | 启动方式 | 地址 |
| --- | --- | --- | --- |
| 基础设施 | PostgreSQL | `./dev.sh start` | `127.0.0.1:5433` |
| 基础设施 | Redis | `./dev.sh start` | `127.0.0.1:6380` |
| 基础设施 | TDengine | `./dev.sh start` | `127.0.0.1:16042` |
| 业务服务 | fund backend | `./dev.sh fund` | `127.0.0.1:7001` |
| 业务服务 | stock api | `./dev.sh stock` | `127.0.0.1:7002` |
| 业务服务 | market api | `./dev.sh market` | `127.0.0.1:17080` |
| 业务服务 | fund research | `./dev.sh fund-research` | `127.0.0.1:17081` |
| 业务服务 | frontend | `./dev.sh frontend` | `127.0.0.1:5173` |

不要把当前本地开发理解为 `docker compose up -d` 启动全部应用。`docker-compose.local.yml` 是当前本地事实，根目录 `docker-compose.yml` 后续需要按真实路径重修后才能作为一体化编排入口。

架构上允许服务解耦：基金后端、股票后端、行情网关、前端可以独立进程运行。第一阶段不做的是完整微服务化治理，例如服务注册、服务网格、跨服务事务、复杂网关编排。

## 2. 首次启动

```bash
cp .env.example .env
./dev.sh start
./dev.sh migrate
```

分别打开四个终端：

```bash
./dev.sh market
```

```bash
./dev.sh fund-research
```

```bash
./dev.sh fund
```

```bash
./dev.sh stock
```

```bash
./dev.sh frontend
```

检查：

```bash
./dev.sh status
```

## 3. 健康检查地址

| 服务 | 检查命令 |
| --- | --- |
| fund backend | `curl -sf http://127.0.0.1:7001/api/health/` |
| stock api | `curl -sf http://127.0.0.1:7002/healthz` |
| market api | `curl -sf http://127.0.0.1:17080/health` |
| fund research | `curl -sf http://127.0.0.1:17081/health` |
| frontend | `curl -sf http://127.0.0.1:5173/` |

## 4. 数据库

PostgreSQL 初始化脚本位于 `docker/init-scripts/postgres/init-db.sql`，创建两个数据库：

| 数据库 | 用户 | 用途 |
| --- | --- | --- |
| `fundval` | `fundval` | 基金、账户、持仓、认证、通知、AI 配置 |
| `stock_etf` | `stock` | 股票自选、采集任务、告警、模拟交易、策略配置 |

进入 PostgreSQL：

```bash
./dev.sh shell-db
```

TDengine 用于股票时序数据，默认 database 为 `stock_etf_ts`。

## 5. 环境变量重点

| 变量 | 当前默认 | 说明 |
| --- | --- | --- |
| `POSTGRES_HOST` | `localhost` | 宿主机访问 Docker PostgreSQL |
| `POSTGRES_PORT` | `5433` | 避免和其他项目冲突 |
| `REDIS_URL` | `redis://localhost:6380/0` | 基金默认 Redis DB |
| `TDENGINE_HOST` | `localhost` | 宿主机访问 Docker TDengine |
| `TDENGINE_PORT` | `16042` | TDengine REST 端口 |
| `MARKET_API_URL` | `http://localhost:17080` | 股票后端访问行情网关 |
| `FUND_RESEARCH_PORT` | `17081` | Go 基金投研服务端口 |

股票服务使用：

| 变量 | 用途 |
| --- | --- |
| `STOCK_DB` | 股票 PostgreSQL database |
| `STOCK_DB_USER` | 股票 PostgreSQL user |
| `STOCK_DB_PASSWORD` | 股票 PostgreSQL password |
| `MARKET_API_URL` | 行情网关地址 |

基金服务使用 Django 标准变量：

| 变量 | 用途 |
| --- | --- |
| `POSTGRES_DB` | 基金 PostgreSQL database |
| `POSTGRES_USER` | 基金 PostgreSQL user |
| `POSTGRES_PASSWORD` | 基金 PostgreSQL password |
| `SECRET_KEY` | Django/JWT 密钥 |
| `DB_TYPE` | 本地应为 `postgresql` |

## 6. 第一阶段启动约束

- 本地先不做全 Docker 业务服务启动。
- 页面开发前必须先确认 API 契约和统一响应结构。
- 股票功能新增先落到 `tradehub-stock` 内部模块，不单独拆新服务。
- 行情源能力统一经过 `tradehub-market-api`，不要让前端直连第三方行情源。
- 指标、形态、选股、回测、AI 报告要走统一任务模型，不在页面请求中同步重算大批量数据。

# TradeHub

TradeHub 是个人量化投资一体化平台，当前采用“一个产品大服务、内部按业务域模块化、关键能力进程级解耦”的演进路线。基金、股票、行情、量化、AI 研究共用同一套基础设施和统一前端，不在第一阶段引入完整微服务治理。

## 当前架构基线

本仓库当前真实开发拓扑如下：

| 层级 | 模块 | 路径 | 本地端口 | 说明 |
| --- | --- | --- | --- | --- |
| 前端 | tradehub-frontend | `tradehub-frontend/` | `5173` | React + Vite 统一前端 |
| 基金后端 | tradehub-fund | `tradehub-fund/` | `7001` | Django + DRF，基金、账户、持仓、通知、AI 配置 |
| 股票后端 | tradehub-stock | `tradehub-stock/` | `7002` | Go API + Worker，股票自选、行情、告警、模拟交易、采集任务 |
| 行情网关 | tradehub-market-api | `tradehub-market-api/` | `17080` | Go 行情源聚合，腾讯/东财/搜狐/同花顺/雪球 |
| 关系库 | PostgreSQL | Docker | `5433` | 基金业务库 `fundval`、股票业务库 `stock_etf` |
| 缓存/队列 | Redis | Docker | `6380` | 基金 Celery、股票缓存/锁/任务状态 |
| 时序库 | TDengine | Docker | `16042` | 股票 K 线、分钟线、资金流等时序数据 |

本地开发以 `dev.sh` 和 `docker-compose.local.yml` 为准：Docker 只跑 PostgreSQL、Redis、TDengine，业务服务直接在宿主机启动。根目录 `docker-compose.yml` 是后续生产/一体化编排候选，不作为当前本地开发入口。

这里的“不做完整微服务”不等于所有代码都塞进一个进程。当前仍保留前端、基金后端、股票后端、行情网关四个进程，但它们按产品大服务内部模块协作，不引入服务注册、服务网格、跨服务事务、复杂链路治理。

## 快速启动

```bash
# 1. 准备环境变量
cp .env.example .env

# 2. 启动基础设施
./dev.sh start

# 3. 初始化基金数据库
./dev.sh migrate

# 4. 分别在多个终端启动业务服务
./dev.sh market
./dev.sh fund
./dev.sh stock
./dev.sh frontend

# 5. 检查状态
./dev.sh status
```

访问地址：

| 服务 | 地址 |
| --- | --- |
| 前端 | `http://127.0.0.1:5173` |
| 基金 API | `http://127.0.0.1:7001/api/` |
| 股票 API | `http://127.0.0.1:7002/` |
| 行情网关 | `http://127.0.0.1:17080/` |

详细启动说明见 [LOCAL_DEV.md](./LOCAL_DEV.md)。

## 第一阶段架构地基

第一阶段不优先写页面，先补架构地基：

1. 固化本地运行事实：README、启动脚本、环境变量、端口、健康检查。
2. 定义 API 契约：统一 OpenAPI 分域、统一响应结构、统一错误码、统一分页。
3. 梳理数据分层：PostgreSQL 业务配置、TDengine 时序行情、Redis 缓存/锁、文件存储报告导出。
4. 整理股票后端结构：把 `tradehub-stock/api` 从大文件模式逐步拆成接口层、应用层、领域层、基础设施层。
5. 建立任务底座：采集、指标计算、K 线形态、选股、回测、AI 报告都走统一任务模型。

架构文档入口：

- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [docs/技术架构/1. 第一期架构地基迭代方案.md](./docs/技术架构/1.%20第一期架构地基迭代方案.md)
- [docs/技术架构/2. OpenAPI与统一响应规范.md](./docs/技术架构/2.%20OpenAPI与统一响应规范.md)
- [docs/技术架构/3. 数据库分层设计.md](./docs/技术架构/3.%20数据库分层设计.md)
- [docs/技术架构/4. 股票API模块化拆包方案.md](./docs/技术架构/4.%20股票API模块化拆包方案.md)

## 目录结构

```text
TradeHub/
├── README.md
├── LOCAL_DEV.md
├── ARCHITECTURE.md
├── dev.sh
├── docker-compose.local.yml
├── docker/
│   └── init-scripts/postgres/init-db.sql
├── docs/
│   ├── 技术架构/
│   └── 1. 基金需求文档/
├── tradehub-fund/
├── tradehub-stock/
├── tradehub-market-api/
└── tradehub-frontend/
```

## 开发原则

- 第一阶段坚持大服务内部模块化和必要的进程级解耦，不做完整微服务治理。
- 股票和基金可以共享基础设施、认证、通知、AI 配置、任务审计，但业务模型保持边界清晰。
- 页面新增前必须先定义 API、数据表、任务流和错误响应。
- market-api 是行情源聚合网关，不承载用户自选、策略、回测、交易等业务逻辑。
- stock-api 是股票业务入口，后续承载自选、行情聚合、指标、形态、策略、回测、模拟交易、AI 研究等模块。

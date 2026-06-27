# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此代码库工作时提供指导。

## 项目概述

TradeHub — 个人量化投资一体化平台，包含基金管理和股票/ETF监控两个独立业务应用，共享底层数据基础设施。

## 服务端口

| 服务 | 端口 | 目录 |
|------|------|------|
| market-api（Go 行情网关） | 17080 | tradehub-market-api |
| 基金后端（Python Django） | 7001 | tradehub-fund |
| 股票后端（Go） | 7002 | tradehub-stock |
| 统一前端（React） | 3000 | tradehub-frontend |
| PostgreSQL | 5433 | docker |
| Redis | 6380 | docker |
| TDengine | 16042 | docker |

## 本地开发快速启动

本地开发模式：**Docker 统一编排所有服务，开发时可选择性启动。**

```bash
# 启动全部服务
docker compose up -d

# 仅启动基础设施（数据库）
docker compose up -d postgres redis tdengine market-api

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f fund-backend
docker compose logs -f stock-api
```

### 环境变量
复制 `.env.example` 为 `.env`，修改密码和密钥：
```bash
cp .env.example .env
# 编辑 .env，生产环境必须修改 SECRET_KEY
```

### 访问地址
- 前端：http://localhost:3000
- 基金 API：http://localhost:3000/api/fund/
- 股票 API：http://localhost:3000/api/stock/
- market-api：http://localhost:18080（内部服务，不直接暴露）

## 架构

### 整体架构图

```
Nginx Gateway (:3000 对外)
  ├── /api/fund/*  → fund-backend :8001 (Django)
  ├── /api/stock/* → stock-api :8002 (Go)
  └── /*           → 前端静态资源 (React)

fund-backend (Django)
  ├── fund-worker (Celery worker)
  └── fund-beat (Celery beat)

stock-api (Go)
  └── stock-worker (Go goroutine)

market-api (Go，共享行情网关)
  ├── 腾讯财经/搜狐/东财/同花顺/雪球
  └── akshare/yfinance（Python 子进程补全）

基础设施
  ├── PostgreSQL（fundval + stock_etf 双 database）
  ├── Redis（db0=基金，db1=股票）
  └── TDengine（仅股票时序数据）
```

### 基金后端（Python Django）

目录结构：
```
tradehub-fund/
├── api/                    # Django App
│   ├── models.py           # ORM 模型（11张表）
│   ├── views.py            # REST API Views
│   ├── serializers.py      # DRF Serializers
│   ├── tasks.py            # Celery 定时任务
│   ├── sources/            # 数据源适配器（东财/小倍养基）
│   ├── services/           # 业务逻辑层
│   └── utils/              # 工具函数
├── fundval/                # Django 项目配置
│   ├── settings.py
│   ├── urls.py
│   └── celery.py
└── manage.py
```

关键 Celery 任务（7个）：
- `update_fund_nav` — 每日净值同步
- `update_fund_today_nav` — 当日确权净值
- `capture_estimate_snapshot` — 收盘估值快照
- `check_notification_rules` — 涨跌提醒
- `audit_accuracy` — 估值准确率审计
- `capture_intraday_snapshots` — 盘中估值快照
- `generate_investment_reports` — AI 投资报告

### 股票后端（Go）

目录结构：
```
tradehub-stock/
├── api/                    # HTTP API 服务
│   ├── main.go
│   ├── alerts.go           # 告警规则
│   ├── paper.go            # 模拟交易
│   └── watchlist.go        # 自选池
├── worker/                 # 采集任务执行器
│   ├── main.go
│   └── alerts.go           # 告警扫描
└── model/                  # 数据模型
```

采集策略（按需，非全市场）：
- 用户加入股票自选 → 启动该标的采集任务
- 日线：历史补全 5 年
- 分钟线：仅盘中活跃期采集，保留 60 天

### market-api（Go 行情网关）

```
tradehub-market-api/
└── cmd/market-api/
    ├── main.go
    ├── cache.go             # stale-while-revalidate 缓存
    ├── handlers_extra.go    # 扩展接口（待实现）
    └── internal/
        ├── tencent/         # 腾讯财经（主力）
        ├── sohu/            # 搜狐财经
        ├── eastmoney/       # 东方财富
        ├── ths/             # 同花顺
        └── xueqiu/          # 雪球
```

缓存策略：
- singleflight 防击穿（同 symbol 并发请求只触发一次）
- stale-while-revalidate（过期值立刻返回，后台异步刷新）

### 前端（React 单应用，双空间）

```
tradehub-frontend/
└── src/
    ├── shell/              # 共享 Shell 层
    │   ├── AppSwitcher.jsx     # 基金/股票切换器
    │   ├── GlobalHeader.jsx    # 顶部导航栏
    │   └── AuthContext.jsx     # 统一认证状态
    ├── fund/               # 基金应用（完整独立）
    │   ├── layout/
    │   ├── pages/
    │   ├── components/
    │   └── api/
    └── stock/              # 股票应用（完整独立）
        ├── layout/
        ├── pages/
        ├── components/
        └── api/
```

路由结构：
- `/login` — 统一登录
- `/fund/*` — 基金应用空间
- `/stock/*` — 股票应用空间

## 开发规则

### 必须遵守

1. **改代码前先查阅设计文档** — `tradehub-docs/system_design/` 相关设计文档
2. **新建表/字段查阅数据字典** — `tradehub-docs/data_dictionary/data_dictionary.md`
3. **数据库 migration 规范**：
   - Django：`python manage.py makemigrations`
   - Go：手动 SQL 脚本，命名格式 `001_xxx.sql`
4. **前端统一 Ant Design 6.x** — 禁止混用其他 UI 库
5. **market-api 为共享服务** — 基金和股票都调用它，修改需兼容双方

### 认证

统一 JWT 认证（由 Django 签发）：
- 基金服务：Django JWT（djangorestframework-simplejwt）
- 股票服务：Go 验证 JWT 签名（用相同 SECRET_KEY）
- 前端：`Authorization: Bearer <token>`

### 代码规范

- Python：PEP 8，Django 最佳实践
- Go：`gofmt`，标准库优先
- TypeScript：ESLint + Prettier
- Git Commit：`[fund]` / `[stock]` / `[market-api]` / `[frontend]` 前缀

### 关键配置文件

- `docker-compose.yml` — 统一服务编排
- `.env` — 环境变量（不提交 git）
- `tradehub-fund/fundval/settings.py` — Django 配置
- `tradehub-stock/api/main.go` — Go API 配置
- `nginx/nginx.conf` — 反向代理路由

## Makefile 命令

```bash
make help              # 显示帮助
make dev               # 启动开发环境
make dev-stop          # 停止开发环境
make logs              # 查看日志
make test-fund         # 测试基金后端
make test-stock        # 测试股票后端
make docs              # 生成文档
make clean             # 清理环境
```

## 系统级提示词

1. **永远使用中文回答问题** — 所有对话、代码注释、commit message、文档均使用中文。
2. **禁止创建无意义的总结文档** — 不要主动创建 markdown 总结文件、工作日志。
3. **回答简洁直接** — 避免冗长的总结和重复性描述。
4. **以实际数据库为准** — 设计时先查询实际表结构（`\d tablename`），文档仅供参考。
5. **绝对禁止未经确认执行破坏性 Docker 操作** — `docker compose down`、`docker rm`、`docker volume rm` 等必须先确认。重启用 `restart`，不用 `down`+`up`。

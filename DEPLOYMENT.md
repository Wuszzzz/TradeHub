# TradeHub 新机器部署文档

本文面向一台全新的 Linux 服务器，目标是把 TradeHub 以 Docker Compose 一体化方式跑起来。当前项目本地开发文档已经覆盖 `dev.sh`、本地端口和宿主机开发模式，但新机器部署还需要明确系统依赖、生产环境变量、数据库迁移、健康检查、备份恢复和排障流程。

## 1. 部署结论

推荐新机器优先使用根目录 `docker-compose.yml`：

- PostgreSQL：基金库 `fundval`、股票库 `stock_etf`
- Redis：基金 Celery、股票任务/缓存
- TDengine：股票 K 线、分钟线、资金流等时序数据
- fund-backend：Django + DRF
- fund-worker：Celery worker
- fund-beat：Celery beat 定时任务
- fund-research：Go 基金投研服务
- stock-api：Go 股票 API
- stock-worker：Go 股票后台任务
- market-api：行情源聚合网关
- frontend：Vite 前端开发服务形态

当前 compose 仍是“可部署候选”，不是严格生产优化版本。生产环境后续建议把 frontend 改为静态构建 + Nginx，而不是容器内 `npm run dev`。

## 2. 机器要求

最低建议：

| 项 | 建议 |
| --- | --- |
| CPU | 4 核以上 |
| 内存 | 8 GB 以上，建议 16 GB |
| 磁盘 | 100 GB 以上，TDengine 和 PostgreSQL 数据会增长 |
| OS | Ubuntu 22.04/24.04 或 Debian 12 |
| Docker | Docker Engine 24+ |
| Compose | Docker Compose v2 |

开放端口：

| 端口 | 服务 | 说明 |
| --- | --- | --- |
| 5173 | frontend | 前端入口 |
| 7001 | fund-backend | 基金 API |
| 17081 | fund-research | Go 基金投研 API |
| 7002 | stock-api | 股票 API |
| 17080 | market-api | 行情 API |
| 5433 | PostgreSQL | 如无需外部访问，生产建议不要暴露公网 |
| 6380 | Redis | 如无需外部访问，生产建议不要暴露公网 |
| 16042 | TDengine REST | 如无需外部访问，生产建议不要暴露公网 |

## 3. 安装基础软件

Ubuntu/Debian 示例：

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git openssl

curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
```

重新登录后确认：

```bash
docker version
docker compose version
```

## 4. 拉取代码

```bash
git clone <your-tradehub-repo-url> TradeHub
cd TradeHub
```

如果是直接拷贝目录，确认根目录包含：

```text
docker-compose.yml
docker/init-scripts/postgres/init-db.sql
tradehub-fund/
tradehub-fund-research/
tradehub-stock/
tradehub-market-api/
tradehub-frontend/
```

## 5. 准备生产环境变量

```bash
cp .env.example .env
```

至少修改这些值：

```bash
POSTGRES_PASSWORD=<strong-postgres-admin-password>

FUND_DB=fundval
FUND_DB_USER=fundval
FUND_DB_PASSWORD=<strong-fund-password>

STOCK_DB=stock_etf
STOCK_DB_USER=stock
STOCK_DB_PASSWORD=<strong-stock-password>

SECRET_KEY=<strong-random-secret>
DEBUG=false
ALLOW_REGISTER=false
ALLOWED_HOSTS=<server-ip-or-domain>,127.0.0.1,localhost

TDENGINE_USER=root
TDENGINE_PASSWORD=<strong-tdengine-password>
TDENGINE_DATABASE=stock_etf_ts

XUEQIU_COOKIE=
```

生成 `SECRET_KEY` 示例：

```bash
openssl rand -base64 48
```

注意：

- `docker-compose.yml` 中 fund 容器使用的是 `FUND_DB_PASSWORD`，股票容器使用的是 `STOCK_DB_PASSWORD`。
- `.env.example` 中本地开发值不能直接用于公网服务器。
- fund 的初始化状态、`bootstrap_key` 和是否允许注册写入 `/app/config/config.json`，必须挂载到持久卷，避免容器重建后重新变成未初始化。

## 6. 首次启动

构建镜像：

```bash
docker compose build
```

启动基础设施和服务：

```bash
docker compose up -d
```

查看容器：

```bash
docker compose ps
```

执行 Django migration：

```bash
docker compose exec fund-backend python manage.py migrate
```

推荐用 bootstrap API 创建第一个管理员账号。先读取 bootstrap key：

```bash
docker compose exec fund-backend python manage.py shell -c \
  "from fundval.config import config; print(config.get('bootstrap_key'))"
```

再调用初始化接口：

```bash
BOOTSTRAP_KEY=<上一步输出的 key>

curl -X POST http://127.0.0.1:7001/api/admin/bootstrap/initialize \
  -H "Content-Type: application/json" \
  -d "{
    \"bootstrap_key\":\"${BOOTSTRAP_KEY}\",
    \"admin_username\":\"admin\",
    \"admin_password\":\"<strong-admin-password>\",
    \"allow_register\":false
  }"
```

初始化后验证：

```bash
curl -X POST http://127.0.0.1:7001/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<strong-admin-password>"}'
```

当前 dev 部署已初始化：

```text
部署目录：/mnt/tradehub/TradeHub
管理员账号：admin
注册状态：关闭
```

当前初始化密码只用于首次交付，登录后必须立即修改。

## 7. 健康检查

```bash
curl -sf http://127.0.0.1:7001/api/health/
curl -sf http://127.0.0.1:17081/health
curl -sf http://127.0.0.1:7002/healthz
curl -sf http://127.0.0.1:17080/health
curl -sf http://127.0.0.1:5173/
```

查看日志：

```bash
docker compose logs -f fund-backend
docker compose logs -f fund-research
docker compose logs -f fund-worker
docker compose logs -f fund-beat
docker compose logs -f stock-api
docker compose logs -f stock-worker
docker compose logs -f market-api
docker compose logs -f frontend
```

## 8. 4G 网关公网访问方案

如果服务器在 4G 网关后面，公网 IP 会变化，而且可能存在 CGNAT。不要依赖普通 A 记录加端口映射作为主要入口。推荐使用 Cloudflare Tunnel 或 frp 这类反向隧道。

### 8.1 Cloudflare Quick Tunnel（临时测试）

Quick Tunnel 不需要 Cloudflare 账号和域名，适合临时验证，但地址会变化，没有可用性保证。

安装：

```bash
curl -L -o /tmp/cloudflared-linux-amd64.deb \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i /tmp/cloudflared-linux-amd64.deb
```

启动临时隧道：

```bash
cloudflared tunnel \
  --url http://127.0.0.1:5173 \
  --http-host-header localhost:5173 \
  --no-autoupdate
```

日志中会出现 `https://*.trycloudflare.com` 地址。由于前端通过 Vite 代理基金 API，Django 的 `ALLOWED_HOSTS` 至少需要包含：

```text
<trycloudflare 临时域名>,fund-backend,fund-backend:8000,tradehub-fund-backend
```

改完 `.env` 后重建基金服务：

```bash
docker compose --env-file .env -p tradehub up -d --force-recreate --no-deps fund-backend fund-worker fund-beat
```

验证：

```bash
curl -I https://<trycloudflare-domain>/
curl https://<trycloudflare-domain>/api/health/
curl https://<trycloudflare-domain>/api/fund-research/v1/health
curl https://<trycloudflare-domain>/api/stock/v1/system/health
```

当前 dev 临时入口以 `cloudflared tunnel` 实时输出为准，不写死到仓库文档。Quick Tunnel 域名会变化，不能作为长期入口。

```text
https://<当前 cloudflared 输出的 trycloudflare 域名>
```

### 8.2 Cloudflare Named Tunnel（固定域名）

长期使用需要固定域名，例如 `tradehub.example.com`。流程：

1. 将域名接入 Cloudflare。
2. 在 Cloudflare Zero Trust 控制台创建 `cloudflared` tunnel。
3. 在 Public Hostname 中配置 `tradehub.example.com -> http://localhost:5173`。
4. 在服务器执行 Cloudflare 控制台给出的 `cloudflared service install <token>`。
5. 将正式域名加入 `ALLOWED_HOSTS`，并重建基金服务。

Named Tunnel 不依赖服务器公网 IP，也不需要 4G 网关入站端口映射。

### 8.3 frp 方案

如果不用 Cloudflare，需要一台固定公网 VPS 跑 `frps`，4G 服务器跑 `frpc` 主动连接 VPS，再把域名解析到 VPS。frp 软件本身免费开源，但 VPS 通常需要付费。

## 9. 初始化数据

基金基础列表：

```bash
docker compose exec fund-backend python manage.py sync_funds
```

基金排行、资料、净值等建议通过 Celery 定时任务跑。需要手动触发时可以进 Django shell：

```bash
docker compose exec fund-backend python manage.py shell
```

示例：

```python
from api.tasks import sync_top_fund_rankings_task
sync_top_fund_rankings_task(limit=500, sync_profiles_limit=500)
```

前 2000 基金近 7 天净值和日事实：

```python
from api.tasks import sync_all_fund_nav_history_and_facts
sync_all_fund_nav_history_and_facts(days=7, batch_size=500, limit=2000)
```

基金资料和持仓：

```python
from api.tasks import sync_all_fund_profiles_and_holdings
sync_all_fund_profiles_and_holdings(batch_size=300, limit=2000, sync_holdings=True)
```

市场板块快照：

```python
from api.tasks import sync_fund_sector_market_snapshots_task
sync_fund_sector_market_snapshots_task(is_close_snapshot=False)
```

## 10. 定时任务

`fund-beat` 负责基金侧 Celery beat 定时任务。部署后必须确认它在运行：

```bash
docker compose ps fund-beat fund-worker
docker compose logs --tail=200 fund-beat
docker compose logs --tail=200 fund-worker
```

如果定时任务不触发，优先检查：

- `REDIS_URL` 是否指向 `redis://redis:6379/0`
- fund-worker 是否正常连接 PostgreSQL 和 Redis
- fund-beat 日志是否有 scheduler 输出
- 容器时区是否影响日期判断。当前容器可能显示 UTC，业务查询交易日时不要只看 `date.today()`。

## 11. 备份与恢复

PostgreSQL 备份：

```bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U postgres fundval > backups/fundval_$(date +%F).sql
docker compose exec -T postgres pg_dump -U postgres stock_etf > backups/stock_etf_$(date +%F).sql
```

PostgreSQL 恢复：

```bash
docker compose exec -T postgres psql -U postgres fundval < backups/fundval_YYYY-MM-DD.sql
docker compose exec -T postgres psql -U postgres stock_etf < backups/stock_etf_YYYY-MM-DD.sql
```

Docker volume 查看：

```bash
docker volume ls | grep tradehub
```

TDengine 数据量较大，生产建议使用云盘快照或 TDengine 官方备份方案。

## 11. 更新发布

```bash
git pull
docker compose build
docker compose up -d
docker compose exec fund-backend python manage.py migrate
docker compose ps
```

发布后验证：

```bash
curl -sf http://127.0.0.1:7001/api/health/
curl -sf http://127.0.0.1:7002/healthz
curl -sf http://127.0.0.1:17080/health
```

回滚方式取决于 Git 和镜像策略。最低限度保留上一次可运行 commit：

```bash
git checkout <last-good-commit>
docker compose build
docker compose up -d
docker compose exec fund-backend python manage.py migrate
```

## 12. 常见问题

### 12.1 数据库初始化后用户密码不一致

首次创建 PostgreSQL volume 时，`docker/init-scripts/postgres/init-db.sql` 会创建 `fundval` 和 `stock` 用户。后续修改 `.env` 不会自动修改已有数据库用户密码。

处理方式：

```bash
docker compose exec postgres psql -U postgres
```

```sql
ALTER USER fundval WITH PASSWORD '<new-password>';
ALTER USER stock WITH PASSWORD '<new-password>';
```

### 12.2 修改 `.env` 后没生效

重新创建相关容器：

```bash
docker compose up -d --force-recreate fund-backend fund-worker fund-beat stock-api stock-worker
```

### 12.3 前端接口地址不对

当前 frontend 容器使用：

```bash
VITE_FUND_API_BASE=http://127.0.0.1:7001/api
VITE_STOCK_API_BASE=http://127.0.0.1:7002
VITE_MARKET_API_BASE=http://127.0.0.1:17080
VITE_FUND_API_TARGET=http://fund-backend:8000
VITE_STOCK_API_TARGET=http://stock-api:8000
VITE_FUND_RESEARCH_API_TARGET=http://fund-research:18081
```

如果通过域名/Nginx 访问，需要改成对应公网域名或反向代理路径，并重新创建 frontend 容器。

### 12.4 基金净值同步很慢

东财部分基金代码会返回 notfound 或连接超时。同步任务应限制范围，例如前 2000，只处理有效展示基金。后续建议把请求超时收紧到 8-10 秒，并优化有效基金池。

### 12.5 服务器时间与交易日不一致

容器可能是 UTC。业务统计交易日时优先用明确日期，例如 `2026-06-25`，不要只依赖容器内 `date.today()`。

## 13. 当前文档完整性评估

已有文档：

- `README.md`：说明架构基线和本地快速启动。
- `LOCAL_DEV.md`：说明本地开发模式，Docker 只跑基础设施。
- `ARCHITECTURE.md`：说明阶段性架构。
- `docs/技术架构/*`：说明 API、数据库、股票模块、Agent 设计。

缺口：

- 缺少新机器部署步骤。
- 缺少生产 `.env` 必改项说明。
- 缺少一体化 Docker Compose 启动顺序。
- 缺少迁移、管理员账号、初始化数据说明。
- 缺少备份恢复和常见故障处理。
- 缺少“文档不随代码提交”的 Git 规则说明。

本文补齐上述部署缺口。

## 14. Git 提交规则：只提交代码，不提交 docs

当前要求是提交 Git 时不要提交 `docs/` 和部署/描述类文档，只提交代码。

建议规则：

```gitignore
docs/
DEPLOYMENT.md
*.部署.md
```

如果这些文件从未被 Git 跟踪，`.gitignore` 生效后不会出现在 `git status`。

如果已经被 Git 跟踪，仅加 `.gitignore` 不够，需要执行：

```bash
git rm -r --cached docs
git rm --cached DEPLOYMENT.md
git commit -m "chore: stop tracking generated docs"
```

以后提交代码前检查：

```bash
git status --short
git diff --name-only --cached
```

只提交代码示例：

```bash
git add tradehub-fund tradehub-stock tradehub-market-api tradehub-frontend docker-compose.yml .env.example .gitignore
git status --short
git commit -m "feat: update tradehub runtime"
```

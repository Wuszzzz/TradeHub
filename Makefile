# TradeHub Makefile

.PHONY: help dev dev-infra dev-stop dev-restart dev-logs dev-status build prod prod-stop clean test-fund test-stock health setup

help: ## 显示帮助信息
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  TradeHub — 个人量化投资平台"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'
	@echo ""

# 开发环境
dev: ## 启动本地基础设施（业务服务用 ./dev.sh fund/stock/market/frontend 分终端启动）
	@./dev.sh start

dev-infra: ## 仅启动基础设施（postgres/redis/tdengine）
	@./dev.sh start

dev-stop: ## 停止全部服务
	@echo "停止 TradeHub..."
	@./dev.sh stop

dev-restart: ## 重启基础设施
	@./dev.sh restart

dev-logs: ## 实时查看基础设施日志
	@./dev.sh logs

dev-logs-fund: ## 基金服务在宿主机终端运行，请查看对应终端日志
	@echo "fund backend logs are printed in the terminal running ./dev.sh fund"

dev-logs-stock: ## 股票服务在宿主机终端运行，请查看对应终端日志
	@echo "stock api logs are printed in the terminal running ./dev.sh stock"

dev-logs-market: ## 行情网关在宿主机终端运行，请查看对应终端日志
	@echo "market api logs are printed in the terminal running ./dev.sh market"

dev-status: ## 查看基础设施和宿主机业务服务状态
	@./dev.sh status

# 构建相关
build: ## 构建全部镜像
	@echo "构建全部镜像..."
	@docker compose build

build-fund: ## 重建基金服务镜像
	@docker compose build fund-backend fund-worker fund-beat

build-stock: ## 重建股票服务镜像
	@docker compose build stock-api stock-worker

build-market: ## 重建行情网关镜像
	@docker compose build market-api

build-frontend: ## 重建前端镜像
	@docker compose build frontend

# 生产环境
prod: ## 启动生产环境
	@echo "启动生产环境..."
	@docker compose -f docker/docker-compose.prod.yml up -d

prod-stop: ## 停止生产环境
	@docker compose -f docker/docker-compose.prod.yml stop

prod-logs: ## 查看生产环境日志
	@docker compose -f docker/docker-compose.prod.yml logs -f

# 数据库
db-migrate-fund: ## 执行基金服务 Django migrations
	@./dev.sh migrate

db-makemigrations: ## 生成 Django migration 文件
	@cd tradehub-fund && python manage.py makemigrations

db-shell-fund: ## 进入基金数据库 Shell
	@docker compose -f docker-compose.local.yml -p tradehub exec postgres psql -U fundval fundval

db-shell-stock: ## 进入股票数据库 Shell
	@docker compose -f docker-compose.local.yml -p tradehub exec postgres psql -U stock stock_etf

db-shell-pg: ## 以 postgres 超级用户进入数据库
	@docker compose -f docker-compose.local.yml -p tradehub exec postgres psql -U postgres

# 测试
test-fund: ## 运行基金后端测试
	@echo "运行基金后端测试..."
	@cd tradehub-fund && python -m pytest tests/ -v

test-stock: ## 运行股票后端测试（Go）
	@echo "运行股票后端测试..."
	@cd tradehub-stock && go test ./...

test-market: ## 运行 market-api 测试
	@cd tradehub-market-api && go test ./...

test-all: ## 运行全部测试
	@make test-fund
	@make test-stock
	@make test-market

# 健康检查
health: ## 检查所有服务健康状态
	@./dev.sh health

# 容器访问
exec-fund: ## 进入基金后端容器
	@echo "fund backend runs on host in local dev. Use the terminal running ./dev.sh fund."

exec-stock: ## 进入股票后端容器
	@echo "stock api runs on host in local dev. Use the terminal running ./dev.sh stock."

exec-postgres: ## 进入 PostgreSQL 容器
	@docker compose -f docker-compose.local.yml -p tradehub exec postgres bash

# 清理
clean: ## 停止服务并清理容器（保留数据卷）
	@echo "清理容器（数据卷保留）..."
	@docker compose -f docker-compose.local.yml -p tradehub down --remove-orphans

clean-all: ## 清理容器和数据卷（危险操作，数据会丢失）
	@echo "警告：此操作将删除所有数据！"
	@read -p "确认继续？输入 yes: " confirm && [ "$$confirm" = "yes" ]
	@docker compose -f docker-compose.local.yml -p tradehub down -v --remove-orphans
	@docker system prune -f

# 环境初始化
setup: ## 初始化项目环境
	@echo "初始化 TradeHub 环境..."
	@docker --version > /dev/null 2>&1 || (echo "请先安装 Docker" && exit 1)
	@[ -f .env ] || cp .env.example .env
	@echo "环境初始化完成，请编辑 .env 配置后运行 make dev"

version: ## 显示版本信息
	@echo "Docker: $$(docker --version)"
	@echo "Docker Compose: $$(docker compose version)"
	@echo "Go: $$(go version 2>/dev/null || echo '未安装')"
	@echo "Python: $$(python3 --version 2>/dev/null || echo '未安装')"

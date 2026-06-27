#!/bin/bash
# TradeHub 本地开发环境管理脚本
# 模式：Docker 只跑基础设施，业务服务在宿主机直接启动

set -e

COMPOSE_FILE="docker-compose.local.yml"
PROJECT_NAME="tradehub"

usage() {
  echo "用法: ./dev.sh <命令>"
  echo ""
  echo "基础设施命令（Docker）："
  echo "  start        启动基础设施（postgres:5433 / redis:6380 / tdengine:16042）"
  echo "  stop         停止基础设施"
  echo "  restart      重启基础设施"
  echo "  status       查看容器状态"
  echo "  logs         查看基础设施日志"
  echo ""
  echo "业务服务命令（宿主机直接运行）："
  echo "  fund         启动基金后端（Django :7001）"
  echo "  stock        启动股票后端（Go :7002）"
  echo "  worker       启动股票后台任务（Go worker）"
  echo "  market       启动行情网关（Go :17080）"
  echo "  frontend     启动前端（Vite :5173）"
  echo ""
  echo "工具命令："
  echo "  health       检查所有服务健康状态"
  echo "  shell-db     进入 PostgreSQL 容器"
  echo "  migrate      执行 Django migrations"
}

start() {
  echo "启动基础设施..."
  [ -f .env ] || (cp .env.example .env && echo "已创建 .env，请检查配置")
  docker compose -f $COMPOSE_FILE -p $PROJECT_NAME up -d
  echo ""
  echo "基础设施已启动："
  echo "  PostgreSQL  → localhost:5433"
  echo "  Redis       → localhost:6380"
  echo "  TDengine    → localhost:16042"
  echo ""
  echo "下一步，分别在新终端启动业务服务："
  echo "  ./dev.sh fund      # 基金后端 :7001"
  echo "  ./dev.sh stock     # 股票后端 :7002"
  echo "  ./dev.sh market    # 行情网关 :17080"
  echo "  ./dev.sh frontend  # 前端 :5173"
}

stop() {
  echo "停止基础设施..."
  docker compose -f $COMPOSE_FILE -p $PROJECT_NAME stop
}

restart() {
  docker compose -f $COMPOSE_FILE -p $PROJECT_NAME restart
}

status() {
  echo "=== 基础设施（Docker）==="
  docker compose -f $COMPOSE_FILE -p $PROJECT_NAME ps
  echo ""
  echo "=== 业务服务（宿主机）==="
  echo -n "fund-backend (:7001):  " && curl -sf http://localhost:7001/api/health/ > /dev/null 2>&1 && echo "运行中" || echo "未运行"
  echo -n "stock-api    (:7002):  " && curl -sf http://localhost:7002/healthz > /dev/null 2>&1 && echo "运行中" || echo "未运行"
  echo -n "market-api   (:17080): " && curl -sf http://localhost:17080/health > /dev/null 2>&1 && echo "运行中" || echo "未运行"
  echo -n "frontend     (:5173):  " && curl -sf http://localhost:5173/ > /dev/null 2>&1 && echo "运行中" || echo "未运行"
}

health() {
  echo "=== TradeHub 健康检查 ==="
  echo ""
  echo "基础设施："
  echo -n "  PostgreSQL  (5433): " && docker compose -f $COMPOSE_FILE exec -T postgres pg_isready -U postgres > /dev/null 2>&1 && echo "OK" || echo "FAIL"
  echo -n "  Redis       (6380): " && docker compose -f $COMPOSE_FILE exec -T redis redis-cli ping > /dev/null 2>&1 && echo "OK" || echo "FAIL"
  echo -n "  TDengine   (16042): " && curl -sf http://localhost:16042 > /dev/null 2>&1; [ $? -ne 7 ] && echo "OK" || echo "FAIL"
  echo ""
  echo "业务服务："
  echo -n "  fund-backend  (7001): " && curl -sf http://localhost:7001/api/health/ > /dev/null 2>&1 && echo "OK" || echo "FAIL"
  echo -n "  stock-api     (7002): " && curl -sf http://localhost:7002/healthz > /dev/null 2>&1 && echo "OK" || echo "FAIL"
  echo -n "  market-api   (17080): " && curl -sf http://localhost:17080/health > /dev/null 2>&1 && echo "OK" || echo "FAIL"
}

logs() {
  docker compose -f $COMPOSE_FILE -p $PROJECT_NAME logs -f "${@}"
}

fund() {
  echo "启动基金后端（Django :7001）..."
  [ -f .env ] && export $(grep -v '^#' .env | xargs)
  export DB_TYPE="${DB_TYPE:-postgresql}"
  export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
  export POSTGRES_PORT="${POSTGRES_PORT:-5433}"
  export POSTGRES_DB="${POSTGRES_DB:-${FUND_DB:-fundval}}"
  export POSTGRES_USER="${POSTGRES_USER:-${FUND_DB_USER:-fundval}}"
  export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-${FUND_DB_PASSWORD:-tradehub_local_password}}"
  cd tradehub-fund
  python manage.py runserver 0.0.0.0:7001
}

stock() {
  echo "启动股票后端（Go :7002）..."
  [ -f .env ] && export $(grep -v '^#' .env | xargs)
  export PATH="$(pwd)/scripts:$PATH"
  export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
  export POSTGRES_PORT="${STOCK_DB_PORT:-${POSTGRES_PORT:-5433}}"
  export POSTGRES_DB="${STOCK_DB:-stock_etf}"
  export POSTGRES_USER="${STOCK_DB_USER:-stock}"
  export POSTGRES_PASSWORD="${STOCK_DB_PASSWORD:-${POSTGRES_PASSWORD}}"
  cd tradehub-stock/api
  API_PORT=7002 go run .
}

worker() {
  echo "启动股票后台任务（Go worker）..."
  [ -f .env ] && export $(grep -v '^#' .env | xargs)
  export PATH="$(pwd)/scripts:$PATH"
  export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
  export POSTGRES_PORT="${STOCK_DB_PORT:-${POSTGRES_PORT:-5433}}"
  export POSTGRES_DB="${STOCK_DB:-stock_etf}"
  export POSTGRES_USER="${STOCK_DB_USER:-stock}"
  export POSTGRES_PASSWORD="${STOCK_DB_PASSWORD:-${POSTGRES_PASSWORD:-tradehub_local_password}}"
  export TDENGINE_HOST="${TDENGINE_HOST:-localhost}"
  export TDENGINE_PORT="${TDENGINE_PORT:-16042}"
  export TDENGINE_USER="${TDENGINE_USER:-root}"
  export TDENGINE_PASSWORD="${TDENGINE_PASSWORD:-taosdata}"
  export TDENGINE_DATABASE="${TDENGINE_DATABASE:-stock_etf_ts}"
  export MARKET_API_URL="${MARKET_API_URL:-http://127.0.0.1:17080}"
  export AKSHARE_PYTHON_BIN="${AKSHARE_PYTHON_BIN:-python3}"
  export AKSHARE_SCRIPT_PATH="${AKSHARE_SCRIPT_PATH:-$(pwd)/tradehub-stock/network/akshare_adapter.py}"
  cd tradehub-stock
  go run ./worker
}

market() {
  echo "启动行情网关（Go :17080）..."
  [ -f .env ] && export $(grep -v '^#' .env | xargs)
  cd tradehub-market-api
  MARKET_API_ADDR=":17080" go run ./cmd/market-api
}

frontend() {
  echo "启动前端（Vite :5173）..."
  [ -f .env ] && export $(grep -v '^#' .env | xargs)
  cd tradehub-frontend
  npm run dev -- --host 0.0.0.0 --port "${FRONTEND_PORT:-5173}"
}

migrate() {
  echo "执行 Django migrations..."
  [ -f .env ] && export $(grep -v '^#' .env | xargs)
  export DB_TYPE="${DB_TYPE:-postgresql}"
  export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
  export POSTGRES_PORT="${POSTGRES_PORT:-5433}"
  export POSTGRES_DB="${POSTGRES_DB:-${FUND_DB:-fundval}}"
  export POSTGRES_USER="${POSTGRES_USER:-${FUND_DB_USER:-fundval}}"
  export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-${FUND_DB_PASSWORD:-tradehub_local_password}}"
  cd tradehub-fund
  python manage.py migrate
}

case "${1:-help}" in
  start)    start ;;
  stop)     stop ;;
  restart)  restart ;;
  status)   status ;;
  logs)     shift; logs "$@" ;;
  health)   health ;;
  fund)     fund ;;
  stock)    stock ;;
  worker)   worker ;;
  market)   market ;;
  frontend) frontend ;;
  migrate)  migrate ;;
  shell-db) docker compose -f $COMPOSE_FILE exec postgres psql -U postgres ;;
  *)        usage ;;
esac

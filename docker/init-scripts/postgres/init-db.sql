-- TradeHub 数据库初始化脚本
-- 在 PostgreSQL 容器启动时自动执行，创建两个业务数据库

-- 创建基金数据库和用户
CREATE DATABASE fundval;
CREATE USER fundval WITH PASSWORD 'tradehub_local_password';
GRANT ALL PRIVILEGES ON DATABASE fundval TO fundval;

-- 创建股票数据库和用户
CREATE DATABASE stock_etf;
CREATE USER stock WITH PASSWORD 'tradehub_local_password';
GRANT ALL PRIVILEGES ON DATABASE stock_etf TO stock;

-- 授予 schema 权限
\c fundval
GRANT ALL ON SCHEMA public TO fundval;

\c stock_etf
GRANT ALL ON SCHEMA public TO stock;

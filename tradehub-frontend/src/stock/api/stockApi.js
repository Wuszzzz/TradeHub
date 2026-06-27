/**
 * Stock API Client - 股票后端 API 客户端
 * TradeHub Stock Backend API v1
 */

import axios from 'axios';

const BASE_URL = import.meta.env.VITE_STOCK_API_URL || '/api/stock/v1';

// 创建 axios 实例
const stockApi = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器 - 添加认证 Token
stockApi.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// 响应拦截器 - 统一错误处理
stockApi.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const message = error.response?.data?.message || error.message || '请求失败';
    console.error('[StockAPI Error]', message);
    return Promise.reject(new Error(message));
  }
);

// =============== System APIs ===============

export const systemApi = {
  /** 健康检查 */
  health: () => stockApi.get('/system/health'),
  /** 架构概览 */
  overview: () => stockApi.get('/system/overview'),
};

// =============== Instrument APIs ===============

export const instrumentApi = {
  /** 搜索股票标的 */
  search: (keyword) => stockApi.get('/instruments/search', { params: { keyword } }),
  /** 获取标的资料 */
  profile: (symbol) => stockApi.get('/instruments/profile', { params: { symbol } }),
};

// =============== Market APIs ===============

export const marketApi = {
  /** 实时快照 */
  snapshot: (symbol) => stockApi.get('/market/snapshot', { params: { symbol } }),
  /** K线数据 */
  kline: (symbol, period = '1d', limit = 200) =>
    stockApi.get('/market/kline', { params: { symbol, period, limit } }),
  /** ETF风控信息 */
  etfRisk: (symbol, limit = 120) =>
    stockApi.get('/etf/risk', { params: { symbol, limit } }),
};

// =============== Watchlist APIs ===============

export const watchlistApi = {
  /** 获取所有分组 */
  getGroups: () => stockApi.get('/watchlist/groups'),
  /** 创建/更新分组 */
  createGroup: (data) => stockApi.post('/watchlist/groups', data),
  /** 删除分组 */
  deleteGroup: (groupId) => stockApi.delete('/watchlist/groups', { params: { group_id: groupId } }),
  /** 获取自选项 */
  getItems: (groupId) => stockApi.get('/watchlist/items', { params: { group_id: groupId } }),
  /** 添加/更新自选项 */
  addItem: (data) => stockApi.post('/watchlist/items', data),
  /** 删除自选项 */
  deleteItem: (itemId) => stockApi.delete('/watchlist/items', { params: { item_id: itemId } }),
  /** 获取自选快照（批量行情） */
  getSnapshot: (groupId) => stockApi.get('/watchlist/snapshot', { params: { group_id: groupId } }),
};

// =============== Alert APIs ===============

export const alertApi = {
  /** 获取告警规则 */
  getRules: () => stockApi.get('/alerts/rules'),
  /** 创建/更新告警规则 */
  createRule: (data) => stockApi.post('/alerts/rules', data),
  /** 删除告警规则 */
  deleteRule: (ruleId) => stockApi.delete('/alerts/rules', { params: { rule_id: ruleId } }),
  /** 获取告警事件 */
  getEvents: (status, limit = 200) =>
    stockApi.get('/alerts/events', { params: { status, limit } }),
  /** 确认告警事件 */
  ackEvent: (eventId) => stockApi.post('/alerts/events/ack', null, { params: { event_id: eventId } }),
};

// =============== Paper Trading APIs ===============

export const paperApi = {
  /** 获取订单列表 */
  getOrders: (symbol, limit = 200) =>
    stockApi.get('/paper/orders', { params: { symbol, limit } }),
  /** 提交订单 */
  placeOrder: (data) => stockApi.post('/paper/orders', data),
  /** 获取持仓 */
  getPositions: () => stockApi.get('/paper/positions'),
  /** 获取账户 */
  getAccount: () => stockApi.get('/paper/account'),
  /** 重置账户 */
  resetAccount: (initial = 1000000) =>
    stockApi.post('/paper/reset', { initial }),
};

// =============== Quant APIs ===============

export const quantApi = {
  /** 获取指标定义 */
  getIndicators: (category, enabledOnly = true) =>
    stockApi.get('/quant/indicators', { params: { category, enabled_only: enabledOnly } }),
  /** 创建/更新指标 */
  createIndicator: (data) => stockApi.post('/quant/indicators', data),
  /** 获取指标值 */
  getIndicatorValues: (symbol, indicatorCode, period = '1d', limit = 200) =>
    stockApi.get('/quant/indicator-values', { params: { symbol, indicator_code: indicatorCode, period, limit } }),
  /** 获取形态定义 */
  getPatterns: (category, enabledOnly = true) =>
    stockApi.get('/quant/patterns', { params: { category, enabled_only: enabledOnly } }),
  /** 创建/更新形态 */
  createPattern: (data) => stockApi.post('/quant/patterns', data),
  /** 获取形态命中 */
  getPatternHits: (symbol, period = '1d', patternCode, limit = 200) =>
    stockApi.get('/quant/pattern-hits', { params: { symbol, period, pattern_code: patternCode, limit } }),
};

// =============== Screener APIs ===============

export const screenerApi = {
  /** 按指标选股 */
  screenByIndicator: (params) => stockApi.get('/screener/indicator', { params }),
  /** 按形态选股 */
  screenByPattern: (params) => stockApi.get('/screener/pattern', { params }),
  /** 获取选股模板 */
  getTemplates: (enabledOnly = false) =>
    stockApi.get('/screener/templates', { params: { enabled_only: enabledOnly } }),
  /** 创建/更新选股模板 */
  createTemplate: (data) => stockApi.post('/screener/templates', data),
  /** 删除选股模板 */
  deleteTemplate: (templateId) =>
    stockApi.delete('/screener/templates', { params: { template_id: templateId } }),
  /** 获取选股结果 */
  getResults: (taskId, templateId, limit = 200) =>
    stockApi.get('/screener/results', { params: { task_id: taskId, template_id: templateId, limit } }),
  /** 创建组合筛选任务 */
  createScreeningTask: (conditions, limit = 200) =>
    stockApi.post('/tasks', { task_type: 'screening', params: { conditions, limit } }),
};

// =============== Strategy APIs ===============

export const strategyApi = {
  /** 获取策略模板 */
  getTemplates: (enabledOnly = false) =>
    stockApi.get('/strategies/templates', { params: { enabled_only: enabledOnly } }),
  /** 创建/更新策略 */
  createTemplate: (data) => stockApi.post('/strategies/templates', data),
  /** 删除策略 */
  deleteTemplate: (strategyId) =>
    stockApi.delete('/strategies/templates', { params: { strategy_id: strategyId } }),
  /** 获取策略运行记录 */
  getRuns: (strategyId, taskId, status, limit = 200) =>
    stockApi.get('/strategies/runs', { params: { strategy_id: strategyId, task_id: taskId, status, limit } }),
};

// =============== Backtest APIs ===============

export const backtestApi = {
  /** 执行回测 */
  execute: (data) => stockApi.post('/backtest/execute', data),
  /** 获取可用策略列表 */
  getStrategies: () => stockApi.get('/backtest/strategies'),
  /** 获取回测结果 */
  getResults: (taskId, symbol, limit = 200) =>
    stockApi.get('/backtest/results', { params: { task_id: taskId, symbol, limit } }),
  /** 获取回测汇总 */
  getSummaries: (taskId, limit = 200) =>
    stockApi.get('/backtest/summaries', { params: { task_id: taskId, limit } }),
};

// =============== Task APIs ===============

export const taskApi = {
  /** 获取采集任务 */
  getIngestionTasks: () => stockApi.get('/tasks/ingestion'),
  /** 创建采集任务 */
  createIngestionTask: (data) => stockApi.post('/tasks/ingestion', data),
  /** 删除采集任务 */
  deleteIngestionTask: (taskId) =>
    stockApi.delete('/tasks/ingestion', { params: { task_id: taskId } }),
  /** 获取通用任务 */
  getTasks: (params) => stockApi.get('/tasks', { params }),
  /** 创建通用任务 */
  createTask: (data) => stockApi.post('/tasks', data),
  /** 获取任务日志 */
  getTaskLogs: (taskId) => stockApi.get('/tasks/logs', { params: { task_id: taskId } }),
};

// =============== Data Center APIs ===============

export const datacenterApi = {
  /** 数据中心健康状态 */
  health: () => stockApi.get('/datacenter/health'),
  /** 数据中心采集任务 */
  getTasks: (taskType, limit = 100) =>
    stockApi.get('/datacenter/tasks', { params: { task_type: taskType, limit } }),
  /** 触发采集 */
  collect: (taskType, targetDate) =>
    stockApi.post('/datacenter/collect', null, { params: { task_type: taskType, target_date: targetDate } }),
};

// =============== Broker APIs ===============

export const brokerApi = {
  /** 获取券商适配状态 */
  getStatus: () => stockApi.get('/broker/status'),
};

export default stockApi;

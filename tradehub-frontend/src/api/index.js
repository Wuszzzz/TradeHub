import api, { publicApi } from './axios';

// 系统管理
export const healthCheck = () => api.get('/health/');

// Bootstrap 初始化（不带 token）
export const verifyBootstrapKey = (key) =>
  publicApi.post('/admin/bootstrap/verify', { bootstrap_key: key });

export const initializeSystem = (data) =>
  publicApi.post('/admin/bootstrap/initialize', data);

// 认证（不带 token）
export const login = (username, password) =>
  publicApi.post('/auth/login', { username, password });

export const register = (username, password, passwordConfirm) =>
  publicApi.post('/users/register/', { username, password, password_confirm: passwordConfirm });

export const refreshToken = (refreshToken) =>
  publicApi.post('/auth/refresh', { refresh_token: refreshToken });

export const getCurrentUser = () => api.get('/auth/me');

export const changePassword = (oldPassword, newPassword) =>
  api.put('/auth/password', {
    old_password: oldPassword,
    new_password: newPassword,
  });

// 基金管理
export const fundsAPI = {
  list: (params) => api.get('/funds/', { params }),
  get: (code) => api.get(`/funds/${code}/`),
  detail: (fundCode) => api.get(`/funds/${fundCode}/`),
  estimate: (fundCode) => api.get(`/funds/${fundCode}/estimate/`),
  marketQuote: (fundCode) => api.get(`/funds/${fundCode}/market_quote/`),
  community: (fundCode, params) => api.get(`/funds/${fundCode}/community/`, { params }),
  search: (keyword) => api.get('/funds/', { params: { search: keyword } }),
  getEstimate: (code, source) => api.get(`/funds/${code}/estimate/`, { params: { source } }),
  getAccuracy: (code) => api.get(`/funds/${code}/accuracy/`),
  batchEstimate: (fundCodes, source = 'eastmoney') => api.post('/funds/batch_estimate/', { fund_codes: fundCodes, source }),
  batchUpdateNav: (fundCodes) => api.post('/funds/batch_update_nav/', { fund_codes: fundCodes }),
  batchUpdateTodayNav: (fundCodes) => api.post('/funds/batch_update_today_nav/', { fund_codes: fundCodes }),
  queryNav: (fundCode, operationDate, before15) => api.post('/funds/query_nav/', {
    fund_code: fundCode,
    operation_date: operationDate,
    before_15: before15
  }),
  navHistory: (fundCode, params) => api.get('/nav-history/', {
    params: { fund_code: fundCode, ...params }
  }),
  syncNavHistory: (fundCodes, startDate, endDate) => api.post('/nav-history/sync/', {
    fund_codes: fundCodes,
    start_date: startDate,
    end_date: endDate
  }),
  indexHoldings: (fundCode, source = 'eastmoney') => api.get(`/funds/${fundCode}/index_holdings/`, { params: { source } }),
  holdingsRealtime: (fundCode) => api.get(`/funds/${fundCode}/holdings-realtime/`),
  syncProfile: (fundCode) => api.post(`/funds/${fundCode}/sync-profile/`),
  syncHoldings: (fundCode, source = 'tencent_fund') => api.post(`/funds/${fundCode}/sync-holdings/`, { source }),
  storedHoldings: (fundCode) => api.get(`/funds/${fundCode}/holdings-stored/`),
  companies: (params) => api.get('/fund-companies/', { params }),
  managers: (params) => api.get('/fund-managers/', { params }),
  managerTenures: (params) => api.get('/fund-manager-tenures/', { params }),
  holdingSnapshots: (params) => api.get('/fund-holding-snapshots/', { params }),
  allocationSnapshots: (params) => api.get('/fund-allocation-snapshots/', { params }),
  sectorMarketSnapshots: (params) => api.get('/fund-sector-market-snapshots/', { params }),
  sectorRotationAnalysis: (params) => api.get('/fund-sector-market-snapshots/rotation-analysis/', { params }),
  syncSectorMarketSnapshots: (data) => api.post('/fund-sector-market-snapshots/sync/', data),
  performanceRanks: (params) => api.get('/fund-performance-ranks/', { params }),
  dailyFacts: (params) => api.get('/fund-daily-facts/', { params }),
  backfillDailyFacts: (data) => api.post('/fund-daily-facts/backfill/', data),
  estimateIntraday: (fundCode, source = 'eastmoney') => api.get(`/funds/${fundCode}/estimate-intraday/`, { params: { source } }),
  marketKline: (fundCode, params) => api.get(`/funds/${fundCode}/market-kline/`, { params }),
  compare: (codes) => api.get('/funds/compare/', { params: { codes: codes.join(',') } }),
  marketIndices: () => api.get('/funds/market-indices/'),
  rankings: (params) => api.get('/funds/rankings/', { params }),
  tencentDetail: (symbol) => api.get(`/tencent-fund/${symbol}/`),
};

export const fundResearchAPI = {
  summary: () => api.get('/fund-research/v1/summary'),
  screen4433: (params) => api.get('/fund-research/v1/funds/4433', { params, timeout: 120000 }),
  filter: (params) => api.get('/fund-research/v1/funds/filter', { params, timeout: 120000 }),
  check: (codes) => api.post('/fund-research/v1/funds/check', { codes }, { timeout: 120000 }),
  similarity: (codes) => api.post('/fund-research/v1/funds/similarity', { codes }, { timeout: 120000 }),
  byStock: (keywords) => api.get('/fund-research/v1/funds/by-stock', { params: { keywords }, timeout: 120000 }),
  managers: (params) => api.get('/fund-research/v1/managers', { params, timeout: 120000 }),
  relatedSectors: (codes, quote = true) => api.get('/fund-research/v1/sectors/related', { params: { codes: codes.join(','), quote: quote ? 1 : 0 }, timeout: 120000 }),
  sectorQuotes: (secids) => api.get('/fund-research/v1/sectors/quotes', { params: { secids: secids.join(',') }, timeout: 120000 }),
  recommendTags: (codes) => api.get('/fund-research/v1/tags/recommend', { params: { codes: codes.join(',') }, timeout: 120000 }),
  syncStatus: () => api.get('/fund-research/v1/sync/status'),
  syncSectorMap: (items, seed = false) => api.post('/fund-research/v1/sync/sector-map', { items, seed }, { timeout: 120000 }),
  syncEvaluations: (payload = {}) => api.post('/fund-research/v1/sync/evaluations', payload, { timeout: 180000 }),
};

// 股票 / ETF 监控
export const stockAPI = {
  overview: () => api.get('/stock/v1/overview'),
  search: (keyword) => api.get('/stock/v1/instruments/search', { params: { keyword } }),
  instruments: (params) => api.get('/stock/v1/instruments', { params }),
  realtime: (symbol) => api.get('/stock/v1/market/realtime', { params: { symbol } }),
  history: (symbol, interval, limit) => api.get('/stock/v1/market/history', { 
    params: { symbol, interval, limit } 
  }),
  daily: (symbol, start, end, limit) => api.get('/stock/v1/market/daily', {
    params: { symbol, start, end, limit }
  }),
  profile: (symbol) => api.get('/stock/v1/instruments/profile', { params: { symbol } }),
  etfRisk: (symbol, limit = 200) => api.get('/stock/v1/etf/risk', { params: { symbol, limit } }),
  listTasks: () => api.get('/stock/v1/ingestion/tasks'),
  createTask: (data) => api.post('/stock/v1/ingestion/tasks', data),
  deleteTask: (taskId) => api.delete('/stock/v1/ingestion/tasks', { params: { task_id: taskId } }),
  watchlistGroups: () => api.get('/stock/v1/watchlist/groups'),
  createWatchlistGroup: (data) => api.post('/stock/v1/watchlist/groups', data),
  deleteWatchlistGroup: (groupId) => api.delete('/stock/v1/watchlist/groups', { params: { group_id: groupId } }),
  watchlistItems: (groupId) => api.get('/stock/v1/watchlist/items', { params: groupId ? { group_id: groupId } : {} }),
  watchlistSnapshot: (groupId) => api.get('/stock/v1/watchlist/snapshot', { params: groupId ? { group_id: groupId } : {} }),
  addWatchlistItem: (data) => api.post('/stock/v1/watchlist/items', data),
  deleteWatchlistItem: (itemId) => api.delete('/stock/v1/watchlist/items', { params: { item_id: itemId } }),
  paperAccount: () => api.get('/stock/v1/paper/account'),
  paperPositions: () => api.get('/stock/v1/paper/positions'),
  paperOrders: (params) => api.get('/stock/v1/paper/orders', { params }),
  placePaperOrder: (data) => api.post('/stock/v1/paper/orders', data),
  paperReset: (data) => api.post('/stock/v1/paper/reset', data),
  alertRules: () => api.get('/stock/v1/alerts/rules'),
  createAlertRule: (data) => api.post('/stock/v1/alerts/rules', data),
  deleteAlertRule: (ruleId) => api.delete('/stock/v1/alerts/rules', { params: { rule_id: ruleId } }),
  alertEvents: (params) => api.get('/stock/v1/alerts/events', { params }),
  ackAlertEvent: (eventId) => api.post('/stock/v1/alerts/events/ack', null, { params: { event_id: eventId } }),
  globalBoard: (params) => api.get('/tencent-market/board/', { params }),
  globalIndices: () => api.get('/tencent-market/indices/'),
  globalUS: (params) => api.get('/tencent-market/us/', { params }),
  globalFX: (params) => api.get('/tencent-market/fx/', { params }),
  globalFutures: (params) => api.get('/tencent-market/futures/', { params }),
};

// 账户管理
export const accountsAPI = {
  list: () => api.get('/accounts/'),
  create: (data) => api.post('/accounts/', data),
  update: (id, data) => api.put(`/accounts/${id}/`, data),
  delete: (id) => api.delete(`/accounts/${id}/`),
  deleteInfo: (id) => api.get(`/accounts/${id}/delete_info/`),
};

// 持仓管理
export const positionsAPI = {
  list: (accountId) => api.get('/positions/', { params: { account_id: accountId } }),
  listByFund: (fundCode, config = {}) => api.get('/positions/', { params: { fund_code: fundCode }, ...config }),
  createOperation: (data) => api.post('/positions/operations/', data),
  listOperations: (params, config = {}) => api.get('/positions/operations/', { params, ...config }),
  deleteOperation: (id) => api.delete(`/positions/operations/${id}/`),
  batchDeleteOperations: (operationIds) => api.post('/positions/operations/batch_delete/', { operation_ids: operationIds }),
  clearPosition: (id) => api.delete(`/positions/${id}/clear/`),
  getHistory: (accountId, days = 30) => api.get('/positions/history/', {
    params: { account_id: accountId, days }
  }),
};

// 自选列表
export const watchlistsAPI = {
  list: () => api.get('/watchlists/'),
  create: (data) => api.post('/watchlists/', data),
  get: (id) => api.get(`/watchlists/${id}/`),
  delete: (id) => api.delete(`/watchlists/${id}/`),
  addItem: (id, fundCode) => api.post(`/watchlists/${id}/items/`, { fund_code: fundCode }),
  removeItem: (id, fundCode) => api.delete(`/watchlists/${id}/items/${fundCode}/`),
  reorder: (id, items) => api.put(`/watchlists/${id}/reorder/`, { items }),
};

// 用户偏好
export const preferencesAPI = {
  get: () => api.get('/preferences/'),
  update: (data) => {
    // 兼容旧调用方式：传 string 视为 preferred_source
    const body = typeof data === 'string' ? { preferred_source: data } : data;
    return api.put('/preferences/', body);
  },
};

// AI配置与分析
export const aiAPI = {
  getConfig: () => api.get('/ai/config/'),
  updateConfig: (data) => api.put('/ai/config/', data),
  listTemplates: (contextType) => api.get('/ai/templates/', { params: contextType ? { context_type: contextType } : {} }),
  createTemplate: (data) => api.post('/ai/templates/', data),
  updateTemplate: (id, data) => api.put(`/ai/templates/${id}/`, data),
  deleteTemplate: (id) => api.delete(`/ai/templates/${id}/`),
  analyze: (templateId, contextType, contextData) => api.post('/ai/analyze/', {
    template_id: templateId,
    context_type: contextType,
    context_data: contextData,
  }, { timeout: 120000 }),
  reportPreview: (period) => api.post('/ai/report-preview/', { period }, { timeout: 180000 }),
};

// 数据源凭证
export const sourceAPI = {
  getQRCode: (sourceName) =>
    api.post('/source-credentials/qrcode/', { source_name: sourceName }),
  checkQRCodeState: (sourceName, qrId) =>
    api.get(`/source-credentials/qrcode/${qrId}/state/`, { params: { source_name: sourceName } }),
  logout: (sourceName) =>
    api.post('/source-credentials/logout/', { source_name: sourceName }),
  getStatus: (sourceName) =>
    api.get('/source-credentials/status/', { params: { source_name: sourceName } }),
  importFromYangJiBao: (overwrite = false) =>
    api.post('/source-credentials/import/', { overwrite }),
  sendSms: (sourceName, phone) =>
    api.post('/source-credentials/phone/send-sms/', { source_name: sourceName, phone }),
  verifyPhone: (sourceName, phone, code) =>
    api.post('/source-credentials/phone/verify/', { source_name: sourceName, phone, code }),
  importHoldings: (sourceName, overwrite = false) =>
    api.post('/source-credentials/import/', { source_name: sourceName, overwrite }),
};

// 通知渠道
export const notificationChannelsAPI = {
  list: () => api.get('/notification-channels/'),
  create: (data) => api.post('/notification-channels/', data),
  update: (id, data) => api.patch(`/notification-channels/${id}/`, data),
  delete: (id) => api.delete(`/notification-channels/${id}/`),
  test: (id) => api.post(`/notification-channels/${id}/test/`),
};

// 通知规则
export const notificationRulesAPI = {
  list: () => api.get('/notification-rules/'),
  create: (data) => api.post('/notification-rules/', data),
  update: (id, data) => api.patch(`/notification-rules/${id}/`, data),
  delete: (id) => api.delete(`/notification-rules/${id}/`),
};

// 通知记录
export const notificationLogsAPI = {
  list: (params) => api.get('/notification-logs/', { params }),
};

// 管理员
export const adminAPI = {
  listUsers: (params) => api.get('/admin/users/', { params }),
  toggleUser: (userId) => api.post(`/admin/users/${userId}/toggle/`),
  resetPassword: (userId) => api.post(`/admin/users/${userId}/reset-password/`),
  getStats: () => api.get('/admin/stats/'),
  triggerTask: (taskName) => api.post(`/admin/tasks/${taskName}/`, {}, { timeout: 120000 }),
};

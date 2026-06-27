/**
 * Stock Utils - 股票模块工具函数
 */

// =============== 格式化函数 ===============

/**
 * 格式化金额
 */
export const formatMoney = (value, decimals = 2) => {
  const n = Number(value || 0);
  return n.toLocaleString('zh-CN', { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
};

/**
 * 格式化数字
 */
export const formatNumber = (value, digits = 2) => {
  if (value === null || value === undefined || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return n.toFixed(digits);
};

/**
 * 格式化百分比
 */
export const formatPercent = (value, digits = 2) => {
  if (value === null || value === undefined || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return `${n > 0 ? '+' : ''}${n.toFixed(digits)}%`;
};

/**
 * 转数字
 */
export const toNumber = (value) => {
  if (value === null || value === undefined || value === '') return null;
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
};

/**
 * 格式化数字（无正负号）
 */
export const fmtNum = (value, digits = 2) => {
  const n = toNumber(value);
  if (n === null) return '--';
  return n.toFixed(digits);
};

/**
 * 格式化百分比（带正负号）
 */
export const fmtPct = (value, digits = 2) => {
  const n = toNumber(value);
  if (n === null) return '--';
  return `${n > 0 ? '+' : ''}${n.toFixed(digits)}%`;
};

/**
 * 格式化大数字（亿/万）
 */
export const fmtBig = (value) => {
  const n = toNumber(value);
  if (n === null) return '--';
  if (Math.abs(n) >= 1e8) return `${(n / 1e8).toFixed(2)}亿`;
  if (Math.abs(n) >= 1e4) return `${(n / 1e4).toFixed(2)}万`;
  return n.toFixed(0);
};

/**
 * 格式化时间（短格式）
 */
export const fmtTimeShort = (ts) => {
  if (!ts) return '--';
  const s = String(ts);
  if (s.length >= 16) {
    return s.slice(5, 16).replace('T', ' ');
  }
  if (s.length >= 10) {
    return s.slice(5, 10);
  }
  return s;
};

/**
 * 涨跌样式类
 */
export const trendClass = (value) => {
  const n = toNumber(value);
  if (n === null || n === 0) return '';
  return n > 0 ? 'up' : 'down';
};

// =============== 扩展工具函数 ===============

/**
 * 格式化价格
 */
export const formatPrice = (price, decimals = 2) => {
  if (price == null || isNaN(price)) return '--';
  return Number(price).toFixed(decimals);
};

/**
 * 格式化涨跌幅
 */
export const formatChangePercent = (percent) => {
  if (percent == null || isNaN(percent)) return '--';
  const sign = percent >= 0 ? '+' : '';
  return `${sign}${Number(percent).toFixed(2)}%`;
};

/**
 * 格式化成交量（万/亿）
 */
export const formatVolume = (volume) => {
  if (volume == null || isNaN(volume)) return '--';
  volume = Number(volume);
  if (volume >= 1e8) {
    return `${(volume / 1e8).toFixed(2)}亿`;
  }
  if (volume >= 1e4) {
    return `${(volume / 1e4).toFixed(2)}万`;
  }
  return volume.toFixed(0);
};

/**
 * 格式化金额（万/亿）
 */
export const formatAmount = (amount, decimals = 2) => {
  if (amount == null || isNaN(amount)) return '--';
  amount = Number(amount);
  if (Math.abs(amount) >= 1e8) {
    return `${(amount / 1e8).toFixed(decimals)}亿`;
  }
  if (Math.abs(amount) >= 1e4) {
    return `${(amount / 1e4).toFixed(decimals)}万`;
  }
  return amount.toFixed(decimals);
};

/**
 * 格式化持仓数量
 */
export const formatQty = (qty) => {
  if (qty == null || isNaN(qty)) return '--';
  qty = Number(qty);
  if (qty >= 1e8) {
    return `${(qty / 1e8).toFixed(2)}亿`;
  }
  if (qty >= 1e4) {
    return `${(qty / 1e4).toFixed(2)}万`;
  }
  return qty.toFixed(0);
};

/**
 * 格式化日期时间
 */
export const formatDateTime = (date) => {
  if (!date) return '--';
  const d = new Date(date);
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
};

/**
 * 格式化日期
 */
export const formatDate = (date) => {
  if (!date) return '--';
  const d = new Date(date);
  return d.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  });
};

// =============== 颜色工具 ===============

/**
 * 根据涨跌幅获取颜色
 */
export const getChangeColor = (change) => {
  if (change == null || change === 0) return '#62748a';
  return change > 0 ? '#ee4444' : '#00a54c';
};

/**
 * 根据涨跌幅获取背景色
 */
export const getChangeBgColor = (change) => {
  if (change == null || change === 0) return 'rgba(98,116,138,0.1)';
  return change > 0 ? 'rgba(238,68,68,0.1)' : 'rgba(0,165,76,0.1)';
};

// =============== 市场工具 ===============

/**
 * 获取市场名称
 */
export const getMarketName = (market) => {
  const marketMap = {
    'CN-A': '沪深A股',
    'CN-B': '沪深B股',
    'HK': '港股',
    'US': '美股',
    'SH': '上海',
    'SZ': '深圳',
    'BJ': '北京',
  };
  return marketMap[market] || market || '未知';
};

/**
 * 标准化股票代码（添加市场前缀）
 */
export const normalizeSymbol = (symbol, market = 'CN-A') => {
  if (!symbol) return '';
  symbol = symbol.toString().trim().toUpperCase();

  // 已经包含前缀
  if (/^(SH|SZ|BJ|HK|US)/.test(symbol)) {
    return symbol;
  }

  // 根据代码判断市场
  if (/^(688|600|601|603|605|000|001|002|003|030688)/.test(symbol)) {
    return `sh${symbol}`;
  }
  if (/^(002|003|300|301)/.test(symbol)) {
    return `sz${symbol}`;
  }
  if (/^8|^4/.test(symbol)) {
    return `bj${symbol}`;
  }
  if (/^\d{5}/.test(symbol)) {
    return `hk${symbol}`;
  }
  if (/^[A-Z]/i.test(symbol)) {
    return `us${symbol}`;
  }

  return symbol;
};

// =============== 技术指标工具 ===============

/**
 * 获取指标分类
 */
export const getIndicatorCategory = (category) => {
  const categoryMap = {
    trend: '趋势指标',
    momentum: '动量指标',
    volatility: '波动指标',
    volume: '成交量指标',
    custom: '自定义指标',
  };
  return categoryMap[category] || category || '未知';
};

/**
 * 获取周期选项
 */
export const getPeriodOptions = () => [
  { value: '1m', label: '1分钟' },
  { value: '5m', label: '5分钟' },
  { value: '15m', label: '15分钟' },
  { value: '30m', label: '30分钟' },
  { value: '1h', label: '1小时' },
  { value: '1d', label: '日线' },
  { value: '1w', label: '周线' },
  { value: '1M', label: '月线' },
];

// =============== K线形态工具 ===============

/**
 * 获取形态方向
 */
export const getPatternDirection = (direction) => {
  const directionMap = {
    bullish: '看涨',
    bearish: '看跌',
    neutral: '中性',
    both: '双向',
  };
  return directionMap[direction] || direction || '未知';
};

/**
 * 获取形态方向颜色
 */
export const getPatternDirectionColor = (direction) => {
  const colorMap = {
    bullish: '#ee4444',
    bearish: '#00a54c',
    neutral: '#666',
    both: '#1890ff',
  };
  return colorMap[direction] || '#666';
};

/**
 * 获取形态中文名
 */
export const getPatternName = (code) => {
  const patternNames = {
    engulfing_pattern: '吞噬模式',
    hammer: '锤头',
    inverted_hammer: '倒锤头',
    doji: '十字星',
    morning_star: '晨星',
    evening_star: '暮星',
    three_white_soldiers: '三个白兵',
    three_black_crows: '三只乌鸦',
    hanging_man: '上吊线',
    shooting_star: '射击之星',
    dark_cloud_cover: '乌云压顶',
    piercing_pattern: '刺透形态',
    abandoned_baby: '弃婴',
    gravestone_doji: '墓碑十字',
    dragonfly_doji: '蜻蜓十字',
  };
  return patternNames[code] || code;
};

// =============== 业务工具 ===============

/**
 * 获取订单方向
 */
export const getSideName = (side) => {
  return side === 'buy' ? '买入' : '卖出';
};

/**
 * 获取订单状态
 */
export const getOrderStatusName = (status) => {
  const statusMap = {
    pending: '待成交',
    filled: '已成交',
    canceled: '已撤单',
    rejected: '已拒绝',
  };
  return statusMap[status] || status || '未知';
};

/**
 * 获取任务状态
 */
export const getTaskStatusName = (status) => {
  const statusMap = {
    pending: '等待中',
    running: '运行中',
    success: '成功',
    failed: '失败',
  };
  return statusMap[status] || status || '未知';
};

/**
 * 获取任务状态颜色
 */
export const getTaskStatusColor = (status) => {
  const colorMap = {
    pending: '#999',
    running: '#1890ff',
    success: '#52c41a',
    failed: '#ff4d4f',
  };
  return colorMap[status] || '#666';
};

// =============== 计算工具 ===============

/**
 * 计算浮盈亏
 */
export const calcUnrealizedPL = (qty, avgCost, lastPrice) => {
  if (!qty || !avgCost || !lastPrice) return 0;
  return (lastPrice - avgCost) * qty;
};

/**
 * 计算浮盈亏率
 */
export const calcUnrealizedPLRate = (avgCost, lastPrice) => {
  if (!avgCost || !lastPrice || avgCost === 0) return 0;
  return ((lastPrice - avgCost) / avgCost) * 100;
};

/**
 * 计算持仓市值
 */
export const calcMarketValue = (qty, price) => {
  if (!qty || !price) return 0;
  return qty * price;
};

// =============== 杂项工具 ===============

/**
 * 防抖函数
 */
export const debounce = (fn, delay = 300) => {
  let timer = null;
  return function (...args) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), delay);
  };
};

/**
 * 节流函数
 */
export const throttle = (fn, delay = 300) => {
  let last = 0;
  return function (...args) {
    const now = Date.now();
    if (now - last >= delay) {
      last = now;
      fn.apply(this, args);
    }
  };
};

/**
 * 深拷贝
 */
export const deepClone = (obj) => {
  if (obj === null || typeof obj !== 'object') return obj;
  if (obj instanceof Date) return new Date(obj);
  if (obj instanceof Array) return obj.map(deepClone);
  if (obj instanceof Object) {
    const copy = {};
    for (const key in obj) {
      if (obj.hasOwnProperty(key)) {
        copy[key] = deepClone(obj[key]);
      }
    }
    return copy;
  }
  return obj;
};

/**
 * 生成唯一ID
 */
export const generateId = (prefix = '') => {
  const timestamp = Date.now().toString(36);
  const random = Math.random().toString(36).substring(2, 8);
  return `${prefix}${timestamp}_${random}`;
};

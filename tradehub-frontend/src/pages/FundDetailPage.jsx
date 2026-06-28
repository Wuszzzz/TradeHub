import { useState, useEffect, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Card,
  Space,
  Spin,
  Empty,
  message,
  Button,
  Table,
  Alert,
  Tag,
  Select,
  Typography,
} from 'antd';
import { CommentOutlined, DatabaseOutlined, LineChartOutlined, RobotOutlined, SyncOutlined } from '@ant-design/icons';
import ReactECharts from 'echarts-for-react';
import { fundsAPI, positionsAPI, stockAPI } from '../api';
import AIAnalysisModal from '../components/AIAnalysisModal';
import { usePreference } from '../contexts/PreferenceContext';

const canTryLoadHoldings = (fundData) => {
  const type = fundData?.fund_type || '';
  const name = fundData?.fund_name || '';
  const text = `${type} ${name}`;
  return /指数|ETF|联接/i.test(text);
};

const communityRequestCache = new Map();
const unwrapList = (payload) => payload?.results || payload?.data?.results || payload || [];
const { Text } = Typography;

const FundDetailPage = () => {
  const { code } = useParams();
  const navigate = useNavigate();
  const { preferredSource } = usePreference();
  const [loading, setLoading] = useState(true);
  const [fund, setFund] = useState(null);
  const [estimate, setEstimate] = useState(null);
  const [marketQuote, setMarketQuote] = useState(null);
  const [navHistory, setNavHistory] = useState([]);
  const [accuracy, setAccuracy] = useState(null);
  const [positions, setPositions] = useState([]);
  const [operations, setOperations] = useState([]);
  const [timeRange, setTimeRange] = useState('1M');
  const [chartLoading, setChartLoading] = useState(false);
  const [holdings, setHoldings] = useState([]);
  const [holdingsLoading, setHoldingsLoading] = useState(false);
  const [holdingsSource, setHoldingsSource] = useState('');
  const [holdingsTargetCode, setHoldingsTargetCode] = useState('');
  const [storedSnapshot, setStoredSnapshot] = useState(null);
  const [storedSnapshotLoading, setStoredSnapshotLoading] = useState(false);
  const [allocationSnapshots, setAllocationSnapshots] = useState([]);
  const [syncSnapshotLoading, setSyncSnapshotLoading] = useState(false);
  const [syncNavLoading, setSyncNavLoading] = useState(false);
  const [intraday, setIntraday] = useState([]);
  const [holdingsEligible, setHoldingsEligible] = useState(false);
  const [tencentDetail, setTencentDetail] = useState(null);
  const [klineData, setKlineData] = useState([]);
  const [klineSource, setKlineSource] = useState('eastmoney');
  const [klinePeriod, setKlinePeriod] = useState('day');
  const [klineLoading, setKlineLoading] = useState(false);
  const [stockProfileMap, setStockProfileMap] = useState({});
  const [community, setCommunity] = useState(null);
  const [communityLoading, setCommunityLoading] = useState(false);
  const [sectorMarketMap, setSectorMarketMap] = useState({});

  // AI 分析
  const [aiModalVisible, setAiModalVisible] = useState(false);

  const buildAiContextData = () => {
    const navHistoryStr = navHistory.slice(-30).map(h => `${h.nav_date}:${h.unit_nav}`).join(',');
    const pos = positions.find(p => p.fund?.fund_code === code);
    return {
      fund_code: fund?.fund_code || '',
      fund_name: fund?.fund_name || '',
      fund_type: fund?.fund_type || '',
      latest_nav: fund?.latest_nav || '',
      latest_nav_date: fund?.latest_nav_date || '',
      estimate_nav: estimate?.estimate_nav || '',
      estimate_growth: estimate?.estimate_growth || '',
      nav_history: navHistoryStr,
      holding_share: pos?.holding_share || '',
      holding_cost: pos?.holding_cost || '',
      holding_value: pos?.market_value || '',
      pnl: pos?.profit || '',
      pnl_rate: pos?.profit_rate || '',
    };
  };

  // 加载历史净值
  const loadNavHistory = async (range) => {
    try {
      // 计算日期范围
      const now = new Date();
      const startDate = new Date();

      switch (range) {
        case '1W':
          startDate.setDate(now.getDate() - 7);
          break;
        case '1M':
          startDate.setMonth(now.getMonth() - 1);
          break;
        case '3M':
          startDate.setMonth(now.getMonth() - 3);
          break;
        case '6M':
          startDate.setMonth(now.getMonth() - 6);
          break;
        case '1Y':
          startDate.setFullYear(now.getFullYear() - 1);
          break;
        case 'ALL':
          // 10 年前
          startDate.setFullYear(now.getFullYear() - 10);
          break;
      }

      const startDateStr = startDate.toISOString().split('T')[0];

      const params = range === 'ALL' ? {} : { start_date: startDateStr };
      const response = await fundsAPI.navHistory(code, params);

      // 按日期正序排列
      const data = response.data.sort((a, b) =>
        new Date(a.nav_date) - new Date(b.nav_date)
      );

      setNavHistory(data);
    } catch (error) {
      console.error('Load nav history error:', error);
    }
  };

  // 加载持仓分布
  const loadPositions = async () => {
    try {
      const response = await positionsAPI.listByFund(code);

      // 计算市值和盈亏
      const positionsWithCalc = response.data.map(pos => {
        // 使用持仓数据中的基金净值，如果没有则使用页面的基金净值
        const latestNav = pos.fund?.latest_nav || fund?.latest_nav || 0;
        const marketValue = parseFloat(pos.holding_share) * parseFloat(latestNav);
        const costValue = parseFloat(pos.holding_cost);
        const profit = marketValue - costValue;
        const profitRate = costValue > 0 ? (profit / costValue * 100) : 0;

        return {
          ...pos,
          market_value: marketValue.toFixed(2),
          profit: profit.toFixed(2),
          profit_rate: profitRate.toFixed(2)
        };
      });

      setPositions(positionsWithCalc);
    } catch (error) {
      // 未认证或没有持仓，不显示错误
      setPositions([]);
    }
  };

  const loadMarketKline = async (source = klineSource, period = klinePeriod) => {
    setKlineLoading(true);
    try {
      const response = await fundsAPI.marketKline(code, {
        source,
        period,
        limit: period === 'day' ? 180 : 120,
      });
      setKlineData(response.data.rows || []);
      setKlineSource(response.data.source || source);
    } catch {
      setKlineData([]);
    } finally {
      setKlineLoading(false);
    }
  };

  const loadStoredHoldings = async () => {
    setStoredSnapshotLoading(true);
    try {
      const response = await fundsAPI.storedHoldings(code);
      setStoredSnapshot(response.data?.latest_snapshot || null);
    } catch {
      setStoredSnapshot(null);
    } finally {
      setStoredSnapshotLoading(false);
    }
  };

  const loadAllocations = async () => {
    try {
      const response = await fundsAPI.allocationSnapshots({ fund_code: code });
      setAllocationSnapshots(response.data?.results || response.data || []);
    } catch {
      setAllocationSnapshots([]);
    }
  };

  const syncStoredHoldings = async () => {
    setSyncSnapshotLoading(true);
    try {
      await fundsAPI.syncProfile(code);
      const response = await fundsAPI.syncHoldings(code, 'tencent_fund');
      if (response.data?.success === false) {
        message.warning('已同步基金资料，腾讯暂未返回可入库持仓');
      } else {
        message.success('已同步腾讯资料和持仓快照');
      }
      await Promise.all([
        fundsAPI.detail(code).then((res) => setFund(res.data)).catch(() => null),
        loadStoredHoldings(),
        loadAllocations(),
      ]);
    } catch (error) {
      message.error(error.response?.data?.error || error.message || '同步入库失败');
    } finally {
      setSyncSnapshotLoading(false);
    }
  };

  const syncNavAndFacts = async () => {
    setSyncNavLoading(true);
    try {
      const now = new Date();
      const start = new Date();
      start.setFullYear(now.getFullYear() - 1);
      const startDate = start.toISOString().slice(0, 10);
      const endDate = now.toISOString().slice(0, 10);
      const response = await fundsAPI.syncNavHistory([code], startDate, endDate);
      const result = response.data?.[code];
      if (result?.success === false) {
        message.error(result.error || '净值历史同步失败');
        return;
      }
      message.success(`已同步净值历史并写入日事实：${result?.count || 0} 条`);
      await Promise.all([
        loadNavHistory(timeRange),
        fundsAPI.detail(code).then((res) => setFund(res.data)).catch(() => null),
      ]);
    } catch (error) {
      message.error(error.response?.data?.error || error.message || '净值历史同步失败');
    } finally {
      setSyncNavLoading(false);
    }
  };

  // 加载操作记录
  const loadOperations = async () => {
    try {
      const response = await positionsAPI.listOperations({ fund_code: code });
      setOperations(response.data);
    } catch (error) {
      // 未认证或没有操作记录，不显示错误
      setOperations([]);
    }
  };

  // 加载成分股持仓（含实时行情）
  const loadHoldings = async (fundType) => {
    if (!holdingsEligible) {
      setHoldings([]);
      return;
    }
    setHoldingsLoading(true);
    try {
      const response = await fundsAPI.holdingsRealtime(code);
      setHoldings(response.data.holdings || []);
      setHoldingsSource(response.data.holdings_source || '');
      setHoldingsTargetCode(response.data.target_code || '');
    } catch {
      setHoldings([]);
      setHoldingsSource('');
      setHoldingsTargetCode('');
    } finally {
      setHoldingsLoading(false);
    }
  };

  const loadCommunity = async () => {
    const cacheKey = `${code}:eastmoney_guba`;
    const cached = communityRequestCache.get(cacheKey);
    if (cached && Date.now() - cached.ts < 30000) {
      setCommunity(cached.data);
      return;
    }
    setCommunityLoading(true);
    try {
      const { data } = await fundsAPI.community(code, { source: 'eastmoney_guba', limit: 20 });
      communityRequestCache.set(cacheKey, { ts: Date.now(), data });
      setCommunity(data);
    } catch (error) {
      setCommunity({
        fund_code: code,
        source: 'eastmoney_guba',
        items: [],
        community_url: `https://guba.eastmoney.com/list,of${code}.html`,
        error: error.response?.data?.error || error.message || '社区动态加载失败',
      });
    } finally {
      setCommunityLoading(false);
    }
  };

  const loadSectorMarkets = async (industryNames) => {
    const names = (industryNames || []).filter(Boolean).slice(0, 8);
    if (!names.length) {
      setSectorMarketMap({});
      return;
    }
    try {
      const responses = await Promise.all(names.map((name) => (
        fundsAPI.sectorMarketSnapshots({
          board_code: 'industry',
          latest: 1,
          keyword: name,
          page_size: 5,
        }).catch(() => null)
      )));
      const nextMap = {};
      responses.forEach((response) => {
        unwrapList(response?.data).forEach((item) => {
          nextMap[item.sector_name] = item;
        });
      });
      setSectorMarketMap(nextMap);
    } catch {
      setSectorMarketMap({});
    }
  };

  // 页面加载
  useEffect(() => {
    const loadData = async () => {
      setLoading(true);

      try {
        // 并发加载基金详情、指定源估值、准确率历史和场内价格
        const [detailRes, estimateRes, accuracyRes, marketRes, intradayRes] = await Promise.all([
          fundsAPI.detail(code),
          fundsAPI.getEstimate(code, preferredSource).catch(() => null),
          fundsAPI.getAccuracy(code).catch(() => null),
          fundsAPI.marketQuote(code).catch(() => null),
          fundsAPI.estimateIntraday(code, preferredSource).catch(() => null),
        ]);

        setFund(detailRes.data);
        setEstimate(estimateRes?.data || null);
        setAccuracy(accuracyRes?.data || null);
        setMarketQuote(marketRes?.data || null);
        setIntraday(intradayRes?.data?.snapshots || []);
        const eligible = canTryLoadHoldings(detailRes.data);
        setHoldingsEligible(eligible);
        setLoading(false);

        const marketSymbol = detailRes.data?.fund_code?.startsWith('5') || detailRes.data?.fund_code?.startsWith('6') ? `sh${detailRes.data.fund_code}` : `sz${detailRes.data.fund_code}`;
        fundsAPI.tencentDetail(marketSymbol).then((res) => setTencentDetail(res.data || null)).catch(() => setTencentDetail(null));

        // 加载成分股（指数 / ETF / 联接基金都尝试）
        if (eligible) {
          loadHoldings(detailRes.data?.fund_type);
        } else {
          setHoldings([]);
        }
        loadStoredHoldings();
        loadAllocations();
        loadMarketKline();
        loadCommunity();
        loadNavHistory(timeRange);
        loadPositions();
        loadOperations();

        // 尝试更新当日净值（静默失败）
        fundsAPI.batchUpdateTodayNav([code]).catch(() => {
          // 静默失败，不影响页面加载
        });
      } catch (error) {
        message.error('加载基金详情失败');
        setLoading(false);
      } finally {
        // 首屏 loading 在主数据返回后立即关闭；后台模块各自维护 loading 状态。
      }
    };

    loadData();
  }, [code, preferredSource]);

  // 持仓穿透 30s 自动刷新
  useEffect(() => {
    if (!holdingsEligible || holdings.length === 0) return;
    const interval = setInterval(() => loadHoldings(fund.fund_type), 30000);
    return () => clearInterval(interval);
  }, [fund?.fund_type, holdings.length, holdingsEligible]);

  // 获取主要数据源的准确率记录
  // 计算场内溢价率: (场内价格 - 实时估值) / 实时估值
  const calculatePremium = () => {
    if (!estimate?.estimate_nav || !marketQuote?.market_price) return null;
    const est = parseFloat(estimate.estimate_nav);
    const mkt = parseFloat(marketQuote.market_price);
    if (est === 0) return null;
    // (场内价格 - 实时估值) / 实时估值
    return ((mkt - est) / est) * 100;
  };

  const premium = calculatePremium();
  const tencentAsset = tencentDetail?.asset || {};
  const tencentRank = tencentDetail?.rank_info || {};
  const tencentSameTypeFunds = tencentDetail?.same_type_funds || [];
  const tencentSameSeriesFunds = tencentDetail?.same_series_funds || [];
  const tencentNotices = tencentDetail?.notices || [];
  const storedItems = storedSnapshot?.items || [];
  const storedIndustries = allocationSnapshots.filter((item) => item.allocation_type === 'industry');
  const storedAssets = allocationSnapshots.filter((item) => item.allocation_type === 'asset');
  const displayIndustries = storedIndustries.length > 0
    ? storedIndustries
    : (tencentAsset.industry || []).map((item) => ({ name: item.name, ratio: item.ratio }));
  const displayAssets = storedAssets.length > 0
    ? storedAssets
    : (tencentAsset.asset || []).map((item) => ({ name: item.name, ratio: item.ratio }));
  const topIndustryNames = displayIndustries.slice(0, 3).map((item) => item.name);
  const displaySectorMarkets = displayIndustries
    .map((item) => ({
      ...item,
      market: sectorMarketMap[item.name],
    }))
    .filter((item) => item.market)
    .sort((a, b) => Number(b.ratio || 0) - Number(a.ratio || 0));
  const holdingsDisplay = useMemo(() => {
    if (holdings.length > 0) return holdings;
    if (storedItems.length > 0) {
      return storedItems.map((item) => ({
        stock_code: item.asset_code,
        stock_name: item.asset_name,
        weight: item.weight,
        price: item.price,
        change_percent: item.change_percent,
        contribution: item.contribution,
        holding_type: item.holding_type,
      }));
    }
    return (tencentAsset.stock || tencentAsset.product || []).map((item) => ({
      stock_code: item.code,
      stock_name: item.name,
      weight: item.ratio,
      price: null,
      change_percent: item.rate,
      contribution: null,
      holding_type: (tencentAsset.stock || []).length > 0 ? 'stock' : 'product',
    }));
  }, [holdings, storedItems, tencentAsset.product, tencentAsset.stock]);
  const holdingsWithProfiles = holdingsDisplay.map((item) => {
    const profile = stockProfileMap[item.stock_code] || {};
    return {
      ...item,
      stock_board: profile.board,
      stock_industry: profile.industry,
      stock_market: profile.market,
    };
  });
  const holdingsDisplaySource = holdings.length > 0
    ? (holdingsSource === 'tencent_fund' ? '腾讯实时' : holdingsSource === 'eastmoney' ? '东方财富实时' : '实时接口')
    : storedItems.length > 0
      ? `数据库快照 ${storedSnapshot?.report_date || ''}`
      : '腾讯资料';
  const activeRangeLabel = {
    '1W': '近1周',
    '1M': '近1月',
    '3M': '近3月',
    '6M': '近6月',
    '1Y': '近1年',
    ALL: '全部',
    INTRADAY: '当日估值',
  }[timeRange] || timeRange;
  const latestNavPoint = navHistory[navHistory.length - 1];
  const firstNavPoint = navHistory[0];
  const navValues = navHistory.map((item) => Number(item.unit_nav)).filter((value) => Number.isFinite(value));
  const minNav = navValues.length ? Math.min(...navValues) : null;
  const maxNav = navValues.length ? Math.max(...navValues) : null;
  const estimateLineValue = estimate?.estimate_nav ? Number(estimate.estimate_nav) : null;
  const trendReturn = firstNavPoint?.unit_nav && latestNavPoint?.unit_nav
    ? ((Number(latestNavPoint.unit_nav) - Number(firstNavPoint.unit_nav)) / Number(firstNavPoint.unit_nav)) * 100
    : null;
  const trendReturnClass = Number(trendReturn) >= 0 ? 'up' : 'down';

  const chartOption = useMemo(() => ({
    backgroundColor: 'transparent',
    color: ['#0f6fff', '#d99000'],
    tooltip: {
      trigger: 'axis',
      appendToBody: true,
      backgroundColor: 'rgba(12, 20, 33, 0.92)',
      borderWidth: 0,
      padding: [10, 12],
      textStyle: { color: '#f8fafc', fontSize: 12 },
      axisPointer: {
        type: 'line',
        lineStyle: { color: 'rgba(15, 111, 255, 0.42)', width: 1.5 },
      },
      formatter: (params = []) => {
        const date = params[0]?.axisValue || '';
        const rows = params.map((item) => {
          const value = Number(item.value);
          return `<div style="display:flex;justify-content:space-between;gap:18px;margin-top:4px;">
            <span>${item.marker}${item.seriesName}</span>
            <b>${Number.isFinite(value) ? value.toFixed(4) : '--'}</b>
          </div>`;
        }).join('');
        return `<div style="font-weight:700;margin-bottom:6px;">${date}</div>${rows}`;
      },
    },
    legend: {
      right: 18,
      top: 12,
      itemWidth: 18,
      itemHeight: 8,
      textStyle: { color: '#475467', fontSize: 12 },
      data: ['单位净值', '实时估值'],
    },
    grid: { left: 42, right: 28, top: 58, bottom: 34, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: navHistory.map(item => item.nav_date),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: 'rgba(15, 23, 42, 0.12)' } },
      axisLabel: {
        color: '#667085',
        hideOverlap: true,
        margin: 14,
        rotate: window.innerWidth < 768 ? 35 : 0,
      },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: { color: '#667085', formatter: (value) => Number(value).toFixed(3) },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: 'rgba(15, 23, 42, 0.07)', type: 'dashed' } },
    },
    dataZoom: navHistory.length > 80 ? [
      { type: 'inside', start: 0, end: 100 },
      {
        type: 'slider',
        height: 18,
        bottom: 4,
        borderColor: 'transparent',
        fillerColor: 'rgba(15, 111, 255, 0.14)',
        handleStyle: { color: '#0f6fff' },
        textStyle: { color: '#98a2b3' },
      },
    ] : [{ type: 'inside', start: 0, end: 100 }],
    series: [
      {
        name: '单位净值',
        type: 'line',
        data: navHistory.map(item => Number(item.unit_nav)),
        smooth: 0.32,
        symbol: 'circle',
        symbolSize: navHistory.length > 90 ? 0 : 5,
        showSymbol: navHistory.length <= 90,
        lineStyle: {
          width: 3,
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 1,
            y2: 0,
            colorStops: [
              { offset: 0, color: '#0f6fff' },
              { offset: 0.58, color: '#23a6d5' },
              { offset: 1, color: '#20b486' },
            ],
          },
          shadowColor: 'rgba(15, 111, 255, 0.24)',
          shadowBlur: 12,
        },
        itemStyle: { color: '#0f6fff', borderColor: '#fff', borderWidth: 2 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(15, 111, 255, 0.22)' },
              { offset: 0.62, color: 'rgba(32, 180, 134, 0.08)' },
              { offset: 1, color: 'rgba(255, 255, 255, 0)' },
            ],
          },
        },
        markPoint: {
          symbolSize: 34,
          data: operations.map(op => {
            const dateIndex = navHistory.findIndex(item => item.nav_date === op.operation_date);
            if (dateIndex === -1) return null;
            return {
              name: op.operation_type === 'BUY' ? '买入' : '卖出',
              coord: [dateIndex, Number(op.nav)],
              value: op.operation_type === 'BUY' ? '买' : '卖',
              itemStyle: { color: op.operation_type === 'BUY' ? '#cf1322' : '#0f9f6e' },
              label: { show: true, formatter: '{c}', color: '#fff', fontWeight: 700 },
            };
          }).filter(Boolean),
        },
      },
      {
        name: '实时估值',
        type: 'line',
        data: navHistory.map(() => Number.isFinite(estimateLineValue) ? estimateLineValue : null),
        smooth: true,
        symbol: 'none',
        connectNulls: true,
        lineStyle: { width: 2, color: '#d99000', type: 'dashed' },
      },
    ],
  }), [navHistory, operations, estimateLineValue]);

  const intradayChartOption = useMemo(() => ({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(12, 20, 33, 0.92)',
      borderWidth: 0,
      textStyle: { color: '#f8fafc' },
    },
    grid: { left: 42, right: 28, top: 34, bottom: 34, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: intraday.map(s => new Date(s.timestamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: 'rgba(15, 23, 42, 0.12)' } },
      axisLabel: { color: '#667085' },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: { color: '#667085', formatter: (value) => Number(value).toFixed(3) },
      splitLine: { lineStyle: { color: 'rgba(15, 23, 42, 0.07)', type: 'dashed' } },
    },
    series: [{
      name: '当日估值',
      type: 'line',
      data: intraday.map(s => Number(s.estimate_nav)),
      smooth: 0.35,
      symbol: 'none',
      lineStyle: { width: 3, color: '#d99000', shadowColor: 'rgba(217, 144, 0, 0.2)', shadowBlur: 10 },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(217, 144, 0, 0.22)' },
            { offset: 1, color: 'rgba(217, 144, 0, 0)' },
          ],
        },
      },
    }],
  }), [intraday]);

  useEffect(() => {
    loadSectorMarkets(displayIndustries.map((item) => item.name));
  }, [allocationSnapshots, tencentDetail]);

  useEffect(() => {
    const stockCodes = [...new Set(
      holdingsDisplay
        .filter((item) => item.holding_type !== 'product')
        .map((item) => String(item.stock_code || '').trim())
        .filter(Boolean)
    )];
    if (stockCodes.length === 0) {
      setStockProfileMap({});
      return;
    }

    let cancelled = false;
    Promise.allSettled(
      stockCodes.slice(0, 30).map(async (stockCode) => {
        const { data } = await stockAPI.profile(stockCode);
        return [stockCode, data.item || {}];
      })
    ).then((results) => {
      if (cancelled) return;
      const nextMap = {};
      results.forEach((result) => {
        if (result.status === 'fulfilled') {
          const [stockCode, profile] = result.value;
          nextMap[stockCode] = profile;
        }
      });
      setStockProfileMap(nextMap);
    });

    return () => {
      cancelled = true;
    };
  }, [holdingsDisplay]);
  const marketPriceChartOption = useMemo(() => ({
    tooltip: { trigger: 'axis' },
    xAxis: {
      type: 'category',
      data: navHistory.map((item) => item.nav_date),
    },
    yAxis: {
      type: 'value',
      scale: true,
      name: '价格',
    },
    series: [{
      name: '场内价格参考线',
      type: 'line',
      data: navHistory.map(() => marketQuote?.market_price ? Number(marketQuote.market_price) : null),
      smooth: true,
      connectNulls: true,
      lineStyle: { color: '#00bddd', width: 2 },
      itemStyle: { color: '#00bddd' },
      areaStyle: { color: 'rgba(0, 189, 221, 0.08)' },
    }],
    grid: { left: '6%', right: '3%', top: 18, bottom: 20, containLabel: true },
  }), [navHistory, marketQuote]);
  const klineChartOption = useMemo(() => ({
    animation: false,
    tooltip: { trigger: 'axis' },
    legend: { data: ['日K', '成交量'], textStyle: { color: '#667085' } },
    axisPointer: { link: [{ xAxisIndex: 'all' }] },
    grid: [
      { left: '5%', right: '3%', top: 24, height: '58%' },
      { left: '5%', right: '3%', top: '72%', height: '18%' },
    ],
    xAxis: [
      {
        type: 'category',
        data: klineData.map((item) => item.time),
        boundaryGap: true,
        axisLine: { lineStyle: { color: '#d9e2ef' } },
        axisLabel: { color: '#667085' },
      },
      {
        type: 'category',
        data: klineData.map((item) => item.time),
        boundaryGap: true,
        gridIndex: 1,
        axisLine: { lineStyle: { color: '#d9e2ef' } },
        axisLabel: { show: false },
      },
    ],
    yAxis: [
      {
        scale: true,
        axisLine: { lineStyle: { color: '#d9e2ef' } },
        splitLine: { lineStyle: { color: '#edf1f7' } },
        axisLabel: { color: '#667085' },
      },
      {
        scale: true,
        gridIndex: 1,
        splitNumber: 2,
        axisLine: { lineStyle: { color: '#d9e2ef' } },
        splitLine: { show: false },
        axisLabel: { color: '#667085' },
      },
    ],
    series: [
      {
        name: '日K',
        type: 'candlestick',
        data: klineData.map((item) => [item.open, item.close, item.low, item.high]),
        itemStyle: {
          color: '#f63c6b',
          color0: '#0f9f6e',
          borderColor: '#f63c6b',
          borderColor0: '#0f9f6e',
        },
      },
      {
        name: '成交量',
        type: 'bar',
        xAxisIndex: 1,
        yAxisIndex: 1,
        data: klineData.map((item) => ({
          value: item.volume_hand || 0,
          itemStyle: { color: Number(item.close) >= Number(item.open) ? '#f63c6b' : '#0f9f6e' },
        })),
      },
    ],
  }), [klineData]);

  // 加载中
  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px 0' }}>
          <Spin description="加载中..." />
        </div>
      </Card>
    );
  }

  // 基金不存在
  if (!fund) {
    return (
      <Card>
        <Empty description="基金不存在" />
      </Card>
    );
  }

  return (
    <div className="fund-detail-qq-page">
      <Card
        className="fund-detail-hero"
        extra={(
          <Space wrap>
            <Button icon={<SyncOutlined />} loading={syncNavLoading} onClick={syncNavAndFacts}>同步净值</Button>
            <Button icon={<DatabaseOutlined />} loading={syncSnapshotLoading} onClick={syncStoredHoldings}>同步入库</Button>
            <Button type="primary" icon={<RobotOutlined />} onClick={() => setAiModalVisible(true)}>AI 分析</Button>
          </Space>
        )}
      >
        <div className="fund-hero-head">
          <div>
            <h1>{fund.fund_name}</h1>
            <div className="fund-hero-meta">
              <span>（{fund.fund_code}）</span>
              <Tag>{fund.fund_type || 'LOF/ETF'}</Tag>
            </div>
          </div>
          <div className="fund-hero-price">
            <strong>{marketQuote?.market_price || estimate?.estimate_nav || fund.latest_nav || '-'}</strong>
            <span style={{ color: Number(marketQuote?.market_growth ?? estimate?.estimate_growth ?? 0) >= 0 ? '#cf1322' : '#3f8600' }}>
              {marketQuote?.market_growth != null ? `${Number(marketQuote.market_growth) >= 0 ? '+' : ''}${Number(marketQuote.market_growth).toFixed(2)}%` : estimate?.estimate_growth != null ? `${Number(estimate.estimate_growth) >= 0 ? '+' : ''}${Number(estimate.estimate_growth).toFixed(2)}%` : '-'}
            </span>
          </div>
        </div>
        <div className="fund-hero-stats">
          <div><span>单位净值</span><strong>{fund.latest_nav || '-'}</strong></div>
          <div><span>累计净值</span><strong>{fund.latest_nav || '-'}</strong></div>
          <div><span>实时估值</span><strong>{estimate?.estimate_nav || '-'}</strong></div>
          <div><span>净值日期</span><strong>{fund.latest_nav_date || '-'}</strong></div>
          <div><span>场内价格</span><strong>{marketQuote?.market_price || '-'}</strong></div>
          <div><span>折溢价率</span><strong>{premium != null ? `${premium.toFixed(2)}%` : '-'}</strong></div>
          <div><span>基金公司</span><strong>{fund.company_name || fund.company?.company_name || '-'}</strong></div>
          <div><span>入库持仓</span><strong>{storedSnapshot?.report_date || '未入库'}</strong></div>
          <div><span>所属板块</span><strong>{topIndustryNames.length > 0 ? topIndustryNames.join(' / ') : '-'}</strong></div>
        </div>
      </Card>

      <Card
        className="fund-panel fund-panel-span-2 fund-trend-card"
        title={(
          <div className="fund-trend-title">
            <span>基金走势</span>
            <strong>净值曲线</strong>
            <small>{activeRangeLabel} · {timeRange === 'INTRADAY' ? intraday.length : navHistory.length} 个数据点</small>
          </div>
        )}
        extra={(
          <div className={`fund-trend-return ${trendReturnClass}`}>
            <span>区间收益</span>
            <strong>{trendReturn != null && timeRange !== 'INTRADAY' ? `${trendReturn >= 0 ? '+' : ''}${trendReturn.toFixed(2)}%` : '--'}</strong>
          </div>
        )}
      >
        <div className="fund-trend-summary">
          <div>
            <span>最新单位净值</span>
            <strong>{latestNavPoint?.unit_nav || fund.latest_nav || '-'}</strong>
            <em>{latestNavPoint?.nav_date || fund.latest_nav_date || '-'}</em>
          </div>
          <div>
            <span>区间高点</span>
            <strong>{maxNav != null ? maxNav.toFixed(4) : '-'}</strong>
            <em>历史净值</em>
          </div>
          <div>
            <span>区间低点</span>
            <strong>{minNav != null ? minNav.toFixed(4) : '-'}</strong>
            <em>历史净值</em>
          </div>
          <div>
            <span>实时估值</span>
            <strong>{estimate?.estimate_nav || '-'}</strong>
            <em>{estimate?.estimate_growth != null ? `${Number(estimate.estimate_growth) >= 0 ? '+' : ''}${Number(estimate.estimate_growth).toFixed(2)}%` : '估值线'}</em>
          </div>
        </div>
        <div className="fund-trend-toolbar">
          <div>
            <span className="fund-trend-label">周期切换</span>
            <Space wrap size={[6, 6]}>
            {['1W', '1M', '3M', '6M', '1Y', 'ALL'].map(range => (
              <Button key={range} className="fund-range-pill" size="small" type={timeRange === range ? 'primary' : 'default'} onClick={() => { setTimeRange(range); loadNavHistory(range); }}>
                {range === 'ALL' ? '全部' : range === '1W' ? '1周' : range}
              </Button>
            ))}
            <Button className="fund-range-pill" size="small" type={timeRange === 'INTRADAY' ? 'primary' : 'default'} onClick={() => { setTimeRange('INTRADAY'); fundsAPI.estimateIntraday(code, preferredSource).then(res => setIntraday(res?.data?.snapshots || [])).catch(() => {}); }}>
              当日估值
            </Button>
            </Space>
          </div>
          <div className="fund-trend-meta">
            <span>图例</span>
            <div className="fund-trend-legend">
              <i className="nav" />单位净值
              <i className="estimate" />实时估值
            </div>
          </div>
        </div>
        <div className="fund-trend-chart">
          {timeRange === 'INTRADAY' ? (
            intraday.length > 0 ? (
              <ReactECharts
                option={intradayChartOption}
                style={{ height: 390 }}
              />
            ) : <Empty description="暂无当日估值数据" />
          ) : navHistory.length > 0 ? (
            <ReactECharts option={chartOption} style={{ height: 430 }} />
          ) : (
            <Empty description="暂无历史数据" />
          )}
        </div>
      </Card>

      <div className="fund-detail-grid">
        <Card title="业绩表现及排名" className="fund-panel">
          <Table
            pagination={false}
            size="small"
            rowKey="label"
            dataSource={[
              { label: '今年以来', growth: tencentRank.jzzf?.year, rank: tencentRank.jz_rank?.year, level: tencentRank.ratio_level?.year, total: tencentRank.total },
              { label: '近一个月', growth: tencentRank.jzzf?.w4, rank: tencentRank.jz_rank?.w4, level: tencentRank.ratio_level?.w4, total: tencentRank.total },
              { label: '近三个月', growth: tencentRank.jzzf?.w13, rank: tencentRank.jz_rank?.w13, level: tencentRank.ratio_level?.w13, total: tencentRank.total },
              { label: '近半年', growth: tencentRank.jzzf?.w26, rank: tencentRank.jz_rank?.w26, level: tencentRank.ratio_level?.w26, total: tencentRank.total },
              { label: '近一年', growth: tencentRank.jzzf?.w52, rank: tencentRank.jz_rank?.w52, level: tencentRank.ratio_level?.w52, total: tencentRank.total },
              { label: '成立以来', growth: tencentRank.jzzf?.total, rank: tencentRank.jz_rank?.total, level: tencentRank.ratio_level?.total, total: tencentRank.total },
            ]}
            columns={[
              { title: '区间', dataIndex: 'label', key: 'label' },
              { title: '净值增长率', dataIndex: 'growth', key: 'growth', render: (v) => v != null ? `${v}%` : '--' },
              { title: '同类排名', key: 'rank', render: (_, record) => record.rank && record.total ? `${record.rank}/${record.total}` : '--' },
              { title: '四分位', dataIndex: 'level', key: 'level', render: (v) => ({ 1: '优秀', 2: '良好', 3: '一般', 4: '不佳' }[v] || '--') },
            ]}
          />
        </Card>

        <Card
          title={`投资组合 · ${tencentAsset.report_time || storedSnapshot?.report_date || holdingsTargetCode || ''}`}
          className="fund-panel"
          extra={storedSnapshotLoading ? <Spin size="small" /> : storedSnapshot ? <Tag color="green">已入库</Tag> : <Tag>未入库</Tag>}
        >
          <div className="fund-asset-split">
            <div>
              <h4>资产配置</h4>
              <ul>{displayAssets.map((item) => <li key={item.name}><span>{item.name}</span><strong>{Number(item.ratio).toFixed(2)}%</strong></li>)}</ul>
            </div>
            <div>
              <h4>所属板块</h4>
              <ul>{displayIndustries.map((item) => <li key={item.name}><span>{item.name}</span><strong>{Number(item.ratio).toFixed(2)}%</strong></li>)}</ul>
            </div>
          </div>
        </Card>

        <Card title="板块行情" className="fund-panel">
          {displaySectorMarkets.length > 0 ? (
            <Table
              size="small"
              dataSource={displaySectorMarkets}
              rowKey={(row) => row.name}
              pagination={false}
              columns={[
                { title: '板块', dataIndex: 'name', key: 'name' },
                { title: '基金占比', dataIndex: 'ratio', key: 'ratio', width: 100, render: (v) => `${Number(v).toFixed(2)}%` },
                {
                  title: '板块涨跌',
                  key: 'change_percent',
                  width: 100,
                  render: (_, record) => {
                    const value = Number(record.market?.change_percent);
                    return Number.isFinite(value)
                      ? <span style={{ color: value >= 0 ? '#cf1322' : '#3f8600' }}>{value >= 0 ? '+' : ''}{value.toFixed(2)}%</span>
                      : '--';
                  },
                },
                { title: '领涨股', key: 'leading_stock_name', width: 140, render: (_, record) => record.market?.leading_stock_name || '--' },
                { title: '快照时间', key: 'snapshot_time', width: 140, render: (_, record) => record.market?.snapshot_time ? new Date(record.market.snapshot_time).toLocaleString('zh-CN', { hour12: false }).slice(5, 16) : '--' },
              ]}
            />
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无板块行情映射" />
          )}
        </Card>

        <Card
          title={`持仓详情 · ${holdingsDisplaySource} ${holdingsTargetCode || storedSnapshot?.target_code ? `(${holdingsTargetCode || storedSnapshot?.target_code})` : ''}`}
          className="fund-panel fund-panel-span-2"
          extra={(
            <Space wrap>
              <Button size="small" icon={<SyncOutlined />} loading={holdingsLoading} disabled={!holdingsEligible} onClick={() => loadHoldings(fund.fund_type)}>刷新实时</Button>
              <Button size="small" icon={<DatabaseOutlined />} loading={syncSnapshotLoading} onClick={syncStoredHoldings}>同步入库</Button>
            </Space>
          )}
        >
          {holdingsWithProfiles.length > 0 ? (
            <Table
              dataSource={holdingsWithProfiles}
              rowKey={(row) => `${row.stock_code}-${row.stock_name}`}
              pagination={{ pageSize: 10, showSizeChanger: false }}
              columns={[
                {
                  title: '代码',
                  dataIndex: 'stock_code',
                  key: 'stock_code',
                  width: 120,
                  render: (value, record) => record.holding_type === 'product' ? value : (
                    <Button
                      type="link"
                      size="small"
                      icon={<LineChartOutlined />}
                      onClick={() => navigate(`/stock/kline?symbol=${value}`)}
                    >
                      {value}
                    </Button>
                  ),
                },
                {
                  title: '名称',
                  dataIndex: 'stock_name',
                  key: 'stock_name',
                  render: (value, record) => record.holding_type === 'product' ? value : (
                    <Button type="link" size="small" onClick={() => navigate(`/stock/kline?symbol=${record.stock_code}`)}>
                      {value || record.stock_code}
                    </Button>
                  ),
                },
                { title: '类型', key: 'holding_type', width: 90, render: (_, record) => record.holding_type === 'product' ? '产品' : '股票' },
                {
                  title: '行业/板块',
                  key: 'stock_profile',
                  width: 180,
                  render: (_, record) => (
                    <Space wrap size={[4, 4]}>
                      {record.stock_board && <Tag>{record.stock_board}</Tag>}
                      {record.stock_industry && <Tag color="blue">{record.stock_industry}</Tag>}
                      {!record.stock_board && !record.stock_industry && '-'}
                    </Space>
                  ),
                },
                { title: '涨跌幅', dataIndex: 'change_percent', key: 'change_percent', width: 110, render: (v) => v != null ? `${Number(v) >= 0 ? '+' : ''}${Number(v).toFixed(2)}%` : '--' },
                { title: '净值占比', dataIndex: 'weight', key: 'weight', width: 110, render: (v) => `${Number(v).toFixed(2)}%` },
              ]}
            />
          ) : (
            <Alert type="info" showIcon title="当前基金暂无可展示的持仓详情" description="已尝试东方财富与腾讯基金两套来源，当前未返回可展示持仓。" />
          )}
        </Card>

        <Card title="基金公告" className="fund-panel">
          <Table
            dataSource={tencentNotices}
            rowKey="id"
            pagination={{ pageSize: 8, showSizeChanger: false }}
            columns={[
              { title: '公告标题', dataIndex: 'title', key: 'title' },
              { title: '日期', dataIndex: 'date', key: 'date', width: 120 },
            ]}
          />
        </Card>

        <Card
          title="社区动态"
          className="fund-panel"
          extra={(
            <Space>
              {community?.source_name && <Tag color="blue">{community.source_name}</Tag>}
              <Button size="small" icon={<SyncOutlined />} loading={communityLoading} onClick={loadCommunity}>刷新</Button>
            </Space>
          )}
        >
          {community?.error && (
            <Alert
              type="warning"
              showIcon
              title="社区源暂不可用"
              description={community.error}
              style={{ marginBottom: 12 }}
            />
          )}
          {communityLoading ? (
            <div className="stock-empty-wrap"><Spin /></div>
          ) : community?.items?.length ? (
            <Table
              size="small"
              dataSource={community.items}
              rowKey={(row) => row.url}
              pagination={{ pageSize: 8, showSizeChanger: false }}
              columns={[
                {
                  title: '帖子',
                  dataIndex: 'title',
                  key: 'title',
                  ellipsis: true,
                  render: (title, record) => (
                    <Space direction="vertical" size={2}>
                      <a href={record.url} target="_blank" rel="noreferrer">{title}</a>
                      <Space size={4} wrap>
                        <Tag>{record.source_name || community?.source_name || record.source}</Tag>
                        {record.author && <Text type="secondary">{record.author}</Text>}
                      </Space>
                    </Space>
                  ),
                },
                { title: '阅读', dataIndex: 'read_count', key: 'read_count', width: 70, render: (v) => v ?? '-' },
                { title: '回复', dataIndex: 'reply_count', key: 'reply_count', width: 70, render: (v) => v ?? '-' },
                { title: '时间', dataIndex: 'posted_at', key: 'posted_at', width: 90, render: (v) => v || '-' },
              ]}
            />
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无社区帖子" />
          )}
          {community?.community_url && (
            <Button
              type="link"
              icon={<CommentOutlined />}
              href={community.community_url}
              target="_blank"
              rel="noreferrer"
              style={{ paddingLeft: 0, marginTop: 8 }}
            >
              打开东方财富基金吧
            </Button>
          )}
        </Card>

        <Card title="同类基金" className="fund-panel">
          <Table
            dataSource={tencentSameTypeFunds}
            rowKey="jjdm"
            pagination={{ pageSize: 8, showSizeChanger: false }}
            columns={[
              { title: '基金代码', dataIndex: 'jjdm', key: 'jjdm', width: 110 },
              { title: '简称', dataIndex: 'jjjc', key: 'jjjc' },
              { title: '单位净值', dataIndex: 'dwjz', key: 'dwjz', width: 110 },
              { title: '增长率', dataIndex: 'jzzf', key: 'jzzf', width: 100, render: (v) => `${Number(v).toFixed(2)}%` },
            ]}
          />
        </Card>

        <Card title="同系基金" className="fund-panel">
          <Table
            dataSource={tencentSameSeriesFunds}
            rowKey="jjdm"
            pagination={{ pageSize: 8, showSizeChanger: false }}
            columns={[
              { title: '基金代码', dataIndex: 'jjdm', key: 'jjdm', width: 110 },
              { title: '简称', dataIndex: 'jjjc', key: 'jjjc' },
              { title: '单位净值', dataIndex: 'dwjz', key: 'dwjz', width: 110 },
              { title: '增长率', dataIndex: 'jzzf', key: 'jzzf', width: 100, render: (v) => `${Number(v).toFixed(2)}%` },
            ]}
          />
        </Card>

        <Card
          title="场内 K 线"
          className="fund-panel fund-panel-span-2"
          extra={(
            <Space wrap>
              <Select
                value={klineSource}
                onChange={(value) => {
                  setKlineSource(value);
                  loadMarketKline(value, klinePeriod);
                }}
                options={[
                  { label: '东方财富', value: 'eastmoney' },
                  { label: '腾讯', value: 'tencent' },
                  { label: '搜狐', value: 'sohu' },
                ]}
                style={{ width: 120 }}
              />
              <Select
                value={klinePeriod}
                onChange={setKlinePeriod}
                options={[
                  { label: '日线', value: 'day' },
                  { label: '周线', value: 'week' },
                  { label: '月线', value: 'month' },
                ]}
                style={{ width: 100 }}
              />
            </Space>
          )}
        >
          {klineLoading ? (
            <div className="stock-empty-wrap"><Spin /></div>
          ) : klineData.length > 0 ? (
            <>
              <Alert
                type="info"
                showIcon
                title={`当前来源：${klineSource === 'eastmoney' ? '东方财富' : klineSource === 'tencent' ? '腾讯' : '搜狐'}`}
                description="这里展示的是基金场内交易 K 线，可切换日 / 周 / 月线和数据源。"
                style={{ marginBottom: 16 }}
              />
              <ReactECharts option={klineChartOption} style={{ height: 420 }} />
            </>
          ) : marketQuote?.market_price && navHistory.length > 0 ? (
            <>
              <Alert
                type="warning"
                showIcon
                title="未取到标准 K 线，已回退到参考走势"
                description="当前源未返回场内 K 线，因此退回到价格参考线。"
                style={{ marginBottom: 16 }}
              />
              <ReactECharts option={marketPriceChartOption} style={{ height: 320 }} />
            </>
          ) : (
            <Empty description="当前基金暂无可展示的场内 K 线" />
          )}
        </Card>

        {positions.length > 0 && (
          <Card title="我的持仓" className="fund-panel fund-panel-span-2">
            <Table
              dataSource={positions}
              rowKey="id"
              pagination={false}
              columns={[
                { title: '账户', dataIndex: 'account_name', key: 'account_name' },
                { title: '持仓份额', dataIndex: 'holding_share', key: 'holding_share', render: (v) => parseFloat(v).toFixed(2) },
                { title: '持仓成本', dataIndex: 'holding_cost', key: 'holding_cost', render: (v) => `¥${parseFloat(v).toFixed(2)}` },
                { title: '市值', dataIndex: 'market_value', key: 'market_value', render: (v) => `¥${v}` },
                { title: '盈亏', dataIndex: 'profit', key: 'profit', render: (v, record) => <span style={{ color: parseFloat(v) >= 0 ? '#cf1322' : '#3f8600' }}>{parseFloat(v) >= 0 ? '+' : ''}¥{v} ({record.profit_rate}%)</span> },
              ]}
            />
          </Card>
        )}

        <AIAnalysisModal
          open={aiModalVisible}
          onClose={() => setAiModalVisible(false)}
          contextType="fund"
          contextData={buildAiContextData()}
          title={`AI 分析 · ${fund?.fund_name || ''}`}
        />
      </div>
    </div>
  );
};

export default FundDetailPage;

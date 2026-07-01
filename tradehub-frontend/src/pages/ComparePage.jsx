import { useState, useEffect } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { Alert, Card, Select, Button, Spin, Empty, Table, message, Space, Skeleton, Typography } from 'antd';
import { RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, Radar, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { fundsAPI, watchlistsAPI } from '../api';

const COLORS = ['#1890ff', '#cf1322', '#faad14', '#52c41a', '#722ed1'];
const METRIC_NAMES = { '1m': '近1月', '3m': '近3月', '6m': '近6月', '1y': '近1年', max_drawdown: '最大回撤(%)', volatility: '波动率(%)', sharpe: '夏普比率' };
const RECENT_PERIODS = [7, 14, 30];
const HISTORY_KEY = 'tradehub.fund.compare.history.v1';
const SELECTION_KEY = 'tradehub.fund.compare.selection.v1';
const { Text } = Typography;

const toNumber = (value) => {
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
};

const formatPct = (value) => {
  const number = toNumber(value);
  if (number === null) return '-';
  return `${number >= 0 ? '+' : ''}${number.toFixed(2)}%`;
};

const normalizeCodes = (codes) => [...new Set((codes || []).map((code) => String(code).trim()).filter(Boolean))].slice(0, 5);

const loadHistoryGroups = () => {
  try {
    return JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]') || [];
  } catch {
    return [];
  }
};

const saveHistoryGroup = (codes, funds) => {
  const normalized = normalizeCodes(codes);
  if (normalized.length < 2) return loadHistoryGroups();
  const fundMap = new Map((funds || []).map((fund) => [fund.fund_code, fund]));
  const nextGroup = {
    id: normalized.join(','),
    codes: normalized,
    funds: normalized.map((code) => ({
      fund_code: code,
      fund_name: fundMap.get(code)?.fund_name || code,
    })),
    compared_at: new Date().toISOString(),
  };
  const oldGroups = loadHistoryGroups().filter((item) => item.id !== nextGroup.id);
  const nextGroups = [nextGroup, ...oldGroups].slice(0, 10);
  localStorage.setItem(HISTORY_KEY, JSON.stringify(nextGroups));
  localStorage.setItem(SELECTION_KEY, JSON.stringify(nextGroup.funds));
  return nextGroups;
};

const calculateRecentMetrics = (records, days) => {
  const startDate = dayjs().subtract(days - 1, 'day').format('YYYY-MM-DD');
  const ascending = (records || [])
    .filter((item) => item.nav_date >= startDate)
    .map((item) => ({
      ...item,
      nav: toNumber(item.unit_nav),
      dailyGrowth: toNumber(item.daily_growth),
    }))
    .filter((item) => item.nav !== null)
    .sort((a, b) => a.nav_date.localeCompare(b.nav_date));

  const daily = ascending.map((item, index) => {
    const previous = index > 0 ? ascending[index - 1].nav : null;
    const calculated = previous ? ((item.nav / previous) - 1) * 100 : null;
    return {
      date: item.nav_date,
      growth: item.dailyGrowth ?? calculated,
      nav: item.nav,
    };
  });
  const usableDaily = daily.filter((item) => item.growth !== null);
  const first = ascending[0];
  const last = ascending[ascending.length - 1];
  const totalReturn = first && last ? ((last.nav / first.nav) - 1) * 100 : null;

  let peak = -Infinity;
  let maxDrawdown = null;
  ascending.forEach((item) => {
    peak = Math.max(peak, item.nav);
    if (peak > 0) {
      const drawdown = ((item.nav / peak) - 1) * 100;
      maxDrawdown = maxDrawdown === null ? drawdown : Math.min(maxDrawdown, drawdown);
    }
  });

  const average = usableDaily.length ? usableDaily.reduce((sum, item) => sum + item.growth, 0) / usableDaily.length : null;
  const volatility = average === null || usableDaily.length < 2
    ? null
    : Math.sqrt(usableDaily.reduce((sum, item) => sum + ((item.growth - average) ** 2), 0) / (usableDaily.length - 1));
  const maxGain = usableDaily.reduce((best, item) => !best || item.growth > best.growth ? item : best, null);
  const maxLoss = usableDaily.reduce((worst, item) => !worst || item.growth < worst.growth ? item : worst, null);

  return {
    days,
    startDate,
    endDate: last?.date || '',
    navCount: ascending.length,
    totalReturn,
    maxDrawdown,
    volatility,
    maxGain,
    maxLoss,
    daily: usableDaily,
  };
};

const ComparePage = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const [funds, setFunds] = useState([]);
  const [loading, setLoading] = useState(false);
  const [selectedCodes, setSelectedCodes] = useState([]);
  const [fundOptions, setFundOptions] = useState([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [recentMetrics, setRecentMetrics] = useState({});
  const [recentLoading, setRecentLoading] = useState(false);
  const [historyGroups, setHistoryGroups] = useState(loadHistoryGroups);

  const codesFromUrl = searchParams.get('codes')?.split(',')?.filter(Boolean) || [];

  useEffect(() => {
    if (codesFromUrl.length >= 2) {
      setSelectedCodes(codesFromUrl);
      loadCompare(codesFromUrl);
    }
  }, []);

  const loadCompare = async (codes) => {
    const normalizedCodes = normalizeCodes(codes);
    if (normalizedCodes.length < 2) return;
    setLoading(true);
    try {
      const { data } = await fundsAPI.compare(normalizedCodes);
      setFunds(data.funds);
      setSearchParams({ codes: normalizedCodes.join(',') });
      setHistoryGroups(saveHistoryGroup(normalizedCodes, data.funds));
      loadRecentMetrics(normalizedCodes);
    } catch (err) {
      message.error(err.response?.data?.error || '对比失败');
    } finally {
      setLoading(false);
    }
  };

  const loadRecentMetrics = async (codes) => {
    setRecentLoading(true);
    try {
      const startDate = dayjs().subtract(45, 'day').format('YYYY-MM-DD');
      const entries = await Promise.all(codes.map(async (code) => {
        const { data } = await fundsAPI.navHistory(code, { start_date: startDate });
        const records = Array.isArray(data) ? data : (data.results || []);
        return [code, Object.fromEntries(RECENT_PERIODS.map((days) => [days, calculateRecentMetrics(records, days)]))];
      }));
      setRecentMetrics(Object.fromEntries(entries));
    } catch (err) {
      setRecentMetrics({});
      message.warning(err.response?.data?.error || '短周期净值明细加载失败');
    } finally {
      setRecentLoading(false);
    }
  };

  const handleSearch = async (keyword) => {
    if (!keyword || keyword.length < 2) { setFundOptions([]); return; }
    setSearchLoading(true);
    try {
      const { data } = await fundsAPI.search(keyword);
      const results = data.results || [];
      setFundOptions(results.slice(0, 20).map(f => ({ value: f.fund_code, label: `${f.fund_code} - ${f.fund_name}` })));
    } catch { setFundOptions([]); }
    finally { setSearchLoading(false); }
  };

  const handleSelect = (code) => {
    if (selectedCodes.includes(code)) return;
    if (selectedCodes.length >= 5) { message.warning('最多对比 5 只基金'); return; }
    const newCodes = [...selectedCodes, code];
    setSelectedCodes(newCodes);
    loadCompare(newCodes);
  };

  const handleRemove = (code) => {
    const newCodes = selectedCodes.filter(c => c !== code);
    setSelectedCodes(newCodes);
    if (newCodes.length >= 2) loadCompare(newCodes);
    else { setFunds([]); setRecentMetrics({}); setSearchParams({}); }
  };

  const handleImportFromWatchlist = async () => {
    try {
      const { data } = await watchlistsAPI.list();
      if (!data.length) { message.info('暂无自选列表'); return; }
      const allCodes = [...new Set(data.flatMap(w => (w.items || []).map(i => i.fund_code)))];
      if (!allCodes.length) { message.info('自选列表为空'); return; }
      const codes = allCodes.slice(0, 5);
      setSelectedCodes(codes);
      loadCompare(codes);
    } catch { message.error('加载自选列表失败'); }
  };

  const buildRadarData = () => {
    const keys = ['1m', '3m', '6m', '1y'];
    return keys.map(k => {
      const entry = { metric: METRIC_NAMES[k] };
      funds.forEach(f => { entry[f.fund_code] = f.returns?.[k] != null ? parseFloat(f.returns[k]) : 0; });
      return entry;
    });
  };

  const buildTableData = () => {
    const rows = [];
    const metricKeys = ['latest_nav', '1m', '3m', '6m', '1y', 'max_drawdown', 'volatility', 'sharpe'];
    metricKeys.forEach(k => {
      const row = { metric: METRIC_NAMES[k] || k };
      let bestVal = Infinity;
      if (k === 'max_drawdown' || k === 'volatility') bestVal = Infinity;
      else bestVal = -Infinity;

      funds.forEach(f => {
        let val = null;
        if (k === 'latest_nav') val = f.latest_nav ? parseFloat(f.latest_nav) : null;
        else if (['1m','3m','6m','1y'].includes(k)) val = f.returns?.[k] != null ? parseFloat(f.returns[k]) : null;
        else val = f.metrics?.[k] != null ? parseFloat(f.metrics[k]) : null;
        row[f.fund_code] = val;
        if (val != null) {
          if (['max_drawdown','volatility'].includes(k)) { if (val < bestVal) bestVal = val; }
          else { if (val > bestVal) bestVal = val; }
        }
      });
      row._best = bestVal;
      rows.push(row);
    });
    return rows;
  };

  const tableData = buildTableData();
  const tableColumns = [
    { title: '指标', dataIndex: 'metric', key: 'metric', width: 120, fixed: 'left' },
    ...funds.map((f, i) => ({
      title: `${f.fund_name} (${f.fund_code})`,
      dataIndex: f.fund_code,
      key: f.fund_code,
      render: (v, record) => {
        if (v == null) return '-';
        const isBest = record._best != null && v === record._best;
        const isReturn = ['近1月','近3月','近6月','近1年'].includes(record.metric);
        const color = isReturn ? (v >= 0 ? '#cf1322' : '#3f8600') : undefined;
        return <span style={{ color, fontWeight: isBest ? 'bold' : 'normal', background: isBest ? '#f6ffed' : undefined, padding: isBest ? '0 4px' : undefined }}>{typeof v === 'number' ? (isReturn ? `${v >= 0 ? '+' : ''}${v.toFixed(2)}%` : v.toFixed(2)) : v}</span>;
      },
    })),
  ];

  const recentMetricRows = RECENT_PERIODS.map((days) => {
    const row = { metric: `近${days}日` };
    funds.forEach((fund) => {
      row[fund.fund_code] = recentMetrics[fund.fund_code]?.[days] || null;
    });
    return row;
  });

  const recentMetricColumns = [
    { title: '周期', dataIndex: 'metric', key: 'metric', width: 100, fixed: 'left' },
    ...funds.map((fund) => ({
      title: `${fund.fund_name} (${fund.fund_code})`,
      dataIndex: fund.fund_code,
      key: fund.fund_code,
      render: (metric) => metric ? (
        <Space direction="vertical" size={2}>
          <Text>涨幅：<Text style={{ color: toNumber(metric.totalReturn) >= 0 ? '#cf1322' : '#3f8600' }}>{formatPct(metric.totalReturn)}</Text></Text>
          <Text>回撤：{formatPct(metric.maxDrawdown)}</Text>
          <Text>波动率：{formatPct(metric.volatility)}</Text>
          <Text>最大涨幅：{formatPct(metric.maxGain?.growth)} {metric.maxGain?.date || ''}</Text>
          <Text>最大跌幅：{formatPct(metric.maxLoss?.growth)} {metric.maxLoss?.date || ''}</Text>
          <Text type="secondary">净值点：{metric.navCount}</Text>
        </Space>
      ) : '-',
    })),
  ];

  const dailyRows = [];
  RECENT_PERIODS.forEach((days) => {
    const dates = new Set();
    funds.forEach((fund) => {
      (recentMetrics[fund.fund_code]?.[days]?.daily || []).forEach((item) => dates.add(item.date));
    });
    [...dates].sort((a, b) => b.localeCompare(a)).forEach((date) => {
      const row = { key: `${days}-${date}`, period: `近${days}日`, date };
      funds.forEach((fund) => {
        row[fund.fund_code] = (recentMetrics[fund.fund_code]?.[days]?.daily || []).find((item) => item.date === date)?.growth;
      });
      dailyRows.push(row);
    });
  });

  const dailyColumns = [
    { title: '周期', dataIndex: 'period', key: 'period', width: 90, fixed: 'left' },
    { title: '日期', dataIndex: 'date', key: 'date', width: 110, fixed: 'left' },
    ...funds.map((fund) => ({
      title: `${fund.fund_name} (${fund.fund_code})`,
      dataIndex: fund.fund_code,
      key: fund.fund_code,
      render: (value) => <span style={{ color: toNumber(value) >= 0 ? '#cf1322' : '#3f8600' }}>{formatPct(value)}</span>,
    })),
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Card title="基金 PK 对比" extra={
        <Space>
          <Button onClick={handleImportFromWatchlist}>从自选导入</Button>
          <Button onClick={() => navigate('/dashboard/rankings')}>去排行选择</Button>
        </Space>
      }>
        {historyGroups.length > 0 && (
          <Card size="small" title="历史对比组合（本机缓存）" style={{ marginBottom: 12 }}>
            <Space wrap>
              {historyGroups.map((group) => (
                <Button
                  key={group.id}
                  size="small"
                  onClick={() => {
                    setSelectedCodes(group.codes);
                    loadCompare(group.codes);
                  }}
                >
                  {group.funds.map((item) => item.fund_name || item.fund_code).join(' / ')}
                </Button>
              ))}
              <Button
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => {
                  localStorage.removeItem(HISTORY_KEY);
                  setHistoryGroups([]);
                }}
              >
                清空历史
              </Button>
            </Space>
          </Card>
        )}
        <Space style={{ marginBottom: 16 }} wrap>
          {selectedCodes.map((code, i) => (
            <Button key={code} size="small" type="primary"
              style={{ background: COLORS[i % COLORS.length], borderColor: COLORS[i % COLORS.length] }}
              onClick={() => handleRemove(code)}
            >
              {funds.find(f => f.fund_code === code)?.fund_name || code} ✕
            </Button>
          ))}
          <Select
            showSearch
            value={undefined}
            placeholder="搜索基金代码或名称添加对比"
            filterOption={false}
            onSearch={handleSearch}
            onSelect={handleSelect}
            options={fundOptions}
            loading={searchLoading}
            style={{ minWidth: 250 }}
            suffixIcon={<PlusOutlined />}
          />
        </Space>
        <Alert
          showIcon
          type="info"
          style={{ marginBottom: 16 }}
          message="默认展示近 7/14/30 日短周期对比"
          description="短周期指标基于基金历史净值计算：区间涨幅、最大回撤、日涨跌波动率、最大单日涨幅/跌幅及发生日期，并在下方列出每日具体涨跌幅。"
        />

        {loading ? <><Skeleton active paragraph={{ rows: 4 }} /><Skeleton active paragraph={{ rows: 4 }} /></> :
         funds.length >= 2 ? (
          <>
            <Card title="收益雷达图" size="small" style={{ marginBottom: 16 }}>
              <ResponsiveContainer width="100%" height={400}>
                <RadarChart data={buildRadarData()}>
                  <PolarGrid />
                  <PolarAngleAxis dataKey="metric" />
                  <PolarRadiusAxis />
                  <Tooltip formatter={(v) => `${v}%`} />
                  <Legend />
                  {funds.map((f, i) => (
                    <Radar key={f.fund_code} name={f.fund_name} dataKey={f.fund_code} stroke={COLORS[i % COLORS.length]} fill={COLORS[i % COLORS.length]} fillOpacity={0.1} />
                  ))}
                </RadarChart>
              </ResponsiveContainer>
            </Card>

            <Card title="指标对比" size="small">
              <Table
                dataSource={tableData}
                columns={tableColumns}
                rowKey="metric"
                pagination={false}
                size="small"
                scroll={{ x: 'max-content' }}
                locale={{ emptyText: '-' }}
              />
            </Card>

            <Card title="近 7/14/30 日涨跌与风险" size="small" style={{ marginTop: 16 }}>
              <Spin spinning={recentLoading}>
                <Table
                  dataSource={recentMetricRows}
                  columns={recentMetricColumns}
                  rowKey="metric"
                  pagination={false}
                  size="small"
                  scroll={{ x: 'max-content' }}
                  locale={{ emptyText: '-' }}
                />
              </Spin>
            </Card>

            <Card title="每日具体涨跌幅" size="small" style={{ marginTop: 16 }}>
              <Spin spinning={recentLoading}>
                <Table
                  dataSource={dailyRows}
                  columns={dailyColumns}
                  rowKey="key"
                  pagination={{ pageSize: 30, showSizeChanger: true }}
                  size="small"
                  scroll={{ x: 'max-content' }}
                  locale={{ emptyText: '-' }}
                />
              </Spin>
            </Card>
          </>
        ) : (
          !loading && <Empty description="请搜索并选择 2-5 只基金开始对比，或点击「从自选导入」" />
        )}
      </Card>
    </Space>
  );
};

export default ComparePage;

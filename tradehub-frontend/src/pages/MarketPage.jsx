import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  AutoComplete,
  Button,
  Card,
  Col,
  Empty,
  Grid,
  Input,
  List,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  AimOutlined,
  FireOutlined,
  FundOutlined,
  LineChartOutlined,
  ReloadOutlined,
  SearchOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import { fundsAPI } from '../api';

const { Text, Title } = Typography;
const { useBreakpoint } = Grid;

const CATEGORIES = [
  { value: '', label: '全部' },
  { value: '股票', label: '股票' },
  { value: '混合', label: '混合' },
  { value: '债券', label: '债券' },
  { value: '指数', label: '指数' },
  { value: 'QDII', label: 'QDII' },
  { value: '黄金', label: '黄金' },
  { value: '半导体', label: '半导体' },
];

const RANK_TYPES = [
  { key: 'gain', label: '涨幅榜', icon: <TrophyOutlined /> },
  { key: 'popular', label: '人气榜', icon: <FireOutlined /> },
  { key: 'accuracy', label: '准度榜', icon: <AimOutlined /> },
];

const PERIODS = [
  { value: 'day', label: '日' },
  { value: 'week', label: '周' },
  { value: 'month', label: '月' },
  { value: 'quarter', label: '季' },
  { value: 'half_year', label: '半年' },
  { value: 'this_year', label: '今年' },
  { value: 'year', label: '1年' },
  { value: 'three_year', label: '3年' },
  { value: 'since_inception', label: '成立以来' },
];

const changeColor = (value) => {
  const n = Number(value || 0);
  if (!Number.isFinite(n) || n === 0) return '#62748a';
  return n > 0 ? '#ee4444' : '#00a54b';
};

const changeBg = (value) => {
  const n = Number(value || 0);
  if (!Number.isFinite(n) || n === 0) return 'rgba(98, 116, 138, 0.1)';
  return n > 0 ? 'rgba(238, 68, 68, 0.1)' : 'rgba(0, 165, 75, 0.1)';
};

const formatPct = (value) => {
  if (value === null || value === undefined || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`;
};

const formatNumber = (value, digits = 2) => {
  if (value === null || value === undefined || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return n.toFixed(digits);
};

const MarketPage = () => {
  const navigate = useNavigate();
  const screens = useBreakpoint();
  const isMobile = !screens.md;

  const [indices, setIndices] = useState([]);
  const [selectedIndex, setSelectedIndex] = useState('');
  const [rankType, setRankType] = useState('gain');
  const [period, setPeriod] = useState('day');
  const [category, setCategory] = useState('');
  const [rankings, setRankings] = useState([]);
  const [fallbackFunds, setFallbackFunds] = useState([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [indicesLoading, setIndicesLoading] = useState(false);
  const [refreshingQuotes, setRefreshingQuotes] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [fundOptions, setFundOptions] = useState([]);
  const [searchLoading, setSearchLoading] = useState(false);

  const loadIndices = async () => {
    setIndicesLoading(true);
    try {
      const { data } = await fundsAPI.marketIndices();
      const nextIndices = data.indices || [];
      setIndices(nextIndices);
      setSelectedIndex((current) => current || nextIndices[0]?.code || nextIndices[0]?.name || '');
    } catch {
      setIndices([]);
    } finally {
      setIndicesLoading(false);
    }
  };

  const loadRankings = async (nextType = rankType, nextCategory = category, nextPage = 1, nextPeriod = period) => {
    setLoading(true);
    try {
      if (nextType === 'gain') {
        const { data } = await fundsAPI.performanceRanks({
          period: nextPeriod,
          category: nextCategory,
          page_size: 20,
        });
        const rows = data.results || data || [];
        setRankings(rows);
        setTotal(rows.length);
      } else {
        const { data } = await fundsAPI.rankings({ type: nextType, category: nextCategory, page: nextPage });
        setRankings(data.results || []);
        setTotal(data.count || 0);
      }
    } catch {
      setRankings([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const loadFallbackFunds = async () => {
    try {
      const { data } = await fundsAPI.list({ page: 1, page_size: 20 });
      setFallbackFunds(data.results || []);
    } catch {
      setFallbackFunds([]);
    }
  };

  useEffect(() => {
    loadIndices();
    loadRankings();
    loadFallbackFunds();
    const timer = setInterval(loadIndices, 30000);
    return () => clearInterval(timer);
  }, []);

  const refreshAll = async () => {
    await Promise.all([loadIndices(), loadRankings(rankType, category, page, period), loadFallbackFunds()]);
  };

  const handleRankTypeChange = (nextType) => {
    setRankType(nextType);
    setPage(1);
    loadRankings(nextType, category, 1, period);
  };

  const handleCategoryChange = (nextCategory) => {
    const value = nextCategory || '';
    setCategory(value);
    setPage(1);
    loadRankings(rankType, value, 1, period);
  };

  const handlePeriodChange = (nextPeriod) => {
    setPeriod(nextPeriod);
    setPage(1);
    loadRankings(rankType, category, 1, nextPeriod);
  };

  const handleSearch = async (keyword) => {
    setSearchKeyword(keyword);
    if (keyword.trim().length < 2) {
      setFundOptions([]);
      return;
    }
    setSearchLoading(true);
    try {
      const { data } = await fundsAPI.search(keyword);
      setFundOptions((data.results || []).slice(0, 20).map((fund) => ({
        value: fund.fund_code,
        label: `${fund.fund_code} - ${fund.fund_name}`,
      })));
    } catch {
      setFundOptions([]);
    } finally {
      setSearchLoading(false);
    }
  };

  const handleRefreshQuotes = async () => {
    const candidateCodes = fallbackFunds.map((item) => item.fund_code).filter(Boolean);
    if (candidateCodes.length === 0) {
      message.warning('当前没有可刷新的基金列表');
      return;
    }
    setRefreshingQuotes(true);
    try {
      await fundsAPI.batchUpdateTodayNav(candidateCodes);
      await refreshAll();
      message.success('已刷新基金行情');
    } catch (error) {
      message.error(error.response?.data?.error || '刷新基金行情失败');
    } finally {
      setRefreshingQuotes(false);
    }
  };

  const selectedIndexItem = useMemo(() => {
    return indices.find((item) => item.code === selectedIndex || item.name === selectedIndex) || indices[0];
  }, [indices, selectedIndex]);

  const displayData = rankings.length > 0 ? rankings : fallbackFunds;
  const topFunds = displayData.slice(0, 3);
  const marketStats = {
    up: indices.filter((item) => Number(item.change_percent || 0) > 0).length,
    down: indices.filter((item) => Number(item.change_percent || 0) < 0).length,
    flat: indices.filter((item) => Number(item.change_percent || 0) === 0).length,
    funds: total || displayData.length,
  };

  const columns = [
    {
      title: '#',
      key: 'rank',
      width: 56,
      render: (_, __, index) => (page - 1) * 20 + index + 1,
    },
    {
      title: '基金',
      dataIndex: 'fund_name',
      key: 'fund',
      ellipsis: true,
      render: (_, record) => (
        <Button type="link" onClick={() => navigate(`/dashboard/funds/${record.fund_code}`)}>
          {record.fund_name || record.fund_code}
        </Button>
      ),
    },
    {
      title: '代码',
      dataIndex: 'fund_code',
      key: 'fund_code',
      width: 110,
    },
    {
      title: '类型',
      dataIndex: 'fund_type',
      key: 'fund_type',
      width: 100,
      responsive: ['md'],
      render: (value) => value ? <Tag>{value}</Tag> : '-',
    },
    {
      title: rankType === 'gain' ? '涨跌' : rankType === 'popular' ? '关注' : '误差',
      key: 'metric',
      width: 120,
      render: (_, record) => {
        if (rankType === 'popular') return `${record.pos_count || 0} 人`;
        if (rankType === 'accuracy') return record.avg_error ? `${(Number(record.avg_error) * 100).toFixed(2)}%` : '-';
        return (
          <Tag style={{ border: 'none', background: changeBg(record.growth), color: changeColor(record.growth) }}>
            {record.growth != null ? formatPct(record.growth) : '-'}
          </Tag>
        );
      },
    },
  ];

  return (
    <div className="stock-page stock-layout">
      <div className="stock-page-header">
        <div>
          <Title level={4} style={{ margin: 0, color: '#122033' }}>基金市场</Title>
          <Text type="secondary">行情、排行和基金搜索</Text>
        </div>
        <Space wrap>
          <AutoComplete
            value={searchKeyword}
            options={fundOptions}
            onSearch={handleSearch}
            onSelect={(code) => navigate(`/dashboard/funds/${code}`)}
            style={{ width: isMobile ? 180 : 260 }}
            notFoundContent={searchLoading ? <Spin size="small" /> : null}
          >
            <Input prefix={<SearchOutlined />} placeholder="基金代码或名称" allowClear />
          </AutoComplete>
          <Button icon={<ReloadOutlined />} loading={refreshingQuotes || indicesLoading} onClick={handleRefreshQuotes}>
            刷新行情
          </Button>
          <Button onClick={refreshAll}>刷新页面</Button>
        </Space>
      </div>

      <div className="stock-content">
        <Spin spinning={indicesLoading}>
          <Row gutter={[16, 16]}>
            {indices.length > 0 ? indices.map((indexItem) => {
              const key = indexItem.code || indexItem.name;
              const change = Number(indexItem.change_percent || 0);
              const active = key === selectedIndex;
              return (
                <Col xs={24} md={12} xl={8} key={key}>
                  <Card className="stock-card" hoverable bodyStyle={{ padding: 16 }} onClick={() => setSelectedIndex(key)}>
                    <Space direction="vertical" size={6} style={{ width: '100%' }}>
                      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                        <Text strong>{indexItem.name || key}</Text>
                        <Tag color={active ? 'blue' : 'default'}>{indexItem.code || '指数'}</Tag>
                      </Space>
                      <Text style={{ fontSize: 26, fontWeight: 700, color: '#122033' }}>
                        {indexItem.price || '-'}
                      </Text>
                      <Tag style={{ width: 'fit-content', border: 'none', background: changeBg(change), color: changeColor(change) }}>
                        {formatPct(change)}
                      </Tag>
                    </Space>
                  </Card>
                </Col>
              );
            }) : (
              <Col span={24}>
                <Card className="stock-card"><Empty description="暂无市场指数数据" /></Card>
              </Col>
            )}
          </Row>
        </Spin>

        <Row gutter={[16, 16]}>
          <Col xs={24} xl={16}>
            <Card
              className="stock-card"
              title={<Space><LineChartOutlined /><span>{selectedIndexItem?.name || '基金排行'}</span></Space>}
              extra={
                <Space wrap>
                  <Select
                    value={rankType}
                    onChange={handleRankTypeChange}
                    options={RANK_TYPES.map((item) => ({ value: item.key, label: item.label }))}
                    style={{ width: 110 }}
                  />
                  {rankType === 'gain' && (
                    <Select
                      value={period}
                      onChange={handlePeriodChange}
                      options={PERIODS}
                      style={{ width: 110 }}
                    />
                  )}
                  <Select
                    value={category || undefined}
                    placeholder="分类"
                    allowClear
                    onChange={handleCategoryChange}
                    options={CATEGORIES.map((item) => ({ value: item.value, label: item.label }))}
                    style={{ width: 110 }}
                  />
                </Space>
              }
            >
              <Spin spinning={loading}>
                {displayData.length > 0 ? (
                  isMobile ? (
                    <List
                      dataSource={displayData}
                      pagination={{
                        current: page,
                        total: total || displayData.length,
                        pageSize: 20,
                        showSizeChanger: false,
                        onChange: (nextPage) => {
                          setPage(nextPage);
                          loadRankings(rankType, category, nextPage, period);
                        },
                      }}
                      renderItem={(item, index) => (
                        <Card size="small" style={{ marginBottom: 8 }} onClick={() => navigate(`/dashboard/funds/${item.fund_code}`)}>
                          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                            <Space direction="vertical" size={2}>
                              <Text strong>{(page - 1) * 20 + index + 1}. {item.fund_name}</Text>
                              <Text type="secondary">{item.fund_code} · {item.fund_type || '-'}</Text>
                            </Space>
                            {rankType === 'gain' ? (
                              <Tag style={{ border: 'none', background: changeBg(item.growth), color: changeColor(item.growth) }}>
                                {item.growth != null ? formatPct(item.growth) : '-'}
                              </Tag>
                            ) : (
                              <Text>{rankType === 'popular' ? `${item.pos_count || 0} 人` : item.avg_error ? `${(Number(item.avg_error) * 100).toFixed(2)}%` : '-'}</Text>
                            )}
                          </Space>
                        </Card>
                      )}
                    />
                  ) : (
                    <Table
                      rowKey="fund_code"
                      dataSource={displayData}
                      columns={columns}
                      size="small"
                      scroll={{ x: 'max-content' }}
                      pagination={{
                        current: page,
                        total: total || displayData.length,
                        pageSize: 20,
                        showSizeChanger: false,
                        onChange: (nextPage) => {
                          setPage(nextPage);
                          loadRankings(rankType, category, nextPage, period);
                        },
                      }}
                    />
                  )
                ) : (
                  <Empty description="暂无排行数据" />
                )}
              </Spin>
            </Card>
          </Col>

          <Col xs={24} xl={8}>
            <Card className="stock-card" title={<Space><FundOutlined /><span>市场概览</span></Space>} bodyStyle={{ padding: 16 }}>
              <Row gutter={[12, 12]}>
                <Col span={12}><Statistic title="上涨指数" value={marketStats.up} valueStyle={{ color: '#ee4444' }} /></Col>
                <Col span={12}><Statistic title="下跌指数" value={marketStats.down} valueStyle={{ color: '#00a54b' }} /></Col>
                <Col span={12}><Statistic title="平盘指数" value={marketStats.flat} valueStyle={{ color: '#62748a' }} /></Col>
                <Col span={12}><Statistic title="样本基金" value={marketStats.funds} valueStyle={{ color: '#122033' }} /></Col>
              </Row>
              <div style={{ marginTop: 16 }}>
                <Text type="secondary">当前榜单</Text>
                <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {topFunds.length > 0 ? topFunds.map((fund) => (
                    <Button key={fund.fund_code} block onClick={() => navigate(`/dashboard/funds/${fund.fund_code}`)}>
                      {fund.fund_name || fund.fund_code}
                    </Button>
                  )) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无基金" />}
                </div>
              </div>
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default MarketPage;

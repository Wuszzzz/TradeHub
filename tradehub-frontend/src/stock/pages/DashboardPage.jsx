import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Empty,
  Input,
  Row,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  BellOutlined,
  DollarOutlined,
  FundOutlined,
  LineChartOutlined,
  ReloadOutlined,
  SearchOutlined,
  SwapOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { alertApi, marketApi, paperApi, watchlistApi } from '../api/stockApi';
import { formatAmount, formatChangePercent, formatPrice, getChangeBgColor, getChangeColor } from '../utils';

const { Text, Title } = Typography;
const { Search } = Input;

const INDEX_SYMBOLS = [
  { symbol: '000001', name: '上证指数' },
  { symbol: '399001', name: '深证成指' },
  { symbol: '399006', name: '创业板指' },
  { symbol: '000300', name: '沪深300' },
];

const unwrap = (res) => res?.data || res || {};

const DashboardPage = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [account, setAccount] = useState(null);
  const [positions, setPositions] = useState([]);
  const [watchlist, setWatchlist] = useState([]);
  const [alerts, setAlerts] = useState([]);
  const [indices, setIndices] = useState([]);
  const [searchResult, setSearchResult] = useState(null);

  const loadData = async () => {
    setLoading(true);
    try {
      const [accountRes, positionRes, watchlistRes, alertRes] = await Promise.allSettled([
        paperApi.getAccount(),
        paperApi.getPositions(),
        watchlistApi.getSnapshot('default'),
        alertApi.getEvents('open', 6),
      ]);

      if (accountRes.status === 'fulfilled') setAccount(unwrap(accountRes.value).item || null);
      if (positionRes.status === 'fulfilled') setPositions((unwrap(positionRes.value).items || []).slice(0, 6));
      if (watchlistRes.status === 'fulfilled') setWatchlist((unwrap(watchlistRes.value).items || []).slice(0, 8));
      if (alertRes.status === 'fulfilled') setAlerts((unwrap(alertRes.value).items || []).slice(0, 6));

      const indexResults = await Promise.all(
        INDEX_SYMBOLS.map(async (item) => {
          try {
            const res = await marketApi.snapshot(item.symbol);
            return { ...item, ...(unwrap(res).item || unwrap(res)) };
          } catch {
            return { ...item };
          }
        })
      );
      setIndices(indexResults);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleSearch = async (keyword) => {
    if (!keyword?.trim()) return;
    try {
      const res = await marketApi.snapshot(keyword.trim());
      setSearchResult(unwrap(res).item || unwrap(res));
    } catch (err) {
      setSearchResult(null);
      message.error(err.message || '标的查询失败');
    }
  };

  const equity = Number(account?.equity || 0);
  const cash = Number(account?.cash || 0);
  const initial = Number(account?.initial || 0);
  const realized = Number(account?.realized_pl || 0);
  const marketValue = Math.max(equity - cash, 0);
  const totalPL = initial > 0 ? equity - initial + realized : realized;
  const totalPLRate = initial > 0 ? ((equity - initial) / initial) * 100 : 0;

  const watchlistColumns = [
    {
      title: '标的',
      key: 'symbol',
      render: (_, record) => (
        <Button type="link" onClick={() => navigate(`/stock/kline?symbol=${record.symbol}`)}>
          {record.name || record.symbol}
        </Button>
      ),
    },
    {
      title: '最新价',
      key: 'price',
      align: 'right',
      render: (_, record) => formatPrice(record.quote?.price),
    },
    {
      title: '涨跌幅',
      key: 'change',
      align: 'right',
      render: (_, record) => {
        const value = record.quote?.change_percent || 0;
        return (
          <Tag style={{ border: 'none', background: getChangeBgColor(value), color: getChangeColor(value) }}>
            {formatChangePercent(value)}
          </Tag>
        );
      },
    },
  ];

  if (loading) {
    return (
      <div className="stock-page stock-layout">
        <div className="stock-loading"><Spin size="large" /></div>
      </div>
    );
  }

  return (
    <div className="stock-page stock-layout">
      <div className="stock-page-header">
        <div>
          <Title level={4} style={{ margin: 0, color: '#122033' }}>股票工作台</Title>
          <Text type="secondary">真实账户、自选、指数和告警聚合视图</Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={loadData}>刷新</Button>
          <Button type="primary" icon={<SwapOutlined />} onClick={() => navigate('/stock/paper')}>模拟交易</Button>
        </Space>
      </div>

      <div className="stock-content">
        <Card className="stock-card">
          <Space size="large" style={{ width: '100%', justifyContent: 'space-between' }}>
            <Search
              placeholder="输入股票或ETF代码"
              allowClear
              onSearch={handleSearch}
              style={{ maxWidth: 420 }}
              size="large"
            />
            <Space wrap>
              <Button icon={<FundOutlined />} onClick={() => navigate('/stock/watchlist')}>自选股</Button>
              <Button icon={<LineChartOutlined />} onClick={() => navigate('/stock/market')}>市场行情</Button>
              <Button icon={<SearchOutlined />} onClick={() => navigate('/stock/screener')}>选股</Button>
            </Space>
          </Space>
          {searchResult ? (
            <div style={{ marginTop: 16, padding: 16, border: '1px solid #d8e2ee', borderRadius: 12, background: '#f7f9fc' }}>
              <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                <Space direction="vertical" size={2}>
                  <Text strong style={{ fontSize: 18 }}>{searchResult.name || searchResult.symbol}</Text>
                  <Text type="secondary">{searchResult.symbol}</Text>
                </Space>
                <Space direction="vertical" align="end" size={4}>
                  <Text style={{ fontSize: 24, fontWeight: 700, color: getChangeColor(searchResult.change_percent) }}>
                    {formatPrice(searchResult.price)}
                  </Text>
                  <Tag style={{ border: 'none', background: getChangeBgColor(searchResult.change_percent), color: getChangeColor(searchResult.change_percent) }}>
                    {formatChangePercent(searchResult.change_percent)}
                  </Tag>
                </Space>
              </Space>
              <Space style={{ marginTop: 12 }}>
                <Button type="primary" size="small" onClick={() => navigate(`/stock/kline?symbol=${searchResult.symbol}`)}>K线分析</Button>
                <Button size="small" onClick={() => navigate(`/stock/realtime?symbol=${searchResult.symbol}`)}>盘口</Button>
                <Button size="small" onClick={() => navigate(`/stock/paper?symbol=${searchResult.symbol}`)}>模拟交易</Button>
              </Space>
            </div>
          ) : null}
        </Card>

        <Row gutter={[16, 16]}>
          <Col xs={24} md={12} xl={6}>
            <Card className="stock-card">
              <Statistic title="总资产" value={equity} precision={2} prefix="¥" valueStyle={{ color: '#122033' }} />
            </Card>
          </Col>
          <Col xs={24} md={12} xl={6}>
            <Card className="stock-card">
              <Statistic title="现金余额" value={cash} precision={2} prefix="¥" valueStyle={{ color: '#0f6fff' }} />
            </Card>
          </Col>
          <Col xs={24} md={12} xl={6}>
            <Card className="stock-card">
              <Statistic title="持仓市值" value={marketValue} precision={2} prefix="¥" valueStyle={{ color: '#122033' }} />
            </Card>
          </Col>
          <Col xs={24} md={12} xl={6}>
            <Card className="stock-card">
              <Statistic title="累计收益率" value={totalPLRate} precision={2} suffix="%" valueStyle={{ color: getChangeColor(totalPL) }} />
            </Card>
          </Col>
        </Row>

        <Row gutter={[16, 16]}>
          <Col xs={24} xl={16}>
            <Card className="stock-card" title={<Space><FundOutlined /><span>自选行情</span></Space>} extra={<Button type="link" onClick={() => navigate('/stock/watchlist')}>全部自选</Button>}>
              <Table
                rowKey="item_id"
                pagination={false}
                dataSource={watchlist}
                columns={watchlistColumns}
                locale={{ emptyText: <Empty description="暂无自选股" /> }}
              />
            </Card>
          </Col>

          <Col xs={24} xl={8}>
            <Card className="stock-card" title={<Space><LineChartOutlined /><span>市场指数</span></Space>} extra={<Button type="link" onClick={() => navigate('/stock/market')}>详情</Button>}>
              <Space direction="vertical" style={{ width: '100%' }} size={12}>
                {indices.map((item) => {
                  const change = Number(item.change_percent || 0);
                  return (
                    <div key={item.symbol} style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
                      <div>
                        <Text strong>{item.name}</Text>
                        <Text type="secondary" style={{ display: 'block', fontSize: 12 }}>{item.symbol}</Text>
                      </div>
                      <div style={{ textAlign: 'right' }}>
                        <Text style={{ color: getChangeColor(change), fontWeight: 700 }}>{formatPrice(item.price)}</Text>
                        <Tag style={{ display: 'block', marginTop: 4, border: 'none', background: getChangeBgColor(change), color: getChangeColor(change) }}>
                          {formatChangePercent(change)}
                        </Tag>
                      </div>
                    </div>
                  );
                })}
              </Space>
            </Card>
          </Col>
        </Row>

        <Row gutter={[16, 16]}>
          <Col xs={24} xl={12}>
            <Card className="stock-card" title={<Space><DollarOutlined /><span>持仓概览</span></Space>} extra={<Button type="link" onClick={() => navigate('/stock/account')}>账户持仓</Button>}>
              {positions.length === 0 ? (
                <Empty description="暂无持仓" />
              ) : (
                <Table
                  rowKey="symbol"
                  size="small"
                  pagination={false}
                  dataSource={positions}
                  columns={[
                    { title: '代码', dataIndex: 'symbol', key: 'symbol' },
                    { title: '数量', dataIndex: 'qty', key: 'qty', align: 'right' },
                    { title: '成本', dataIndex: 'avg_cost', key: 'avg_cost', align: 'right', render: formatPrice },
                    { title: '现价', dataIndex: 'last_price', key: 'last_price', align: 'right', render: formatPrice },
                  ]}
                />
              )}
            </Card>
          </Col>

          <Col xs={24} xl={12}>
            <Card className="stock-card" title={<Space><BellOutlined /><span>待处理告警</span></Space>} extra={<Button type="link" onClick={() => navigate('/stock/alerts')}>告警中心</Button>}>
              {alerts.length === 0 ? (
                <Empty description="暂无待处理告警" />
              ) : (
                <Space direction="vertical" style={{ width: '100%' }}>
                  {alerts.map((item) => (
                    <div key={item.event_id || `${item.symbol}-${item.created_at}`} style={{ padding: 12, border: '1px solid #d8e2ee', borderRadius: 10, background: '#f7f9fc' }}>
                      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                        <Text strong>{item.symbol}</Text>
                        <Tag color="red">{item.status || 'open'}</Tag>
                      </Space>
                      <Text type="secondary">{item.message || `${item.metric || '规则'} 触发告警`}</Text>
                    </div>
                  ))}
                </Space>
              )}
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default DashboardPage;

import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Empty,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd';
import { LineChartOutlined, ReloadOutlined } from '@ant-design/icons';
import ReactECharts from 'echarts-for-react';
import { useNavigate } from 'react-router-dom';
import { marketApi } from '../api/stockApi';
import { formatChangePercent, formatPrice, getChangeBgColor, getChangeColor } from '../utils';

const { Text, Title } = Typography;

const MAJOR_INDICES = [
  { symbol: '000001', name: '上证指数' },
  { symbol: '399001', name: '深证成指' },
  { symbol: '399006', name: '创业板指' },
  { symbol: '000300', name: '沪深300' },
  { symbol: '000016', name: '上证50' },
  { symbol: '000905', name: '中证500' },
];

const unwrap = (res) => res?.data || res || {};

const MarketPage = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState('000001');
  const [indices, setIndices] = useState([]);
  const [bars, setBars] = useState([]);
  const [loadError, setLoadError] = useState('');

  const loadIndices = async () => {
    setLoading(true);
    setLoadError('');
    try {
      const responses = await Promise.all(
        MAJOR_INDICES.map(async (item) => {
          try {
            const res = await marketApi.snapshot(item.symbol);
            const payload = unwrap(res);
            return { ...item, ...(payload.item || payload) };
          } catch {
            return { ...item };
          }
        })
      );
      setIndices(responses);
    } catch (err) {
      setLoadError(err.message || '指数行情加载失败');
      setIndices([]);
    } finally {
      setLoading(false);
    }
  };

  const loadKline = async (symbol) => {
    try {
      const res = await marketApi.kline(symbol, '1d', 90);
      const payload = unwrap(res);
      setBars(payload.bars || []);
    } catch (err) {
      setBars([]);
    }
  };

  useEffect(() => {
    loadIndices();
  }, []);

  useEffect(() => {
    loadKline(selectedIndex);
  }, [selectedIndex]);

  const selectedItem = indices.find((item) => item.symbol === selectedIndex) || MAJOR_INDICES.find((item) => item.symbol === selectedIndex);

  const chartOption = {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    grid: { left: 48, right: 24, top: 24, bottom: 36 },
    xAxis: {
      type: 'category',
      data: bars.map((bar) => (bar.ts || '').slice(0, 10)),
      boundaryGap: false,
    },
    yAxis: {
      type: 'value',
      scale: true,
      splitLine: { lineStyle: { color: '#e6edf5' } },
    },
    series: [
      {
        type: 'line',
        data: bars.map((bar) => bar.close),
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2, color: '#0f6fff' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(15, 111, 255, 0.24)' },
              { offset: 1, color: 'rgba(15, 111, 255, 0.02)' },
            ],
          },
        },
      },
    ],
  };

  const marketStats = {
    up: indices.filter((item) => Number(item.change_percent || 0) > 0).length,
    down: indices.filter((item) => Number(item.change_percent || 0) < 0).length,
    flat: indices.filter((item) => Number(item.change_percent || 0) === 0).length,
  };

  return (
    <div className="stock-page stock-layout">
      <div className="stock-page-header">
        <Title level={4} style={{ margin: 0, color: '#122033' }}>
          市场行情
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={loadIndices}>刷新</Button>
        </Space>
      </div>

      <div className="stock-content">
        <Row gutter={[16, 16]}>
          {MAJOR_INDICES.map((base) => {
            const item = indices.find((candidate) => candidate.symbol === base.symbol) || base;
            const changePercent = Number(item.change_percent || 0);
            const change = Number(item.change || 0);
            return (
              <Col xs={24} md={12} xl={8} key={base.symbol}>
                <Card
                  className="stock-card"
                  hoverable
                  bodyStyle={{ padding: 16 }}
                  onClick={() => setSelectedIndex(base.symbol)}
                >
                  <Space direction="vertical" size={6} style={{ width: '100%' }}>
                    <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                      <Text strong>{base.name}</Text>
                      <Tag color={selectedIndex === base.symbol ? 'blue' : 'default'}>{base.symbol}</Tag>
                    </Space>
                    <Text style={{ fontSize: 26, fontWeight: 700, color: getChangeColor(changePercent) }}>
                      {formatPrice(item.price)}
                    </Text>
                    <Space>
                      <Tag style={{ border: 'none', background: getChangeBgColor(changePercent), color: getChangeColor(changePercent) }}>
                        {formatChangePercent(changePercent)}
                      </Tag>
                      <Text style={{ color: getChangeColor(change) }}>
                        {change > 0 ? '+' : ''}{Number.isFinite(change) ? change.toFixed(2) : '--'}
                      </Text>
                    </Space>
                  </Space>
                </Card>
              </Col>
            );
          })}
        </Row>

        <Row gutter={[16, 16]}>
          <Col xs={24} xl={16}>
            <Card
              className="stock-card"
              title={
                <Space>
                  <LineChartOutlined />
                  <span>{selectedItem?.name || '指数走势'}</span>
                  <Select
                    size="small"
                    value={selectedIndex}
                    onChange={setSelectedIndex}
                    options={MAJOR_INDICES.map((item) => ({ value: item.symbol, label: item.name }))}
                    style={{ width: 140 }}
                  />
                </Space>
              }
            >
              {bars.length > 0 ? (
                <ReactECharts option={chartOption} style={{ height: 360 }} notMerge />
              ) : loading ? (
                <div className="stock-loading"><Spin /></div>
              ) : (
                <Empty description="暂无指数K线数据" />
              )}
            </Card>
          </Col>

          <Col xs={24} xl={8}>
            <Card className="stock-card" title="市场概览" bodyStyle={{ padding: 16 }}>
              <Row gutter={[12, 12]}>
                <Col span={12}>
                  <Statistic title="上涨指数" value={marketStats.up} valueStyle={{ color: '#ee4444' }} />
                </Col>
                <Col span={12}>
                  <Statistic title="下跌指数" value={marketStats.down} valueStyle={{ color: '#00a54b' }} />
                </Col>
                <Col span={12}>
                  <Statistic title="平盘指数" value={marketStats.flat} valueStyle={{ color: '#62748a' }} />
                </Col>
                <Col span={12}>
                  <Statistic
                    title="当前选中"
                    value={selectedItem?.name || '--'}
                    valueStyle={{ color: '#122033', fontSize: 16 }}
                  />
                </Col>
              </Row>
              <div style={{ marginTop: 16, color: '#62748a', fontSize: 12 }}>
                当前页面只展示已接入真实接口的大盘指数，不再使用静态快讯和伪板块数据。
              </div>
            </Card>
          </Col>
        </Row>

        <Card className="stock-card" title="指数列表">
          <Table
            rowKey="symbol"
            pagination={false}
            dataSource={indices}
            locale={{ emptyText: loadError || '暂无指数行情数据' }}
            columns={[
              {
                title: '指数',
                dataIndex: 'name',
                key: 'name',
                render: (_, record) => (
                  <Button type="link" onClick={() => navigate(`/stock/kline?symbol=${record.symbol}`)}>
                    {record.name || record.symbol}
                  </Button>
                ),
              },
              { title: '代码', dataIndex: 'symbol', key: 'symbol' },
              {
                title: '最新价',
                dataIndex: 'price',
                key: 'price',
                render: (value) => formatPrice(value),
              },
              {
                title: '涨跌幅',
                dataIndex: 'change_percent',
                key: 'change_percent',
                render: (value) => <Text style={{ color: getChangeColor(Number(value || 0)) }}>{formatChangePercent(value)}</Text>,
              },
              {
                title: '更新时间',
                dataIndex: 'ts',
                key: 'ts',
                render: (value) => value ? new Date(value).toLocaleString() : '-',
              },
            ]}
          />
        </Card>
      </div>
    </div>
  );
};

export default MarketPage;

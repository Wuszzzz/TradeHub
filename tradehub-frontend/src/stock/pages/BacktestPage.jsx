/**
 * BacktestPage - 专业级回测页面
 * 功能：策略选择、参数配置、回测执行、结果展示
 */

import React, { useState, useEffect, useCallback } from 'react';
import {
  Layout,
  Card,
  Row,
  Col,
  Table,
  Button,
  Select,
  Input,
  Space,
  Typography,
  Tag,
  Form,
  Modal,
  Divider,
  message,
  Spin,
  Empty,
  Tabs,
  Statistic,
  Collapse,
  Slider,
  InputNumber,
  Progress,
} from 'antd';
import {
  PlayCircleOutlined,
  ReloadOutlined,
  LineChartOutlined,
  TableOutlined,
  TrophyOutlined,
  ThunderboltOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { useSearchParams, useNavigate } from 'react-router-dom';
import ReactECharts from 'echarts-for-react';
import { backtestApi, marketApi } from '../api/stockApi';
import {
  formatPrice,
  formatPercent,
  formatAmount,
  formatDate,
  getChangeColor,
  getChangeBgColor,
} from '../utils';

const { Text, Title } = Typography;
const { Panel } = Collapse;

const STRATEGIES = [
  {
    id: 'buy_and_hold',
    name: '买入持有',
    description: '在回测开始时买入，持有到最后',
    params: [],
  },
  {
    id: 'ma_5_20',
    name: '均线策略(MA5/MA20)',
    description: 'MA5上穿MA20买入，下穿卖出',
    params: ['fast_period', 'slow_period'],
  },
  {
    id: 'ma_10_60',
    name: '均线策略(MA10/MA60)',
    description: 'MA10上穿MA60买入，下穿卖出',
    params: ['fast_period', 'slow_period'],
  },
  {
    id: 'ma_20_60',
    name: '均线策略(MA20/MA60)',
    description: 'MA20上穿MA60买入，下穿卖出',
    params: ['fast_period', 'slow_period'],
  },
  {
    id: 'macd',
    name: 'MACD策略',
    description: 'MACD金叉买入，死叉卖出',
    params: ['fast_period', 'slow_period', 'signal_period'],
  },
];

const BacktestPage = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // 状态
  const [loading, setLoading] = useState(false);
  const [running, setRunning] = useState(false);
  const [symbol, setSymbol] = useState(searchParams.get('symbol') || '000001');
  const [strategies, setStrategies] = useState([]);
  const [selectedStrategy, setSelectedStrategy] = useState('ma_5_20');
  const [period, setPeriod] = useState('1d');
  const [result, setResult] = useState(null);
  const [history, setHistory] = useState([]);

  // 参数
  const [params, setParams] = useState({
    lookback: 260,
    hold_bars: 20,
    initial_cash: 1000000,
    fee_rate: 0.00025,
    slippage_rate: 0.0005,
    stop_loss: 0,
    take_profit: 0,
  });

  // 加载策略列表
  const loadStrategies = useCallback(async () => {
    try {
      const res = await backtestApi.getStrategies();
      setStrategies(res.items || []);
    } catch (err) {
      setStrategies(STRATEGIES);
    }
  }, []);

  // 加载历史
  const loadHistory = useCallback(async () => {
    try {
      const res = await backtestApi.getResults();
      setHistory(res.items?.slice(0, 20) || []);
    } catch (err) {
      console.error('加载历史失败', err);
    }
  }, []);

  // 执行回测
  const executeBacktest = async () => {
    if (!symbol) {
      message.warning('请输入股票代码');
      return;
    }

    setRunning(true);
    setResult(null);

    try {
      const res = await backtestApi.execute({
        strategy_id: selectedStrategy,
        strategy_name: strategies.find((s) => s.id === selectedStrategy)?.name || selectedStrategy,
        symbol: symbol,
        period: period,
        lookback: params.lookback,
        hold_bars: params.hold_bars,
        initial_cash: params.initial_cash,
        fee_rate: params.fee_rate,
        slippage_rate: params.slippage_rate,
        stop_loss: params.stop_loss,
        take_profit: params.take_profit,
      });

      setResult(res.item);
      message.success('回测完成');
      loadHistory();
    } catch (err) {
      message.error(err.message || '回测失败');
    } finally {
      setRunning(false);
    }
  };

  // 初始化
  useEffect(() => {
    loadStrategies();
    loadHistory();
  }, [loadStrategies, loadHistory]);

  // 获取策略详情
  const currentStrategy = strategies.find((s) => s.id === selectedStrategy) ||
    STRATEGIES.find((s) => s.id === selectedStrategy);

  // 权益曲线图配置
  const getEquityCurveOption = () => {
    if (!result?.equity_curve || result.equity_curve.length === 0) return null;

    const dates = result.equity_curve.map((p) => formatDate(p.date));
    const equities = result.equity_curve.map((p) => p.equity);

    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(255, 255, 255, 0.96)',
        borderColor: '#d8e2ee',
        textStyle: { color: '#122033' },
      },
      grid: { left: 60, right: 20, top: 20, bottom: 40 },
      xAxis: {
        type: 'category',
        data: dates,
        axisLine: { lineStyle: { color: '#d8e2ee' } },
        axisLabel: { color: '#62748a', fontSize: 10 },
      },
      yAxis: {
        type: 'value',
        axisLine: { lineStyle: { color: '#d8e2ee' } },
        axisLabel: { color: '#62748a', formatter: (v) => formatAmount(v) },
        splitLine: { lineStyle: { color: '#e1e8f2', type: 'dashed' } },
      },
      dataZoom: [
        { type: 'inside', start: 0, end: 100 },
        {
          type: 'slider',
          height: 20,
          borderColor: '#d8e2ee',
          backgroundColor: '#f7f9fc',
          fillerColor: 'rgba(15, 111, 255, 0.16)',
          handleStyle: { color: '#0f6fff' },
          textStyle: { color: '#62748a' },
        },
      ],
      series: [
        {
          name: '账户权益',
          type: 'line',
          data: equities,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: '#1890ff', width: 2 },
          areaStyle: {
            color: {
              type: 'linear',
              x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(24, 144, 255, 0.3)' },
                { offset: 1, color: 'rgba(24, 144, 255, 0.05)' },
              ],
            },
          },
        },
        {
          name: '初始资金',
          type: 'line',
          data: new Array(dates.length).fill(params.initial_cash),
          symbol: 'none',
          lineStyle: { color: '#888', width: 1, type: 'dashed' },
        },
      ],
    };
  };

  // 交易记录表
  const tradeColumns = [
    {
      title: '入场时间',
      dataIndex: 'entry_date',
      key: 'entry_date',
      width: 120,
      render: (date) => formatDate(date),
    },
    {
      title: '出场时间',
      dataIndex: 'exit_date',
      key: 'exit_date',
      width: 120,
      render: (date) => formatDate(date),
    },
    {
      title: '入场价',
      dataIndex: 'entry_price',
      key: 'entry_price',
      width: 90,
      align: 'right',
      render: (p) => formatPrice(p),
    },
    {
      title: '出场价',
      dataIndex: 'exit_price',
      key: 'exit_price',
      width: 90,
      align: 'right',
      render: (p) => formatPrice(p),
    },
    {
      title: '持仓天数',
      key: 'days',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const days = Math.round((new Date(record.exit_date) - new Date(record.entry_date)) / 86400000);
        return days;
      },
    },
    {
      title: '盈亏',
      dataIndex: 'pnl',
      key: 'pnl',
      width: 100,
      align: 'right',
      render: (pnl) => (
        <Text style={{ color: getChangeColor(pnl), fontFamily: 'SF Mono, Consolas, monospace' }}>
          {pnl >= 0 ? '+' : ''}{formatAmount(pnl)}
        </Text>
      ),
    },
    {
      title: '收益率',
      dataIndex: 'pnl_rate',
      key: 'pnl_rate',
      width: 90,
      align: 'right',
      render: (rate) => (
        <Tag
          style={{
            background: getChangeBgColor(rate * 100),
            color: getChangeColor(rate * 100),
            border: 'none',
          }}
        >
          {formatPercent(rate * 100)}
        </Tag>
      ),
    },
    {
      title: '出场原因',
      dataIndex: 'reason',
      key: 'reason',
      render: (reason) => <Text type="secondary">{reason}</Text>,
    },
  ];

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <Title level={4} style={{ margin: 0, color: '#122033' }}>
          策略回测
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={loadHistory}>
            刷新历史
          </Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        <Row gutter={16}>
          {/* 左侧配置 */}
          <Col span={8}>
            <Card
              className="stock-card"
              title={
                <Space>
                  <SettingOutlined />
                  <span>回测配置</span>
                </Space>
              }
              style={{ marginBottom: 16 }}
            >
              {/* 基础配置 */}
              <Form layout="vertical">
                <Form.Item label="股票代码">
                  <Input
                    value={symbol}
                    onChange={(e) => setSymbol(e.target.value.toUpperCase())}
                    placeholder="例如：000001"
                  />
                </Form.Item>

                <Form.Item label="K线周期">
                  <Select
                    value={period}
                    onChange={setPeriod}
                    options={[
                      { value: '1d', label: '日线' },
                      { value: '1w', label: '周线' },
                      { value: '1M', label: '月线' },
                    ]}
                  />
                </Form.Item>

                <Form.Item label="回测策略">
                  <Select
                    value={selectedStrategy}
                    onChange={setSelectedStrategy}
                    options={(strategies.length > 0 ? strategies : STRATEGIES).map((s) => ({
                      value: s.id,
                      label: s.name,
                    }))}
                  />
                </Form.Item>

                {currentStrategy?.description && (
                  <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
                    {currentStrategy.description}
                  </Text>
                )}

                <Divider>回测参数</Divider>

                <Form.Item label={`回看周期（{params.lookback}根K线）`}>
                  <Slider
                    min={60}
                    max={500}
                    value={params.lookback}
                    onChange={(v) => setParams({ ...params, lookback: v })}
                    marks={{ 60: '60', 260: '260', 500: '500' }}
                  />
                </Form.Item>

                <Form.Item label="初始资金">
                  <InputNumber
                    style={{ width: '100%' }}
                    value={params.initial_cash}
                    onChange={(v) => setParams({ ...params, initial_cash: v || 1000000 })}
                    formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}
                    parser={(v) => v.replace(/,/g, '')}
                  />
                </Form.Item>

                <Form.Item label="手续费率">
                  <InputNumber
                    style={{ width: '100%' }}
                    value={params.fee_rate}
                    onChange={(v) => setParams({ ...params, fee_rate: v || 0.00025 })}
                    precision={5}
                    suffix="%"
                    formatter={(v) => (v * 100).toFixed(4)}
                    parser={(v) => parseFloat(v) / 100}
                  />
                </Form.Item>

                <Divider>风控参数</Divider>

                <Row gutter={8}>
                  <Col span={12}>
                    <Form.Item label="止损">
                      <InputNumber
                        style={{ width: '100%' }}
                        value={params.stop_loss * 100}
                        onChange={(v) => setParams({ ...params, stop_loss: (v || 0) / 100 })}
                        precision={2}
                        suffix="%"
                      />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="止盈">
                      <InputNumber
                        style={{ width: '100%' }}
                        value={params.take_profit * 100}
                        onChange={(v) => setParams({ ...params, take_profit: (v || 0) / 100 })}
                        precision={2}
                        suffix="%"
                      />
                    </Form.Item>
                  </Col>
                </Row>

                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  block
                  size="large"
                  loading={running}
                  onClick={executeBacktest}
                  style={{ marginTop: 16 }}
                >
                  {running ? '回测中...' : '开始回测'}
                </Button>
              </Form>
            </Card>
          </Col>

          {/* 右侧结果 */}
          <Col span={16}>
            {running && (
              <Card className="stock-card" style={{ marginBottom: 16 }}>
                <div style={{ textAlign: 'center', padding: 48 }}>
                  <Spin size="large" />
                  <div style={{ marginTop: 16, color: '#888' }}>
                    正在执行回测，请稍候...
                  </div>
                </div>
              </Card>
            )}

            {result && !running && (
              <>
                {/* 指标卡片 */}
                <Row gutter={16} style={{ marginBottom: 16 }}>
                  <Col span={4}>
                    <Card className="stock-card" bodyStyle={{ textAlign: 'center' }}>
                      <Statistic
                        title="总收益率"
                        value={result.metrics?.total_return || 0}
                        precision={2}
                        suffix="%"
                        valueStyle={{ color: getChangeColor(result.metrics?.total_return), fontSize: 20 }}
                      />
                    </Card>
                  </Col>
                  <Col span={4}>
                    <Card className="stock-card" bodyStyle={{ textAlign: 'center' }}>
                      <Statistic
                        title="年化收益率"
                        value={result.metrics?.annual_return || 0}
                        precision={2}
                        suffix="%"
                        valueStyle={{ color: getChangeColor(result.metrics?.annual_return), fontSize: 20 }}
                      />
                    </Card>
                  </Col>
                  <Col span={4}>
                    <Card className="stock-card" bodyStyle={{ textAlign: 'center' }}>
                      <Statistic
                        title="夏普比率"
                        value={result.metrics?.sharpe_ratio || 0}
                        precision={2}
                        valueStyle={{ color: '#1890ff', fontSize: 20 }}
                      />
                    </Card>
                  </Col>
                  <Col span={4}>
                    <Card className="stock-card" bodyStyle={{ textAlign: 'center' }}>
                      <Statistic
                        title="最大回撤"
                        value={result.metrics?.max_drawdown || 0}
                        precision={2}
                        suffix="%"
                        valueStyle={{ color: '#ff4d4f', fontSize: 20 }}
                      />
                    </Card>
                  </Col>
                  <Col span={4}>
                    <Card className="stock-card" bodyStyle={{ textAlign: 'center' }}>
                      <Statistic
                        title="胜率"
                        value={result.metrics?.win_rate || 0}
                        precision={1}
                        suffix="%"
                        valueStyle={{ color: '#52c41a', fontSize: 20 }}
                      />
                    </Card>
                  </Col>
                  <Col span={4}>
                    <Card className="stock-card" bodyStyle={{ textAlign: 'center' }}>
                      <Statistic
                        title="交易次数"
                        value={result.metrics?.total_trades || 0}
                        valueStyle={{ color: '#122033', fontSize: 20 }}
                      />
                    </Card>
                  </Col>
                </Row>

                {/* 权益曲线 */}
                <Card
                  className="stock-card"
                  title={
                    <Space>
                      <LineChartOutlined />
                      <span>权益曲线</span>
                    </Space>
                  }
                  style={{ marginBottom: 16 }}
                >
                  {result.equity_curve?.length > 0 ? (
                    <ReactECharts
                      option={getEquityCurveOption()}
                      style={{ height: 300 }}
                      notMerge={true}
                    />
                  ) : (
                    <Empty description="暂无数据" />
                  )}
                </Card>

                {/* 交易记录 */}
                <Card
                  className="stock-card"
                  title={
                    <Space>
                      <TableOutlined />
                      <span>交易记录</span>
                      <Tag>{result.trades?.length || 0}</Tag>
                    </Space>
                  }
                  bodyStyle={{ padding: 0 }}
                >
                  {result.trades?.length > 0 ? (
                    <Table
                      className="stock-table"
                      columns={tradeColumns}
                      dataSource={result.trades}
                      rowKey={(r, i) => i}
                      pagination={{ pageSize: 10, showTotal: (t) => `共 ${t} 条` }}
                    />
                  ) : (
                    <Empty description="暂无交易记录" />
                  )}
                </Card>
              </>
            )}

            {!result && !running && (
              <Card className="stock-card">
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <Space direction="vertical">
                      <Text>配置回测参数后点击"开始回测"</Text>
                      <Text type="secondary">
                        支持均线策略、MACD策略、买入持有等多种策略
                      </Text>
                    </Space>
                  }
                />
              </Card>
            )}
          </Col>
        </Row>

        {/* 历史记录 */}
        {history.length > 0 && (
          <Card
            className="stock-card"
            title={
              <Space>
                <TrophyOutlined />
                <span>回测历史</span>
              </Space>
            }
            style={{ marginTop: 16 }}
            bodyStyle={{ padding: 0 }}
          >
            <Table
              className="stock-table"
              dataSource={history}
              rowKey="task_id"
              pagination={false}
              columns={[
                { title: '股票', dataIndex: 'symbol', key: 'symbol', width: 100 },
                {
                  title: '收益率',
                  dataIndex: 'total_return_pct',
                  key: 'total_return_pct',
                  width: 100,
                  align: 'right',
                  render: (v) => (
                    <Tag
                      style={{
                        background: getChangeBgColor(v),
                        color: getChangeColor(v),
                        border: 'none',
                      }}
                    >
                      {formatPercent(v)}
                    </Tag>
                  ),
                },
                {
                  title: '夏普比率',
                  dataIndex: 'sharpe_ratio',
                  key: 'sharpe_ratio',
                  width: 80,
                  align: 'right',
                },
                {
                  title: '最大回撤',
                  dataIndex: 'max_drawdown_pct',
                  key: 'max_drawdown_pct',
                  width: 100,
                  align: 'right',
                  render: (v) => (
                    <Text style={{ color: '#ff4d4f' }}>{formatPercent(v)}</Text>
                  ),
                },
                {
                  title: '交易次数',
                  dataIndex: 'total_trades',
                  key: 'total_trades',
                  width: 80,
                  align: 'right',
                },
                {
                  title: '创建时间',
                  dataIndex: 'created_at',
                  key: 'created_at',
                  width: 160,
                  render: (d) => formatDate(d),
                },
              ]}
            />
          </Card>
        )}
      </div>
    </div>
  );
};

export default BacktestPage;

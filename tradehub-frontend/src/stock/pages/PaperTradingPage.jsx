/**
 * PaperTradingPage - 专业级模拟交易页面
 * 功能：账户概览、下单面板、持仓管理、订单记录
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
  Descriptions,
  InputNumber,
  Alert,
  Tooltip,
  Popconfirm,
} from 'antd';
import {
  PlusOutlined,
  SwapOutlined,
  DeleteOutlined,
  ReloadOutlined,
  FundOutlined,
  HistoryOutlined,
  WalletOutlined,
  RiseOutlined,
  FallOutlined,
} from '@ant-design/icons';
import { useSearchParams, useNavigate } from 'react-router-dom';
import ReactECharts from 'echarts-for-react';
import { paperApi, marketApi } from '../api/stockApi';
import {
  formatPrice,
  formatChangePercent,
  formatMoney,
  formatAmount,
  formatQty,
  formatDateTime,
  getChangeColor,
  getChangeBgColor,
  getSideName,
  getOrderStatusName,
  calcUnrealizedPL,
  calcUnrealizedPLRate,
  calcMarketValue,
} from '../utils';

const { Text, Title } = Typography;
const { Search } = Input;

const PaperTradingPage = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // 状态
  const [loading, setLoading] = useState(false);
  const [account, setAccount] = useState(null);
  const [positions, setPositions] = useState([]);
  const [orders, setOrders] = useState([]);
  const [orderModalVisible, setOrderModalVisible] = useState(false);
  const [orderForm] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [resetModalVisible, setResetModalVisible] = useState(false);
  const [resetAmount, setResetAmount] = useState(1000000);
  const [activeTab, setActiveTab] = useState('positions');

  // 初始化股票代码
  const initSymbol = searchParams.get('symbol') || '';

  // 加载账户数据
  const loadAccount = useCallback(async () => {
    setLoading(true);
    try {
      const res = await paperApi.getAccount();
      setAccount(res.item);
    } catch (err) {
      message.error('加载账户失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 加载持仓
  const loadPositions = useCallback(async () => {
    try {
      const res = await paperApi.getPositions();
      setPositions(res.items || []);
    } catch (err) {
      message.error('加载持仓失败');
    }
  }, []);

  // 加载订单
  const loadOrders = useCallback(async () => {
    try {
      const res = await paperApi.getOrders();
      setOrders(res.items || []);
    } catch (err) {
      message.error('加载订单失败');
    }
  }, []);

  // 初始化
  useEffect(() => {
    loadAccount();
    loadPositions();
    loadOrders();
  }, [loadAccount, loadPositions, loadOrders]);

  // 如果URL带了symbol，打开下单弹窗
  useEffect(() => {
    if (initSymbol) {
      orderForm.setFieldsValue({ symbol: initSymbol, side: 'buy' });
      setOrderModalVisible(true);
    }
  }, [initSymbol]);

  // 获取行情快照
  const fetchSnapshot = async (symbol) => {
    if (!symbol) return null;
    try {
      const res = await marketApi.snapshot(symbol);
      return res.data || res;
    } catch (err) {
      return null;
    }
  };

  // 下单
  const handleSubmitOrder = async () => {
    try {
      const values = await orderForm.validateFields();
      setSubmitting(true);

      // 如果没有输入价格，先获取实时价格
      if (!values.price) {
        const snapshot = await fetchSnapshot(values.symbol);
        if (snapshot) {
          values.price = snapshot.price;
        } else {
          message.error('无法获取行情价格');
          setSubmitting(false);
          return;
        }
      }

      await paperApi.placeOrder({
        symbol: values.symbol,
        name: values.name || values.symbol,
        market: values.market || 'CN-A',
        side: values.side,
        qty: values.qty,
        price: values.price,
        note: values.note || '',
      });

      message.success(`${values.side === 'buy' ? '买入' : '卖出'}成功`);
      setOrderModalVisible(false);
      orderForm.resetFields();
      loadAccount();
      loadPositions();
      loadOrders();
    } catch (err) {
      message.error(err.message || '下单失败');
    } finally {
      setSubmitting(false);
    }
  };

  // 重置账户
  const handleReset = async () => {
    try {
      await paperApi.resetAccount(resetAmount);
      message.success('账户已重置');
      setResetModalVisible(false);
      loadAccount();
      loadPositions();
      loadOrders();
    } catch (err) {
      message.error('重置失败');
    }
  };

  // 格式化金额
  const fmtMoney = (v) => formatMoney(v, 2);

  // 账户统计
  const accountStats = [
    {
      label: '总资产',
      value: account?.equity || 0,
      icon: <WalletOutlined />,
      color: '#122033',
      prefix: '¥',
    },
    {
      label: '现金余额',
      value: account?.cash || 0,
      icon: <WalletOutlined />,
      color: '#1890ff',
      prefix: '¥',
    },
    {
      label: '持仓市值',
      value: (account?.equity || 0) - (account?.cash || 0),
      icon: <FundOutlined />,
      color: '#722ed1',
      prefix: '¥',
    },
    {
      label: '总收益率',
      value: account?.total_return || 0,
      icon: (account?.total_return || 0) >= 0 ? <RiseOutlined /> : <FallOutlined />,
      color: getChangeColor(account?.total_return),
      suffix: '%',
      precision: 2,
    },
    {
      label: '已实现盈亏',
      value: account?.realized_pl || 0,
      icon: (account?.realized_pl || 0) >= 0 ? <RiseOutlined /> : <FallOutlined />,
      color: getChangeColor(account?.realized_pl),
      prefix: (account?.realized_pl || 0) >= 0 ? '+' : '',
      precision: 2,
    },
    {
      label: '初始资金',
      value: account?.initial || 0,
      icon: <WalletOutlined />,
      color: '#888',
      prefix: '¥',
    },
  ];

  // 持仓表格列
  const positionColumns = [
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 160,
      render: (symbol, record) => (
        <Space direction="vertical" size={0}>
          <Space>
            <Text strong className="stock-symbol" style={{ cursor: 'pointer' }}>
              {symbol}
            </Text>
          </Space>
          <Text className="stock-name">{record.name}</Text>
        </Space>
      ),
    },
    {
      title: '持仓数量',
      dataIndex: 'qty',
      key: 'qty',
      width: 100,
      align: 'right',
      render: (qty) => (
        <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>{formatQty(qty)}</Text>
      ),
    },
    {
      title: '成本价',
      dataIndex: 'avg_cost',
      key: 'avg_cost',
      width: 90,
      align: 'right',
      render: (cost) => (
        <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>{formatPrice(cost)}</Text>
      ),
    },
    {
      title: '现价',
      dataIndex: 'last_price',
      key: 'last_price',
      width: 90,
      align: 'right',
      render: (price, record) => {
        const change = ((price - record.avg_cost) / record.avg_cost) * 100;
        return (
          <Text
            style={{
              color: getChangeColor(change),
              fontFamily: 'SF Mono, Consolas, monospace',
            }}
          >
            {formatPrice(price)}
          </Text>
        );
      },
    },
    {
      title: '浮动盈亏',
      key: 'unrealized_pl',
      width: 110,
      align: 'right',
      render: (_, record) => {
        const pl = calcUnrealizedPL(record.qty, record.avg_cost, record.last_price);
        const rate = calcUnrealizedPLRate(record.avg_cost, record.last_price);
        return (
          <Space direction="vertical" size={0} align="end">
            <Text
              style={{
                color: getChangeColor(pl),
                fontFamily: 'SF Mono, Consolas, monospace',
              }}
            >
              {pl >= 0 ? '+' : ''}{fmtMoney(pl)}
            </Text>
            <Text
              style={{
                color: getChangeColor(pl),
                fontSize: 11,
                fontFamily: 'SF Mono, Consolas, monospace',
              }}
            >
              {formatChangePercent(rate)}
            </Text>
          </Space>
        );
      },
    },
    {
      title: '持仓市值',
      key: 'market_value',
      width: 100,
      align: 'right',
      render: (_, record) => {
        const mv = calcMarketValue(record.qty, record.last_price);
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatAmount(mv)}
          </Text>
        );
      },
    },
    {
      title: '盈亏比例',
      key: 'pl_rate',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const rate = calcUnrealizedPLRate(record.avg_cost, record.last_price);
        return (
          <Tag
            style={{
              background: getChangeBgColor(rate),
              color: getChangeColor(rate),
              border: 'none',
            }}
          >
            {formatChangePercent(rate)}
          </Tag>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="primary"
            size="small"
            icon={<SwapOutlined />}
            onClick={() => {
              orderForm.setFieldsValue({
                symbol: record.symbol,
                name: record.name,
                market: record.market,
                side: 'sell',
                qty: record.qty,
              });
              setOrderModalVisible(true);
            }}
          >
            卖出
          </Button>
          <Button
            size="small"
            icon={<FundOutlined />}
            onClick={() => navigate(`/stock/kline?symbol=${record.symbol}`)}
          >
            K线
          </Button>
        </Space>
      ),
    },
  ];

  // 订单表格列
  const orderColumns = [
    {
      title: '时间',
      dataIndex: 'placed_at',
      key: 'placed_at',
      width: 160,
      render: (time) => (
        <Text style={{ color: '#888', fontSize: 12 }}>{formatDateTime(time)}</Text>
      ),
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 70,
      render: (side) => (
        <Tag color={side === 'buy' ? '#ee4444' : '#00a54c'}>
          {getSideName(side)}
        </Tag>
      ),
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 120,
      render: (symbol, record) => (
        <Space direction="vertical" size={0}>
          <Text strong className="stock-symbol">{symbol}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.name}</Text>
        </Space>
      ),
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 90,
      align: 'right',
      render: (price) => (
        <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>{formatPrice(price)}</Text>
      ),
    },
    {
      title: '数量',
      dataIndex: 'qty',
      key: 'qty',
      width: 90,
      align: 'right',
      render: (qty) => (
        <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>{formatQty(qty)}</Text>
      ),
    },
    {
      title: '金额',
      dataIndex: 'amount',
      key: 'amount',
      width: 100,
      align: 'right',
      render: (amount) => (
        <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>{fmtMoney(amount)}</Text>
      ),
    },
    {
      title: '手续费',
      dataIndex: 'fee',
      key: 'fee',
      width: 80,
      align: 'right',
      render: (fee) => (
        <Text type="secondary" style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
          {fmtMoney(fee)}
        </Text>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status) => {
        const statusConfig = {
          filled: { color: '#52c41a', text: '已成交' },
          canceled: { color: '#888', text: '已撤单' },
          pending: { color: '#1890ff', text: '待成交' },
        };
        const config = statusConfig[status] || { color: '#888', text: status };
        return <Tag color={config.color}>{config.text}</Tag>;
      },
    },
    {
      title: '备注',
      dataIndex: 'note',
      key: 'note',
      width: 100,
      render: (note) => <Text type="secondary">{note || '-'}</Text>,
    },
  ];

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <Title level={4} style={{ margin: 0, color: '#122033' }}>
          模拟交易
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => { loadAccount(); loadPositions(); loadOrders(); }}>
            刷新
          </Button>
          <Button danger icon={<DeleteOutlined />} onClick={() => setResetModalVisible(true)}>
            重置账户
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              orderForm.resetFields();
              setOrderModalVisible(true);
            }}
          >
            新建订单
          </Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        {/* 账户概览 */}
        <Row gutter={16} style={{ marginBottom: 16 }}>
          {accountStats.map((stat, i) => (
            <Col span={4} key={i}>
              <Card className="stock-card" bodyStyle={{ padding: 16 }}>
                <Statistic
                  title={<Text type="secondary">{stat.label}</Text>}
                  value={stat.value}
                  precision={stat.precision ?? 2}
                  prefix={stat.prefix}
                  suffix={stat.suffix}
                  valueStyle={{
                    color: stat.color,
                    fontSize: 20,
                    fontFamily: 'SF Mono, Consolas, monospace',
                  }}
                />
              </Card>
            </Col>
          ))}
        </Row>

        {/* 持仓和订单 */}
        <Card className="stock-card" bodyStyle={{ padding: 0 }}>
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            tabBarStyle={{ padding: '0 16px', marginBottom: 0 }}
            items={[
              {
                key: 'positions',
                label: (
                  <Space>
                    <FundOutlined />
                    <span>持仓</span>
                    <Tag>{positions.length}</Tag>
                  </Space>
                ),
                children: (
                  <Table
                    className="stock-table"
                    columns={positionColumns}
                    dataSource={positions}
                    rowKey="symbol"
                    loading={loading}
                    pagination={false}
                    locale={{
                      emptyText: (
                        <Empty description="暂无持仓" image={Empty.PRESENTED_IMAGE_SIMPLE}>
                          <Button type="primary" onClick={() => navigate('/stock/market')}>
                            去行情
                          </Button>
                        </Empty>
                      ),
                    }}
                  />
                ),
              },
              {
                key: 'orders',
                label: (
                  <Space>
                    <HistoryOutlined />
                    <span>订单记录</span>
                    <Tag>{orders.length}</Tag>
                  </Space>
                ),
                children: (
                  <Table
                    className="stock-table"
                    columns={orderColumns}
                    dataSource={orders}
                    rowKey="order_id"
                    loading={loading}
                    scroll={{ x: 1000 }}
                    pagination={{
                      pageSize: 20,
                      showSizeChanger: true,
                      showTotal: (total) => `共 ${total} 条`,
                    }}
                    locale={{
                      emptyText: (
                        <Empty description="暂无订单" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                      ),
                    }}
                  />
                ),
              },
            ]}
          />
        </Card>
      </div>

      {/* 下单弹窗 */}
      <Modal
        title={
          <Space>
            <SwapOutlined />
            <span>新建订单</span>
          </Space>
        }
        open={orderModalVisible}
        onOk={handleSubmitOrder}
        onCancel={() => {
          setOrderModalVisible(false);
          orderForm.resetFields();
        }}
        okText="提交"
        cancelText="取消"
        confirmLoading={submitting}
        width={480}
      >
        <Form form={orderForm} layout="vertical" initialValues={{ side: 'buy', qty: 100 }}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="symbol"
                label="股票代码"
                rules={[{ required: true, message: '请输入股票代码' }]}
              >
                <Input
                  placeholder="例如：000001"
                  onChange={(e) => {
                    const val = e.target.value.toUpperCase();
                    orderForm.setFieldValue('symbol', val);
                  }}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="name" label="股票名称">
                <Input placeholder="自动获取" disabled />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="side"
            label="交易方向"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: 'buy', label: '买入', color: '#ee4444' },
                { value: 'sell', label: '卖出', color: '#00a54c' },
              ]}
              onChange={(v) => {
                if (v === 'buy') {
                  orderForm.setFieldsValue({ qty: 100 });
                }
              }}
            />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="price"
                label="委托价格"
                rules={[{ required: true, message: '请输入价格' }]}
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  step={0.01}
                  precision={2}
                  placeholder="留空使用实时价格"
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="qty"
                label="委托数量"
                rules={[{ required: true, message: '请输入数量' }]}
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={100}
                  step={100}
                  precision={0}
                  formatter={(value) => `${value}`.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}
                />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="note" label="备注">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>

          <Alert
            message="模拟交易提示"
            description="本模拟交易仅供学习参考，不构成任何投资建议。实际交易结果与模拟结果可能存在差异。"
            type="info"
            showIcon
          />
        </Form>
      </Modal>

      {/* 重置账户弹窗 */}
      <Modal
        title="重置模拟账户"
        open={resetModalVisible}
        onOk={handleReset}
        onCancel={() => setResetModalVisible(false)}
        okText="确认重置"
        okButtonProps={{ danger: true }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text>重置后所有持仓和订单将被清空，账户将恢复至初始状态。</Text>
          <Form layout="vertical">
            <Form.Item label="初始资金">
              <InputNumber
                style={{ width: '100%' }}
                value={resetAmount}
                onChange={setResetAmount}
                min={10000}
                step={10000}
                precision={0}
                formatter={(value) => `${value}`.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}
              />
            </Form.Item>
          </Form>
          <Alert message="此操作不可逆，请确认！" type="warning" showIcon />
        </Space>
      </Modal>
    </div>
  );
};

export default PaperTradingPage;

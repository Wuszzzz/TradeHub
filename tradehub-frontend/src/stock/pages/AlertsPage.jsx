/**
 * AlertsPage - 专业级告警管理页面
 * 功能：告警规则管理、告警事件查看、告警确认
 */

import React, { useState, useEffect, useCallback } from 'react';
import {
  Layout,
  Card,
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
  Tabs,
  Badge,
  Popconfirm,
  Descriptions,
  Timeline,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  ReloadOutlined,
  BellOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { alertApi } from '../api/stockApi';
import { formatDateTime, getChangeColor, getChangeBgColor } from '../utils';

const { Text, Title } = Typography;

const AlertsPage = () => {
  // 状态
  const [loading, setLoading] = useState(false);
  const [rules, setRules] = useState([]);
  const [events, setEvents] = useState([]);
  const [ruleModalVisible, setRuleModalVisible] = useState(false);
  const [ruleForm] = Form.useForm();
  const [activeTab, setActiveTab] = useState('rules');

  // 加载告警规则
  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await alertApi.getRules();
      setRules(res.items || []);
    } catch (err) {
      message.error('加载告警规则失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 加载告警事件
  const loadEvents = useCallback(async () => {
    setLoading(true);
    try {
      const res = await alertApi.getEvents();
      setEvents(res.items || []);
    } catch (err) {
      message.error('加载告警事件失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 初始化
  useEffect(() => {
    loadRules();
    loadEvents();
  }, [loadRules, loadEvents]);

  // 创建告警规则
  const handleCreateRule = async () => {
    try {
      const values = await ruleForm.validateFields();
      await alertApi.createRule({
        rule_id: values.rule_id || undefined,
        symbol: values.symbol,
        name: values.name,
        market: values.market || 'CN-A',
        metric: values.metric,
        op: values.op,
        threshold: values.threshold,
        cooldown_seconds: values.cooldown_seconds || 300,
        enabled: true,
      });
      message.success('告警规则已创建');
      setRuleModalVisible(false);
      ruleForm.resetFields();
      loadRules();
    } catch (err) {
      message.error('创建失败');
    }
  };

  // 删除规则
  const handleDeleteRule = async (ruleId) => {
    try {
      await alertApi.deleteRule(ruleId);
      message.success('规则已删除');
      loadRules();
    } catch (err) {
      message.error('删除失败');
    }
  };

  // 确认事件
  const handleAckEvent = async (eventId) => {
    try {
      await alertApi.ackEvent(eventId);
      message.success('已确认');
      loadEvents();
    } catch (err) {
      message.error('确认失败');
    }
  };

  // 规则表格列
  const ruleColumns = [
    {
      title: '规则名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name) => <Text strong style={{ color: '#122033' }}>{name}</Text>,
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 100,
      render: (symbol, record) => (
        <Space direction="vertical" size={0}>
          <Text className="stock-symbol">{symbol}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.market}</Text>
        </Space>
      ),
    },
    {
      title: '告警指标',
      dataIndex: 'metric',
      key: 'metric',
      width: 100,
      render: (metric) => {
        const metricMap = {
          price: '价格',
          change_percent: '涨跌幅',
          volume: '成交量',
          amount: '成交额',
        };
        return <Tag>{metricMap[metric] || metric}</Tag>;
      },
    },
    {
      title: '条件',
      key: 'condition',
      width: 140,
      render: (_, record) => {
        const opMap = { gt: '>', gte: '>=', lt: '<', lte: '<=', eq: '=' };
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {opMap[record.op]} {record.threshold}
          </Text>
        );
      },
    },
    {
      title: '冷却时间',
      dataIndex: 'cooldown_seconds',
      key: 'cooldown_seconds',
      width: 100,
      render: (seconds) => {
        if (!seconds) return <Text type="secondary">--</Text>;
        if (seconds < 60) return `${seconds}秒`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟`;
        return `${Math.floor(seconds / 3600)}小时`;
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled) => (
        <Tag color={enabled ? 'green' : 'default'}>
          {enabled ? '已启用' : '已禁用'}
        </Tag>
      ),
    },
    {
      title: '最近触发',
      dataIndex: 'last_triggered_at',
      key: 'last_triggered_at',
      width: 160,
      render: (time) => {
        if (!time || time === 'epoch') return <Text type="secondary">从未触发</Text>;
        return (
          <Space direction="vertical" size={0}>
            <Text style={{ fontSize: 12 }}>{formatDateTime(time)}</Text>
            {time !== 'epoch' && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                触发值: {events.find(e => e.rule_id === time)?.value || '--'}
              </Text>
            )}
          </Space>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_, record) => (
        <Space size="small">
          <Popconfirm
            title="确认删除此规则？"
            onConfirm={() => handleDeleteRule(record.rule_id)}
            okText="删除"
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // 事件表格列
  const eventColumns = [
    {
      title: '触发时间',
      dataIndex: 'triggered_at',
      key: 'triggered_at',
      width: 160,
      render: (time) => <Text style={{ color: '#888' }}>{formatDateTime(time)}</Text>,
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 120,
      render: (symbol, record) => (
        <Space direction="vertical" size={0}>
          <Text className="stock-symbol">{symbol}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.name}</Text>
        </Space>
      ),
    },
    {
      title: '告警内容',
      dataIndex: 'message',
      key: 'message',
      render: (msg, record) => (
        <Space direction="vertical" size={0}>
          <Text>{msg || `${record.metric} 触发告警`}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {record.metric} {record.op} {record.threshold}，实际值: {record.value}
          </Text>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => {
        const config = {
          active: { color: '#ff4d4f', text: '待确认', icon: <WarningOutlined /> },
          acked: { color: '#52c41a', text: '已确认', icon: <CheckCircleOutlined /> },
          sent: { color: '#1890ff', text: '已发送', icon: <BellOutlined /> },
        };
        const c = config[status] || { color: '#888', text: status, icon: null };
        return (
          <Tag icon={c.icon} color={c.color}>
            {c.text}
          </Tag>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      render: (_, record) => (
        record.status === 'active' && (
          <Button
            type="link"
            size="small"
            icon={<CheckCircleOutlined />}
            onClick={() => handleAckEvent(record.event_id)}
          >
            确认
          </Button>
        )
      ),
    },
  ];

  // 统计
  const activeEventsCount = events.filter((e) => e.status === 'active').length;
  const ackedEventsCount = events.filter((e) => e.status === 'acked').length;

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <Space>
          <Title level={4} style={{ margin: 0, color: '#122033' }}>
            告警管理
          </Title>
          <Badge count={activeEventsCount} overflowCount={99}>
            <Tag color="red" icon={<WarningOutlined />}>
              待确认 {activeEventsCount}
            </Tag>
          </Badge>
        </Space>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => { loadRules(); loadEvents(); }}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              ruleForm.resetFields();
              setRuleModalVisible(true);
            }}
          >
            新建规则
          </Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        <Card
          className="stock-card"
          bodyStyle={{ padding: 0 }}
        >
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            tabBarStyle={{ padding: '0 16px' }}
            items={[
              {
                key: 'rules',
                label: (
                  <Space>
                    <BellOutlined />
                    <span>告警规则</span>
                    <Badge count={rules.length} />
                  </Space>
                ),
                children: (
                  <Table
                    className="stock-table"
                    columns={ruleColumns}
                    dataSource={rules}
                    rowKey="rule_id"
                    loading={loading}
                    pagination={{
                      pageSize: 20,
                      showTotal: (total) => `共 ${total} 条`,
                    }}
                    locale={{
                      emptyText: (
                        <div style={{ padding: 48, textAlign: 'center' }}>
                          <BellOutlined style={{ fontSize: 48, color: '#333', marginBottom: 16 }} />
                          <div>暂无告警规则</div>
                          <Button
                            type="primary"
                            style={{ marginTop: 16 }}
                            onClick={() => setRuleModalVisible(true)}
                          >
                            新建规则
                          </Button>
                        </div>
                      ),
                    }}
                  />
                ),
              },
              {
                key: 'events',
                label: (
                  <Space>
                    <WarningOutlined />
                    <span>告警事件</span>
                    {activeEventsCount > 0 && (
                      <Badge count={activeEventsCount} />
                    )}
                  </Space>
                ),
                children: (
                  <Table
                    className="stock-table"
                    columns={eventColumns}
                    dataSource={events}
                    rowKey="event_id"
                    loading={loading}
                    pagination={{
                      pageSize: 20,
                      showTotal: (total) => `共 ${total} 条`,
                    }}
                  />
                ),
              },
            ]}
          />
        </Card>
      </div>

      {/* 新建规则弹窗 */}
      <Modal
        title={
          <Space>
            <BellOutlined />
            <span>新建告警规则</span>
          </Space>
        }
        open={ruleModalVisible}
        onOk={handleCreateRule}
        onCancel={() => {
          setRuleModalVisible(false);
          ruleForm.resetFields();
        }}
        okText="创建"
        cancelText="取消"
        width={520}
      >
        <Form form={ruleForm} layout="vertical">
          <Form.Item
            name="symbol"
            label="股票代码"
            rules={[{ required: true, message: '请输入股票代码' }]}
          >
            <Input placeholder="例如：000001" />
          </Form.Item>

          <Form.Item
            name="name"
            label="规则名称"
            rules={[{ required: true, message: '请输入规则名称' }]}
          >
            <Input placeholder="例如：贵州茅台价格告警" />
          </Form.Item>

          <Form.Item name="market" label="市场" initialValue="CN-A">
            <Select
              options={[
                { value: 'CN-A', label: '沪深A股' },
                { value: 'HK', label: '港股' },
                { value: 'US', label: '美股' },
              ]}
            />
          </Form.Item>

          <Divider>告警条件</Divider>

          <Space style={{ width: '100%' }}>
            <Form.Item
              name="metric"
              label="指标"
              rules={[{ required: true }]}
              initialValue="price"
            >
              <Select
                style={{ width: 120 }}
                options={[
                  { value: 'price', label: '价格' },
                  { value: 'change_percent', label: '涨跌幅%' },
                  { value: 'volume', label: '成交量' },
                  { value: 'amount', label: '成交额' },
                ]}
              />
            </Form.Item>

            <Form.Item
              name="op"
              label="条件"
              rules={[{ required: true }]}
              initialValue="gt"
            >
              <Select
                style={{ width: 100 }}
                options={[
                  { value: 'gt', label: '大于 >' },
                  { value: 'gte', label: '大于等于 >=' },
                  { value: 'lt', label: '小于 <' },
                  { value: 'lte', label: '小于等于 <=' },
                  { value: 'eq', label: '等于 =' },
                ]}
              />
            </Form.Item>

            <Form.Item
              name="threshold"
              label="阈值"
              rules={[{ required: true, message: '请输入阈值' }]}
            >
              <Input type="number" style={{ width: 120 }} placeholder="告警触发值" />
            </Form.Item>
          </Space>

          <Form.Item
            name="cooldown_seconds"
            label="冷却时间（秒）"
            extra="同一条规则触发后的最小间隔时间"
            initialValue={300}
          >
            <Select
              options={[
                { value: 60, label: '1分钟' },
                { value: 300, label: '5分钟' },
                { value: 600, label: '10分钟' },
                { value: 1800, label: '30分钟' },
                { value: 3600, label: '1小时' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AlertsPage;

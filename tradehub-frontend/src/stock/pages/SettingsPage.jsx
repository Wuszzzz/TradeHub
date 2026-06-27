/**
 * SettingsPage - 专业级股票设置页面
 * 功能：数据源配置、告警设置、采集任务、系统参数
 */

import React, { useState, useEffect, useCallback } from 'react';
import {
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
  Switch,
  Tabs,
  Descriptions,
  Badge,
  Popconfirm,
  Alert,
} from 'antd';
import {
  SettingOutlined,
  ReloadOutlined,
  PlusOutlined,
  DeleteOutlined,
  BellOutlined,
  DatabaseOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { taskApi, brokerApi } from '../api/stockApi';
import { formatDateTime } from '../utils';

const { Text, Title } = Typography;

const SettingsPage = () => {
  // 状态
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('general');
  const [tasks, setTasks] = useState([]);
  const [brokerStatus, setBrokerStatus] = useState(null);
  const [taskModalVisible, setTaskModalVisible] = useState(false);
  const [taskForm] = Form.useForm();

  // 加载采集任务
  const loadTasks = useCallback(async () => {
    setLoading(true);
    try {
      const res = await taskApi.getIngestionTasks();
      setTasks(res.items || []);
    } catch (err) {
      console.error('加载任务失败', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // 加载券商状态
  const loadBrokerStatus = useCallback(async () => {
    try {
      const res = await brokerApi.getStatus();
      setBrokerStatus(res.item || null);
    } catch (err) {
      console.error('加载券商状态失败', err);
    }
  }, []);

  // 初始化
  useEffect(() => {
    loadTasks();
    loadBrokerStatus();
  }, [loadTasks, loadBrokerStatus]);

  // 创建采集任务
  const handleCreateTask = async () => {
    try {
      const values = await taskForm.validateFields();
      await taskApi.createIngestionTask({
        symbol: values.symbol,
        period: values.period,
        enabled: true,
      });
      message.success('任务已创建');
      setTaskModalVisible(false);
      taskForm.resetFields();
      loadTasks();
    } catch (err) {
      message.error('创建失败');
    }
  };

  // 删除任务
  const handleDeleteTask = async (taskId) => {
    try {
      await taskApi.deleteIngestionTask(taskId);
      message.success('任务已删除');
      loadTasks();
    } catch (err) {
      message.error('删除失败');
    }
  };

  // 任务表格列
  const taskColumns = [
    {
      title: '股票代码',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 120,
      render: (symbol) => <Text strong className="stock-symbol">{symbol}</Text>,
    },
    {
      title: '采集周期',
      dataIndex: 'period',
      key: 'period',
      width: 100,
      render: (period) => {
        const periodMap = {
          '1m': '1分钟',
          '5m': '5分钟',
          '15m': '15分钟',
          '30m': '30分钟',
          '1h': '1小时',
          '1d': '日线',
        };
        return <Tag>{periodMap[period] || period}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled) => (
        <Switch
          checked={enabled}
          checkedChildren={<CheckCircleOutlined />}
          unCheckedChildren={<WarningOutlined />}
          disabled
        />
      ),
    },
    {
      title: '最近采集',
      dataIndex: 'last_run_at',
      key: 'last_run_at',
      width: 160,
      render: (time) => (
        <Text type="secondary">{time ? formatDateTime(time) : '从未运行'}</Text>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (time) => <Text type="secondary">{formatDateTime(time)}</Text>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      render: (_, record) => (
        <Popconfirm
          title="确认删除此任务？"
          onConfirm={() => handleDeleteTask(record.task_id)}
          okText="删除"
          cancelText="取消"
        >
          <Button type="link" size="small" danger icon={<DeleteOutlined />}>
            删除
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <Title level={4} style={{ margin: 0, color: '#122033' }}>
          系统设置
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => { loadTasks(); loadBrokerStatus(); }}>
            刷新
          </Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'general',
              label: (
                <Space>
                  <SettingOutlined />
                  <span>通用设置</span>
                </Space>
              ),
              children: (
                <Row gutter={16}>
                  <Col span={12}>
                    <Card
                      className="stock-card"
                      title={
                        <Space>
                          <BellOutlined />
                          <span>告警设置</span>
                        </Space>
                      }
                    >
                      <Descriptions column={1} bordered size="small">
                        <Descriptions.Item label="价格告警">
                          <Switch defaultChecked />
                        </Descriptions.Item>
                        <Descriptions.Item label="涨跌幅告警">
                          <Switch defaultChecked />
                        </Descriptions.Item>
                        <Descriptions.Item label="成交量异动">
                          <Switch defaultChecked />
                        </Descriptions.Item>
                        <Descriptions.Item label="邮件通知">
                          <Switch />
                        </Descriptions.Item>
                        <Descriptions.Item label="WebHook通知">
                          <Switch />
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  </Col>
                  <Col span={12}>
                    <Card
                      className="stock-card"
                      title={
                        <Space>
                          <ClockCircleOutlined />
                          <span>刷新频率</span>
                        </Space>
                      }
                    >
                      <Form layout="vertical">
                        <Form.Item label="行情刷新间隔">
                          <Select
                            defaultValue="30"
                            options={[
                              { value: '10', label: '10秒' },
                              { value: '30', label: '30秒' },
                              { value: '60', label: '1分钟' },
                              { value: '300', label: '5分钟' },
                            ]}
                          />
                        </Form.Item>
                        <Form.Item label="K线更新间隔">
                          <Select
                            defaultValue="60"
                            options={[
                              { value: '30', label: '30秒' },
                              { value: '60', label: '1分钟' },
                              { value: '300', label: '5分钟' },
                            ]}
                          />
                        </Form.Item>
                      </Form>
                    </Card>
                  </Col>
                </Row>
              ),
            },
            {
              key: 'tasks',
              label: (
                <Space>
                  <DatabaseOutlined />
                  <span>采集任务</span>
                  <Badge count={tasks.length} />
                </Space>
              ),
              children: (
                <Card
                  className="stock-card"
                  extra={
                    <Button
                      type="primary"
                      icon={<PlusOutlined />}
                      onClick={() => {
                        taskForm.resetFields();
                        setTaskModalVisible(true);
                      }}
                    >
                      新建任务
                    </Button>
                  }
                  bodyStyle={{ padding: 0 }}
                >
                  {tasks.length === 0 ? (
                    <div style={{ padding: 48, textAlign: 'center' }}>
                      <DatabaseOutlined style={{ fontSize: 48, color: '#333', marginBottom: 16 }} />
                      <div>暂无采集任务</div>
                      <Button type="primary" style={{ marginTop: 16 }} onClick={() => setTaskModalVisible(true)}>
                        创建第一个任务
                      </Button>
                    </div>
                  ) : (
                    <Table
                      className="stock-table"
                      columns={taskColumns}
                      dataSource={tasks}
                      rowKey="task_id"
                      loading={loading}
                      pagination={{ pageSize: 20 }}
                    />
                  )}
                </Card>
              ),
            },
            {
              key: 'broker',
              label: (
                <Space>
                  <SettingOutlined />
                  <span>券商适配</span>
                </Space>
              ),
              children: (
                <Card
                  className="stock-card"
                  title={
                    <Space>
                      <span>券商连接状态</span>
                      {brokerStatus?.connected ? (
                        <Badge status="success" text="已连接" />
                      ) : (
                        <Badge status="error" text="未连接" />
                      )}
                    </Space>
                  }
                >
                  {brokerStatus ? (
                    <Descriptions column={2} bordered size="small">
                      <Descriptions.Item label="券商名称">
                        {brokerStatus.name || '--'}
                      </Descriptions.Item>
                      <Descriptions.Item label="API状态">
                        <Badge
                          status={brokerStatus.api_enabled ? 'success' : 'default'}
                          text={brokerStatus.api_enabled ? '已启用' : '未启用'}
                        />
                      </Descriptions.Item>
                      <Descriptions.Item label="最后同步">
                        {brokerStatus.last_sync ? formatDateTime(brokerStatus.last_sync) : '--'}
                      </Descriptions.Item>
                      <Descriptions.Item label="同步状态">
                        {brokerStatus.sync_status || '--'}
                      </Descriptions.Item>
                    </Descriptions>
                  ) : (
                    <Alert
                      message="暂无券商配置"
                      description="当前未配置任何券商账户，模拟交易不受影响。"
                      type="info"
                      showIcon
                    />
                  )}
                  <Divider />
                  <Text type="secondary">
                    券商适配功能用于连接真实券商账户进行交易，当前为模拟环境，此功能暂不可用。
                  </Text>
                </Card>
              ),
            },
            {
              key: 'about',
              label: (
                <Space>
                  <CheckCircleOutlined />
                  <span>关于</span>
                </Space>
              ),
              children: (
                <Card className="stock-card">
                  <Descriptions column={1} bordered size="small">
                    <Descriptions.Item label="应用名称">TradeHub 股票</Descriptions.Item>
                    <Descriptions.Item label="版本">v2.5.0</Descriptions.Item>
                    <Descriptions.Item label="构建时间">2026-06-17</Descriptions.Item>
                    <Descriptions.Item label="后端服务">stock-api</Descriptions.Item>
                    <Descriptions.Item label="行情数据源">market-api</Descriptions.Item>
                  </Descriptions>
                  <Divider />
                  <Alert
                    message="免责声明"
                    description="本应用仅供学习研究使用，不构成任何投资建议。股票投资有风险，入市需谨慎。"
                    type="warning"
                    showIcon
                  />
                </Card>
              ),
            },
          ]}
        />
      </div>

      {/* 新建任务弹窗 */}
      <Modal
        title={
          <Space>
            <PlusOutlined />
            <span>新建采集任务</span>
          </Space>
        }
        open={taskModalVisible}
        onOk={handleCreateTask}
        onCancel={() => {
          setTaskModalVisible(false);
          taskForm.resetFields();
        }}
        okText="创建"
        cancelText="取消"
      >
        <Form form={taskForm} layout="vertical">
          <Form.Item
            name="symbol"
            label="股票代码"
            rules={[{ required: true, message: '请输入股票代码' }]}
          >
            <Input placeholder="例如：000001" />
          </Form.Item>

          <Form.Item
            name="period"
            label="采集周期"
            rules={[{ required: true }]}
            initialValue="1d"
          >
            <Select
              options={[
                { value: '1m', label: '1分钟' },
                { value: '5m', label: '5分钟' },
                { value: '15m', label: '15分钟' },
                { value: '30m', label: '30分钟' },
                { value: '1h', label: '1小时' },
                { value: '1d', label: '日线' },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="lookback_days"
            label="历史回溯天数"
            initialValue={365}
          >
            <Select
              options={[
                { value: 30, label: '30天' },
                { value: 90, label: '90天' },
                { value: 180, label: '180天' },
                { value: 365, label: '1年' },
                { value: 730, label: '2年' },
                { value: 1825, label: '5年' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SettingsPage;

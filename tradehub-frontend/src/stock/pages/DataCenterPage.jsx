/**
 * DataCenterPage - 数据中心页面
 * 功能：数据管理、数据源健康监控、定时任务
 */

import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Row,
  Col,
  Table,
  Button,
  Space,
  Typography,
  Tag,
  Statistic,
  Tabs,
  Badge,
  Descriptions,
  Alert,
  Empty,
} from 'antd';
import {
  DatabaseOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  ExclamationCircleOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';
import { datacenterApi } from '../api/stockApi';

const { Text, Title } = Typography;

const taskTypeLabels = {
  daily_spot: '每日股票行情',
  lhb: '龙虎榜',
  dzjy: '大宗交易',
};

const unwrap = (res) => res?.data || res || {};

const formatTime = (value) => {
  if (!value || value === '1970-01-01T00:00:00Z') return '-';
  return new Date(value).toLocaleString();
};

const DataCenterPage = () => {
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('sources');
  const [health, setHealth] = useState(null);
  const [tasks, setTasks] = useState([]);
  const [loadError, setLoadError] = useState('');

  const loadData = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const [healthRes, tasksRes] = await Promise.all([
        datacenterApi.health(),
        datacenterApi.getTasks('', 100),
      ]);
      setHealth(unwrap(healthRes));
      const taskPayload = unwrap(tasksRes);
      setTasks(taskPayload.data || taskPayload.items || []);
    } catch (err) {
      setLoadError(err.message || '数据中心状态加载失败');
      setHealth(null);
      setTasks([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const getStatusConfig = (status) => {
    const config = {
      online: { color: 'success', text: '正常', icon: <CheckCircleOutlined /> },
      degraded: { color: 'warning', text: '降级', icon: <WarningOutlined /> },
      offline: { color: 'error', text: '离线', icon: <ExclamationCircleOutlined /> },
      running: { color: 'processing', text: '运行中', icon: <ClockCircleOutlined /> },
      success: { color: 'success', text: '成功', icon: <CheckCircleOutlined /> },
      ok: { color: 'success', text: '正常', icon: <CheckCircleOutlined /> },
      slow: { color: 'warning', text: '缓慢', icon: <WarningOutlined /> },
      failed: { color: 'error', text: '失败', icon: <ExclamationCircleOutlined /> },
      pending: { color: 'default', text: '等待', icon: <ClockCircleOutlined /> },
      skipped: { color: 'default', text: '跳过', icon: <ClockCircleOutlined /> },
    };
    return config[status] || { color: 'default', text: status, icon: null };
  };

  const sources = health?.sources || [];

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <Title level={4} style={{ margin: 0, color: '#122033' }}>
          数据中心
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={loadData}>刷新</Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        {loadError && (
          <Alert
            style={{ marginBottom: 16 }}
            type="error"
            showIcon
            message="数据中心接口不可用"
            description={loadError}
          />
        )}
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'sources',
              label: (
                <Space>
                  <CloudServerOutlined />
                  <span>数据源状态</span>
                </Space>
              ),
              children: (
                sources.length > 0 ? (
                  <Row gutter={[16, 16]}>
                    {sources.map((source) => {
                      const status = getStatusConfig(source.status);
                      return (
                        <Col xs={24} md={12} lg={8} key={source.name}>
                          <Card
                            className="stock-card"
                            bodyStyle={{ padding: 16 }}
                          >
                            <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                              <Space direction="vertical" size={0}>
                                <Text strong style={{ color: '#122033', fontSize: 16 }}>
                                  {source.name}
                                </Text>
                                <Text type="secondary">实时探测</Text>
                              </Space>
                              <Badge
                                status={status.color}
                                text={
                                  <Text style={{ color: status.color === 'success' ? '#52c41a' : status.color === 'warning' ? '#faad14' : '#ff4d4f' }}>
                                    {status.text}
                                  </Text>
                                }
                              />
                            </Space>
                            <div style={{ marginTop: 12 }}>
                              <Descriptions size="small" column={1}>
                                <Descriptions.Item label="响应延迟">
                                  {source.latency || '-'}
                                </Descriptions.Item>
                              </Descriptions>
                            </div>
                          </Card>
                        </Col>
                      );
                    })}
                  </Row>
                ) : (
                  <Card className="stock-card">
                    <Empty description="暂无数据源健康信息" />
                  </Card>
                )
              ),
            },
            {
              key: 'tasks',
              label: (
                <Space>
                  <ClockCircleOutlined />
                  <span>采集任务</span>
                </Space>
              ),
              children: (
                <Card className="stock-card" bodyStyle={{ padding: 0 }}>
                  <Table
                    className="stock-table"
                    loading={loading}
                    dataSource={tasks}
                    rowKey="task_id"
                    pagination={{ pageSize: 10 }}
                    columns={[
                      {
                        title: '任务类型',
                        dataIndex: 'task_type',
                        key: 'task_type',
                        render: (type) => <Text strong style={{ color: '#122033' }}>{taskTypeLabels[type] || type || '-'}</Text>,
                      },
                      {
                        title: '目标日期',
                        dataIndex: 'target_date',
                        key: 'target_date',
                        width: 100,
                        render: (date) => <Tag>{date || '-'}</Tag>,
                      },
                      {
                        title: '状态',
                        dataIndex: 'status',
                        key: 'status',
                        width: 100,
                        render: (status) => {
                          const config = getStatusConfig(status);
                          return (
                            <Badge status={config.color} text={<Text>{config.text}</Text>} />
                          );
                        },
                      },
                      {
                        title: '成功/总数',
                        key: 'count',
                        width: 120,
                        render: (_, record) => <Text>{record.success_count || 0}/{record.total_count || 0}</Text>,
                      },
                      {
                        title: '开始时间',
                        dataIndex: 'started_at',
                        key: 'started_at',
                        width: 160,
                        render: (time) => <Text type="secondary">{formatTime(time)}</Text>,
                      },
                      {
                        title: '结束时间',
                        dataIndex: 'finished_at',
                        key: 'finished_at',
                        width: 160,
                        render: (time) => <Text type="secondary">{formatTime(time)}</Text>,
                      },
                    ]}
                  />
                </Card>
              ),
            },
            {
              key: 'stats',
              label: (
                <Space>
                  <DatabaseOutlined />
                  <span>数据统计</span>
                </Space>
              ),
              children: (
                <Row gutter={[16, 16]}>
                  <Col span={6}>
                    <Card className="stock-card">
                      <Statistic
                        title="服务状态"
                        value={health?.status || 'unknown'}
                        valueStyle={{ color: '#122033' }}
                      />
                    </Card>
                  </Col>
                  <Col span={6}>
                    <Card className="stock-card">
                      <Statistic
                        title="今日采集"
                        value={health?.today_count || 0}
                        suffix="条"
                        valueStyle={{ color: '#122033' }}
                      />
                    </Card>
                  </Col>
                  <Col span={6}>
                    <Card className="stock-card">
                      <Statistic
                        title="今日错误"
                        value={health?.error_count || 0}
                        suffix="条"
                        valueStyle={{ color: '#122033' }}
                      />
                    </Card>
                  </Col>
                  <Col span={6}>
                    <Card className="stock-card">
                      <Statistic
                        title="最近成功采集"
                        value={formatTime(health?.last_collect)}
                        valueStyle={{ color: '#122033' }}
                      />
                    </Card>
                  </Col>
                </Row>
              ),
            },
          ]}
        />
      </div>
    </div>
  );
};

export default DataCenterPage;

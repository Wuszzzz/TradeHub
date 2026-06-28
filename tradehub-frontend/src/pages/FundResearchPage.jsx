import { useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  Progress,
  Row,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  ExperimentOutlined,
  FilterOutlined,
  LinkOutlined,
  ReloadOutlined,
  SearchOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import { fundResearchAPI } from '../api';

const { Text, Title } = Typography;

const unwrapItems = (payload) => payload?.data?.items || payload?.items || [];
const pct = (value) => value === null || value === undefined || value === 0 ? '-' : `${Number(value).toFixed(2)}%`;
const num = (value, digits = 2) => value === null || value === undefined || value === 0 ? '-' : Number(value).toFixed(digits);

const fundColumns = [
  { title: '代码', dataIndex: 'code', width: 96, fixed: 'left' },
  {
    title: '基金',
    dataIndex: 'name',
    render: (name, row) => (
      <Space direction="vertical" size={0}>
        <Text strong>{name || '-'}</Text>
        <Text type="secondary">{row.type || '未知类型'}</Text>
      </Space>
    ),
  },
  { title: '4433', dataIndex: 'is_4433', width: 82, render: (v) => <Tag color={v ? 'green' : 'default'}>{v ? '通过' : '未过'}</Tag> },
  { title: '3月排名', render: (_, row) => pct(row.performance?.month_3?.rank_ratio) },
  { title: '6月排名', render: (_, row) => pct(row.performance?.month_6?.rank_ratio) },
  { title: '1年排名', render: (_, row) => pct(row.performance?.year_1?.rank_ratio) },
  { title: '3年排名', render: (_, row) => pct(row.performance?.year_3?.rank_ratio) },
  { title: '5年排名', render: (_, row) => pct(row.performance?.year_5?.rank_ratio) },
  { title: '规模', render: (_, row) => `${num(row.net_assets_scale_yi)} 亿` },
  { title: '经理', render: (_, row) => row.manager?.name || '-' },
  { title: '夏普', render: (_, row) => num(row.sharp?.avg_135) },
  { title: '回撤', render: (_, row) => pct(row.max_retracement?.avg_135) },
];

const FundResearchPage = () => {
  const [activeTab, setActiveTab] = useState('screen');
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [meta, setMeta] = useState({});
  const [error, setError] = useState('');
  const [checkItems, setCheckItems] = useState([]);
  const [similarityItems, setSimilarityItems] = useState([]);
  const [byStockItems, setByStockItems] = useState([]);
  const [managerItems, setManagerItems] = useState([]);
  const [sectorItems, setSectorItems] = useState([]);
  const [tagItems, setTagItems] = useState([]);
  const [syncInfo, setSyncInfo] = useState(null);

  const run = async (fn, setter, successText) => {
    setLoading(true);
    setError('');
    try {
      const res = await fn();
      const data = res.data?.data || {};
      setter(unwrapItems(data));
      setMeta(data.meta || { count: data.count });
      if (successText) message.success(successText);
    } catch (err) {
      const msg = err.response?.data?.error || err.message || '投研服务请求失败';
      setError(msg);
      message.error(msg);
    } finally {
      setLoading(false);
    }
  };

  const load4433 = () => run(
    () => fundResearchAPI.screen4433({ limit: 600, enrich: 40, require_five_year: 1, sort: 'year_1' }),
    setItems,
    '4433 筛选完成',
  );

  const loadStrict = (values = {}) => run(
    () => fundResearchAPI.filter({
      limit: 700,
      enrich: 120,
      result_limit: 100,
      min_scale: values.minScale ?? 2,
      max_scale: values.maxScale ?? 50,
      min_manager_years: values.minManagerYears ?? 5,
      min_estab_years: values.minEstabYears ?? 5,
      max_135_avg_stddev: values.maxStddev ?? 25,
      min_135_avg_sharp: values.minSharp ?? 1,
      max_135_avg_retr: values.maxRetr ?? 25,
      require_five_year: 1,
      sort: 'sharp',
    }),
    setItems,
    '严选筛选完成',
  );

  const checkFunds = (values) => {
    const codes = (values.codes || '').split(/[\s,，]+/).filter(Boolean);
    return run(() => fundResearchAPI.check(codes), setCheckItems, '基金检测完成');
  };

  const calcSimilarity = (values) => {
    const codes = (values.codes || '').split(/[\s,，]+/).filter(Boolean);
    return run(() => fundResearchAPI.similarity(codes), setSimilarityItems, '相似度计算完成');
  };

  const queryByStock = (values) => run(
    () => fundResearchAPI.byStock(values.keywords),
    setByStockItems,
    '股票选基查询完成',
  );

  const queryManagers = (values = {}) => run(
    () => fundResearchAPI.managers({
      limit: 300,
      min_working_years: values.minWorkingYears ?? 8,
      min_yieldse: values.minYieldse ?? 15,
      max_current_fund_count: values.maxCurrentFundCount ?? 10,
      min_scale: values.minScale ?? 60,
    }),
    setManagerItems,
    '基金经理筛选完成',
  );

  const querySectorsAndTags = async (values) => {
    const codes = (values.codes || '').split(/[\s,，]+/).filter(Boolean);
    if (!codes.length) return;
    setLoading(true);
    setError('');
    try {
      const [sectorRes, tagRes, statusRes] = await Promise.all([
        fundResearchAPI.relatedSectors(codes, true),
        fundResearchAPI.recommendTags(codes),
        fundResearchAPI.syncStatus(),
      ]);
      setSectorItems(unwrapItems(sectorRes.data?.data || {}));
      setTagItems(unwrapItems(tagRes.data?.data || {}));
      setSyncInfo(statusRes.data?.data || null);
      message.success('板块和标签查询完成');
    } catch (err) {
      const msg = err.response?.data?.error || err.message || '板块标签查询失败';
      setError(msg);
      message.error(msg);
    } finally {
      setLoading(false);
    }
  };

  const syncSeedMap = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fundResearchAPI.syncSectorMap([], true);
      message.success(`已同步 ${res.data?.data?.synced || 0} 条内置板块映射`);
      const statusRes = await fundResearchAPI.syncStatus();
      setSyncInfo(statusRes.data?.data || null);
    } catch (err) {
      const msg = err.response?.data?.error || err.message || '同步板块映射失败';
      setError(msg);
      message.error(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fund-research-page">
      <div className="fund-data-center-header">
        <Space direction="vertical" size={2}>
          <Title level={4} style={{ margin: 0 }}>基金投研工具</Title>
          <Text type="secondary">Go 服务复刻 investool 基金工作流：4433、严选、检测、相似度、股票选基、经理筛选</Text>
        </Space>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load4433}>运行 4433</Button>
      </div>

      {error && <Alert type="error" showIcon message="Go 投研服务不可用" description={error} style={{ marginBottom: 16 }} />}

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} lg={6}><Card><Statistic title="当前结果" value={items.length || checkItems.length || similarityItems.length || byStockItems.length || managerItems.length || sectorItems.length} prefix={<ExperimentOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="数据源" value="EastMoney" prefix={<SearchOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="主体语言" value="Go" prefix={<LinkOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="上游方法" value="investool" prefix={<FilterOutlined />} /></Card></Col>
      </Row>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'screen',
            label: '4433 / 严选',
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Card>
                  <Space wrap>
                    <Button type="primary" icon={<ExperimentOutlined />} loading={loading} onClick={load4433}>4433 筛选</Button>
                    <Button icon={<FilterOutlined />} loading={loading} onClick={() => loadStrict()}>默认严选</Button>
                  </Space>
                  <Divider />
                  <Form layout="inline" onFinish={loadStrict}>
                    <Form.Item name="minScale" label="最小规模"><InputNumber min={0} placeholder="2" /></Form.Item>
                    <Form.Item name="maxScale" label="最大规模"><InputNumber min={0} placeholder="50" /></Form.Item>
                    <Form.Item name="minManagerYears" label="经理任期"><InputNumber min={0} placeholder="5" /></Form.Item>
                    <Form.Item name="minSharp" label="夏普"><InputNumber min={0} step={0.1} placeholder="1" /></Form.Item>
                    <Form.Item><Button htmlType="submit" loading={loading}>自定义严选</Button></Form.Item>
                  </Form>
                </Card>
                <Table rowKey="code" loading={loading} columns={fundColumns} dataSource={items} scroll={{ x: 1200 }} pagination={{ pageSize: 20 }} />
              </Space>
            ),
          },
          {
            key: 'check',
            label: '基金检测',
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Card>
                  <Form layout="inline" onFinish={checkFunds}>
                    <Form.Item name="codes" rules={[{ required: true, message: '请输入基金代码' }]}>
                      <Input.TextArea rows={2} placeholder="基金代码，支持空格/逗号分隔，如 260104 163406" style={{ width: 420 }} />
                    </Form.Item>
                    <Form.Item><Button type="primary" loading={loading} htmlType="submit">检测</Button></Form.Item>
                  </Form>
                </Card>
                <Row gutter={[16, 16]}>
                  {checkItems.map((fund) => (
                    <Col xs={24} lg={12} key={fund.code}>
                      <Card title={`${fund.code} ${fund.name || ''}`}>
                        <Space direction="vertical" style={{ width: '100%' }}>
                          {(fund.diagnostics || []).map((item) => (
                            <div key={item.key} style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
                              <Text>{item.label}</Text>
                              <Space>
                                <Text type="secondary">{item.value}</Text>
                                <Tag color={item.passed ? 'green' : 'red'}>{item.passed ? '通过' : '未过'}</Tag>
                              </Space>
                            </div>
                          ))}
                        </Space>
                      </Card>
                    </Col>
                  ))}
                </Row>
              </Space>
            ),
          },
          {
            key: 'similarity',
            label: '持仓相似度',
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Card>
                  <Form layout="inline" onFinish={calcSimilarity}>
                    <Form.Item name="codes" rules={[{ required: true, message: '请输入至少两只基金代码' }]}>
                      <Input placeholder="260104, 163406, 110011" style={{ width: 360 }} />
                    </Form.Item>
                    <Form.Item><Button type="primary" loading={loading} htmlType="submit">计算</Button></Form.Item>
                  </Form>
                </Card>
                <Table
                  rowKey={(row) => row.fund?.code}
                  dataSource={similarityItems}
                  loading={loading}
                  columns={[
                    { title: '基金', render: (_, row) => `${row.fund?.code || ''} ${row.fund?.name || ''}` },
                    { title: '相似度', render: (_, row) => <Progress percent={Number(row.similarity_value * 100).toFixed(1)} size="small" /> },
                    { title: '共同持仓', render: (_, row) => (row.same_stocks || []).map((name) => <Tag key={name}>{name}</Tag>) },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: 'by-stock',
            label: '股票选基',
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Card>
                  <Form layout="inline" onFinish={queryByStock}>
                    <Form.Item name="keywords" rules={[{ required: true, message: '请输入股票名称或代码' }]}>
                      <Input placeholder="贵州茅台 宁德时代" style={{ width: 360 }} />
                    </Form.Item>
                    <Form.Item><Button type="primary" loading={loading} htmlType="submit">查询持有基金</Button></Form.Item>
                  </Form>
                </Card>
                <Table
                  rowKey="code"
                  dataSource={byStockItems}
                  loading={loading}
                  locale={{ emptyText: <Empty description="暂无命中基金" /> }}
                  columns={[
                    { title: '基金代码', dataIndex: 'code' },
                    { title: '基金名称', dataIndex: 'name' },
                    { title: '类型', dataIndex: 'type' },
                    { title: '匹配股票数', dataIndex: 'matched_stock_count' },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: 'sectors',
            label: '板块标签',
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Card>
                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    <Alert
                      type="info"
                      showIcon
                      message="吸收 real-time-fund 的工程经验"
                      description="采用基金代码到关联板块、板块到东财 secid、批量行情和标签推荐的链路；TradeHub 用 Go 服务和 PostgreSQL 承接，不引入 Supabase/localStorage 作为主存储。"
                    />
                    <Form layout="inline" onFinish={querySectorsAndTags}>
                      <Form.Item name="codes" rules={[{ required: true, message: '请输入基金代码' }]}>
                        <Input placeholder="260104, 000055, 161725" style={{ width: 380 }} />
                      </Form.Item>
                      <Form.Item><Button type="primary" loading={loading} htmlType="submit">查询板块与标签</Button></Form.Item>
                      <Form.Item><Button loading={loading} disabled={syncInfo && !syncInfo.db_available} onClick={syncSeedMap}>同步内置映射到库</Button></Form.Item>
                    </Form>
                    {syncInfo && (
                      <Space wrap>
                        <Tag color="blue">模式：{syncInfo.mode}</Tag>
                        <Tag>内置基金映射：{syncInfo.seed_related_count}</Tag>
                        <Tag>内置 secid：{syncInfo.seed_secid_count}</Tag>
                        <Tag color={syncInfo.db_available ? 'green' : 'orange'}>{syncInfo.db_available ? 'PostgreSQL 可用' : '仅内置种子'}</Tag>
                      </Space>
                    )}
                  </Space>
                </Card>
                <Table
                  rowKey="fund_code"
                  title={() => '关联板块与实时涨跌'}
                  dataSource={sectorItems}
                  loading={loading}
                  columns={[
                    { title: '基金代码', dataIndex: 'fund_code', width: 100 },
                    { title: '关联板块', dataIndex: 'sector' },
                    { title: 'SecID', dataIndex: 'secid' },
                    { title: '行情名称', render: (_, row) => row.quote?.name || '-' },
                    {
                      title: '板块涨跌',
                      render: (_, row) => {
                        const value = row.quote?.change_pct;
                        if (value === null || value === undefined) return '-';
                        return <Text type={value >= 0 ? 'danger' : 'success'}>{pct(value)}</Text>;
                      },
                    },
                    { title: '来源', dataIndex: 'source', render: (v) => <Tag color={v === 'db' ? 'green' : 'blue'}>{v}</Tag> },
                  ]}
                />
                <Table
                  rowKey={(row) => `${row.fund_code}-${row.id}`}
                  title={() => '推荐标签'}
                  dataSource={tagItems}
                  loading={loading}
                  columns={[
                    { title: '基金代码', dataIndex: 'fund_code', width: 100 },
                    { title: '标签', dataIndex: 'name', render: (name, row) => <Tag color={row.theme === 'sector' ? 'blue' : 'geekblue'}>{name}</Tag> },
                    { title: '主题', dataIndex: 'theme' },
                    { title: '原因', dataIndex: 'reason' },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: 'managers',
            label: '基金经理',
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Card>
                  <Form layout="inline" onFinish={queryManagers}>
                    <Form.Item name="minWorkingYears" label="从业年限"><InputNumber min={0} placeholder="8" /></Form.Item>
                    <Form.Item name="minYieldse" label="年化回报"><InputNumber min={0} placeholder="15" /></Form.Item>
                    <Form.Item name="maxCurrentFundCount" label="最多基金数"><InputNumber min={1} placeholder="10" /></Form.Item>
                    <Form.Item name="minScale" label="管理规模"><InputNumber min={0} placeholder="60" /></Form.Item>
                    <Form.Item><Button type="primary" icon={<TeamOutlined />} loading={loading} htmlType="submit">筛选经理</Button></Form.Item>
                  </Form>
                </Card>
                <Table
                  rowKey={(row) => row.manager?.id || row.manager?.name}
                  dataSource={managerItems}
                  loading={loading}
                  columns={[
                    { title: '经理', render: (_, row) => row.manager?.name || '-' },
                    { title: '公司', dataIndex: 'company' },
                    { title: '从业年限', render: (_, row) => num((row.manager?.working_days || 0) / 365, 1) },
                    { title: '年化回报', render: (_, row) => pct(row.manager?.years_avg_repay) },
                    { title: '管理规模', render: (_, row) => `${num(row.manager?.scale_yi)} 亿` },
                    { title: '现任基金数', render: (_, row) => row.manager?.current_fund_count || '-' },
                  ]}
                />
              </Space>
            ),
          },
        ]}
      />

      {meta?.universe_count && (
        <Alert
          type="info"
          showIcon
          style={{ marginTop: 16 }}
          message={`本次扫描基金样本 ${meta.universe_count} 只，5 年排名要求：${meta.require_five_year ? '开启' : '关闭'}`}
        />
      )}
    </div>
  );
};

export default FundResearchPage;

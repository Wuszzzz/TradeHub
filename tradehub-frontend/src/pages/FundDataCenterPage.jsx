import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  message,
  Row,
  Segmented,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import {
  BankOutlined,
  DatabaseOutlined,
  FieldTimeOutlined,
  ReloadOutlined,
  TeamOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import { fundsAPI, sourceAPI } from '../api';

const { Text, Title } = Typography;

const unwrapList = (payload) => payload?.results || payload?.data?.results || payload || [];
const formatDate = (value) => value ? String(value).slice(0, 10) : '-';
const formatPercent = (value) => value === null || value === undefined || value === '' ? '-' : `${Number(value).toFixed(2)}%`;
const profitColor = (value) => Number(value) >= 0 ? '#d9363e' : '#389e0d';

const FundDataCenterPage = () => {
  const [activeTab, setActiveTab] = useState('snapshots');
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState('');
  const [form] = Form.useForm();
  const [companies, setCompanies] = useState([]);
  const [managers, setManagers] = useState([]);
  const [snapshots, setSnapshots] = useState([]);
  const [allocations, setAllocations] = useState([]);
  const [sectorMarkets, setSectorMarkets] = useState([]);
  const [sectorMode, setSectorMode] = useState('latest');
  const [sectorBoard, setSectorBoard] = useState('industry');
  const [sectorSyncError, setSectorSyncError] = useState('');
  const [ranks, setRanks] = useState([]);
  const [dailyFacts, setDailyFacts] = useState([]);
  const [syncSource, setSyncSource] = useState('tencent_fund');

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [companyRes, managerRes, snapshotRes, allocationRes, sectorMarketRes, rankRes, factRes] = await Promise.all([
        fundsAPI.companies({ page_size: 50 }),
        fundsAPI.managers({ page_size: 50 }),
        fundsAPI.holdingSnapshots({ page_size: 50 }),
        fundsAPI.allocationSnapshots({ page_size: 100 }),
        fundsAPI.sectorMarketSnapshots({ board_code: sectorBoard, latest: 1, close_only: sectorMode === 'close' ? 1 : undefined, page_size: 300 }),
        fundsAPI.performanceRanks({ page_size: 100 }),
        fundsAPI.dailyFacts({ page_size: 50 }),
      ]);
      setCompanies(unwrapList(companyRes.data));
      setManagers(unwrapList(managerRes.data));
      setSnapshots(unwrapList(snapshotRes.data));
      setAllocations(unwrapList(allocationRes.data));
      setSectorMarkets(unwrapList(sectorMarketRes.data));
      setRanks(unwrapList(rankRes.data));
      setDailyFacts(unwrapList(factRes.data));
    } catch (err) {
      setError(err.response?.data?.error || err.message || '基金数据中心加载失败');
    } finally {
      setLoading(false);
    }
  }, [sectorBoard, sectorMode]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const stats = useMemo(() => ({
    companies: companies.length,
    managers: managers.length,
    snapshots: snapshots.length,
    allocations: allocations.length,
    sectorMarkets: sectorMarkets.length,
    ranks: ranks.length,
    dailyFacts: dailyFacts.length,
  }), [companies, managers, snapshots, allocations, sectorMarkets, ranks, dailyFacts]);

  const syncFund = async () => {
    const { fundCode } = await form.validateFields();
    setSyncing(true);
    try {
      if (syncSource === 'xiaobeiyangji') {
        const statusRes = await sourceAPI.getStatus('xiaobeiyangji');
        if (!statusRes.data?.logged_in) {
          message.warning('请先在设置页登录小倍养基，再同步基金画像');
          return;
        }
      }
      await fundsAPI.syncProfile(fundCode, syncSource);
      const res = await fundsAPI.syncHoldings(fundCode, syncSource);
      if (res.data?.success === false) {
        message.warning(syncSource === 'xiaobeiyangji'
          ? '小倍养基画像已同步，但暂未命中可入库重仓'
          : '基金资料已同步，但暂未命中披露持仓');
      } else {
        message.success(syncSource === 'xiaobeiyangji'
          ? '已同步小倍养基资料和持仓快照'
          : '已同步基金资料和腾讯持仓快照');
      }
      await loadData();
    } catch (err) {
      message.error(err.response?.data?.error || err.message || '同步失败');
    } finally {
      setSyncing(false);
    }
  };

  const syncFundNav = async () => {
    const { fundCode } = await form.validateFields();
    setSyncing(true);
    try {
      const now = new Date();
      const start = new Date();
      start.setFullYear(now.getFullYear() - 1);
      const response = await fundsAPI.syncNavHistory(
        [fundCode],
        start.toISOString().slice(0, 10),
        now.toISOString().slice(0, 10),
      );
      const result = response.data?.[fundCode];
      if (result?.success === false) {
        message.error(result.error || '净值历史同步失败');
        return;
      }
      message.success(`已同步净值历史并写入日事实：${result?.count || 0} 条`);
      await loadData();
    } catch (err) {
      message.error(err.response?.data?.error || err.message || '净值历史同步失败');
    } finally {
      setSyncing(false);
    }
  };

  const backfillFacts = async () => {
    setSyncing(true);
    try {
      const res = await fundsAPI.backfillDailyFacts({ limit: 200 });
      message.success(`已回填 ${res.data?.count || 0} 条日事实`);
      await loadData();
    } catch (err) {
      message.error(err.response?.data?.error || err.message || '回填失败');
    } finally {
      setSyncing(false);
    }
  };

  const syncSectorMarkets = async (isCloseSnapshot = false) => {
    setSyncing(true);
    setSectorSyncError('');
    try {
      const res = await fundsAPI.syncSectorMarketSnapshots({
        board_codes: [sectorBoard],
        sort_type: 'f3',
        direct: 'down',
        count: 500,
        is_close_snapshot: isCloseSnapshot,
      });
      const syncErrors = res.data?.errors || [];
      if (syncErrors.length) {
        setSectorSyncError(syncErrors.map((item) => `${item.board_code}: ${item.error}`).join('\n'));
        message.warning('板块实时源暂不可用，已保留最近一次落库快照');
      } else {
        message.success(`已同步板块行情 ${res.data?.count || 0} 条`);
      }
      await loadData();
    } catch (err) {
      message.error(err.response?.data?.error || err.message || '板块行情同步失败');
    } finally {
      setSyncing(false);
    }
  };

  return (
    <div className="fund-data-center-page">
      <div className="fund-data-center-header">
        <Space direction="vertical" size={2}>
          <Title level={4} style={{ margin: 0 }}>基金数据中心</Title>
          <Text type="secondary">披露持仓、板块配置、业绩排行、净值和估值事实入库</Text>
        </Space>
        <Space wrap>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={loadData}>刷新</Button>
          <Button icon={<FieldTimeOutlined />} loading={syncing} onClick={backfillFacts}>回填日事实</Button>
        </Space>
      </div>

      {error && <Alert type="error" showIcon message="接口不可用" description={error} style={{ marginBottom: 16 }} />}

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} lg={6}><Card><Statistic title="基金板块配置" value={stats.allocations} prefix={<BankOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="市场板块快照" value={stats.sectorMarkets} prefix={<ReloadOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="排行快照" value={stats.ranks} prefix={<TeamOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="持仓/日事实" value={stats.snapshots + stats.dailyFacts} prefix={<DatabaseOutlined />} /></Card></Col>
      </Row>

      <Card style={{ marginBottom: 16 }}>
        <Form form={form} layout="inline" onFinish={syncFund}>
          <Form.Item name="fundCode" rules={[{ required: true, message: '请输入基金代码' }]}>
            <Input placeholder="基金代码，如 159915" style={{ width: 220 }} />
          </Form.Item>
          <Form.Item>
            <Segmented
              value={syncSource}
              onChange={setSyncSource}
              options={[
                { label: '腾讯', value: 'tencent_fund' },
                { label: '小倍养基', value: 'xiaobeiyangji' },
              ]}
            />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<UploadOutlined />} loading={syncing}>
              {syncSource === 'xiaobeiyangji' ? '同步小倍资料和持仓' : '同步腾讯资料和持仓'}
            </Button>
          </Form.Item>
          <Form.Item>
            <Button icon={<FieldTimeOutlined />} loading={syncing} onClick={syncFundNav}>同步净值和日事实</Button>
          </Form.Item>
        </Form>
      </Card>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'sector-market',
            label: '板块管理',
            children: (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Space wrap>
                  <Button type="primary" icon={<ReloadOutlined />} loading={syncing} onClick={() => syncSectorMarkets(false)}>拉取盘中板块</Button>
                  <Button icon={<FieldTimeOutlined />} loading={syncing} onClick={() => syncSectorMarkets(true)}>写入盘后最后一包</Button>
                  <Button onClick={() => setSectorBoard('industry')} type={sectorBoard === 'industry' ? 'primary' : 'default'}>行业板块</Button>
                  <Button onClick={() => setSectorBoard('concept')} type={sectorBoard === 'concept' ? 'primary' : 'default'}>概念板块</Button>
                  <Button onClick={() => setSectorMode('latest')} type={sectorMode === 'latest' ? 'primary' : 'default'}>最新快照</Button>
                  <Button onClick={() => setSectorMode('close')} type={sectorMode === 'close' ? 'primary' : 'default'}>盘后快照</Button>
                </Space>
                {sectorSyncError && <Alert type="warning" showIcon message="实时板块源暂不可用" description={<pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{sectorSyncError}</pre>} />}
                <Table loading={loading} dataSource={sectorMarkets} rowKey="id" pagination={{ pageSize: 20 }} scroll={{ x: 'max-content' }} columns={[
                  { title: '快照时间', dataIndex: 'snapshot_time', width: 180, render: (v) => v ? new Date(v).toLocaleString('zh-CN', { hour12: false }) : '-' },
                  { title: '交易日', dataIndex: 'trade_date', width: 120, render: formatDate },
                  { title: '类型', dataIndex: 'board_code', width: 110, render: (v) => <Tag color={v === 'industry' ? 'blue' : 'purple'}>{v === 'industry' ? '行业' : v === 'concept' ? '概念' : v}</Tag> },
                  { title: '板块代码', dataIndex: 'sector_code', width: 120 },
                  { title: '板块名称', dataIndex: 'sector_name', width: 180 },
                  { title: '涨跌幅', dataIndex: 'change_percent', width: 110, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                  { title: '最新价', dataIndex: 'latest_price', width: 110 },
                  { title: '领涨股', dataIndex: 'leading_stock_name', width: 150, render: (v, r) => v ? `${v}${r.leading_stock_code ? ` (${r.leading_stock_code})` : ''}` : '-' },
                  { title: '5日', dataIndex: 'five_day_change', width: 100, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                  { title: '20日', dataIndex: 'twenty_day_change', width: 100, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                  { title: '60日', dataIndex: 'sixty_day_change', width: 100, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                  { title: '今年', dataIndex: 'ytd_change', width: 100, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                  { title: '类型', dataIndex: 'is_close_snapshot', width: 110, render: (v) => v ? <Tag color="green">盘后最后一包</Tag> : <Tag>盘中</Tag> },
                ]} />
              </Space>
            ),
          },
          {
            key: 'snapshots',
            label: '披露持仓',
            children: (
              <Table
                loading={loading}
                dataSource={snapshots}
                rowKey="id"
                pagination={{ pageSize: 10 }}
                expandable={{ expandedRowRender: (record) => (
                  record.items?.length ? (
                    <Table
                      size="small"
                      pagination={false}
                      dataSource={record.items}
                      rowKey="id"
                      columns={[
                        { title: '代码', dataIndex: 'asset_code', width: 120 },
                        { title: '名称', dataIndex: 'asset_name' },
                        { title: '权重', dataIndex: 'weight', width: 100, render: formatPercent },
                        { title: '涨跌', dataIndex: 'change_percent', width: 100, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                      ]}
                    />
                  ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无明细" />
                ) }}
                columns={[
                  { title: '基金', key: 'fund', render: (_, r) => <Text strong>{r.fund_code} {r.fund_name}</Text> },
                  { title: '报告日', dataIndex: 'report_date', width: 120, render: formatDate },
                  { title: '目标代码', dataIndex: 'target_code', width: 120, render: (v) => <Tag>{v || '-'}</Tag> },
                  { title: '来源', dataIndex: 'source', width: 130, render: (v) => <Tag color="blue">{v}</Tag> },
                  { title: '持仓数', dataIndex: 'item_count', width: 100 },
                  { title: '合计权重', dataIndex: 'total_weight', width: 120, render: formatPercent },
                ]}
              />
            ),
          },
          {
            key: 'ranks',
            label: '排行快照',
            children: (
              <Table loading={loading} dataSource={ranks} rowKey="id" pagination={{ pageSize: 12 }} columns={[
                { title: '基金', key: 'fund', render: (_, r) => <Text strong>{r.fund_code} {r.fund_name}</Text> },
                { title: '日期', dataIndex: 'rank_date', width: 120, render: formatDate },
                { title: '周期', dataIndex: 'period', width: 130, render: (v) => <Tag color="blue">{{
                  day: '日',
                  week: '周',
                  month: '月',
                  quarter: '季',
                  half_year: '半年',
                  this_year: '今年',
                  year: '1年',
                  three_year: '3年',
                  since_inception: '成立以来',
                }[v] || v}</Tag> },
                { title: '涨幅', dataIndex: 'growth', width: 110, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                { title: '排名', key: 'rank', width: 130, render: (_, r) => r.rank && r.total ? `${r.rank}/${r.total}` : '-' },
                { title: '四分位', dataIndex: 'quartile', width: 110, render: (v) => ({ 1: '优秀', 2: '良好', 3: '一般', 4: '不佳' }[v] || '-') },
                { title: '来源', dataIndex: 'source', width: 130, render: (v) => <Tag>{v}</Tag> },
              ]} />
            ),
          },
          {
            key: 'allocations',
            label: '板块配置',
            children: (
              <Table loading={loading} dataSource={allocations} rowKey="id" pagination={{ pageSize: 12 }} columns={[
                { title: '基金', key: 'fund', render: (_, r) => <Text strong>{r.fund_code} {r.fund_name}</Text> },
                { title: '报告日', dataIndex: 'report_date', width: 120, render: formatDate },
                { title: '类型', dataIndex: 'allocation_type', width: 110, render: (v) => <Tag color={v === 'industry' ? 'purple' : 'cyan'}>{v === 'industry' ? '行业' : '资产'}</Tag> },
                { title: '板块', dataIndex: 'name' },
                { title: '占比', dataIndex: 'ratio', width: 110, render: formatPercent },
                { title: '来源', dataIndex: 'source', width: 130, render: (v) => <Tag>{v}</Tag> },
              ]} />
            ),
          },
          {
            key: 'companies',
            label: '基金公司',
            children: (
              <Table loading={loading} dataSource={companies} rowKey="id" pagination={{ pageSize: 12 }} columns={[
                { title: '公司代码', dataIndex: 'company_code', width: 160 },
                { title: '公司名称', dataIndex: 'company_name' },
                { title: '简称', dataIndex: 'short_name', width: 160 },
                { title: '基金数', dataIndex: 'fund_count', width: 100 },
                { title: '经理数', dataIndex: 'manager_count', width: 100 },
                { title: '来源', dataIndex: 'source', width: 120, render: (v) => <Tag>{v}</Tag> },
              ]} />
            ),
          },
          {
            key: 'managers',
            label: '基金经理',
            children: (
              <Table loading={loading} dataSource={managers} rowKey="id" pagination={{ pageSize: 12 }} columns={[
                { title: '经理代码', dataIndex: 'manager_code', width: 160 },
                { title: '姓名', dataIndex: 'manager_name', width: 160 },
                { title: '公司', dataIndex: 'company_name' },
                { title: '管理基金数', dataIndex: 'fund_count', width: 120 },
                { title: '来源', dataIndex: 'source', width: 120, render: (v) => <Tag>{v}</Tag> },
              ]} />
            ),
          },
          {
            key: 'facts',
            label: '日事实',
            children: (
              <Table loading={loading} dataSource={dailyFacts} rowKey="id" pagination={{ pageSize: 12 }} columns={[
                { title: '基金', key: 'fund', render: (_, r) => <Text strong>{r.fund_code} {r.fund_name}</Text> },
                { title: '交易日', dataIndex: 'trade_date', width: 120, render: formatDate },
                { title: '单位净值', dataIndex: 'unit_nav', width: 110 },
                { title: '日涨跌', dataIndex: 'daily_growth', width: 110, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                { title: '估算净值', dataIndex: 'estimate_nav', width: 110 },
                { title: '估算涨跌', dataIndex: 'estimate_growth', width: 110, render: (v) => <Text style={{ color: profitColor(v) }}>{formatPercent(v)}</Text> },
                { title: '误差率', dataIndex: 'estimate_error_rate', width: 110, render: formatPercent },
                { title: '来源', dataIndex: 'source', width: 130, render: (v) => <Tag>{v}</Tag> },
              ]} />
            ),
          },
        ]}
      />
    </div>
  );
};

export default FundDataCenterPage;

import { useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Progress,
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
import { ReloadOutlined } from '@ant-design/icons';
import ReactECharts from 'echarts-for-react';
import { accountsAPI, positionsAPI } from '../api';

const { Text, Paragraph } = Typography;

const pct = (value) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return `${number >= 0 ? '+' : ''}${number.toFixed(2)}%`;
};

const money = (value) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return `¥${number.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
};

const scoreColor = (score) => {
  if (score >= 85) return '#16a34a';
  if (score >= 70) return '#2563eb';
  if (score >= 60) return '#d97706';
  return '#dc2626';
};

const breakdownColumns = [
  { title: '分类', dataIndex: 'name', key: 'name' },
  { title: '占比', dataIndex: 'weight', key: 'weight', render: (v) => `${Number(v || 0).toFixed(2)}%` },
  { title: '市值', dataIndex: 'market_value', key: 'market_value', render: money },
  { title: '数量', dataIndex: 'count', key: 'count' },
  { title: '今年以来', dataIndex: 'return_ytd', key: 'return_ytd', render: pct },
];

function pieOption(title, data = []) {
  return {
    title: { text: title, left: 'center', top: 8, textStyle: { fontSize: 14, fontWeight: 700 } },
    tooltip: { trigger: 'item', formatter: '{b}: {d}%' },
    series: [{
      type: 'pie',
      radius: ['42%', '70%'],
      top: 24,
      avoidLabelOverlap: true,
      data: data.map((item) => ({ name: item.name, value: Number(item.weight || 0) })),
    }],
  };
}

function radarOption(dimensions = []) {
  return {
    radar: {
      radius: '68%',
      indicator: dimensions.map((item) => ({ name: item.label, max: item.max_score })),
    },
    series: [{
      type: 'radar',
      areaStyle: { opacity: 0.18 },
      data: [{ value: dimensions.map((item) => item.score), name: '体检得分' }],
    }],
  };
}

function PortfolioHealthPage() {
  const [accounts, setAccounts] = useState([]);
  const [selectedAccount, setSelectedAccount] = useState();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);

  const loadAccounts = async () => {
    try {
      const { data: accountData } = await accountsAPI.list();
      const childAccounts = [];
      (accountData || []).forEach((account) => {
        if (account.parent) {
          childAccounts.push(account);
        }
        (account.children || []).forEach((child) => childAccounts.push(child));
      });
      setAccounts(childAccounts);
      if (!selectedAccount && childAccounts.length > 0) {
        setSelectedAccount(childAccounts[0].id);
      }
    } catch (error) {
      message.error(error.response?.data?.detail || '账户加载失败');
    }
  };

  const loadHealth = async (accountId = selectedAccount) => {
    setLoading(true);
    try {
      const { data: health } = await positionsAPI.quality(accountId);
      setData(health);
    } catch (error) {
      setData(null);
      message.error(error.response?.data?.error || '持仓体检失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAccounts();
  }, []);

  useEffect(() => {
    if (selectedAccount) loadHealth(selectedAccount);
  }, [selectedAccount]);

  const overview = data?.overview || {};
  const dimensionColumns = [
    { title: '维度', dataIndex: 'label', key: 'label' },
    {
      title: '得分',
      key: 'score',
      render: (_, row) => (
        <Space>
          <Progress percent={Math.round((row.score / row.max_score) * 100)} size="small" showInfo={false} style={{ width: 120 }} />
          <Text>{row.score}/{row.max_score}</Text>
        </Space>
      ),
    },
    { title: '原因', dataIndex: 'reasons', key: 'reasons', render: (items = []) => items.join('；') || '-' },
  ];

  const positionColumns = [
    { title: '基金', key: 'fund', fixed: 'left', render: (_, row) => <Space direction="vertical" size={0}><Text strong>{row.fund_name}</Text><Text type="secondary">{row.fund_code} · {row.fund_type}</Text></Space> },
    { title: '角色', dataIndex: 'role', key: 'role', render: (v) => <Tag color={v === '核心仓' ? 'blue' : v === '防守仓' ? 'green' : v === '进攻仓' ? 'red' : 'default'}>{v}</Tag> },
    { title: '风险', dataIndex: 'risk_level', key: 'risk_level', render: (v) => <Tag color={v === '高' ? 'red' : v === '低' ? 'green' : 'gold'}>{v}</Tag> },
    { title: '仓位', dataIndex: 'weight', key: 'weight', render: (v) => `${Number(v || 0).toFixed(2)}%` },
    { title: '市值', dataIndex: 'market_value', key: 'market_value', render: money },
    { title: '盈亏', dataIndex: 'pnl', key: 'pnl', render: (v, row) => <Text style={{ color: Number(v) >= 0 ? '#dc2626' : '#16a34a' }}>{money(v)} ({pct(row.pnl_rate)})</Text> },
    { title: '30天', dataIndex: 'return_30d', key: 'return_30d', render: pct },
    { title: '3个月', dataIndex: 'return_3m', key: 'return_3m', render: pct },
    { title: '今年以来', dataIndex: 'return_ytd', key: 'return_ytd', render: pct },
    { title: '标签', dataIndex: 'problem_tags', key: 'problem_tags', render: (items = []) => <Space wrap>{items.map((item) => <Tag key={item}>{item}</Tag>)}</Space> },
  ];

  return (
    <div className="portfolio-health-page">
      <Card
        title="持仓现状体检"
        extra={(
          <Space wrap>
            <Select
              placeholder="选择账户"
              style={{ minWidth: 220 }}
              value={selectedAccount}
              onChange={setSelectedAccount}
              options={accounts.map((account) => ({ label: account.name, value: account.id }))}
            />
            <Button icon={<ReloadOutlined />} onClick={() => loadHealth()} loading={loading}>重新体检</Button>
          </Space>
        )}
      >
        <Paragraph type="secondary">
          体检计算由 Go 投研服务完成，AI 只解释结构化结果，不参与原始打分。
        </Paragraph>
        {!selectedAccount && <Alert type="info" showIcon message="请先创建子账户并导入基金持仓，再进行体检。" />}
      </Card>

      {loading ? (
        <Card style={{ marginTop: 16 }}><Spin /></Card>
      ) : !data ? (
        <Card style={{ marginTop: 16 }}><Empty description="暂无体检结果" /></Card>
      ) : (
        <>
          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={6}>
              <Card>
                <Statistic
                  title="组合评分"
                  value={data.score}
                  suffix={`/ ${data.level}`}
                  valueStyle={{ color: scoreColor(data.score), fontSize: 38 }}
                />
                <Space wrap style={{ marginTop: 12 }}>{(data.tags || []).map((tag) => <Tag key={tag}>{tag}</Tag>)}</Space>
              </Card>
            </Col>
            <Col xs={12} lg={4}><Card><Statistic title="总市值" value={overview.total_market_value} formatter={money} /></Card></Col>
            <Col xs={12} lg={4}><Card><Statistic title="总盈亏" value={overview.total_pnl} formatter={money} valueStyle={{ color: Number(overview.total_pnl) >= 0 ? '#dc2626' : '#16a34a' }} /></Card></Col>
            <Col xs={12} lg={4}><Card><Statistic title="收益率" value={overview.total_pnl_rate} formatter={pct} /></Card></Col>
            <Col xs={12} lg={3}><Card><Statistic title="最大仓位" value={overview.max_position_weight} suffix="%" precision={2} /></Card></Col>
            <Col xs={12} lg={3}><Card><Statistic title="前三仓位" value={overview.top3_position_weight} suffix="%" precision={2} /></Card></Col>
          </Row>

          <Card style={{ marginTop: 16 }}>
            <Alert type="info" showIcon message={data.summary} />
          </Card>

          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={10}>
              <Card title="五维评分">
                <ReactECharts option={radarOption(data.dimensions)} style={{ height: 320 }} />
              </Card>
            </Col>
            <Col xs={24} lg={14}>
              <Card title="扣分原因">
                <Table rowKey="key" pagination={false} size="small" columns={dimensionColumns} dataSource={data.dimensions || []} />
              </Card>
            </Col>
          </Row>

          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={8}><Card title="基金类型拆解"><ReactECharts option={pieOption('基金类型', data.fund_type_breakdown)} style={{ height: 300 }} /></Card></Col>
            <Col xs={24} lg={8}><Card title="底层资产拆解"><ReactECharts option={pieOption('底层资产', data.asset_breakdown)} style={{ height: 300 }} /></Card></Col>
            <Col xs={24} lg={8}><Card title="行业/主题暴露"><ReactECharts option={pieOption('行业主题', data.industry_breakdown)} style={{ height: 300 }} /></Card></Col>
          </Row>

          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={8}><Card title="基金类型明细"><Table rowKey="name" size="small" pagination={false} columns={breakdownColumns} dataSource={data.fund_type_breakdown || []} /></Card></Col>
            <Col xs={24} lg={8}><Card title="底层资产明细"><Table rowKey="name" size="small" pagination={false} columns={breakdownColumns} dataSource={data.asset_breakdown || []} /></Card></Col>
            <Col xs={24} lg={8}><Card title="行业主题明细"><Table rowKey="name" size="small" pagination={false} columns={breakdownColumns} dataSource={(data.industry_breakdown || []).slice(0, 8)} /></Card></Col>
          </Row>

          <Card title="单只基金角色诊断" style={{ marginTop: 16 }}>
            <Table rowKey="fund_code" size="small" scroll={{ x: 1200 }} columns={positionColumns} dataSource={data.position_analysis || []} />
          </Card>

          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={12}>
              <Card title="主要发现">
                <Space direction="vertical" style={{ width: '100%' }}>
                  {(data.findings || []).map((item) => (
                    <Alert key={`${item.section}-${item.title}`} type={item.level === 'success' ? 'success' : item.level === 'warning' ? 'warning' : 'info'} showIcon message={item.title} description={item.detail} />
                  ))}
                </Space>
              </Card>
            </Col>
            <Col xs={24} lg={12}>
              <Card title="建议动作">
                <Space direction="vertical" style={{ width: '100%' }}>
                  {(data.suggestions || []).map((item) => <Alert key={item} type="info" showIcon message={item} />)}
                </Space>
              </Card>
            </Col>
          </Row>

          <Card title="底层重仓重合度" style={{ marginTop: 16 }}>
            <Alert
              type="info"
              showIcon
              message={`可识别重复暴露约 ${Number(data.overlap?.estimated_duplication || 0).toFixed(2)}%，最大底层持仓 ${data.overlap?.max_holding_name || '-'} 暴露 ${Number(data.overlap?.max_holding_exposure || 0).toFixed(2)}%。`}
              style={{ marginBottom: 12 }}
            />
            <Table
              rowKey={(row) => row.code || row.name}
              size="small"
              pagination={false}
              columns={[
                { title: '底层资产', dataIndex: 'name', key: 'name' },
                { title: '组合暴露', dataIndex: 'exposure', key: 'exposure', render: (v) => `${Number(v || 0).toFixed(2)}%` },
                { title: '涉及基金', dataIndex: 'fund_codes', key: 'fund_codes', render: (codes = []) => codes.join(', ') },
              ]}
              dataSource={data.overlap?.repeated_holdings || []}
            />
          </Card>

          <Card title="核心提示词" style={{ marginTop: 16 }}>
            <Paragraph copyable style={{ whiteSpace: 'pre-wrap' }}>{data.ai_prompt}</Paragraph>
          </Card>
        </>
      )}
    </div>
  );
}

export default PortfolioHealthPage;

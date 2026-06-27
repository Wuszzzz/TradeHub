import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Col, DatePicker, Empty, Input, Row, Segmented, Space, Statistic, Table, Tag, Typography, message } from 'antd';
import { FieldTimeOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { fundsAPI } from '../api';

const { Text, Title } = Typography;

const unwrapList = (payload) => payload?.results || payload?.data?.results || payload || [];
const formatDate = (value) => value ? String(value).slice(0, 10) : '-';
const formatDateTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-';
const formatPercent = (value) => value === null || value === undefined || value === '' ? '-' : `${Number(value).toFixed(2)}%`;
const profitColor = (value) => Number(value) >= 0 ? '#d9363e' : '#389e0d';
const sourceLabel = (value) => ({
  tencent_market: '腾讯行情',
  akshare: 'AkShare',
  stock_api_akshare: 'Stock API / AkShare',
})[value] || value || '-';

const FundSectorsPage = () => {
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState('');
  const [syncError, setSyncError] = useState('');
  const [rows, setRows] = useState([]);
  const [boardCode, setBoardCode] = useState('industry');
  const [snapshotMode, setSnapshotMode] = useState('latest');
  const [tradeDate, setTradeDate] = useState(null);
  const [keyword, setKeyword] = useState('');

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await fundsAPI.sectorMarketSnapshots({
        board_code: boardCode,
        latest: tradeDate ? 0 : 1,
        close_only: snapshotMode === 'close' ? 1 : undefined,
        trade_date: tradeDate ? tradeDate.format('YYYY-MM-DD') : undefined,
        keyword: keyword || undefined,
        page_size: 500,
      });
      setRows(unwrapList(response.data));
    } catch (err) {
      setError(err.response?.data?.error || err.message || '板块行情加载失败');
    } finally {
      setLoading(false);
    }
  }, [boardCode, keyword, snapshotMode, tradeDate]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const stats = useMemo(() => {
    const upCount = rows.filter((item) => Number(item.change_percent) > 0).length;
    const downCount = rows.filter((item) => Number(item.change_percent) < 0).length;
    const latestTime = rows[0]?.snapshot_time;
    return { total: rows.length, upCount, downCount, latestTime };
  }, [rows]);

  const syncSectorMarkets = async (isCloseSnapshot = false) => {
    setSyncing(true);
    setSyncError('');
    try {
      const response = await fundsAPI.syncSectorMarketSnapshots({
        board_codes: [boardCode],
        count: 500,
        is_close_snapshot: isCloseSnapshot,
      });
      const errors = response.data?.errors || [];
      if (errors.length > 0) {
        setSyncError(errors.map((item) => `${item.board_code}: ${item.error}`).join('\n'));
        message.warning('板块实时源暂不可用，已保留最近一次落库快照');
      } else {
        message.success(`已同步板块行情 ${response.data?.count || 0} 条`);
      }
      await loadData();
    } catch (err) {
      message.error(err.response?.data?.error || err.message || '板块行情同步失败');
    } finally {
      setSyncing(false);
    }
  };

  const columns = [
    { title: '排名', width: 72, fixed: 'left', render: (_, __, index) => index + 1 },
    { title: '板块', dataIndex: 'sector_name', width: 180, fixed: 'left', render: (value, record) => <Space direction="vertical" size={0}><Text strong>{value}</Text><Text type="secondary">{record.sector_code}</Text></Space> },
    { title: '类型', dataIndex: 'board_code', width: 90, render: (value) => <Tag color={value === 'industry' ? 'blue' : 'purple'}>{value === 'industry' ? '行业' : value === 'concept' ? '概念' : value}</Tag> },
    { title: '涨跌幅', dataIndex: 'change_percent', width: 110, sorter: (a, b) => Number(a.change_percent || 0) - Number(b.change_percent || 0), render: (value) => <Text strong style={{ color: profitColor(value) }}>{formatPercent(value)}</Text> },
    { title: '最新价', dataIndex: 'latest_price', width: 110 },
    { title: '涨跌额', dataIndex: 'change_amount', width: 110, render: (value) => value ?? '-' },
    { title: '5日', dataIndex: 'five_day_change', width: 100, render: (value) => <Text style={{ color: profitColor(value) }}>{formatPercent(value)}</Text> },
    { title: '20日', dataIndex: 'twenty_day_change', width: 100, render: (value) => <Text style={{ color: profitColor(value) }}>{formatPercent(value)}</Text> },
    { title: '60日', dataIndex: 'sixty_day_change', width: 100, render: (value) => <Text style={{ color: profitColor(value) }}>{formatPercent(value)}</Text> },
    { title: '今年', dataIndex: 'ytd_change', width: 100, render: (value) => <Text style={{ color: profitColor(value) }}>{formatPercent(value)}</Text> },
    { title: '领涨股', dataIndex: 'leading_stock_name', width: 160, render: (value, record) => value ? `${value}${record.leading_stock_code ? ` (${record.leading_stock_code})` : ''}` : '-' },
    { title: '成交额', dataIndex: 'amount', width: 120, render: (value) => value || '-' },
    { title: '信源', dataIndex: 'source', width: 110, render: (value) => <Tag>{sourceLabel(value)}</Tag> },
    { title: '快照', dataIndex: 'snapshot_time', width: 180, render: formatDateTime },
    { title: '交易日', dataIndex: 'trade_date', width: 120, render: formatDate },
    { title: '快照类型', dataIndex: 'is_close_snapshot', width: 120, render: (value) => value ? <Tag color="green">盘后最后一包</Tag> : <Tag>盘中</Tag> },
  ];

  return (
    <div className="fund-sectors-page">
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>基金板块</Title>
          <Text type="secondary">市场行业、概念板块涨跌快照，盘中动态拉取，盘后保留最后一包</Text>
        </div>

        {error && <Alert type="error" showIcon message={error} />}
        {syncError && <Alert type="warning" showIcon message="实时板块源暂不可用" description={<pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{syncError}</pre>} />}

        <Row gutter={[12, 12]}>
          <Col xs={12} lg={6}><Card><Statistic title="板块数量" value={stats.total} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="上涨" value={stats.upCount} valueStyle={{ color: '#d9363e' }} suffix="个" /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="下跌" value={stats.downCount} valueStyle={{ color: '#389e0d' }} suffix="个" /></Card></Col>
          <Col xs={24} lg={6}><Card><Statistic title="最新快照" value={formatDateTime(stats.latestTime)} /></Card></Col>
        </Row>

        <Card>
          <Space wrap style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space wrap>
              <Segmented
                value={boardCode}
                onChange={setBoardCode}
                options={[
                  { label: '行业板块', value: 'industry' },
                  { label: '概念板块', value: 'concept' },
                ]}
              />
              <Segmented
                value={snapshotMode}
                onChange={setSnapshotMode}
                options={[
                  { label: '最新快照', value: 'latest' },
                  { label: '盘后', value: 'close' },
                ]}
              />
              <DatePicker
                allowClear
                value={tradeDate}
                onChange={setTradeDate}
                placeholder="交易日"
                disabledDate={(date) => date && date.isAfter(dayjs(), 'day')}
              />
              <Input
                allowClear
                prefix={<SearchOutlined />}
                placeholder="搜索板块/领涨股"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                style={{ width: 220 }}
              />
            </Space>
            <Space wrap>
              <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>刷新</Button>
              <Button type="primary" icon={<ReloadOutlined />} onClick={() => syncSectorMarkets(false)} loading={syncing}>拉取盘中板块</Button>
              <Button icon={<FieldTimeOutlined />} onClick={() => syncSectorMarkets(true)} loading={syncing}>写入盘后最后一包</Button>
            </Space>
          </Space>
        </Card>

        <Card>
          <Table
            loading={loading}
            dataSource={rows}
            rowKey="id"
            columns={columns}
            pagination={{ pageSize: 30, showSizeChanger: true }}
            scroll={{ x: 1700 }}
            locale={{ emptyText: <Empty description="暂无板块行情数据" /> }}
          />
        </Card>
      </Space>
    </div>
  );
};

export default FundSectorsPage;

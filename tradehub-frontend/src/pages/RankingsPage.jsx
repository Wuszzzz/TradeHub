import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { Alert, Button, Card, Col, DatePicker, Drawer, Row, Statistic, Tabs, Table, Select, Spin, Empty, Tag, Space, Input, Typography, message } from 'antd';
import { AimOutlined, FireOutlined, FundOutlined, PlusOutlined, StarOutlined, SyncOutlined, TrophyOutlined } from '@ant-design/icons';
import { fundResearchAPI, fundsAPI, watchlistsAPI } from '../api';

const { Text } = Typography;
const STORAGE_KEY = 'tradehub.rankings.filters.v2';

const CATEGORIES = [
  { value: '', label: '全部' },
  { value: '股票', label: '股票型' },
  { value: '混合', label: '混合型' },
  { value: '债券', label: '债券型' },
  { value: '指数', label: '指数型' },
  { value: 'QDII', label: 'QDII' },
  { value: '黄金', label: '黄金' },
  { value: '半导体', label: '半导体' },
];

const PERIODS = [
  { value: 'day', label: '日' },
  { value: 'week', label: '周' },
  { value: 'month', label: '月' },
  { value: 'quarter', label: '季' },
  { value: 'half_year', label: '半年' },
  { value: 'this_year', label: '今年' },
  { value: 'year', label: '1年' },
  { value: 'three_year', label: '3年' },
  { value: 'since_inception', label: '成立以来' },
];

const SIZE_RANGES = [
  { value: '', label: '全部规模' },
  { value: '0-10', label: '10亿以下' },
  { value: '10-50', label: '10-50亿' },
  { value: '50-100', label: '50-100亿' },
  { value: '100-999999', label: '100亿以上' },
];

const pct = (value) => value === null || value === undefined || value === '' ? '-' : `${Number(value).toFixed(2)}%`;
const signedPct = (value) => value === null || value === undefined || value === '' ? '-' : `${Number(value) >= 0 ? '+' : ''}${Number(value).toFixed(2)}%`;
const metricColor = (value, reverse = false) => {
  const num = Number(value);
  if (!Number.isFinite(num) || num === 0) return undefined;
  const good = reverse ? num < 0 : num > 0;
  return good ? '#3f8600' : '#cf1322';
};
const evaluationColor = (level) => ({ 优选: 'green', 观察: 'gold', 谨慎: 'red' }[level] || 'default');

const RankingsPage = () => {
  const navigate = useNavigate();
  const cachedFilters = (() => {
    try {
      return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
    } catch {
      return {};
    }
  })();
  const [type, setType] = useState(cachedFilters.type || 'gain');
  const [period, setPeriod] = useState(cachedFilters.period || 'day');
  const [rankDate, setRankDate] = useState(cachedFilters.rankDate || '');
  const [category, setCategory] = useState(cachedFilters.category || '');
  const [sizeRange, setSizeRange] = useState(cachedFilters.sizeRange || '');
  const [industry, setIndustry] = useState(cachedFilters.industry || '');
  const [data, setData] = useState([]);
  const [sectorMap, setSectorMap] = useState({});
  const [researchMap, setResearchMap] = useState({});
  const [meta, setMeta] = useState({});
  const [watchlists, setWatchlists] = useState([]);
  const [selectedWatchlistId, setSelectedWatchlistId] = useState(cachedFilters.watchlistId || '');
  const [detailRecord, setDetailRecord] = useState(null);
  const [analysisOpen, setAnalysisOpen] = useState(false);
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const [analysisData, setAnalysisData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [watchlistLoading, setWatchlistLoading] = useState(false);
  const requestIdRef = useRef(0);
  const isStoredRanking = type === 'performance' || type === 'gain' || type === 'popular';
  const displayData = isStoredRanking ? data.filter((item) => !item.period || item.period === period) : data;
  const enrichedData = displayData.map((item) => ({ ...item, research: researchMap[item.fund_code] || {} }));

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      type,
      period,
      rankDate,
      category,
      sizeRange,
      industry,
      watchlistId: selectedWatchlistId,
    }));
  }, [type, period, rankDate, category, sizeRange, industry, selectedWatchlistId]);

  useEffect(() => {
    watchlistsAPI.list()
      .then(({ data: list }) => {
        setWatchlists(list || []);
        if (!selectedWatchlistId && list?.[0]?.id) {
          setSelectedWatchlistId(list[0].id);
        }
      })
      .catch(() => setWatchlists([]));
  }, []);

  const loadData = async () => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setResearchMap({});
    try {
      if (isStoredRanking) {
        const [minSize, maxSize] = sizeRange ? sizeRange.split('-') : [];
        const [{ data: res }, { data: sectorRes }] = await Promise.all([
          fundsAPI.performanceRanks({
            rank_type: type === 'popular' ? 'popular' : 'performance',
            period,
            rank_date: period === 'day' && rankDate ? rankDate : undefined,
            category,
            min_size: minSize || undefined,
            max_size: maxSize || undefined,
            industry: industry || undefined,
            page_size: 100,
          }),
          fundsAPI.allocationSnapshots({ allocation_type: 'industry', page_size: 500 }),
        ]);
        const rows = res.results || res || [];
        const sectors = sectorRes.results || sectorRes || [];
        const nextMap = {};
        sectors.forEach((item) => {
          if (!nextMap[item.fund_code]) nextMap[item.fund_code] = [];
          if (nextMap[item.fund_code].length < 3) {
            nextMap[item.fund_code].push(item);
          }
        });
        if (requestId !== requestIdRef.current) return;
        setSectorMap(nextMap);
        setData(rows);
        setMeta({
          rankDate: rows[0]?.rank_date || '',
          period: rows[0]?.period || period,
          source: rows[0]?.source || '',
        });
        enrichResearch(rows, requestId);
        return;
      }
      const [minSize, maxSize] = sizeRange ? sizeRange.split('-') : [];
      const [{ data: res }, { data: sectorRes }] = await Promise.all([
        fundsAPI.rankings({
          type,
          category,
          min_size: minSize || undefined,
          max_size: maxSize || undefined,
          industry: industry || undefined,
          page: 1,
        }),
        fundsAPI.allocationSnapshots({ allocation_type: 'industry', page_size: 500 }),
      ]);
      const sectors = sectorRes.results || sectorRes || [];
      const nextMap = {};
      sectors.forEach((item) => {
        if (!nextMap[item.fund_code]) nextMap[item.fund_code] = [];
        if (nextMap[item.fund_code].length < 3) {
          nextMap[item.fund_code].push(item);
        }
      });
      if (requestId !== requestIdRef.current) return;
      setSectorMap(nextMap);
      setData(res.results || []);
      setMeta({});
      enrichResearch(res.results || [], requestId);
    } catch {
      if (requestId !== requestIdRef.current) return;
      setData([]);
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  };

  const enrichResearch = async (rows, requestId) => {
    const codes = rows.slice(0, 80).map((item) => item.fund_code).filter(Boolean);
    if (!codes.length) {
      setResearchMap({});
      return;
    }
    try {
      const [sectorRes, tagRes] = await Promise.all([
        fundResearchAPI.relatedSectors(codes, true),
        fundResearchAPI.recommendTags(codes),
      ]);
      if (requestId !== requestIdRef.current) return;
      const nextMap = {};
      (sectorRes.data?.data?.items || []).forEach((item) => {
        nextMap[item.fund_code] = { ...(nextMap[item.fund_code] || {}), relatedSector: item };
      });
      (tagRes.data?.data?.items || []).forEach((item) => {
        if (!nextMap[item.fund_code]) nextMap[item.fund_code] = {};
        if (!nextMap[item.fund_code].tags) nextMap[item.fund_code].tags = [];
        nextMap[item.fund_code].tags.push(item);
      });
      setResearchMap(nextMap);
    } catch {
      if (requestId === requestIdRef.current) {
        setResearchMap({});
      }
    }
  };

  useEffect(() => { loadData(); }, [type, period, rankDate, category, sizeRange, industry]);

  const loadSectorAnalysis = async () => {
    const fundCodes = displayData.slice(0, 80).map((item) => item.fund_code).filter(Boolean);
    if (fundCodes.length === 0) {
      setAnalysisData([]);
      setAnalysisOpen(true);
      return;
    }
    setAnalysisLoading(true);
    setAnalysisOpen(true);
    try {
      const { data: res } = await fundsAPI.sectorRotationAnalysis({
        fund_codes: fundCodes.join(','),
        trade_date: period === 'day' && rankDate ? rankDate : undefined,
        board_code: 'industry',
        close_only: 1,
        page_size: fundCodes.length,
      });
      setAnalysisData(res.items || []);
    } catch (err) {
      setAnalysisData([]);
      message.error(err.response?.data?.error || err.message || '高级分析加载失败');
    } finally {
      setAnalysisLoading(false);
    }
  };

  const addToWatchlist = async (record) => {
    if (!selectedWatchlistId) {
      message.warning('请先选择或创建自选列表');
      navigate('/dashboard/watchlists');
      return;
    }
    setWatchlistLoading(true);
    try {
      await watchlistsAPI.addItem(selectedWatchlistId, record.fund_code);
      message.success(`已加入自选：${record.fund_name || record.fund_code}`);
    } catch (err) {
      const msg = err.response?.data?.error || err.message || '加入自选失败';
      if (msg.includes('已在自选')) {
        message.info(msg);
      } else {
        message.error(msg);
      }
    } finally {
      setWatchlistLoading(false);
    }
  };

  const syncGoEvaluations = async () => {
    setSyncLoading(true);
    try {
      const codes = displayData.slice(0, 120).map((item) => item.fund_code).filter(Boolean);
      const { data: res } = await fundResearchAPI.syncEvaluations({
        codes,
        limit: codes.length || 500,
        window_days: 370,
      });
      message.success(`Go 评估计算完成：${res.data?.synced || 0} 只基金`);
      await loadData();
    } catch (err) {
      message.error(err.response?.data?.error || err.message || 'Go 评估同步失败');
    } finally {
      setSyncLoading(false);
    }
  };

  const columns = [
    { title: '排名', key: 'rank_index', width: 70, render: (_, record, i) => record.rank || i + 1 },
    {
      title: '基金代码', dataIndex: 'fund_code', key: 'code', width: 100,
      render: (code) => <a onClick={() => navigate(`/dashboard/funds/${code}`)}>{code}</a>,
    },
    {
      title: '基金名称',
      dataIndex: 'fund_name',
      key: 'name',
      ellipsis: true,
      render: (name, record) => <a onClick={() => setDetailRecord(record)}>{name}</a>,
    },
    { title: '类型', dataIndex: 'fund_type', key: 'type', width: 80, responsive: ['md'] },
    ...((type === 'performance' || type === 'gain') ? [
      {
        title: '周期',
        dataIndex: 'period',
        key: 'period',
        width: 100,
        render: v => <Tag color="blue">{PERIODS.find(item => item.value === v)?.label || v}</Tag>,
      },
      {
        title: '涨幅',
        dataIndex: 'growth',
        key: 'growth',
        width: 100,
        render: v => <span style={{ color: parseFloat(v) >= 0 ? '#cf1322' : '#3f8600' }}>{v != null ? `${parseFloat(v) >= 0 ? '+' : ''}${parseFloat(v).toFixed(2)}%` : '-'}</span>,
      },
    ] : []),
    {
      title: '基金规模',
      key: 'fund_size',
      width: 110,
      render: (_, record) => record.fund_size_text || (record.fund_size ? `${Number(record.fund_size).toFixed(2)}亿` : '-'),
    },
    {
      title: '所属板块',
      key: 'sectors',
      width: 260,
      render: (_, record) => {
        const sectors = sectorMap[record.fund_code] || [];
        const related = record.research?.relatedSector;
        const tags = record.research?.tags || [];
        return sectors.length > 0 ? (
          <Space wrap size={[4, 4]}>
            {related?.sector && (
              <Tag color="geekblue">
                {related.sector}
                {related.quote?.change_pct !== undefined ? ` ${signedPct(related.quote.change_pct)}` : ''}
              </Tag>
            )}
            {sectors.map((item) => <Tag key={item.name}>{item.name} {Number(item.ratio).toFixed(1)}%</Tag>)}
            {tags.slice(0, 2).map((item) => <Tag key={item.id} color={item.theme === 'sector' ? 'blue' : 'purple'}>{item.name}</Tag>)}
          </Space>
        ) : related?.sector ? (
          <Space wrap size={[4, 4]}>
            <Tag color="geekblue">{related.sector}{related.quote?.change_pct !== undefined ? ` ${signedPct(related.quote.change_pct)}` : ''}</Tag>
            {tags.slice(0, 2).map((item) => <Tag key={item.id} color={item.theme === 'sector' ? 'blue' : 'purple'}>{item.name}</Tag>)}
          </Space>
        ) : '-';
      },
    },
    {
      title: '回撤',
      dataIndex: 'max_drawdown',
      key: 'max_drawdown',
      width: 90,
      render: v => <span style={{ color: metricColor(v, true) }}>{pct(v)}</span>,
      sorter: (a, b) => Math.abs(Number(a.max_drawdown || 0)) - Math.abs(Number(b.max_drawdown || 0)),
    },
    {
      title: '波动',
      dataIndex: 'volatility',
      key: 'volatility',
      width: 90,
      render: v => pct(v),
      responsive: ['lg'],
      sorter: (a, b) => Number(a.volatility || 0) - Number(b.volatility || 0),
    },
    {
      title: '夏普',
      dataIndex: 'sharpe',
      key: 'sharpe',
      width: 90,
      render: v => <span style={{ color: metricColor(v) }}>{v ?? '-'}</span>,
      sorter: (a, b) => Number(a.sharpe || -999) - Number(b.sharpe || -999),
    },
    {
      title: '评估',
      key: 'evaluation',
      width: 120,
      render: (_, record) => {
        const level = record.evaluation?.level;
        return level ? <Tag color={evaluationColor(level)}>{level} {record.evaluation?.score || 0}</Tag> : '-';
      },
    },
    ...(isStoredRanking ? [
      { title: '排行日', dataIndex: 'rank_date', key: 'rank_date', width: 120 },
    ] : []),
    ...(type === 'performance' ? [
      {
        title: '同类排名',
        key: 'rank',
        width: 120,
        render: (_, record) => record.rank && record.total ? `${record.rank}/${record.total}` : '-',
      },
      {
        title: '四分位',
        dataIndex: 'quartile',
        key: 'quartile',
        width: 100,
        render: v => ({ 1: '优秀', 2: '良好', 3: '一般', 4: '不佳' }[v] || '-'),
      },
    ] : []),
    ...(type === 'popular' ? [{
      title: '人气分', dataIndex: 'growth', key: 'popular', width: 90, render: v => v != null ? Number(v).toFixed(0) : '-',
    }] : []),
    ...(type === 'accuracy' ? [{
      title: '平均误差', dataIndex: 'avg_error', key: 'error', width: 100,
      render: v => v ? `${(parseFloat(v) * 100).toFixed(2)}%` : '-',
    }] : []),
    {
      title: '操作',
      key: 'actions',
      fixed: 'right',
      width: 130,
      render: (_, record) => (
        <Space size={4}>
          <Button size="small" icon={<StarOutlined />} loading={watchlistLoading} onClick={() => addToWatchlist(record)}>自选</Button>
        </Space>
      ),
    },
  ];

  return (
    <Card title="基金评估与选择" extra={
      <Space wrap>
        {isStoredRanking && (
          <Select
            value={period}
            style={{ width: 120 }}
            onChange={setPeriod}
            options={PERIODS}
          />
        )}
        {isStoredRanking && period === 'day' && (
          <DatePicker
            value={rankDate ? dayjs(rankDate) : null}
            placeholder="排行日"
            onChange={(_, value) => setRankDate(value || '')}
          />
        )}
        <Select value={category || undefined} placeholder="基金类型" allowClear style={{ width: 120 }}
          onChange={v => { setCategory(v || ''); }}
          options={CATEGORIES.map(c => ({ value: c.value, label: c.label }))}
        />
        <Select
          value={sizeRange}
          style={{ width: 130 }}
          onChange={setSizeRange}
          options={SIZE_RANGES}
        />
        <Input.Search
          placeholder="行业/板块"
          allowClear
          style={{ width: 180 }}
          onSearch={setIndustry}
          onChange={(event) => {
            if (!event.target.value) {
              setIndustry('');
            }
          }}
        />
        <Select
          value={selectedWatchlistId || undefined}
          placeholder="加入到自选"
          style={{ width: 160 }}
          onChange={setSelectedWatchlistId}
          options={watchlists.map(item => ({ value: item.id, label: item.name }))}
        />
        <Button icon={<SyncOutlined />} loading={syncLoading} onClick={syncGoEvaluations}>同步Go评估</Button>
        <Button icon={<FundOutlined />} onClick={loadSectorAnalysis}>高级分析</Button>
      </Space>
    }>
      {isStoredRanking && (
        <Alert
          showIcon
          type="info"
          style={{ marginBottom: 12 }}
          message="排行已合并基金评估与选择"
          description={`数据优先来自 PostgreSQL 落库排行和净值历史，回撤/波动/夏普/评估分由 Go 投研服务批量计算写回数据库；板块和标签也由 Go 投研服务增强。当前周期：${PERIODS.find(item => item.value === period)?.label || period} / 排行日：${meta.rankDate || rankDate || '最新落库'} / 首行周期：${displayData[0]?.period || data[0]?.period || '-'}`}
        />
      )}
      <Row gutter={[12, 12]} style={{ marginBottom: 12 }}>
        <Col xs={12} md={6}><Card size="small"><Statistic title="候选基金" value={enrichedData.length} /></Card></Col>
        <Col xs={12} md={6}><Card size="small"><Statistic title="优选" value={enrichedData.filter(item => item.evaluation?.level === '优选').length} /></Card></Col>
        <Col xs={12} md={6}><Card size="small"><Statistic title="有夏普率" value={enrichedData.filter(item => item.sharpe !== null && item.sharpe !== undefined).length} /></Card></Col>
        <Col xs={12} md={6}><Card size="small"><Statistic title="已匹配板块" value={enrichedData.filter(item => item.research?.relatedSector?.sector).length} /></Card></Col>
      </Row>
      <Tabs activeKey={type} onChange={setType}
        items={[
          { key: 'performance', label: <span><TrophyOutlined />落库排行</span> },
          { key: 'gain', label: <span><TrophyOutlined />涨幅榜</span> },
          { key: 'popular', label: <span><FireOutlined />人气榜</span> },
          { key: 'accuracy', label: <span><AimOutlined />准度榜</span> },
        ]}
      />
      <Spin spinning={loading}>
        {enrichedData.length > 0 ? (
          <Table
            key={`${type}-${period}-${category}-${sizeRange}-${industry}`}
            dataSource={enrichedData}
            columns={columns}
            rowKey="fund_code"
            pagination={false}
            size="small"
            scroll={{ x: 'max-content' }}
          />
        ) : (
          !loading && <Empty description={type === 'popular' ? '暂无系统人气样本，请先添加持仓或自选基金' : '暂无数据'} />
        )}
      </Spin>
      <Drawer
        title={detailRecord ? `${detailRecord.fund_name} (${detailRecord.fund_code})` : '基金详情'}
        open={!!detailRecord}
        onClose={() => setDetailRecord(null)}
        width={420}
        extra={detailRecord && (
          <Space>
            <Button icon={<PlusOutlined />} loading={watchlistLoading} onClick={() => addToWatchlist(detailRecord)}>加入自选</Button>
            <Button type="primary" onClick={() => navigate(`/dashboard/funds/${detailRecord.fund_code}`)}>查看详情</Button>
          </Space>
        )}
      >
        {detailRecord && (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Text>类型：{detailRecord.fund_type || '-'}</Text>
            <Text>规模：{detailRecord.fund_size_text || (detailRecord.fund_size ? `${Number(detailRecord.fund_size).toFixed(2)}亿` : '-')}</Text>
            <Text>排行日：{detailRecord.rank_date || '-'}</Text>
            <Text>周期：{PERIODS.find(item => item.value === detailRecord.period)?.label || detailRecord.period || '-'}</Text>
            <Text>涨幅：{detailRecord.growth != null ? `${Number(detailRecord.growth).toFixed(2)}%` : '-'}</Text>
            <Row gutter={[8, 8]}>
              <Col span={8}><Card size="small"><Statistic title="最大回撤" value={detailRecord.max_drawdown ?? '-'} suffix={detailRecord.max_drawdown ? '%' : ''} /></Card></Col>
              <Col span={8}><Card size="small"><Statistic title="波动率" value={detailRecord.volatility ?? '-'} suffix={detailRecord.volatility ? '%' : ''} /></Card></Col>
              <Col span={8}><Card size="small"><Statistic title="夏普" value={detailRecord.sharpe ?? '-'} /></Card></Col>
            </Row>
            {detailRecord.evaluation?.level && (
              <Alert
                showIcon
                type={detailRecord.evaluation.level === '优选' ? 'success' : detailRecord.evaluation.level === '观察' ? 'warning' : 'error'}
                message={`评估结论：${detailRecord.evaluation.level}（${detailRecord.evaluation.score || 0}分）`}
                description={(detailRecord.evaluation.reasons || []).join('、') || '暂无充分评估理由'}
              />
            )}
            <div>
              <Text type="secondary">所属板块</Text>
              <div style={{ marginTop: 8 }}>
                <Space wrap size={[4, 4]}>
                  {detailRecord.research?.relatedSector?.sector && (
                    <Tag color="geekblue">
                      {detailRecord.research.relatedSector.sector}
                      {detailRecord.research.relatedSector.quote?.change_pct !== undefined ? ` ${signedPct(detailRecord.research.relatedSector.quote.change_pct)}` : ''}
                    </Tag>
                  )}
                  {(sectorMap[detailRecord.fund_code] || []).map(item => <Tag key={item.name}>{item.name} {Number(item.ratio).toFixed(1)}%</Tag>)}
                  {!(detailRecord.research?.relatedSector?.sector || (sectorMap[detailRecord.fund_code] || []).length) && '-'}
                </Space>
              </div>
            </div>
            <div>
              <Text type="secondary">推荐标签</Text>
              <div style={{ marginTop: 8 }}>
                {(detailRecord.research?.tags || []).length ? (
                  <Space wrap size={[4, 4]}>
                    {detailRecord.research.tags.map(item => <Tag key={item.id} color={item.theme === 'sector' ? 'blue' : 'purple'}>{item.name}</Tag>)}
                  </Space>
                ) : '-'}
              </div>
            </div>
          </Space>
        )}
      </Drawer>
      <Drawer
        title="基金板块高级分析"
        open={analysisOpen}
        onClose={() => setAnalysisOpen(false)}
        width={680}
      >
        <Spin spinning={analysisLoading}>
          {analysisData.length > 0 ? (
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              {analysisData.map((item) => (
                <Card
                  key={item.fund_code}
                  size="small"
                  title={
                    <Space wrap>
                      <a onClick={() => navigate(`/dashboard/funds/${item.fund_code}`)}>{item.fund_name}</a>
                      <Text type="secondary">{item.fund_code}</Text>
                      <Tag color={Number(item.weighted_change_percent) >= 0 ? 'red' : 'green'}>
                        加权 {Number(item.weighted_change_percent || 0).toFixed(2)}%
                      </Tag>
                      <Tag color={item.rotation_signal === '板块走强' ? 'red' : item.rotation_signal === '板块走弱' ? 'green' : 'gold'}>
                        {item.rotation_signal}
                      </Tag>
                    </Space>
                  }
                  extra={<Button size="small" icon={<StarOutlined />} onClick={() => addToWatchlist(item)}>自选</Button>}
                >
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Text>{item.analysis_summary}</Text>
                    <Space wrap size={[6, 6]}>
                      {(item.sectors || []).map((sector) => (
                        <Tag key={`${item.fund_code}-${sector.matched_sector_code}-${sector.matched_sector_name}`}>
                          {sector.matched_sector_name} {Number(sector.allocation_ratio || 0).toFixed(1)}% / {Number(sector.change_percent || 0).toFixed(2)}%
                        </Tag>
                      ))}
                    </Space>
                  </Space>
                </Card>
              ))}
            </Space>
          ) : (
            !analysisLoading && <Empty description="暂无板块归属分析结果" />
          )}
        </Spin>
      </Drawer>
    </Card>
  );
};

export default RankingsPage;

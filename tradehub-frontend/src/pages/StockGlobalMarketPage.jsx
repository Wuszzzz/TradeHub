import { useEffect, useMemo, useState } from 'react';
import { Card, Empty, Select, Space, Spin, Table, Tag, message } from 'antd';
import { stockAPI } from '../api';

const SIDEBAR_GROUPS = [
  {
    title: '沪深股市',
    items: [
      { key: 'sh_hsj', label: '沪深京', mode: 'board', board_code: 'aStock' },
      { key: 'hy_second', label: '行业', mode: 'board', board_code: 'aStock', defaultSort: 'zdf_y' },
      { key: 'hs_cyb', label: '创业板', mode: 'board', board_code: 'cybStock' },
      { key: 'hs_kcb', label: '科创板', mode: 'board', board_code: 'kcbStock' },
    ],
  },
  {
    title: '香港股市',
    items: [
      { key: 'hk_market', label: '香港股市', mode: 'placeholder', description: '腾讯侧接口仍有限制，当前环境未拿到稳定服务端数据。' },
    ],
  },
  {
    title: '美国股市',
    items: [
      { key: 'us_zgg', label: '中概股', mode: 'us', board_type: 'cdr' },
      { key: 'us_kjg', label: '美股科技股', mode: 'us', board_type: 'tec' },
    ],
  },
  {
    title: '全球外汇',
    items: [
      { key: 'exchange', label: '所有汇率', mode: 'fx' },
    ],
  },
  {
    title: '期货',
    items: [
      { key: 'qh_global', label: '全球期货', mode: 'futures', category: 'all' },
      { key: 'qh_stockIndex', label: '全球股指', mode: 'futures', category: 'stockIndex' },
      { key: 'qh_preciousMetal', label: '贵金属', mode: 'futures', category: 'preciousMetal' },
      { key: 'qh_basicMetal', label: '基本金属', mode: 'futures', category: 'basicMetal' },
      { key: 'qh_agriculture', label: '农产品', mode: 'futures', category: 'agriculture' },
      { key: 'qh_energy', label: '能源', mode: 'futures', category: 'energy' },
      { key: 'qh_interestRate', label: '利率', mode: 'futures', category: 'interestRate' },
      { key: 'qh_exchangeRate', label: '货币', mode: 'futures', category: 'exchangeRate' },
    ],
  },
  {
    title: '全球股指',
    items: [
      { key: 'indices', label: '全部股指', mode: 'indices', list_key: 'common' },
      { key: 'indices_EU', label: '欧洲指数', mode: 'indices', list_key: 'europe' },
      { key: 'indices_AM', label: '美洲指数', mode: 'indices', list_key: 'america' },
      { key: 'indices_AS', label: '亚洲指数', mode: 'indices', list_key: 'asia' },
      { key: 'indices_OA', label: '其他指数', mode: 'indices', list_key: 'other' },
    ],
  },
];

const ALL_ITEMS = SIDEBAR_GROUPS.flatMap((group) => group.items);
const BOARD_SORT_OPTIONS = [
  { label: '按最新价', value: 'price' },
  { label: '按涨跌幅', value: 'zdf' },
  { label: '按5日涨跌幅', value: 'zdf_d5' },
  { label: '按20日涨跌幅', value: 'zdf_d20' },
  { label: '按60日涨跌幅', value: 'zdf_d60' },
  { label: '按52周涨跌幅', value: 'zdf_w52' },
  { label: '按年初至今', value: 'zdf_y' },
];
const US_SORT_OPTIONS = [
  { label: '按最新价', value: 'price' },
  { label: '按涨跌幅', value: 'zdf' },
  { label: '按涨跌额', value: 'zd' },
  { label: '按换手率', value: 'hsl' },
  { label: '按成交量', value: 'volume' },
  { label: '按总市值', value: 'zsz' },
];

function NumberCell({ value, suffix = '', withSign = false }) {
  const num = Number(value);
  if (!Number.isFinite(num)) return '--';
  const cls = num > 0 ? 'up' : num < 0 ? 'down' : '';
  const text = `${withSign && num > 0 ? '+' : ''}${num.toFixed(2)}${suffix}`;
  return <span className={cls}>{text}</span>;
}

function StockGlobalMarketPage() {
  const [activeKey, setActiveKey] = useState('sh_hsj');
  const [loading, setLoading] = useState(false);
  const [boardData, setBoardData] = useState([]);
  const [boardTotal, setBoardTotal] = useState(0);
  const [indicesData, setIndicesData] = useState({});
  const [usData, setUSData] = useState([]);
  const [usTotal, setUSTotal] = useState(0);
  const [fxData, setFXData] = useState([]);
  const [futuresData, setFuturesData] = useState({ rows: [], groups: {} });
  const [boardSortType, setBoardSortType] = useState('price');
  const [boardDirect, setBoardDirect] = useState('down');
  const [usSortType, setUSSortType] = useState('price');
  const [usDirect, setUSDirect] = useState('down');
  const [pageSize, setPageSize] = useState(20);

  const activeGroup = ALL_ITEMS.find((item) => item.key === activeKey) || ALL_ITEMS[0];

  useEffect(() => {
    if (activeGroup.mode === 'board') {
      setBoardSortType(activeGroup.defaultSort || 'price');
      setBoardDirect('down');
    }
    if (activeGroup.mode === 'us') {
      setUSSortType('price');
      setUSDirect('down');
    }
  }, [activeKey]);

  const loadData = async (group = activeGroup) => {
    setLoading(true);
    try {
      if (group.mode === 'board') {
        const { data } = await stockAPI.globalBoard({
          board_code: group.board_code,
          sort_type: boardSortType,
          direct: boardDirect,
          offset: 0,
          count: 80,
        });
        setBoardData(data.rank_list || []);
        setBoardTotal(data.total || 0);
      } else if (group.mode === 'us') {
        const { data } = await stockAPI.globalUS({
          board_type: group.board_type,
          sort_type: usSortType,
          direct: usDirect,
          offset: 0,
          count: 80,
        });
        setUSData(data.rank_list || []);
        setUSTotal(data.total || 0);
      } else if (group.mode === 'fx') {
        const { data } = await stockAPI.globalFX();
        setFXData(data.rows || []);
      } else if (group.mode === 'futures') {
        const { data } = await stockAPI.globalFutures({
          category: group.category,
        });
        setFuturesData({ rows: data.rows || [], groups: data.groups || {} });
      } else if (group.mode === 'indices') {
        const { data } = await stockAPI.globalIndices();
        setIndicesData(data || {});
      }
    } catch (error) {
      message.error(error.response?.data?.error || '加载全球行情失败');
      setBoardData([]);
      setBoardTotal(0);
      setIndicesData({});
      setUSData([]);
      setUSTotal(0);
      setFXData([]);
      setFuturesData({ rows: [], groups: {} });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData(activeGroup);
  }, [activeKey, boardSortType, boardDirect, usSortType, usDirect]);

  const toolbar = useMemo(() => {
    if (activeGroup.mode === 'board') {
      return (
        <Space wrap size={12}>
          <Select value={boardSortType} onChange={setBoardSortType} options={BOARD_SORT_OPTIONS} style={{ width: 180 }} />
          <Select
            value={boardDirect}
            onChange={setBoardDirect}
            options={[
              { label: '降序', value: 'down' },
              { label: '升序', value: 'up' },
            ]}
            style={{ width: 120 }}
          />
          <Tag color="cyan">共 {boardTotal} 个板块</Tag>
        </Space>
      );
    }
    if (activeGroup.mode === 'us') {
      return (
        <Space wrap size={12}>
          <Select value={usSortType} onChange={setUSSortType} options={US_SORT_OPTIONS} style={{ width: 180 }} />
          <Select
            value={usDirect}
            onChange={setUSDirect}
            options={[
              { label: '降序', value: 'down' },
              { label: '升序', value: 'up' },
            ]}
            style={{ width: 120 }}
          />
          <Tag color="cyan">共 {usTotal} 只</Tag>
        </Space>
      );
    }
    if (activeGroup.mode === 'futures') {
      const count = futuresData.rows.length;
      return <Tag color="cyan">当前 {count} 条</Tag>;
    }
    if (activeGroup.mode === 'fx') {
      return <Tag color="cyan">当前 {fxData.length} 条</Tag>;
    }
    if (activeGroup.mode === 'indices') {
      return <Tag color="cyan">当前 {(indicesData[activeGroup.list_key] || []).length} 条</Tag>;
    }
    return null;
  }, [activeGroup, boardSortType, boardDirect, usSortType, usDirect, boardTotal, usTotal, futuresData.rows.length, fxData.length, indicesData]);

  const boardColumns = [
    { title: '代码', dataIndex: 'code', key: 'code', width: 110 },
    { title: '名称', dataIndex: 'name', key: 'name', width: 180 },
    { title: '最新价', dataIndex: 'zxj', key: 'zxj', width: 120 },
    { title: '涨跌幅', dataIndex: 'zdf', key: 'zdf', width: 110, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '领涨股', dataIndex: 'pn', key: 'pn', width: 120 },
    { title: '5日', dataIndex: 'zdf_d5', key: 'zdf_d5', width: 100, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '20日', dataIndex: 'zdf_d20', key: 'zdf_d20', width: 100, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '60日', dataIndex: 'zdf_d60', key: 'zdf_d60', width: 100, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '52周', dataIndex: 'zdf_w52', key: 'zdf_w52', width: 110, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '年初至今', dataIndex: 'zdf_y', key: 'zdf_y', width: 120, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
  ];

  const usColumns = [
    { title: '代码', dataIndex: 'code', key: 'code', width: 110 },
    { title: '名称', dataIndex: 'name', key: 'name', width: 220 },
    { title: '最新价', dataIndex: 'zxj', key: 'zxj', width: 120 },
    { title: '涨跌幅', dataIndex: 'zdf', key: 'zdf', width: 110, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '涨跌额', dataIndex: 'zd', key: 'zd', width: 110, render: (v) => <NumberCell value={v} withSign /> },
    { title: '换手率', dataIndex: 'hsl', key: 'hsl', width: 110, render: (v) => <NumberCell value={v} suffix="%" /> },
    { title: '振幅', dataIndex: 'zf', key: 'zf', width: 100, render: (v) => <NumberCell value={v} suffix="%" /> },
    { title: '成交量', dataIndex: 'volume', key: 'volume', width: 120 },
    { title: '市盈率', dataIndex: 'pe_ttm', key: 'pe_ttm', width: 120 },
    { title: '总市值', dataIndex: 'zsz', key: 'zsz', width: 140 },
  ];

  const fxColumns = [
    { title: '汇率名称', dataIndex: 'name', key: 'name', width: 180 },
    { title: '代码', dataIndex: 'code', key: 'code', width: 120 },
    { title: '最新价', dataIndex: 'zxj', key: 'zxj', width: 120 },
    { title: '涨跌额', dataIndex: 'zd', key: 'zd', width: 120, render: (v) => <NumberCell value={v} withSign /> },
    { title: '涨跌幅', dataIndex: 'zdf', key: 'zdf', width: 110, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '最高', dataIndex: 'high', key: 'high', width: 120 },
    { title: '最低', dataIndex: 'low', key: 'low', width: 120 },
    { title: '交易日期', dataIndex: 'trade_date', key: 'trade_date', width: 120 },
  ];

  const futuresColumns = [
    { title: '代码', dataIndex: 'code', key: 'code', width: 100 },
    { title: '名称', dataIndex: 'name', key: 'name', width: 180 },
    { title: '交易所', dataIndex: 'location', key: 'location', width: 140 },
    { title: '最新价', dataIndex: 'zxj', key: 'zxj', width: 120 },
    { title: '涨跌额', dataIndex: 'zd', key: 'zd', width: 110, render: (v) => <NumberCell value={v} withSign /> },
    { title: '涨跌幅', dataIndex: 'zdf', key: 'zdf', width: 110, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '状态', dataIndex: 'state', key: 'state', width: 100, render: (v) => v === 'open' ? '交易中' : v === 'close' ? '闭市' : v || '--' },
  ];

  const indicesColumns = [
    { title: '指数名称', dataIndex: 'name', key: 'name', width: 220 },
    { title: '地区', dataIndex: 'location', key: 'location', width: 140 },
    { title: '最新价', dataIndex: 'zxj', key: 'zxj', width: 120 },
    { title: '涨跌幅', dataIndex: 'zdf', key: 'zdf', width: 100, render: (v) => <NumberCell value={v} suffix="%" withSign /> },
    { title: '交易状态', dataIndex: 'state', key: 'state', width: 110, render: (v) => v === 'close' ? '闭市' : v === 'open' ? '交易中' : v || '--' },
  ];

  const renderContent = () => {
    if (loading) {
      return <div className="stock-empty-wrap"><Spin /></div>;
    }
    if (activeGroup.mode === 'placeholder') {
      return <Empty description={activeGroup.description} />;
    }
    if (activeGroup.mode === 'board') {
      return boardData.length > 0 ? (
        <Table
          rowKey="code"
          dataSource={boardData}
          pagination={{ pageSize, showSizeChanger: true, total: boardTotal, onShowSizeChange: (_, size) => setPageSize(size) }}
          scroll={{ x: 'max-content' }}
          columns={boardColumns}
        />
      ) : <Empty description="暂无板块行情数据" />;
    }
    if (activeGroup.mode === 'us') {
      return usData.length > 0 ? (
        <Table
          rowKey="code"
          dataSource={usData}
          pagination={{ pageSize, showSizeChanger: true, total: usTotal, onShowSizeChange: (_, size) => setPageSize(size) }}
          scroll={{ x: 'max-content' }}
          columns={usColumns}
        />
      ) : <Empty description="暂无美股行情数据" />;
    }
    if (activeGroup.mode === 'fx') {
      return fxData.length > 0 ? (
        <Table rowKey="qtcode" dataSource={fxData} pagination={false} columns={fxColumns} />
      ) : <Empty description="暂无全球外汇数据" />;
    }
    if (activeGroup.mode === 'futures') {
      return futuresData.rows.length > 0 ? (
        <Table
          rowKey="qtcode"
          dataSource={futuresData.rows}
          pagination={{ pageSize, showSizeChanger: true, onShowSizeChange: (_, size) => setPageSize(size) }}
          scroll={{ x: 'max-content' }}
          columns={futuresColumns}
        />
      ) : <Empty description="暂无全球期货数据" />;
    }
    const rows = indicesData[activeGroup.list_key] || [];
    return rows.length > 0 ? (
      <Table rowKey="qtcode" dataSource={rows} pagination={false} columns={indicesColumns} />
    ) : <Empty description="暂无全球股指数据" />;
  };

  return (
    <div className="stock-page-stack">
      <section className="terminal-panel">
        <header>
          <h3>全球行情中心</h3>
        </header>
        <div className="global-market-shell">
          <aside className="global-market-sidebar">
            {SIDEBAR_GROUPS.map((group) => (
              <div key={group.title} className="global-market-group">
                <button type="button" className={group.items.some((item) => item.key === activeKey) ? 'active' : ''}>
                  {group.title}
                </button>
                {group.items.map((section) => (
                  <button
                    key={section.key}
                    type="button"
                    className={activeKey === section.key ? 'sub active' : 'sub'}
                    onClick={() => setActiveKey(section.key)}
                  >
                    {section.label}
                  </button>
                ))}
              </div>
            ))}
          </aside>

          <div className="global-market-content">
            <Card className="fund-panel" bordered={false}>
              <div className="global-market-placeholder">
                <div className="global-market-header">
                  <div>
                    <h4>{activeGroup.label}</h4>
                    <p>{activeGroup.mode === 'placeholder' ? '当前分类暂未打通稳定服务端接口' : '筛选项会直接影响实际请求结果'}</p>
                  </div>
                  <div className="global-market-toolbar">
                    {toolbar}
                  </div>
                </div>
                {renderContent()}
              </div>
            </Card>
          </div>
        </div>
      </section>
    </div>
  );
}

export default StockGlobalMarketPage;

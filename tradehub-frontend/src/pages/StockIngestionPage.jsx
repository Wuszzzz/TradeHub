import { useCallback, useEffect, useState } from 'react';
import { Button, Checkbox, Empty, Input, Popconfirm, Select, Space, Tag, message } from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { fmtBig, fmtNum, fmtPct, toNumber } from '../stock/utils';

const PERIODS = ['秒级', '5s', '10s', '30s', '1m', '5m', '10m', '30m', '1h', '1d'];
const FIELD_OPTIONS = [
  { label: '成交价/开高低收', value: 'price' },
  { label: '成交量', value: 'volume' },
  { label: '成交额', value: 'amount' },
  { label: '换手率', value: 'turnover_rate' },
  { label: '换手量', value: 'turnover_amount' },
  { label: '量比', value: 'volume_ratio' },
  { label: '溢价率', value: 'premium_ratio' },
  { label: '买五卖五', value: 'buy_sell_5' },
  { label: '大中小单', value: 'order_flow' },
];

const STATUS_COLOR = { completed: 'success', running: 'processing', pending: 'default', retry: 'warning', failed: 'error' };
const STATUS_TEXT = { completed: '已完成', running: '运行中', pending: '待执行', retry: '重试中', failed: '失败' };

function StockIngestionPage() {
  const [mode, setMode] = useState('browse');
  const [selected, setSelected] = useState(null);
  const [interval, setInterval] = useState('1m');
  const [fields, setFields] = useState(['price', 'volume', 'amount']);
  const [creating, setCreating] = useState(false);
  const [tasks, setTasks] = useState([]);
  const [reloading, setReloading] = useState(false);

  const [market, setMarket] = useState('all');
  const [keyword, setKeyword] = useState('');
  const [browseItems, setBrowseItems] = useState([]);
  const [browseLoading, setBrowseLoading] = useState(false);
  const [page, setPage] = useState(1);
  const pageSize = 50;

  const [searchKeyword, setSearchKeyword] = useState('513310');
  const [searchItems, setSearchItems] = useState([]);
  const [searching, setSearching] = useState(false);

  const loadBrowse = useCallback(async () => {
    setBrowseLoading(true);
    try {
      const { data } = await stockAPI.instruments({ market, keyword, limit: pageSize, offset: (page - 1) * pageSize });
      setBrowseItems(data.items || []);
      if (!selected && (data.items || []).length > 0) setSelected(data.items[0]);
    } catch (e) {
      message.error(e.response?.data?.message || '加载市场列表失败');
    } finally {
      setBrowseLoading(false);
    }
  }, [market, keyword, page, selected]);

  const loadTasks = useCallback(async () => {
    try {
      const { data } = await stockAPI.listTasks();
      setTasks(data.items || []);
    } catch (e) {
      message.error(e.response?.data?.message || '加载任务失败');
    }
  }, []);

  useEffect(() => { if (mode === 'browse') loadBrowse(); }, [mode, market, page, loadBrowse]);
  useEffect(() => {
    loadTasks();
    const id = window.setInterval(loadTasks, 30000);
    return () => window.clearInterval(id);
  }, [loadTasks]);
  useEffect(() => {
    if (mode !== 'browse') return undefined;
    const id = window.setTimeout(() => { setPage(1); loadBrowse(); }, 350);
    return () => window.clearTimeout(id);
  }, [keyword, mode, loadBrowse]);

  const search = async () => {
    if (!searchKeyword.trim()) return message.warning('请输入关键字');
    setSearching(true);
    try {
      const { data } = await stockAPI.search(searchKeyword);
      setSearchItems(data.items || []);
      setSelected((data.items || [])[0] || null);
      if (!data.items?.length) message.info('未找到匹配标的');
    } catch (e) {
      message.error(e.response?.data?.message || '搜索失败');
    } finally {
      setSearching(false);
    }
  };

  const deleteTask = async (task) => {
    try {
      await stockAPI.deleteTask(task.task_id);
      message.success(`已删除：${task.name || task.symbol}`);
      await loadTasks();
    } catch (e) {
      message.error(e.response?.data?.message || '删除失败');
    }
  };

  const createTask = async () => {
    if (!selected) return message.warning('请先选择标的');
    setCreating(true);
    try {
      await stockAPI.createTask({ symbol: selected.symbol, name: selected.name, market: selected.market, interval, fields });
      message.success(`任务已创建：${selected.name}`);
      await loadTasks();
    } catch (e) {
      message.error(e.response?.data?.message || '创建任务失败');
    } finally {
      setCreating(false);
    }
  };

  const itemRow = (item) => {
    const pct = toNumber(item.pct_change);
    const isSelected = selected?.symbol === item.symbol;
    return (
      <button
        key={item.symbol} type="button" onClick={() => setSelected(item)}
        className={`ingestion-item-row ${isSelected ? 'is-selected' : ''}`}
      >
        <span style={{ fontFamily: 'monospace', minWidth: 80 }}>{item.symbol}</span>
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.name}</span>
        <span style={{ minWidth: 70, textAlign: 'right' }}>{fmtNum(item.price, 3)}</span>
        <span style={{ minWidth: 70, textAlign: 'right' }}
          className={pct !== null ? (pct > 0 ? 'up' : pct < 0 ? 'down' : '') : ''}>
          {fmtPct(item.pct_change)}
        </span>
        <span style={{ minWidth: 80, textAlign: 'right' }}>{fmtBig(item.amount)}</span>
        {isSelected && <Tag color="blue" style={{ marginLeft: 8 }}>已选</Tag>}
      </button>
    );
  };

  return (
    <div className="stock-page-stack">
      <div className="stock-page-grid" style={{ gridTemplateColumns: '3fr 7fr' }}>
        <section className="terminal-panel">
          <header>
            <h3>任务配置</h3>
            {selected && <Tag color="processing" style={{ fontFamily: 'monospace' }}>{selected.symbol}</Tag>}
          </header>
          <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <div style={{ marginBottom: 6, fontSize: 12, color: 'var(--trade-muted)' }}>采集周期</div>
              <Select value={interval} onChange={setInterval}
                options={PERIODS.map((p) => ({ label: p, value: p }))} style={{ width: '100%' }} />
              <div style={{ marginTop: 4, fontSize: 11, color: 'rgba(255,255,255,0.3)' }}>
                日线选 1d；分钟/小时走 minute；秒级走快照
              </div>
            </div>
            <div>
              <div style={{ marginBottom: 6, fontSize: 12, color: 'var(--trade-muted)' }}>采集字段</div>
              <Checkbox.Group value={fields} onChange={setFields} options={FIELD_OPTIONS}
                style={{ display: 'flex', flexDirection: 'column', gap: 6 }} />
            </div>
            <Button type="primary" icon={<PlusOutlined />} loading={creating} onClick={createTask} disabled={!selected}>
              {selected ? `为 ${selected.symbol} 创建落库任务` : '请先选中标的'}
            </Button>
          </div>
        </section>

        <section className="terminal-panel">
          <header>
            <h3>选择标的</h3>
            <Space>
              <Button size="small" type={mode === 'browse' ? 'primary' : 'default'} onClick={() => setMode('browse')}>
                浏览全市场
              </Button>
              <Button size="small" type={mode === 'search' ? 'primary' : 'default'} onClick={() => setMode('search')}>
                关键字搜索
              </Button>
            </Space>
          </header>
          <div style={{ padding: 16 }}>
            {mode === 'browse' ? (
              <>
                <Space style={{ width: '100%', marginBottom: 12 }}>
                  <Input value={keyword} onChange={(e) => setKeyword(e.target.value)}
                    placeholder="按代码或名称过滤" prefix={<SearchOutlined />} style={{ width: 220 }} />
                  <Select value={market} onChange={(v) => { setMarket(v); setPage(1); }}
                    options={[{ label: '全部', value: 'all' }, { label: 'A 股', value: 'stock' }, { label: 'ETF', value: 'etf' }]}
                    style={{ width: 110 }} />
                </Space>
                <div style={{ maxHeight: 400, overflowY: 'auto', marginBottom: 8 }}>
                  {browseLoading
                    ? <div style={{ padding: 20, textAlign: 'center', color: 'var(--trade-muted)' }}>加载中...</div>
                    : browseItems.length === 0 ? <Empty description="暂无数据" />
                    : <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                        {browseItems.map(itemRow)}
                      </div>
                  }
                </div>
                <Space>
                  <Button size="small" disabled={page <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>上一页</Button>
                  <span style={{ fontSize: 12, color: 'var(--trade-muted)' }}>第 {page} 页</span>
                  <Button size="small" disabled={browseItems.length < pageSize} onClick={() => setPage((p) => p + 1)}>下一页</Button>
                </Space>
              </>
            ) : (
              <>
                <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
                  <Input value={searchKeyword} onChange={(e) => setSearchKeyword(e.target.value)}
                    placeholder="股票/ETF 代码或名称" prefix={<SearchOutlined />} onPressEnter={search} />
                  <Button type="primary" loading={searching} onClick={search}>查找标的</Button>
                </Space.Compact>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {searchItems.length === 0 ? <Empty description="输入关键字后点击「查找标的」" />
                    : searchItems.map((item) => {
                      const isSelected = selected?.symbol === item.symbol;
                      return (
                        <button key={item.symbol} type="button" onClick={() => setSelected(item)}
                          className={`ingestion-item-row ${isSelected ? 'is-selected' : ''}`}
                          style={{ flexDirection: 'column', alignItems: 'flex-start', gap: 2 }}>
                          <span style={{ fontWeight: 500 }}>{item.name}</span>
                          <div style={{ display: 'flex', gap: 8 }}>
                            <span style={{ fontFamily: 'monospace', fontSize: 11 }}>{item.symbol}</span>
                            <Tag style={{ fontSize: 10, margin: 0 }}>{item.board || item.market}</Tag>
                          </div>
                        </button>
                      );
                    })
                  }
                </div>
              </>
            )}
          </div>
        </section>
      </div>

      <section className="terminal-panel">
        <header>
          <h3>
            任务中心
            <span style={{ marginLeft: 8, fontSize: 12, color: 'var(--trade-muted)', fontWeight: 400 }}>
              共 {tasks.length} 条 · 30s 自动刷新
            </span>
          </h3>
          <Button className="stock-inline-button" size="small" loading={reloading}
            icon={<ReloadOutlined />} onClick={() => { setReloading(true); loadTasks().finally(() => setReloading(false)); }}>
            刷新
          </Button>
        </header>
        {tasks.length === 0 ? (
          <div className="stock-empty-wrap"><Empty description="尚未创建任何落库任务" /></div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>标的</th><th>代码</th><th>市场</th><th>周期</th><th>字段</th>
                <th>状态</th><th>最近执行</th><th>备注</th><th>操作</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((task) => (
                <tr key={task.task_id}>
                  <td style={{ maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis' }}>{task.name}</td>
                  <td style={{ fontFamily: 'monospace' }}>{task.symbol}</td>
                  <td>{task.market}</td>
                  <td style={{ fontFamily: 'monospace' }}>{task.interval}</td>
                  <td style={{ maxWidth: 160, overflow: 'hidden', textOverflow: 'ellipsis', fontSize: 11 }}>
                    {(task.fields || []).join(', ')}
                  </td>
                  <td><Tag color={STATUS_COLOR[task.status] || 'default'}>{STATUS_TEXT[task.status] || task.status}</Tag></td>
                  <td style={{ fontFamily: 'monospace', fontSize: 11 }}>
                    {task.last_run_at?.slice(0, 16).replace('T', ' ') || '--'}
                  </td>
                  <td style={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', fontSize: 11 }}>
                    {task.last_message || '--'}
                  </td>
                  <td>
                    <Popconfirm title={`确认删除任务「${task.name || task.symbol}」？`} onConfirm={() => deleteTask(task)}>
                      <button type="button" className="stock-text-button"><DeleteOutlined /> 删除</button>
                    </Popconfirm>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

export default StockIngestionPage;

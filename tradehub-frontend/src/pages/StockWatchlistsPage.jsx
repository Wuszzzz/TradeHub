import { useCallback, useEffect, useRef, useState } from 'react';
import { Button, Empty, Input, Popconfirm, Select, Tag, message } from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { fmtBig, fmtNum, fmtPct, toNumber, trendClass } from '../stock/utils';

const REFRESH_MS = 8_000;

function StockWatchlistsPage() {
  const [loading, setLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [groupId, setGroupId] = useState('default');
  const groupIdRef = useRef(groupId);
  groupIdRef.current = groupId;
  const [newGroupName, setNewGroupName] = useState('');
  const [items, setItems] = useState([]);
  const [lastUpdated, setLastUpdated] = useState(null);

  const loadGroups = useCallback(async () => {
    try {
      const { data } = await stockAPI.watchlistGroups();
      const next = data.items || [];
      setGroups(next);
      if (!next.find((g) => g.group_id === groupIdRef.current) && next[0]) {
        setGroupId(next[0].group_id);
      }
    } catch (error) {
      message.error(error.response?.data?.message || '加载分组失败');
    }
  }, []);

  const loadSnapshot = useCallback(async (gid, silent = false) => {
    if (!gid) return;
    if (!silent) setLoading(true);
    try {
      const { data } = await stockAPI.watchlistSnapshot(gid);
      setItems(data.items || []);
      setLastUpdated(new Date());
    } catch (error) {
      if (!silent) message.error(error.response?.data?.message || '加载股票自选失败');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadGroups().then(() => loadSnapshot('default'));
  }, [loadGroups, loadSnapshot]);

  useEffect(() => {
    loadSnapshot(groupId);
  }, [groupId, loadSnapshot]);

  // 8s 自动刷新快照
  useEffect(() => {
    const id = window.setInterval(() => loadSnapshot(groupIdRef.current, true), REFRESH_MS);
    return () => window.clearInterval(id);
  }, [loadSnapshot]);

  const handleCreateGroup = async () => {
    const name = newGroupName.trim();
    if (!name) return;
    try {
      await stockAPI.createWatchlistGroup({ name, sort_order: groups.length });
      setNewGroupName('');
      await loadGroups();
    } catch (error) {
      message.error(error.response?.data?.message || '创建分组失败');
    }
  };

  const handleDeleteItem = async (itemId) => {
    try {
      await stockAPI.deleteWatchlistItem(itemId);
      await Promise.all([loadGroups(), loadSnapshot(groupId)]);
      message.success('已移除自选');
    } catch (error) {
      message.error(error.response?.data?.message || '删除失败');
    }
  };

  return (
    <div className="stock-page-stack">
      <section className="terminal-panel">
        <header>
          <h3>
            股票自选
            {lastUpdated && (
              <span style={{ marginLeft: 10, fontSize: 12, color: 'var(--trade-muted)', fontWeight: 400 }}>
                · {lastUpdated.toLocaleTimeString('zh-CN', { hour12: false })} 自动刷新 {REFRESH_MS / 1000}s
              </span>
            )}
          </h3>
          <Button
            className="stock-inline-button"
            size="small"
            icon={<ReloadOutlined />}
            loading={loading}
            onClick={() => loadSnapshot(groupId)}
          >
            刷新
          </Button>
        </header>
        <div className="stock-toolbar">
          <Select
            value={groupId}
            onChange={setGroupId}
            options={groups.map((g) => ({ value: g.group_id, label: `${g.name} (${g.item_count ?? 0})` }))}
            placeholder="选择分组"
            className="stock-toolbar-control"
          />
          <Input
            value={newGroupName}
            onChange={(e) => setNewGroupName(e.target.value)}
            onPressEnter={handleCreateGroup}
            placeholder="新建分组名"
            className="stock-toolbar-control"
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateGroup}>
            添加分组
          </Button>
        </div>
        {items.length === 0 ? (
          <div className="stock-empty-wrap">
            <Empty description="当前分组还没有股票，从行情搜索页加入自选" />
          </div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>代码</th>
                <th>名称</th>
                <th>市场</th>
                <th style={{ textAlign: 'right' }}>最新价</th>
                <th style={{ textAlign: 'right' }}>涨跌幅</th>
                <th style={{ textAlign: 'right' }}>IOPV</th>
                <th style={{ textAlign: 'right' }}>溢价率</th>
                <th style={{ textAlign: 'right' }}>换手率</th>
                <th style={{ textAlign: 'right' }}>成交额</th>
                <th>备注</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => {
                const q = item.quote || {};
                const pct = toNumber(q.pct_change ?? q.change_percent);
                const premium = toNumber(q.premium_ratio);
                return (
                  <tr key={item.item_id}>
                    <td style={{ fontFamily: 'monospace' }}>{item.symbol}</td>
                    <td>{item.name}</td>
                    <td><Tag>{item.market || '-'}</Tag></td>
                    <td style={{ textAlign: 'right' }} className={trendClass(pct)}>
                      {fmtNum(q.price, 3)}
                    </td>
                    <td style={{ textAlign: 'right' }} className={trendClass(pct)}>
                      {fmtPct(pct)}
                    </td>
                    <td style={{ textAlign: 'right' }}>{fmtNum(q.iopv, 4)}</td>
                    <td style={{ textAlign: 'right' }} className={trendClass(premium)}>
                      {premium !== null ? fmtPct(premium) : '--'}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      {q.turnover_rate != null ? fmtPct(q.turnover_rate) : '--'}
                    </td>
                    <td style={{ textAlign: 'right' }}>{fmtBig(q.amount)}</td>
                    <td>{item.note || '-'}</td>
                    <td>
                      <Popconfirm title="确认移除该自选？" onConfirm={() => handleDeleteItem(item.item_id)}>
                        <button type="button" className="stock-text-button">
                          <DeleteOutlined /> 删除
                        </button>
                      </Popconfirm>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

export default StockWatchlistsPage;

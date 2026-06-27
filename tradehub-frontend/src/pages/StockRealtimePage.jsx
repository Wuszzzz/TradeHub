import { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Button, Input, Space, Switch, Tag, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { fmtNum, fmtBig, fmtPct, toNumber, trendClass } from '../stock/utils';
import OrderBook from '../components/stock/OrderBook';

const REFRESH_MS = 5_000;

const Stat = ({ label, value, trend }) => (
  <div className="rt-stat">
    <span className="rt-stat-label">{label}</span>
    <span className={`rt-stat-value ${trend || ''}`}>{value}</span>
  </div>
);

function StockRealtimePage() {
  const [symbol, setSymbol] = useState('513310');
  const [item, setItem] = useState(null);
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);
  const [updatedAt, setUpdatedAt] = useState(null);
  const [profile, setProfile] = useState(null);
  const symbolRef = useRef(symbol);
  symbolRef.current = symbol;

  const load = useCallback(async (silent = false) => {
    if (!symbolRef.current.trim()) return;
    if (!silent) setLoading(true);
    try {
      const { data } = await stockAPI.realtime(symbolRef.current);
      setItem(data.item || null);
      setUpdatedAt(new Date());
    } catch (error) {
      if (!silent) message.error(error.response?.data?.message || '实盘查询失败');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  const loadProfile = useCallback(async () => {
    if (!symbolRef.current.trim()) return;
    try {
      const { data } = await stockAPI.profile(symbolRef.current);
      setProfile(data.item || null);
    } catch {
      setProfile(null);
    }
  }, []);

  useEffect(() => {
    load();
    loadProfile();
  }, [load, loadProfile]);

  useEffect(() => {
    if (!auto) return undefined;
    const id = window.setInterval(() => load(true), REFRESH_MS);
    return () => window.clearInterval(id);
  }, [auto, load]);

  const handleQuery = () => {
    load();
    loadProfile();
  };

  const pct = toNumber(item?.pct_change);
  const trend = trendClass(pct);
  const price = toNumber(item?.price);
  const change = toNumber(item?.change);

  return (
    <div className="stock-page-stack">
      <div className="stock-page-grid" style={{ gridTemplateColumns: '7fr 5fr', gap: 12 }}>
        <section className="terminal-panel">
          <header>
            <h3>
              实时快照
              {updatedAt && (
                <span style={{ marginLeft: 10, fontSize: 12, color: 'var(--trade-muted)', fontWeight: 400 }}>
                  · {updatedAt.toLocaleTimeString('zh-CN', { hour12: false })}
                </span>
              )}
            </h3>
            <Space>
              <span style={{ fontSize: 12, color: 'var(--trade-muted)' }}>自动刷新</span>
              <Switch
                size="small"
                checked={auto}
                onChange={setAuto}
                checkedChildren="ON"
                unCheckedChildren="OFF"
              />
              <Button
                className="stock-inline-button"
                size="small"
                icon={<ReloadOutlined />}
                loading={loading}
                onClick={() => load()}
              >
                刷新
              </Button>
            </Space>
          </header>

          <div style={{ padding: '16px 16px 8px' }}>
            <Space.Compact style={{ width: '100%', maxWidth: 400 }}>
              <Input
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
                onPressEnter={handleQuery}
                placeholder="例如 513310 / 600519 / 159915"
              />
              <Button type="primary" loading={loading} onClick={handleQuery}>
                查询
              </Button>
            </Space.Compact>
          </div>

          {profile && (profile.industry || profile.board || profile.listed_at) && (
            <div className="rt-profile-row">
              {profile.board && <Tag>{profile.board}</Tag>}
              {profile.industry && <Tag>{profile.industry}</Tag>}
              {profile.listed_at && <span style={{ fontSize: 11, color: 'var(--trade-muted)' }}>上市 {profile.listed_at}</span>}
              {profile.market_cap && <span style={{ fontSize: 11, color: 'var(--trade-muted)' }}>总市值 {fmtBig(profile.market_cap)}</span>}
            </div>
          )}

          {item ? (
            <div style={{ padding: '0 16px 16px' }}>
              <div className="rt-hero">
                <div>
                  <div className="rt-name">
                    {item.name || '--'}
                    <span className="rt-symbol">{item.symbol || symbol}</span>
                  </div>
                  <div className={`rt-price ${trend}`}>
                    {price !== null ? fmtNum(price, 3) : '--'}
                  </div>
                  <div className={`rt-change ${trend}`}>
                    <span>{change !== null ? (change > 0 ? `+${fmtNum(change, 3)}` : fmtNum(change, 3)) : '--'}</span>
                    <span>{pct !== null ? fmtPct(pct) : '--'}</span>
                  </div>
                </div>
                <div className="rt-stats-grid">
                  <Stat label="开" value={fmtNum(item.open, 3)} />
                  <Stat label="昨收" value={fmtNum(item.prev_close, 3)} />
                  <Stat label="高" value={fmtNum(item.high, 3)} trend="up" />
                  <Stat label="低" value={fmtNum(item.low, 3)} trend="down" />
                  <Stat label="成交量" value={fmtBig(item.volume)} />
                  <Stat label="成交额" value={fmtBig(item.amount)} />
                  <Stat label="量比" value={fmtNum(item.volume_ratio)} />
                  <Stat label="换手率" value={item.turnover_rate ? fmtPct(item.turnover_rate) : '--'} />
                  <Stat label="溢价率" value={item.premium_ratio ? fmtPct(item.premium_ratio) : '--'} />
                  <Stat label="IOPV" value={fmtNum(item.iopv, 3)} />
                </div>
              </div>

              <div className="rt-extra-grid">
                <div>
                  <div className="rt-section-label">资金流向</div>
                  <div className="rt-stats-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
                    <Stat label="大单" value={fmtBig(item.big_order_volume)} />
                    <Stat label="中单" value={fmtBig(item.medium_order_volume)} />
                    <Stat label="小单" value={fmtBig(item.small_order_volume)} />
                  </div>
                </div>
                <div>
                  <div className="rt-section-label">其他指标</div>
                  <div className="rt-stats-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
                    <Stat label="振幅" value={item.amplitude ? fmtPct(item.amplitude) : '--'} />
                    <Stat label="总市值" value={fmtBig(item.market_cap)} />
                    <Stat label="流通市值" value={fmtBig(item.float_market_cap)} />
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="stock-empty-wrap" style={{ textAlign: 'center', color: 'var(--trade-muted)' }}>
              {loading ? '正在拉取行情快照…' : '请输入标的代码后回车或点击查询'}
            </div>
          )}
        </section>

        <section className="terminal-panel">
          <header>
            <h3>买卖五档梯度</h3>
          </header>
          {item ? (
            <div style={{ padding: '8px 0 12px' }}>
              <OrderBook snapshot={item} />
            </div>
          ) : (
            <div className="stock-empty-wrap" style={{ textAlign: 'center', color: 'var(--trade-muted)' }}>
              行情未就绪
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

export default StockRealtimePage;

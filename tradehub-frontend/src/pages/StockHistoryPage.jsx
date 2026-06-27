import { useState } from 'react';
import { Button, Empty, Input, Select, Space, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { fmtNum, fmtBig, fmtPct, fmtTimeShort, toNumber } from '../stock/utils';
import CandleChart from '../components/stock/CandleChart';

const PERIODS = ['秒级', '5s', '10s', '30s', '1m', '5m', '10m', '30m', '1h', '1d'];
const LIMITS = [60, 120, 240, 480];

function StockHistoryPage() {
  const [symbol, setSymbol] = useState('513310');
  const [interval, setInterval] = useState('1m');
  const [limit, setLimit] = useState(200);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    if (!symbol.trim()) {
      message.warning('请输入标的代码');
      return;
    }
    setLoading(true);
    try {
      const res = interval === '1d'
        ? await stockAPI.daily(symbol, undefined, undefined, limit)
        : await stockAPI.history(symbol, interval, limit);
      const data = res.data;
      setItems(data.items || []);
      if ((data.items || []).length === 0) {
        message.info(interval === '1d'
          ? 'AKShare 未返回日线数据'
          : '该周期下 TDengine 暂无历史，请先到数据采集页创建任务');
      }
    } catch (error) {
      message.error(error.response?.data?.message || '查询失败');
    } finally {
      setLoading(false);
    }
  };

  const last = items[items.length - 1];
  const first = items[0];
  const lastClose = toNumber(last?.close);
  const firstOpen = toNumber(first?.open);
  const change = lastClose !== null && firstOpen !== null ? lastClose - firstOpen : null;
  const pct = change !== null && firstOpen ? (change / firstOpen) * 100 : null;
  const totalVol = items.reduce((acc, r) => acc + (toNumber(r.volume) || 0), 0);
  const totalAmt = items.reduce((acc, r) => acc + (toNumber(r.amount) || 0), 0);

  return (
    <div className="stock-page-stack">
      <section className="terminal-panel">
        <header>
          <h3>历史回放配置</h3>
        </header>
        <div className="stock-toolbar">
          <Input
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            onPressEnter={load}
            placeholder="例如 513310 / 600519"
            style={{ maxWidth: 220 }}
          />
          <Select
            value={interval}
            onChange={setInterval}
            options={PERIODS.map((p) => ({ label: p, value: p }))}
            style={{ width: 100 }}
          />
          <Select
            value={limit}
            onChange={setLimit}
            options={LIMITS.map((n) => ({ label: `${n} 根`, value: n }))}
            style={{ width: 110 }}
          />
          <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={load}>
            查询历史
          </Button>
        </div>
      </section>

      <div className="stock-stat-grid" style={{ padding: '0 0 4px' }}>
        <div>
          <span>最新收盘</span>
          <strong className={pct !== null ? (pct > 0 ? 'up' : 'down') : ''}>
            {fmtNum(lastClose, 3)}
          </strong>
        </div>
        <div>
          <span>区间涨跌</span>
          <strong className={pct !== null ? (pct > 0 ? 'up' : 'down') : ''}>
            {pct !== null ? fmtPct(pct) : '--'}
          </strong>
        </div>
        <div>
          <span>区间成交量</span>
          <strong>{fmtBig(totalVol)}</strong>
        </div>
        <div>
          <span>区间成交额</span>
          <strong>{fmtBig(totalAmt)}</strong>
        </div>
      </div>

      <section className="terminal-panel">
        <header>
          <h3>
            K 线图
            <span style={{ marginLeft: 8, fontSize: 12, color: 'var(--trade-muted)', fontWeight: 400 }}>
              来自 TDengine · {symbol} / {interval}
            </span>
          </h3>
        </header>
        <div style={{ padding: '12px 16px' }}>
          <CandleChart data={items} height={360} />
        </div>
      </section>

      <section className="terminal-panel">
        <header>
          <h3>历史明细</h3>
        </header>
        {items.length === 0 ? (
          <div className="stock-empty-wrap">
            <Empty description="尚未查询到历史数据，先到数据采集页创建任务并执行" />
          </div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>时间</th>
                <th style={{ textAlign: 'right' }}>开</th>
                <th style={{ textAlign: 'right' }}>收</th>
                <th style={{ textAlign: 'right' }}>高</th>
                <th style={{ textAlign: 'right' }}>低</th>
                <th style={{ textAlign: 'right' }}>成交量</th>
                <th style={{ textAlign: 'right' }}>成交额</th>
                <th style={{ textAlign: 'right' }}>换手率</th>
              </tr>
            </thead>
            <tbody>
              {[...items].reverse().map((row, i) => {
                const o = toNumber(row.open);
                const c = toNumber(row.close);
                const cls = o !== null && c !== null ? (c > o ? 'up' : c < o ? 'down' : '') : '';
                return (
                  <tr key={row.ts || i}>
                    <td style={{ fontFamily: 'monospace' }}>{fmtTimeShort(row.ts)}</td>
                    <td style={{ textAlign: 'right' }}>{fmtNum(row.open, 3)}</td>
                    <td style={{ textAlign: 'right' }} className={cls}>{fmtNum(row.close, 3)}</td>
                    <td style={{ textAlign: 'right' }} className="up">{fmtNum(row.high, 3)}</td>
                    <td style={{ textAlign: 'right' }} className="down">{fmtNum(row.low, 3)}</td>
                    <td style={{ textAlign: 'right' }}>{fmtBig(row.volume)}</td>
                    <td style={{ textAlign: 'right' }}>{fmtBig(row.amount)}</td>
                    <td style={{ textAlign: 'right' }}>{row.turnover_rate ? fmtPct(row.turnover_rate) : '--'}</td>
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

export default StockHistoryPage;

import { useCallback, useEffect, useState } from 'react';
import { Button, Input, Space, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { fmtNum, fmtBig, fmtPct, toNumber } from '../stock/utils';
import CandleChart from '../components/stock/CandleChart';

function Histogram({ data, height = 120 }) {
  if (!data || data.length === 0) {
    return <div style={{ padding: 20, textAlign: 'center', color: 'var(--trade-muted)' }}>无数据</div>;
  }
  const maxCount = Math.max(...data.map((d) => d.count), 1);
  const width = 320;
  const barW = (width - 30) / data.length;
  return (
    <svg width="100%" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none"
      style={{ background: 'var(--trade-bg-soft)', borderRadius: 8 }}>
      {data.map((d, i) => {
        const h = (d.count / maxCount) * (height - 30);
        const x = 15 + i * barW;
        const y = height - 18 - h;
        return (
          <g key={d.bucket_pct ?? i}>
            <rect x={x + 2} y={y} width={barW - 4} height={h}
              fill="var(--trade-blue)" opacity={0.7} />
            <text x={x + barW / 2} y={height - 4} fontSize="10"
              fill="#98a2b3" textAnchor="middle">
              {d.bucket_pct > 0 ? `+${d.bucket_pct}` : d.bucket_pct}%
            </text>
            <text x={x + barW / 2} y={y - 4} fontSize="10"
              fill="#667085" textAnchor="middle">
              {d.count}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

const Stat = ({ label, value, trend }) => (
  <div className="rt-stat">
    <span className="rt-stat-label">{label}</span>
    <span className={`rt-stat-value ${trend || ''}`}>{value}</span>
  </div>
);

function StockETFRiskPage() {
  const [symbol, setSymbol] = useState('513310');
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!symbol.trim()) return;
    setLoading(true);
    try {
      const { data: res } = await stockAPI.etfRisk(symbol, 200);
      setData(res);
    } catch (error) {
      message.error(error.response?.data?.message || `查询失败: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }, [symbol]);

  useEffect(() => { load(); }, [load]);

  const rt = data?.realtime || {};
  const history = data?.history || [];
  const distribution = data?.distribution || [];

  const iopv = toNumber(rt.iopv);
  const price = toNumber(rt.price);
  const realPremium = (price && iopv) ? ((price - iopv) / iopv) * 100 : null;
  const apiPremium = toNumber(rt.premium_ratio);
  const premium = realPremium !== null ? realPremium : apiPremium;
  const premiumTrend = premium === null ? '' : premium > 0 ? 'up' : premium < 0 ? 'down' : '';
  const pct = toNumber(rt.pct_change);
  const pctTrend = pct === null ? '' : pct > 0 ? 'up' : pct < 0 ? 'down' : '';

  let level = 'safe';
  let levelText = '正常';
  let levelColor = 'var(--trade-blue)';
  if (premium !== null) {
    const abs = Math.abs(premium);
    if (abs >= 3) {
      level = 'high';
      levelText = '高溢价/折价';
      levelColor = 'var(--trade-red)';
    } else if (abs >= 1) {
      level = 'medium';
      levelText = '关注';
      levelColor = 'var(--trade-yellow)';
    }
  }

  return (
    <div className="stock-page-stack">
      <section className="terminal-panel">
        <header>
          <h3>ETF 风控查询</h3>
        </header>
        <div className="stock-toolbar">
          <Space.Compact>
            <Input
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              onPressEnter={load}
              placeholder="ETF 代码，例如 513310 / 159915 / 510300"
              style={{ maxWidth: 320 }}
            />
            <Button type="primary" loading={loading} icon={<ReloadOutlined />} onClick={load}>
              查询
            </Button>
          </Space.Compact>
        </div>
      </section>

      <section className="terminal-panel">
        <header>
          <h3>
            {rt.name || symbol} · 实时风控
            <span style={{
              marginLeft: 12,
              padding: '2px 8px',
              borderRadius: 4,
              fontSize: 11,
              fontWeight: 500,
              color: levelColor,
              border: `1px solid ${levelColor}`,
            }}>
              {levelText}
            </span>
          </h3>
        </header>
        <div style={{ padding: '16px 16px 20px' }}>
          <div className="rt-stats-grid" style={{ gridTemplateColumns: 'repeat(4, 1fr)', marginBottom: 16 }}>
            <Stat label="最新价" value={price !== null ? fmtNum(price, 3) : '--'} trend={pctTrend} />
            <Stat label="IOPV 估值" value={iopv !== null ? fmtNum(iopv, 4) : '--'} />
            <Stat
              label="折溢价率"
              value={premium !== null ? `${premium > 0 ? '+' : ''}${fmtPct(premium)}` : '--'}
              trend={premiumTrend}
            />
            <Stat label="涨跌幅" value={pct !== null ? fmtPct(pct) : '--'} trend={pctTrend} />
            <Stat label="换手率" value={rt.turnover_rate ? fmtPct(rt.turnover_rate) : '--'} />
            <Stat label="成交量" value={fmtBig(rt.volume)} />
            <Stat label="成交额" value={fmtBig(rt.amount)} />
            <Stat label="价 - IOPV" value={(price !== null && iopv !== null) ? fmtNum(price - iopv, 4) : '--'} />
          </div>
          <div style={{
            padding: '12px 16px',
            background: 'var(--trade-bg-soft)',
            borderRadius: 8,
            fontSize: 12,
            color: 'var(--trade-muted)',
            lineHeight: 1.8,
          }}>
            <strong style={{ color: 'var(--trade-text)' }}>读数提示：</strong>
            折溢价率 = (现价 - IOPV) / IOPV × 100%。
            正值表示市场溢价（贵）、负值表示折价（便宜）。
            QDII / 跨境 ETF 长期高溢价（&gt;3%）通常意味着外汇额度紧张、限购或情绪过热，
            可结合换手率与成交额判断是否回归。
          </div>
        </div>
      </section>

      <div className="stock-page-grid" style={{ gridTemplateColumns: '7fr 5fr' }}>
        <section className="terminal-panel">
          <header>
            <h3>
              日线走势
              <span style={{ marginLeft: 8, fontSize: 12, color: 'var(--trade-muted)', fontWeight: 400 }}>
                近 {history.length} 个交易日
              </span>
            </h3>
          </header>
          <div style={{ padding: '12px 16px' }}>
            <CandleChart data={history} height={320} />
          </div>
        </section>

        <section className="terminal-panel">
          <header>
            <h3>价格相对均线偏离分布</h3>
          </header>
          <div style={{ padding: '16px 16px 12px' }}>
            <Histogram data={distribution} height={220} />
            <div style={{ marginTop: 12, fontSize: 11, color: 'var(--trade-muted)', lineHeight: 1.7 }}>
              X 轴为收盘价相对均线偏离百分点（向下取整到整数），Y 轴为交易日数。
              可用于估计当前价位在历史区间的相对位置。
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}

export default StockETFRiskPage;

import { useMemo, useState } from 'react';
import { fmtNum, fmtTimeShort, fmtBig } from '../../stock/utils';

export default function CandleChart({ data = [], height = 360 }) {
  const [hover, setHover] = useState(null);

  const view = useMemo(() => {
    if (!data || data.length === 0) return null;
    const rows = data
      .map((r) => ({
        ts: r.ts,
        open: Number(r.open) || 0,
        close: Number(r.close) || 0,
        high: Number(r.high) || 0,
        low: Number(r.low) || 0,
        volume: Number(r.volume) || 0,
      }))
      .filter((r) => r.high > 0 || r.low > 0 || r.close > 0);
    if (rows.length === 0) return null;

    const W = 1000;
    const PAD_L = 56;
    const PAD_R = 12;
    const PAD_T = 12;
    const PAD_B = 22;
    const VOL_RATIO = 0.22;
    const innerW = W - PAD_L - PAD_R;
    const totalH = height;
    const volH = totalH * VOL_RATIO;
    const priceH = totalH - PAD_T - PAD_B - volH - 8;

    const highs = rows.map((r) => Math.max(r.high, r.open, r.close));
    const lows = rows.map((r) => {
      const lo = r.low > 0 ? r.low : Math.min(r.open || Infinity, r.close || Infinity);
      return lo === Infinity ? 0 : lo;
    });
    let pMax = Math.max(...highs);
    let pMin = Math.min(...lows.filter((v) => v > 0));
    if (!Number.isFinite(pMin)) pMin = 0;
    const pPad = (pMax - pMin) * 0.08 || pMax * 0.01 || 1;
    pMax += pPad;
    pMin = Math.max(0, pMin - pPad);
    const pRange = pMax - pMin || 1;

    const vMax = Math.max(...rows.map((r) => r.volume), 1);

    const n = rows.length;
    const slot = innerW / n;
    const candleW = Math.max(1.5, Math.min(slot * 0.7, 14));

    const yPrice = (p) => PAD_T + (1 - (p - pMin) / pRange) * priceH;
    const xCenter = (i) => PAD_L + slot * (i + 0.5);
    const yVolTop = PAD_T + priceH + 8;
    const yVol = (v) => yVolTop + (1 - v / vMax) * volH;

    const yTicks = Array.from({ length: 5 }, (_, i) => {
      const p = pMin + (pRange * i) / 4;
      return { y: yPrice(p), label: fmtNum(p, 2) };
    });

    const xTickIdx = [];
    const xCount = Math.min(6, n);
    for (let i = 0; i < xCount; i++) {
      xTickIdx.push(Math.round(((n - 1) * i) / Math.max(1, xCount - 1)));
    }

    return {
      rows, W, totalH, PAD_L, PAD_R, PAD_T, PAD_B,
      innerW, priceH, volH, yVolTop, slot, candleW,
      yPrice, xCenter, yVol, yTicks, xTickIdx,
    };
  }, [data, height]);

  if (!view) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'rgba(255,255,255,0.3)', fontSize: 13 }}>
        暂无历史数据，请先创建落库任务并执行
      </div>
    );
  }

  const handleMove = (e) => {
    const svg = e.currentTarget;
    const rect = svg.getBoundingClientRect();
    const xPx = e.clientX - rect.left;
    const xVB = (xPx / rect.width) * view.W;
    const idx = Math.max(0, Math.min(view.rows.length - 1, Math.floor((xVB - view.PAD_L) / view.slot)));
    setHover({ idx, xPx, yPx: e.clientY - rect.top });
  };

  const r = hover ? view.rows[hover.idx] : null;
  const upColor = '#ef4f4f';
  const downColor = '#16c784';
  const flatColor = '#9ba8c0';

  return (
    <div style={{ position: 'relative' }} onMouseLeave={() => setHover(null)}>
      <svg
        viewBox={`0 0 ${view.W} ${view.totalH}`}
        preserveAspectRatio="none"
        style={{ width: '100%', height, display: 'block' }}
        onMouseMove={handleMove}
      >
        {view.yTicks.map((t, i) => (
          <g key={`yt-${i}`}>
            <line
              x1={view.PAD_L} x2={view.W - view.PAD_R}
              y1={t.y} y2={t.y}
              stroke="rgba(148,163,184,0.08)" strokeDasharray="3 4"
            />
            <text
              x={view.PAD_L - 6} y={t.y + 3}
              fill="#5d6b85" fontSize="10" textAnchor="end"
              fontFamily="JetBrains Mono, SF Mono, monospace"
            >
              {t.label}
            </text>
          </g>
        ))}

        {view.rows.map((row, i) => {
          const cx = view.xCenter(i);
          const isUp = row.close >= row.open;
          const color = row.close === row.open ? flatColor : isUp ? upColor : downColor;
          const yHi = view.yPrice(Math.max(row.high, row.open, row.close) || row.close);
          const yLo = view.yPrice(Math.min(row.low > 0 ? row.low : row.close, row.open, row.close) || row.close);
          const yO = view.yPrice(row.open || row.close);
          const yC = view.yPrice(row.close);
          const top = Math.min(yO, yC);
          const bodyH = Math.max(1, Math.abs(yC - yO));
          return (
            <g key={`c-${i}`}>
              <line x1={cx} x2={cx} y1={yHi} y2={yLo} stroke={color} strokeWidth="1" />
              <rect
                x={cx - view.candleW / 2} y={top}
                width={view.candleW} height={bodyH}
                fill={color} opacity={isUp ? 0.95 : 1}
              />
            </g>
          );
        })}

        {view.rows.map((row, i) => {
          const isUp = row.close >= row.open;
          const color = isUp ? 'rgba(239,79,79,0.55)' : 'rgba(22,199,132,0.55)';
          const cx = view.xCenter(i);
          const yTop = view.yVol(row.volume);
          const yBottom = view.yVolTop + view.volH;
          return (
            <rect
              key={`v-${i}`}
              x={cx - view.candleW / 2} y={yTop}
              width={view.candleW} height={Math.max(1, yBottom - yTop)}
              fill={color}
            />
          );
        })}

        {view.xTickIdx.map((i) => (
          <text
            key={`xt-${i}`}
            x={view.xCenter(i)}
            y={view.totalH - 6}
            fill="#5d6b85" fontSize="10" textAnchor="middle"
            fontFamily="JetBrains Mono, SF Mono, monospace"
          >
            {fmtTimeShort(view.rows[i].ts)}
          </text>
        ))}

        {hover && (
          <line
            x1={view.xCenter(hover.idx)} x2={view.xCenter(hover.idx)}
            y1={view.PAD_T} y2={view.totalH - view.PAD_B}
            stroke="rgba(15,23,42,0.16)" strokeDasharray="2 3"
          />
        )}
      </svg>

      {hover && r && (
        <div style={{
          position: 'absolute',
          left: Math.min(hover.xPx + 12, 300),
          top: Math.max(8, hover.yPx - 100),
          background: '#ffffff',
          border: '1px solid #d9e2ef',
          boxShadow: '0 14px 32px rgba(15, 23, 42, 0.12)',
          borderRadius: 6,
          padding: '8px 12px',
          fontSize: 12,
          fontFamily: 'JetBrains Mono, SF Mono, monospace',
          pointerEvents: 'none',
          zIndex: 10,
          lineHeight: 1.8,
          minWidth: 130,
        }}>
          <div style={{ color: '#667085', marginBottom: 4 }}>{fmtTimeShort(r.ts)}</div>
          {[['开', r.open], ['高', r.high], ['低', r.low], ['收', r.close]].map(([k, v]) => (
            <div key={k} style={{ display: 'flex', justifyContent: 'space-between', gap: 16 }}>
              <span style={{ color: '#667085' }}>{k}</span>
              <span style={{ color: r.close >= r.open ? '#ef4f4f' : '#16c784' }}>{fmtNum(v)}</span>
            </div>
          ))}
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16 }}>
            <span style={{ color: '#667085' }}>量</span>
            <span>{fmtBig(r.volume)}</span>
          </div>
        </div>
      )}
    </div>
  );
}

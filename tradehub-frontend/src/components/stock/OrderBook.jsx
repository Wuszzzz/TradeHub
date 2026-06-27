import { fmtNum, fmtBig, toNumber } from '../../stock/utils';

export default function OrderBook({ snapshot }) {
  if (!snapshot) return null;

  const ask = [5, 4, 3, 2, 1].map((i) => ({
    label: `卖${i}`,
    price: toNumber(snapshot[`ask_${i}_price`]),
    qty: toNumber(snapshot[`ask_${i}_volume`]),
  }));
  
  const bid = [1, 2, 3, 4, 5].map((i) => ({
    label: `买${i}`,
    price: toNumber(snapshot[`bid_${i}_price`]),
    qty: toNumber(snapshot[`bid_${i}_volume`]),
  }));

  const maxQty = Math.max(
    ...ask.map((r) => r.qty || 0),
    ...bid.map((r) => r.qty || 0),
    1,
  );

  return (
    <div className="stock-orderbook">
      {ask.map((row) => (
        <div key={row.label} className="orderbook-row ask">
          <span className="label">{row.label}</span>
          <span className="price">{row.price ? fmtNum(row.price, 3) : '--'}</span>
          <span className="qty">{row.qty ? fmtBig(row.qty) : '--'}</span>
          <span className="bar" style={{ width: `${((row.qty || 0) / maxQty) * 70}%` }} />
        </div>
      ))}
      <div className="orderbook-divider" />
      {bid.map((row) => (
        <div key={row.label} className="orderbook-row bid">
          <span className="label">{row.label}</span>
          <span className="price">{row.price ? fmtNum(row.price, 3) : '--'}</span>
          <span className="qty">{row.qty ? fmtBig(row.qty) : '--'}</span>
          <span className="bar" style={{ width: `${((row.qty || 0) / maxQty) * 70}%` }} />
        </div>
      ))}
    </div>
  );
}

import { useEffect, useState } from 'react';
import { Button, Empty, Input, InputNumber, Select, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { formatMoney, formatNumber } from '../stock/utils';

function StockOrdersPage() {
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [orders, setOrders] = useState([]);
  const [form, setForm] = useState({
    symbol: '',
    name: '',
    market: 'CN-A',
    side: 'buy',
    qty: 100,
    price: 10,
    fee: 5,
    note: '',
  });

  const loadOrders = async () => {
    setLoading(true);
    try {
      const { data } = await stockAPI.paperOrders();
      setOrders(data.items || []);
    } catch (error) {
      message.error(error.response?.data?.message || '加载委托失败');
      setOrders([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOrders();
  }, []);

  const updateField = (key, value) => {
    setForm((current) => ({ ...current, [key]: value }));
  };

  const handleSubmit = async () => {
    if (!form.symbol.trim()) {
      message.error('请输入股票代码');
      return;
    }
    setSubmitting(true);
    try {
      await stockAPI.placePaperOrder({
        ...form,
        symbol: form.symbol.trim().toUpperCase(),
      });
      message.success('模拟委托已成交');
      await loadOrders();
      setForm((current) => ({ ...current, symbol: '', name: '', note: '' }));
    } catch (error) {
      message.error(error.response?.data?.message || '下单失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="stock-page-grid">
      <section className="terminal-panel">
        <header>
          <h3>模拟下单</h3>
        </header>
        <div className="stock-form-grid">
          <label>
            <span>股票代码</span>
            <Input value={form.symbol} onChange={(event) => updateField('symbol', event.target.value)} placeholder="如 600519 / AAPL" />
          </label>
          <label>
            <span>名称</span>
            <Input value={form.name} onChange={(event) => updateField('name', event.target.value)} placeholder="可选" />
          </label>
          <label>
            <span>市场</span>
            <Select value={form.market} onChange={(value) => updateField('market', value)} options={[
              { value: 'CN-A', label: 'A 股' },
              { value: 'CN-ETF', label: 'ETF' },
              { value: 'US', label: '美股' },
            ]} />
          </label>
          <label>
            <span>方向</span>
            <Select value={form.side} onChange={(value) => updateField('side', value)} options={[
              { value: 'buy', label: '买入' },
              { value: 'sell', label: '卖出' },
            ]} />
          </label>
          <label>
            <span>数量</span>
            <InputNumber value={form.qty} min={1} onChange={(value) => updateField('qty', value || 1)} />
          </label>
          <label>
            <span>价格</span>
            <InputNumber value={form.price} min={0.01} step={0.01} onChange={(value) => updateField('price', value || 0.01)} />
          </label>
          <label>
            <span>手续费</span>
            <InputNumber value={form.fee} min={0} step={0.01} onChange={(value) => updateField('fee', value || 0)} />
          </label>
          <label className="stock-form-span-2">
            <span>备注</span>
            <Input value={form.note} onChange={(event) => updateField('note', event.target.value)} placeholder="记录本次策略原因" />
          </label>
        </div>
        <div className="stock-toolbar">
          <Button type="primary" loading={submitting} onClick={handleSubmit}>提交委托</Button>
        </div>
      </section>

      <section className="terminal-panel">
        <header>
          <h3>委托记录</h3>
          <Button className="stock-inline-button" size="small" icon={<ReloadOutlined />} loading={loading} onClick={loadOrders}>
            刷新
          </Button>
        </header>
        {orders.length === 0 ? (
          <div className="stock-empty-wrap"><Empty description="暂无委托记录" /></div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>代码</th>
                <th>方向</th>
                <th>数量</th>
                <th>价格</th>
                <th>金额</th>
                <th>手续费</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((item) => (
                <tr key={item.order_id}>
                  <td>{item.placed_at?.slice(0, 16).replace('T', ' ') || '-'}</td>
                  <td>{item.symbol}</td>
                  <td className={item.side === 'buy' ? 'up' : 'down'}>{item.side === 'buy' ? '买入' : '卖出'}</td>
                  <td>{formatNumber(item.qty)}</td>
                  <td>{formatNumber(item.price, 3)}</td>
                  <td>{formatMoney(item.amount)}</td>
                  <td>{formatMoney(item.fee)}</td>
                  <td>{item.status || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

export default StockOrdersPage;

import { useEffect, useMemo, useState } from 'react';
import { Button, Empty, InputNumber, Popconfirm, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { formatMoney, formatNumber, formatPercent } from '../stock/utils';

function StockAccountPage() {
  const [loading, setLoading] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [initial, setInitial] = useState(1000000);
  const [account, setAccount] = useState(null);
  const [positions, setPositions] = useState([]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [accountRes, positionsRes] = await Promise.all([
        stockAPI.paperAccount(),
        stockAPI.paperPositions(),
      ]);
      setAccount(accountRes.data?.item || null);
      setPositions(positionsRes.data?.items || []);
      setInitial(Number(accountRes.data?.item?.initial || 1000000));
    } catch (error) {
      message.error(error.response?.data?.message || '加载账户失败');
      setAccount(null);
      setPositions([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const stats = useMemo(() => ({
    cash: Number(account?.cash || 0),
    equity: Number(account?.equity || 0),
    realized: Number(account?.realized_pl || 0),
    totalReturn: Number(account?.total_return || 0),
  }), [account]);

  const handleReset = async () => {
    setResetting(true);
    try {
      await stockAPI.paperReset({ initial });
      message.success('模拟账户已重置');
      await loadData();
    } catch (error) {
      message.error(error.response?.data?.message || '重置失败');
    } finally {
      setResetting(false);
    }
  };

  return (
    <div className="stock-page-stack">
      <section className="terminal-panel">
        <header>
          <h3>模拟账户</h3>
          <Button className="stock-inline-button" size="small" icon={<ReloadOutlined />} loading={loading} onClick={loadData}>
            刷新
          </Button>
        </header>
        <div className="stock-stat-grid">
          <div><span>可用现金</span><strong>${formatMoney(stats.cash)}</strong></div>
          <div><span>总资产</span><strong>${formatMoney(stats.equity)}</strong></div>
          <div><span>已实现盈亏</span><strong>${formatMoney(stats.realized)}</strong></div>
          <div><span>累计收益率</span><strong className={stats.totalReturn >= 0 ? 'up' : 'down'}>{formatPercent(stats.totalReturn * 100)}</strong></div>
        </div>
        <div className="stock-toolbar">
          <InputNumber value={initial} min={10000} step={10000} onChange={(value) => setInitial(value || 1000000)} />
          <Popconfirm title="确认重置模拟账户？现有委托和持仓会清空。" onConfirm={handleReset}>
            <Button danger loading={resetting}>重置账户</Button>
          </Popconfirm>
        </div>
      </section>

      <section className="terminal-panel">
        <header>
          <h3>模拟持仓</h3>
        </header>
        {positions.length === 0 ? (
          <div className="stock-empty-wrap"><Empty description="暂无持仓" /></div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>代码</th>
                <th>名称</th>
                <th>数量</th>
                <th>成本价</th>
                <th>现价</th>
                <th>市值</th>
                <th>浮盈亏</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((item) => (
                <tr key={item.symbol}>
                  <td>{item.symbol}</td>
                  <td>{item.name || '-'}</td>
                  <td>{formatNumber(item.qty)}</td>
                  <td>{formatNumber(item.avg_cost, 3)}</td>
                  <td>{formatNumber(item.last_price, 3)}</td>
                  <td>{formatMoney(item.market_value)}</td>
                  <td className={Number(item.unrealized_pl) >= 0 ? 'up' : 'down'}>{formatMoney(item.unrealized_pl)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

export default StockAccountPage;

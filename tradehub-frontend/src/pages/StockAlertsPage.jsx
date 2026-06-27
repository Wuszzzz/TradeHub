import { useEffect, useState } from 'react';
import { Button, Empty, Input, InputNumber, Popconfirm, Select, Switch, message } from 'antd';
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { formatNumber } from '../stock/utils';

const metricOptions = [
  { value: 'price', label: '价格' },
  { value: 'pct_change', label: '涨跌幅' },
  { value: 'volume_ratio', label: '量比' },
  { value: 'premium_ratio', label: '溢价率' },
  { value: 'iopv', label: 'IOPV' },
  { value: 'turnover_rate', label: '换手率' },
];

const opOptions = [
  { value: 'gt', label: '>' },
  { value: 'gte', label: '>=' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '<=' },
  { value: 'eq', label: '=' },
];

function StockAlertsPage() {
  const [loading, setLoading] = useState(false);
  const [rules, setRules] = useState([]);
  const [events, setEvents] = useState([]);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({
    symbol: '',
    name: '',
    market: 'CN-A',
    metric: 'price',
    op: 'gt',
    threshold: 0,
    cooldown_seconds: 300,
    enabled: true,
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [rulesRes, eventsRes] = await Promise.all([
        stockAPI.alertRules(),
        stockAPI.alertEvents({ limit: 50 }),
      ]);
      setRules(rulesRes.data?.items || []);
      setEvents(eventsRes.data?.items || []);
    } catch (error) {
      message.error(error.response?.data?.message || '加载告警失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const updateField = (key, value) => {
    setForm((current) => ({ ...current, [key]: value }));
  };

  const handleCreate = async () => {
    if (!form.symbol.trim()) {
      message.error('请输入股票代码');
      return;
    }
    setCreating(true);
    try {
      await stockAPI.createAlertRule({
        ...form,
        symbol: form.symbol.trim().toUpperCase(),
      });
      message.success('告警规则已创建');
      setForm({
        symbol: '',
        name: '',
        market: 'CN-A',
        metric: 'price',
        op: 'gt',
        threshold: 0,
        cooldown_seconds: 300,
        enabled: true,
      });
      await loadData();
    } catch (error) {
      message.error(error.response?.data?.message || '创建规则失败');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (ruleId) => {
    try {
      await stockAPI.deleteAlertRule(ruleId);
      message.success('规则已删除');
      await loadData();
    } catch (error) {
      message.error(error.response?.data?.message || '删除失败');
    }
  };

  const handleAck = async (eventId) => {
    try {
      await stockAPI.ackAlertEvent(eventId);
      message.success('事件已确认');
      await loadData();
    } catch (error) {
      message.error(error.response?.data?.message || '确认失败');
    }
  };

  return (
    <div className="stock-page-grid">
      <section className="terminal-panel">
        <header>
          <h3>新建告警</h3>
        </header>
        <div className="stock-form-grid">
          <label>
            <span>股票代码</span>
            <Input value={form.symbol} onChange={(event) => updateField('symbol', event.target.value)} />
          </label>
          <label>
            <span>名称</span>
            <Input value={form.name} onChange={(event) => updateField('name', event.target.value)} />
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
            <span>指标</span>
            <Select value={form.metric} onChange={(value) => updateField('metric', value)} options={metricOptions} />
          </label>
          <label>
            <span>条件</span>
            <Select value={form.op} onChange={(value) => updateField('op', value)} options={opOptions} />
          </label>
          <label>
            <span>阈值</span>
            <InputNumber value={form.threshold} onChange={(value) => updateField('threshold', value || 0)} />
          </label>
          <label>
            <span>冷却秒数</span>
            <InputNumber value={form.cooldown_seconds} min={60} onChange={(value) => updateField('cooldown_seconds', value || 300)} />
          </label>
          <label className="stock-form-switch">
            <span>启用</span>
            <Switch checked={form.enabled} onChange={(checked) => updateField('enabled', checked)} />
          </label>
        </div>
        <div className="stock-toolbar">
          <Button type="primary" loading={creating} onClick={handleCreate}>创建规则</Button>
        </div>
      </section>

      <section className="terminal-panel">
        <header>
          <h3>规则列表</h3>
          <Button className="stock-inline-button" size="small" icon={<ReloadOutlined />} loading={loading} onClick={loadData}>
            刷新
          </Button>
        </header>
        {rules.length === 0 ? (
          <div className="stock-empty-wrap"><Empty description="暂无告警规则" /></div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>代码</th>
                <th>指标</th>
                <th>条件</th>
                <th>阈值</th>
                <th>冷却</th>
                <th>启用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((item) => (
                <tr key={item.rule_id}>
                  <td>{item.symbol}</td>
                  <td>{item.metric}</td>
                  <td>{item.op}</td>
                  <td>{formatNumber(item.threshold, 3)}</td>
                  <td>{item.cooldown_seconds}s</td>
                  <td>{item.enabled ? '是' : '否'}</td>
                  <td>
                    <Popconfirm title="确认删除该规则？" onConfirm={() => handleDelete(item.rule_id)}>
                      <button type="button" className="stock-text-button"><DeleteOutlined /> 删除</button>
                    </Popconfirm>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="terminal-panel stock-page-span-2">
        <header>
          <h3>告警事件</h3>
        </header>
        {events.length === 0 ? (
          <div className="stock-empty-wrap"><Empty description="暂无告警事件" /></div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>代码</th>
                <th>指标</th>
                <th>触发值</th>
                <th>说明</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {events.map((item) => (
                <tr key={item.event_id}>
                  <td>{item.triggered_at?.slice(0, 16).replace('T', ' ') || '-'}</td>
                  <td>{item.symbol}</td>
                  <td>{item.metric}</td>
                  <td>{formatNumber(item.value, 3)}</td>
                  <td>{item.message || '-'}</td>
                  <td>{item.status || '-'}</td>
                  <td>
                    {item.status === 'open' ? (
                      <button type="button" className="stock-text-button" onClick={() => handleAck(item.event_id)}>确认</button>
                    ) : '已确认'}
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

export default StockAlertsPage;

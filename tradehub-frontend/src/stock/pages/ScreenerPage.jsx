import React, { useEffect, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  Col,
  Empty,
  Input,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  DeleteOutlined,
  FilterOutlined,
  FundViewOutlined,
  PlusOutlined,
  SaveOutlined,
  SearchOutlined,
  StarOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { screenerApi, taskApi, watchlistApi } from '../api/stockApi';
import { formatAmount, formatChangePercent, formatPrice, getChangeBgColor, getChangeColor, getMarketName } from '../utils';

const { Text, Title } = Typography;

const INDICATOR_OPTIONS = [
  { code: 'MACD', name: 'MACD', fields: [{ key: 'macd', label: 'MACD值' }, { key: 'dif', label: 'DIF' }, { key: 'dea', label: 'DEA' }] },
  { code: 'KDJ', name: 'KDJ', fields: [{ key: 'kdjk', label: 'K值' }, { key: 'kdjd', label: 'D值' }, { key: 'kdjj', label: 'J值' }] },
  { code: 'RSI', name: 'RSI', fields: [{ key: 'rsi_6', label: 'RSI6' }, { key: 'rsi_12', label: 'RSI12' }] },
  { code: 'BOLL', name: 'BOLL', fields: [{ key: 'boll', label: '中轨' }, { key: 'boll_ub', label: '上轨' }, { key: 'boll_lb', label: '下轨' }] },
  { code: 'MA', name: 'MA', fields: [{ key: 'ma5', label: 'MA5' }, { key: 'ma10', label: 'MA10' }, { key: 'ma20', label: 'MA20' }] },
];

const PATTERN_OPTIONS = [
  { code: 'engulfing_pattern', name: '吞噬模式' },
  { code: 'hammer', name: '锤头' },
  { code: 'morning_star', name: '晨星' },
  { code: 'three_white_soldiers', name: '三个白兵' },
  { code: 'doji', name: '十字星' },
  { code: 'shooting_star', name: '射击之星' },
];

const OPERATORS = [
  { value: 'gt', label: '>' },
  { value: 'gte', label: '>=' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '<=' },
  { value: 'eq', label: '=' },
];

const PATTERN_DIRECTIONS = [
  { value: 'bullish', label: '看涨' },
  { value: 'bearish', label: '看跌' },
  { value: 'neutral', label: '中性' },
];

const buildConditionLabel = (condition) => {
  if (condition.type === 'indicator') {
    const indicator = INDICATOR_OPTIONS.find((item) => item.code === condition.indicator_code);
    const field = indicator?.fields.find((item) => item.key === condition.field);
    return `${indicator?.name || condition.indicator_code} · ${field?.label || condition.field}`;
  }
  const pattern = PATTERN_OPTIONS.find((item) => item.code === condition.pattern_code);
  return `${pattern?.name || condition.pattern_code} · ${condition.direction || '任意方向'}`;
};

const normalizeResultItem = (item) => {
  if (!item) return null;
  return {
    symbol: item.symbol || item.code,
    name: item.name || item.stock_name || '-',
    market: item.market || item.board || 'CN-A',
    price: item.price || item.latest || item.last_price,
    change_percent: item.change_percent ?? item.pct_change ?? 0,
    amount: item.amount || item.turnover || 0,
    turnover_rate: item.turnover_rate,
    pe: item.pe || item.pe_ratio,
    market_cap: item.market_cap,
    score: Number(item.score || 0),
  };
};

const mergeConditionResults = (conditionResults, logic) => {
  if (conditionResults.length === 0) return [];

  const mapList = conditionResults.map((result, index) => {
    const map = new Map();
    result.forEach((item) => {
      if (!item?.symbol) return;
      map.set(item.symbol, { ...item, _matchCount: 1, _conditionIndexes: [index] });
    });
    return map;
  });

  if (logic === 'or') {
    const merged = new Map();
    mapList.forEach((map, index) => {
      map.forEach((item, symbol) => {
        if (!merged.has(symbol)) {
          merged.set(symbol, { ...item, _matchCount: 1, _conditionIndexes: [index] });
          return;
        }
        const current = merged.get(symbol);
        merged.set(symbol, {
          ...current,
          ...item,
          _matchCount: current._matchCount + 1,
          _conditionIndexes: [...new Set([...current._conditionIndexes, index])],
        });
      });
    });
    return Array.from(merged.values())
      .map((item) => ({ ...item, score: item._matchCount / conditionResults.length }))
      .sort((a, b) => b.score - a.score || Number(b.change_percent || 0) - Number(a.change_percent || 0));
  }

  const base = mapList[0];
  const merged = [];
  base.forEach((item, symbol) => {
    const existsInAll = mapList.every((map) => map.has(symbol));
    if (!existsInAll) return;
    merged.push({
      ...item,
      score: 1,
      _matchCount: conditionResults.length,
    });
  });
  return merged.sort((a, b) => Number(b.change_percent || 0) - Number(a.change_percent || 0));
};

const ScreenerPage = () => {
  const navigate = useNavigate();
  const [logic, setLogic] = useState('and');
  const [conditions, setConditions] = useState([]);
  const [results, setResults] = useState([]);
  const [templates, setTemplates] = useState([]);
  const [templateName, setTemplateName] = useState('');
  const [templateDesc, setTemplateDesc] = useState('');
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [screening, setScreening] = useState(false);
  const [activeTaskId, setActiveTaskId] = useState('');

  const addIndicatorCondition = (indicator, field) => {
    setConditions((current) => current.concat({
      id: `${indicator.code}_${field.key}_${Date.now()}`,
      type: 'indicator',
      indicator_code: indicator.code,
      field: field.key,
      op: 'gt',
      threshold: 0,
      period: '1d',
    }));
  };

  const addPatternCondition = (pattern) => {
    setConditions((current) => current.concat({
      id: `${pattern.code}_${Date.now()}`,
      type: 'pattern',
      pattern_code: pattern.code,
      direction: 'bullish',
      period: '1d',
    }));
  };

  const updateCondition = (id, updates) => {
    setConditions((current) => current.map((item) => (item.id === id ? { ...item, ...updates } : item)));
  };

  const removeCondition = (id) => {
    setConditions((current) => current.filter((item) => item.id !== id));
  };

  const loadTemplates = async () => {
    setLoadingTemplates(true);
    try {
      const res = await screenerApi.getTemplates();
      setTemplates(res.items || []);
    } catch (err) {
      message.error(err.message || '加载模板失败');
      setTemplates([]);
    } finally {
      setLoadingTemplates(false);
    }
  };

  useEffect(() => {
    loadTemplates();
  }, []);

  const executeScreening = async () => {
    if (conditions.length === 0) {
      message.warning('请至少添加一个筛选条件');
      return;
    }

    setScreening(true);
    try {
      const payload = {
        logic,
        indicator_conditions: conditions
          .filter((item) => item.type === 'indicator')
          .map(({ period, indicator_code, field, op, threshold }) => ({ period, indicator_code, field, op, threshold })),
        pattern_conditions: conditions
          .filter((item) => item.type === 'pattern')
          .map(({ period, pattern_code, direction }) => ({ period, pattern_code, direction })),
      };

      try {
        const taskRes = await screenerApi.createScreeningTask(payload, 300);
        const task = taskRes.item;
        if (!task?.task_id) {
          throw new Error('筛选任务创建失败');
        }
        setActiveTaskId(task.task_id);

        const deadline = Date.now() + 30000;
        while (Date.now() < deadline) {
          const statusRes = await taskApi.getTasks({ task_id: task.task_id });
          const currentTask = statusRes.item;
          if (!currentTask) break;
          if (currentTask.status === 'succeeded') {
            const resultRes = await screenerApi.getResults(task.task_id, '', 300);
            const nextResults = (resultRes.items || []).map((item) => {
              let snapshot = {};
              try {
                snapshot = JSON.parse(item.snapshot_json || '{}');
              } catch {
                snapshot = {};
              }
              return normalizeResultItem({
                symbol: item.symbol,
                score: item.score,
                ...snapshot,
              });
            }).filter(Boolean);
            setResults(nextResults);
            message.success(`筛选完成，共 ${nextResults.length} 只标的`);
            setScreening(false);
            return;
          }
          if (currentTask.status === 'failed' || currentTask.status === 'cancelled') {
            throw new Error(currentTask.last_error || `筛选任务${currentTask.status}`);
          }
          await new Promise((resolve) => window.setTimeout(resolve, 1200));
        }
        throw new Error('筛选任务等待超时');
      } catch (taskErr) {
        const conditionResults = await Promise.all(
          conditions.map(async (condition) => {
            if (condition.type === 'indicator') {
              const res = await screenerApi.screenByIndicator({
                period: condition.period || '1d',
                indicator_code: condition.indicator_code,
                field: condition.field,
                op: condition.op,
                threshold: condition.threshold,
                limit: 300,
              });
              return (res.items || []).map(normalizeResultItem).filter(Boolean);
            }
            const res = await screenerApi.screenByPattern({
              period: condition.period || '1d',
              pattern_code: condition.pattern_code,
              direction: condition.direction,
              limit: 300,
            });
            return (res.items || []).map(normalizeResultItem).filter(Boolean);
          })
        );

        const merged = mergeConditionResults(conditionResults, logic);
        setResults(merged);
        setActiveTaskId('');
        message.warning(`已回退到前端组合筛选: ${taskErr.message || '任务流不可用'}`);
      }
    } catch (err) {
      message.error(err.message || '筛选失败');
      setResults([]);
    } finally {
      setScreening(false);
    }
  };

  const saveTemplate = async () => {
    if (!templateName.trim()) {
      message.warning('请输入模板名称');
      return;
    }
    if (conditions.length === 0) {
      message.warning('当前没有可保存的条件');
      return;
    }
    try {
      await screenerApi.createTemplate({
        name: templateName.trim(),
        description: templateDesc.trim(),
        conditions: {
          logic,
          indicator_conditions: conditions
            .filter((item) => item.type === 'indicator')
            .map(({ period, indicator_code, field, op, threshold }) => ({ period, indicator_code, field, op, threshold })),
          pattern_conditions: conditions
            .filter((item) => item.type === 'pattern')
            .map(({ period, pattern_code, direction }) => ({ period, pattern_code, direction })),
        },
        enabled: true,
      });
      setTemplateName('');
      setTemplateDesc('');
      message.success('模板已保存');
      loadTemplates();
    } catch (err) {
      message.error(err.message || '保存模板失败');
    }
  };

  const loadTemplate = (template) => {
    try {
      const parsed = JSON.parse(template.conditions_json || '{}');
      const indicatorConditions = (parsed.indicator_conditions || []).map((item, index) => ({
        id: `indicator_tpl_${index}_${Date.now()}`,
        type: 'indicator',
        ...item,
      }));
      const patternConditions = (parsed.pattern_conditions || []).map((item, index) => ({
        id: `pattern_tpl_${index}_${Date.now()}`,
        type: 'pattern',
        ...item,
      }));
      setLogic(parsed.logic || 'and');
      setConditions([...indicatorConditions, ...patternConditions]);
      message.success(`已载入模板: ${template.name}`);
    } catch (err) {
      message.error('模板解析失败');
    }
  };

  const addToWatchlist = async (record) => {
    try {
      await watchlistApi.addItem({
        group_id: 'default',
        symbol: record.symbol,
        name: record.name,
        market: record.market,
      });
      message.success(`已添加 ${record.symbol} 到自选`);
    } catch (err) {
      message.error(err.message || '加入自选失败');
    }
  };

  const columns = [
    {
      title: '标的',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 180,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Space>
            <Text strong className="stock-symbol">{record.symbol}</Text>
            <Tag className="stock-market-tag">{getMarketName(record.market)}</Tag>
          </Space>
          <Text className="stock-name">{record.name}</Text>
        </Space>
      ),
    },
    {
      title: '最新价',
      dataIndex: 'price',
      key: 'price',
      align: 'right',
      render: (value, record) => <Text style={{ color: getChangeColor(record.change_percent) }}>{formatPrice(value)}</Text>,
    },
    {
      title: '涨跌幅',
      dataIndex: 'change_percent',
      key: 'change_percent',
      align: 'right',
      render: (value) => (
        <Tag style={{ border: 'none', background: getChangeBgColor(value), color: getChangeColor(value) }}>
          {formatChangePercent(value)}
        </Tag>
      ),
    },
    {
      title: '成交额',
      dataIndex: 'amount',
      key: 'amount',
      align: 'right',
      render: (value) => formatAmount(value),
    },
    {
      title: '命中分数',
      dataIndex: 'score',
      key: 'score',
      align: 'right',
      render: (value) => `${((Number(value) || 0) * 100).toFixed(0)}%`,
    },
    {
      title: '操作',
      key: 'actions',
      width: 160,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" icon={<FundViewOutlined />} onClick={() => navigate(`/stock/kline?symbol=${record.symbol}`)}>
            K线
          </Button>
          <Button type="link" size="small" icon={<PlusOutlined />} onClick={() => addToWatchlist(record)}>
            自选
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="stock-page stock-layout">
      <div className="stock-page-header">
        <Title level={4} style={{ margin: 0, color: '#122033' }}>
          量化选股
        </Title>
        <Space>
          <Button icon={<SaveOutlined />} onClick={saveTemplate} disabled={conditions.length === 0}>
            保存模板
          </Button>
          <Button type="primary" icon={<SearchOutlined />} loading={screening} onClick={executeScreening}>
            执行筛选
          </Button>
        </Space>
      </div>

      <div className="stock-content">
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={7}>
            <Card className="stock-card" title={<Space><FilterOutlined /><span>条件配置</span></Space>}>
              <div style={{ marginBottom: 16 }}>
                <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>组合逻辑</Text>
                <Select
                  value={logic}
                  onChange={setLogic}
                  options={[
                    { value: 'and', label: 'AND 同时满足' },
                    { value: 'or', label: 'OR 满足其一' },
                  ]}
                  style={{ width: '100%' }}
                />
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div>
                  <Text strong>技术指标</Text>
                  <div style={{ marginTop: 8 }}>
                    {INDICATOR_OPTIONS.map((indicator) => (
                      <div key={indicator.code} style={{ marginBottom: 10 }}>
                        <Text type="secondary">{indicator.name}</Text>
                        <div style={{ marginTop: 6, display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                          {indicator.fields.map((field) => (
                            <Button
                              key={field.key}
                              size="small"
                              onClick={() => addIndicatorCondition(indicator, field)}
                            >
                              {field.label}
                            </Button>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <Text strong>K线形态</Text>
                  <div style={{ marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {PATTERN_OPTIONS.map((item) => (
                      <Button key={item.code} size="small" onClick={() => addPatternCondition(item)}>
                        {item.name}
                      </Button>
                    ))}
                  </div>
                </div>
              </div>
            </Card>

            <Card
              className="stock-card"
              title={<Space><span>已选条件</span><Badge count={conditions.length} /></Space>}
              style={{ marginTop: 16 }}
            >
              {conditions.length === 0 ? (
                <Empty description="先从左侧添加筛选条件" />
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  {conditions.map((condition) => (
                    <div key={condition.id} style={{ padding: 12, border: '1px solid #d8e2ee', borderRadius: 10, background: '#f7f9fc' }}>
                      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                        <Tag color="blue">{condition.type === 'indicator' ? '指标' : '形态'}</Tag>
                        <Button type="text" danger icon={<DeleteOutlined />} onClick={() => removeCondition(condition.id)} />
                      </Space>
                      <Text strong style={{ display: 'block', marginTop: 4 }}>{buildConditionLabel(condition)}</Text>
                      <Space wrap style={{ marginTop: 10 }}>
                        <Select
                          size="small"
                          value={condition.period || '1d'}
                          onChange={(value) => updateCondition(condition.id, { period: value })}
                          options={[
                            { value: '1d', label: '日线' },
                            { value: '1w', label: '周线' },
                            { value: '1m', label: '月线' },
                          ]}
                          style={{ width: 90 }}
                        />
                        {condition.type === 'indicator' ? (
                          <>
                            <Select
                              size="small"
                              value={condition.op}
                              onChange={(value) => updateCondition(condition.id, { op: value })}
                              options={OPERATORS}
                              style={{ width: 80 }}
                            />
                            <Input
                              size="small"
                              value={condition.threshold}
                              onChange={(event) => updateCondition(condition.id, { threshold: Number(event.target.value) || 0 })}
                              style={{ width: 88 }}
                            />
                          </>
                        ) : (
                          <Select
                            size="small"
                            value={condition.direction}
                            onChange={(value) => updateCondition(condition.id, { direction: value })}
                            options={PATTERN_DIRECTIONS}
                            style={{ width: 100 }}
                          />
                        )}
                      </Space>
                    </div>
                  ))}
                </div>
              )}
            </Card>

            <Card className="stock-card" title={<Space><StarOutlined /><span>模板</span></Space>} style={{ marginTop: 16 }}>
              <Input
                placeholder="模板名称"
                value={templateName}
                onChange={(event) => setTemplateName(event.target.value)}
                style={{ marginBottom: 8 }}
              />
              <Input.TextArea
                rows={3}
                placeholder="模板说明"
                value={templateDesc}
                onChange={(event) => setTemplateDesc(event.target.value)}
                style={{ marginBottom: 12 }}
              />
              <Button block icon={<SaveOutlined />} onClick={saveTemplate} disabled={conditions.length === 0}>
                保存当前条件
              </Button>
              <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
                {loadingTemplates ? (
                  <Text type="secondary">模板加载中…</Text>
                ) : templates.length === 0 ? (
                  <Text type="secondary">暂无模板</Text>
                ) : (
                  templates.map((template) => (
                    <Card
                      key={template.template_id}
                      size="small"
                      hoverable
                      onClick={() => loadTemplate(template)}
                      bodyStyle={{ padding: 12 }}
                    >
                      <Text strong>{template.name}</Text>
                      {template.description ? (
                        <Text type="secondary" style={{ display: 'block', marginTop: 4 }}>{template.description}</Text>
                      ) : null}
                    </Card>
                  ))
                )}
              </div>
            </Card>
          </Col>

          <Col xs={24} xl={17}>
            <Card className="stock-card" title="筛选结果">
              <div style={{ marginBottom: 12, color: '#62748a', fontSize: 12 }}>
                {activeTaskId ? `当前任务: ${activeTaskId}` : '优先使用 screening 任务流；任务不可用时回退为前端组合筛选。'}
              </div>
              <Table
                rowKey="symbol"
                loading={screening}
                dataSource={results}
                columns={columns}
                pagination={{ pageSize: 12 }}
                locale={{ emptyText: conditions.length === 0 ? '添加条件后执行筛选' : '暂无命中结果' }}
              />
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default ScreenerPage;

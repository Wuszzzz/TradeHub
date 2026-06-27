/**
 * KLinePage - 专业级K线分析页面
 * 功能：K线图、技术指标叠加、形态识别、筹码分布
 */

import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import {
  Layout,
  Card,
  Row,
  Col,
  Button,
  Select,
  Space,
  Typography,
  Input,
  Tag,
  Divider,
  Spin,
  Empty,
  Tooltip,
  Segmented,
  Badge,
  Statistic,
} from 'antd';
import {
  ZoomInOutlined,
  ZoomOutOutlined,
  FullscreenOutlined,
  ReloadOutlined,
  SettingOutlined,
  LineChartOutlined,
  BarChartOutlined,
} from '@ant-design/icons';
import ReactECharts from 'echarts-for-react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { marketApi, quantApi } from '../api/stockApi';
import {
  formatPrice,
  formatChangePercent,
  formatVolume,
  formatDateTime,
  formatDate,
  getChangeColor,
  getChangeBgColor,
  getPatternDirection,
  getPatternDirectionColor,
  getPatternName,
  debounce,
} from '../utils';

const { Text, Title } = Typography;

const PERIOD_OPTIONS = [
  { value: '1m', label: '1分钟' },
  { value: '5m', label: '5分钟' },
  { value: '15m', label: '15分钟' },
  { value: '30m', label: '30分钟' },
  { value: '1h', label: '1小时' },
  { value: '1d', label: '日线' },
  { value: '1w', label: '周线' },
  { value: '1M', label: '月线' },
];

const INDICATOR_OPTIONS = [
  { value: 'MA', label: 'MA均线', params: { windows: [5, 10, 20, 30, 60] } },
  { value: 'EMA', label: 'EMA指数均线', params: { windows: [5, 10, 12, 20, 26, 60] } },
  { value: 'MACD', label: 'MACD', params: { fast: 12, slow: 26, signal: 9 } },
  { value: 'KDJ', label: 'KDJ', params: { n: 9, m1: 3, m2: 3 } },
  { value: 'BOLL', label: 'BOLL布林线', params: { window: 20, std: 2 } },
  { value: 'RSI', label: 'RSI相对强弱', params: { windows: [6, 12, 24] } },
  { value: 'WR', label: 'WR威廉指标', params: { windows: [10, 14] } },
  { value: 'CCI', label: 'CCI顺势指标', params: { window: 14 } },
  { value: 'DMI', label: 'DMI动向指标', params: {} },
  { value: 'OBV', label: 'OBV能量潮', params: {} },
  { value: 'SAR', label: 'SAR抛物线', params: {} },
  { value: 'VR', label: 'VR容量比率', params: {} },
];

const KLinePage = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const chartRef = useRef(null);

  // 状态
  const [symbol, setSymbol] = useState(searchParams.get('symbol') || '000001');
  const [period, setPeriod] = useState('1d');
  const [klineData, setKlineData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [indicators, setIndicators] = useState(['MA']);
  const [indicatorData, setIndicatorData] = useState({});
  const [snapshot, setSnapshot] = useState(null);
  const [selectedIndicator, setSelectedIndicator] = useState('MA');
  const [patternHits, setPatternHits] = useState([]);

  // 加载K线数据
  const loadKlineData = useCallback(async () => {
    if (!symbol) return;
    setLoading(true);
    try {
      const res = await marketApi.kline(symbol, period, 500);
      const bars = res.data?.bars || res.bars || [];
      setKlineData(bars);

      // 获取实时快照
      try {
        const snap = await marketApi.snapshot(symbol);
        setSnapshot(snap.data || snap);
      } catch (e) {
        console.error('快照加载失败', e);
      }

      // 获取K线形态
      try {
        const patterns = await quantApi.getPatternHits(symbol, period, null, 50);
        setPatternHits(patterns.data || []);
      } catch (e) {
        console.error('形态数据加载失败', e);
      }
    } catch (err) {
      console.error('K线数据加载失败', err);
    } finally {
      setLoading(false);
    }
  }, [symbol, period]);

  // 加载指标数据
  const loadIndicatorData = useCallback(async () => {
    if (!symbol || indicators.length === 0) return;
    try {
      const results = {};
      for (const ind of indicators) {
        try {
          const res = await quantApi.getIndicatorValues(symbol, ind, period, 200);
          results[ind] = res.data || [];
        } catch (e) {
          console.error(`${ind} 指标加载失败`, e);
        }
      }
      setIndicatorData(results);
    } catch (err) {
      console.error('指标数据加载失败', err);
    }
  }, [symbol, period, indicators]);

  // 初始化
  useEffect(() => {
    loadKlineData();
  }, [loadKlineData]);

  useEffect(() => {
    loadIndicatorData();
  }, [loadIndicatorData]);

  // 处理symbol变化
  const handleSymbolChange = (value) => {
    setSymbol(value);
    setSearchParams({ symbol: value });
  };

  // K线图配置
  const getKlineOption = useMemo(() => {
    if (!klineData || klineData.length === 0) return null;

    const dates = klineData.map((bar) => bar.ts || bar.date);
    const opens = klineData.map((bar) => bar.open);
    const closes = klineData.map((bar) => bar.close);
    const lows = klineData.map((bar) => bar.low);
    const highs = klineData.map((bar) => bar.high);
    const volumes = klineData.map((bar) => bar.volume || 0);

    // 颜色处理：红涨绿跌
    const candleColors = closes.map((close, i) =>
      i === 0 ? (close >= opens[i] ? '#ee4444' : '#00a54c')
        : (close >= closes[i - 1] ? '#ee4444' : '#00a54c')
    );

    const option = {
      backgroundColor: 'transparent',
      animation: false,
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
        backgroundColor: 'rgba(255, 255, 255, 0.96)',
        borderColor: '#d8e2ee',
        textStyle: { color: '#122033', fontSize: 12 },
        formatter: (params) => {
          if (!params || params.length === 0) return '';
          const candle = params.find((p) => p.seriesType === 'candlestick');
          if (!candle) return '';
          const [open, close, low, high] = candle.data;
          const date = candle.axisValue;
          const change = ((close - opens[0]) / opens[0] * 100).toFixed(2);
          return `
            <div style="font-family: 'SF Mono', Consolas, monospace;">
              <div style="margin-bottom: 4px; color: #62748a;">${date}</div>
              <div>开: <span style="color: #122033;">${open?.toFixed(2)}</span></div>
              <div>高: <span style="color: #ee4444;">${high?.toFixed(2)}</span></div>
              <div>低: <span style="color: #00a54c;">${low?.toFixed(2)}</span></div>
              <div>收: <span style="color: ${close >= open ? '#ee4444' : '#00a54c'};">${close?.toFixed(2)}</span></div>
              <div style="margin-top: 4px; color: ${change >= 0 ? '#ee4444' : '#00a54c'};">
                涨跌: ${change >= 0 ? '+' : ''}${change}%
              </div>
            </div>
          `;
        },
      },
      legend: {
        show: true,
        bottom: 10,
        textStyle: { color: '#62748a' },
        data: ['K线', ...indicators],
      },
      grid: [
        { left: 60, right: 20, top: 40, height: '55%' },
        { left: 60, right: 20, top: '70%', height: '15%' },
        { left: 60, right: 20, top: '88%', height: '12%' },
      ],
      xAxis: [
        {
          type: 'category',
          data: dates,
          gridIndex: 0,
          axisLine: { lineStyle: { color: '#d8e2ee' } },
          axisTick: { show: false },
          axisLabel: { color: '#62748a', fontSize: 10 },
          splitLine: { show: false },
        },
        {
          type: 'category',
          data: dates,
          gridIndex: 1,
          axisLine: { lineStyle: { color: '#d8e2ee' } },
          axisTick: { show: false },
          axisLabel: { show: false },
          splitLine: { show: false },
        },
        {
          type: 'category',
          data: dates,
          gridIndex: 2,
          axisLine: { lineStyle: { color: '#d8e2ee' } },
          axisTick: { show: false },
          axisLabel: { color: '#62748a', fontSize: 10 },
          splitLine: { show: false },
        },
      ],
      yAxis: [
        {
          scale: true,
          gridIndex: 0,
          position: 'left',
          axisLine: { lineStyle: { color: '#d8e2ee' } },
          axisLabel: { color: '#62748a', fontSize: 10, formatter: (v) => v.toFixed(2) },
          splitLine: { lineStyle: { color: '#e1e8f2', type: 'dashed' } },
        },
        {
          scale: true,
          gridIndex: 1,
          position: 'left',
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { show: false },
          splitLine: { show: false },
        },
        {
          scale: true,
          gridIndex: 2,
          position: 'left',
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: '#62748a', fontSize: 10, formatter: (v) => formatVolume(v) },
          splitLine: { show: false },
        },
      ],
      dataZoom: [
        {
          type: 'inside',
          xAxisIndex: [0, 1, 2],
          start: 70,
          end: 100,
        },
        {
          type: 'slider',
          xAxisIndex: [0, 1, 2],
          bottom: 0,
          height: 20,
          borderColor: '#d8e2ee',
          backgroundColor: '#f7f9fc',
          fillerColor: 'rgba(15, 111, 255, 0.16)',
          handleStyle: { color: '#0f6fff' },
          textStyle: { color: '#62748a' },
        },
      ],
      series: [
        // K线
        {
          name: 'K线',
          type: 'candlestick',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: klineData.map((bar) => [bar.open, bar.close, bar.low, bar.high]),
          itemStyle: {
            color: '#ee4444',
            color0: '#00a54c',
            borderColor: '#ee4444',
            borderColor0: '#00a54c',
          },
        },
        // 成交量
        {
          name: '成交量',
          type: 'bar',
          xAxisIndex: 2,
          yAxisIndex: 2,
          data: klineData.map((bar, i) => ({
            value: bar.volume || 0,
            itemStyle: {
              color: candleColors[i],
            },
          })),
          barMaxWidth: 8,
        },
      ],
    };

    // 添加指标
    const indicatorColors = {
      MA: ['#0f6fff', '#ff8a00', '#7c3aed', '#00a54c', '#ef476f', '#8a6d3b'],
      EMA: ['#ff6b00', '#00bcd4', '#e91e63', '#9c27b0', '#4caf50', '#ff9800'],
      MACD: { DIF: '#1890ff', DEA: '#ff6b00', BAR: '#ee4444' },
      KDJ: { K: '#ff6b00', D: '#00bcd4', J: '#e91e63' },
      BOLL: ['#ff6b00', '#00bcd4', '#e91e63'],
    };

    // 处理MA/EMA
    if (indicatorData.MA || indicatorData.EMA) {
      const data = indicatorData.MA || indicatorData.EMA;
      const prefix = indicatorData.EMA ? 'ema' : 'ma';
      const colors = indicatorColors[indicatorData.EMA ? 'EMA' : 'MA'];
      const windows = indicatorData.EMA ? [5, 10, 12, 20, 26, 60] : [5, 10, 20, 30, 60];

      windows.forEach((w, i) => {
        option.series.push({
          name: `${prefix.toUpperCase()}${w}`,
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: data.map((d) => d.values?.[`${prefix}${w}`] || null),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: colors[i], width: 1 },
        });
      });
    }

    // 处理MACD
    if (indicatorData.MACD) {
      const macdData = indicatorData.MACD;
      option.series.push(
        {
          name: 'MACD',
          type: 'line',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: macdData.map((d) => d.values?.dif),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.MACD.DIF, width: 1 },
        },
        {
          name: 'DEA',
          type: 'line',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: macdData.map((d) => d.values?.dea),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.MACD.DEA, width: 1 },
        }
      );
    }

    // 处理KDJ
    if (indicatorData.KDJ) {
      const kdjData = indicatorData.KDJ;
      option.series.push(
        {
          name: 'K',
          type: 'line',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: kdjData.map((d) => d.values?.kdjk),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.KDJ.K, width: 1 },
        },
        {
          name: 'D',
          type: 'line',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: kdjData.map((d) => d.values?.kdjd),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.KDJ.D, width: 1 },
        },
        {
          name: 'J',
          type: 'line',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: kdjData.map((d) => d.values?.kdjj),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.KDJ.J, width: 1, type: 'dashed' },
        }
      );
    }

    // 处理BOLL
    if (indicatorData.BOLL) {
      const bollData = indicatorData.BOLL;
      option.series.push(
        {
          name: 'BOLL_U',
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: bollData.map((d) => d.values?.boll_ub),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.BOLL[0], width: 1, type: 'dashed' },
        },
        {
          name: 'BOLL',
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: bollData.map((d) => d.values?.boll),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.BOLL[1], width: 1 },
        },
        {
          name: 'BOLL_L',
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: bollData.map((d) => d.values?.boll_lb),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: indicatorColors.BOLL[2], width: 1, type: 'dashed' },
        }
      );
    }

    // 处理RSI
    if (indicatorData.RSI) {
      const rsiData = indicatorData.RSI;
      const colors = ['#ff6b00', '#00bcd4', '#e91e63'];
      [6, 12, 24].forEach((w, i) => {
        option.series.push({
          name: `RSI${w}`,
          type: 'line',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: rsiData.map((d) => d.values?.[`rsi_${w}`]),
          smooth: false,
          symbol: 'none',
          lineStyle: { color: colors[i], width: 1 },
        });
      });
    }

    return option;
  }, [klineData, indicators, indicatorData]);

  // 获取最近形态
  const recentPatterns = useMemo(() => {
    return patternHits.slice(-10).reverse();
  }, [patternHits]);

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <Space size="large">
          <Space>
            <Text style={{ color: '#888' }}>股票代码:</Text>
            <Input
              value={symbol}
              onChange={(e) => setSymbol(e.target.value.toUpperCase())}
              onPressEnter={() => handleSymbolChange(symbol)}
              style={{ width: 120 }}
              placeholder="000001"
            />
            <Button type="primary" onClick={() => handleSymbolChange(symbol)}>
              确定
            </Button>
          </Space>

          <Select
            value={period}
            onChange={setPeriod}
            options={PERIOD_OPTIONS}
            style={{ width: 100 }}
          />

          <Select
            mode="multiple"
            placeholder="添加指标"
            value={indicators}
            onChange={setIndicators}
            options={INDICATOR_OPTIONS}
            style={{ minWidth: 200 }}
            maxTagCount={2}
          />
        </Space>

        <Space>
          <Button icon={<ReloadOutlined />} onClick={loadKlineData} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        <Row gutter={16}>
          {/* K线图 */}
          <Col span={18}>
            <Card
              className="stock-card"
              title={
                <Space>
                  <Text strong style={{ color: '#122033', fontSize: 16 }}>
                    {symbol}
                  </Text>
                  {snapshot && (
                    <>
                      <Text
                        strong
                        style={{
                          color: getChangeColor(snapshot.change_percent),
                          fontSize: 18,
                          fontFamily: 'SF Mono, Consolas, monospace',
                        }}
                      >
                        {formatPrice(snapshot.price)}
                      </Text>
                      <Tag
                        style={{
                          background: getChangeBgColor(snapshot.change_percent),
                          color: getChangeColor(snapshot.change_percent),
                          border: 'none',
                        }}
                      >
                        {formatChangePercent(snapshot.change_percent)}
                      </Tag>
                    </>
                  )}
                </Space>
              }
              extra={
                <Space>
                  <Text type="secondary">
                    {period === '1d' ? '日K' : period === '1w' ? '周K' : period === '1M' ? '月K' : period}
                  </Text>
                </Space>
              }
              bodyStyle={{ padding: 8 }}
            >
              {loading ? (
                <div className="stock-loading">
                  <Spin size="large" />
                </div>
              ) : klineData.length === 0 ? (
                <Empty description="暂无数据" />
              ) : (
                <ReactECharts
                  ref={chartRef}
                  option={getKlineOption}
                  style={{ height: 600 }}
                  notMerge={true}
                  opts={{ devicePixelRatio: window.devicePixelRatio }}
                />
              )}
            </Card>
          </Col>

          {/* 右侧信息 */}
          <Col span={6}>
            {/* 实时行情 */}
            <Card
              className="stock-card"
              title="实时行情"
              size="small"
              style={{ marginBottom: 16 }}
            >
              {snapshot ? (
                <div>
                  <Row gutter={[8, 8]}>
                    <Col span={12}>
                      <Statistic
                        title="今开"
                        value={snapshot.open}
                        precision={2}
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="昨收"
                        value={snapshot.prev_close}
                        precision={2}
                        valueStyle={{ color: '#888', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="最高"
                        value={snapshot.high}
                        precision={2}
                        valueStyle={{ color: '#ee4444', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="最低"
                        value={snapshot.low}
                        precision={2}
                        valueStyle={{ color: '#00a54c', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="成交量"
                        value={snapshot.volume}
                        formatter={(v) => formatVolume(v)}
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="成交额"
                        value={snapshot.amount}
                        formatter={(v) => formatAmount(v)}
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="换手率"
                        value={snapshot.turnover_rate || 0}
                        precision={2}
                        suffix="%"
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="量比"
                        value={snapshot.volume_ratio || 0}
                        precision={2}
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="市盈率"
                        value={snapshot.pe || 0}
                        precision={2}
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                    <Col span={12}>
                      <Statistic
                        title="市净率"
                        value={snapshot.pb || 0}
                        precision={2}
                        valueStyle={{ color: '#122033', fontSize: 14 }}
                      />
                    </Col>
                  </Row>
                </div>
              ) : (
                <Text type="secondary">加载中...</Text>
              )}
            </Card>

            {/* 最近形态 */}
            <Card
              className="stock-card"
              title={
                <Space>
                  <span>K线形态</span>
                  <Badge count={recentPatterns.length} />
                </Space>
              }
              size="small"
              style={{ marginBottom: 16 }}
              bodyStyle={{ padding: 8 }}
            >
              {recentPatterns.length > 0 ? (
                <div>
                  {recentPatterns.map((hit, i) => (
                    <div
                      key={i}
                      style={{
                        padding: '8px 4px',
                        borderBottom: i < recentPatterns.length - 1 ? '1px solid #d8e2ee' : 'none',
                      }}
                    >
                      <Space>
                        <Tag
                          color={getPatternDirectionColor(hit.direction)}
                          style={{ fontSize: 10 }}
                        >
                          {getPatternDirection(hit.direction)}
                        </Tag>
                        <Text style={{ color: '#ccc', fontSize: 12 }}>
                          {getPatternName(hit.pattern_code)}
                        </Text>
                      </Space>
                      <div style={{ marginTop: 4 }}>
                        <Text type="secondary" style={{ fontSize: 11 }}>
                          {formatDate(hit.ts)} · {hit.extra?.close?.toFixed(2)}
                        </Text>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <Text type="secondary" style={{ padding: 16, display: 'block', textAlign: 'center' }}>
                  暂无形态信号
                </Text>
              )}
            </Card>

            {/* 指标说明 */}
            <Card className="stock-card" title="指标说明" size="small">
              {indicators.map((ind) => (
                <div key={ind} style={{ marginBottom: 8 }}>
                  <Text strong style={{ color: '#122033' }}>
                    {INDICATOR_OPTIONS.find((i) => i.value === ind)?.label || ind}
                  </Text>
                  {indicatorData[ind] && indicatorData[ind].length > 0 && (
                    <div style={{ marginTop: 4 }}>
                      {Object.entries(indicatorData[ind][indicatorData[ind].length - 1]?.values || {})
                        .slice(0, 3)
                        .map(([key, value]) => (
                          <Text key={key} style={{ color: '#888', fontSize: 11, marginRight: 8 }}>
                            {key}: {typeof value === 'number' ? value.toFixed(2) : value}
                          </Text>
                        ))}
                    </div>
                  )}
                </div>
              ))}
              {indicators.length === 0 && (
                <Text type="secondary">从上方选择指标添加到图表</Text>
              )}
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default KLinePage;

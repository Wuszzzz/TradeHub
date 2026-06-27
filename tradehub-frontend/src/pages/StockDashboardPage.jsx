import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Empty, Input, Space, Tag, message } from 'antd';
import {
  AppstoreOutlined,
  AreaChartOutlined,
  BellOutlined,
  CameraOutlined,
  CloseOutlined,
  LineChartOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  StockOutlined,
  ToolOutlined,
} from '@ant-design/icons';
import { stockAPI } from '../api';

const formatMoney = (value) => {
  const n = Number(value || 0);
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

const formatPercent = (value) => {
  if (value === null || value === undefined || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`;
};

const fallbackWatchlist = [
  { symbol: 'MSFT', name: 'Microsoft Corp', market: 'NASDAQ', price: 406.32, pct_change: 0.26 },
  { symbol: 'AAPL', name: 'Apple', market: 'NASDAQ', price: 189.96, pct_change: 1.18 },
  { symbol: 'NVDA', name: 'Nvidia', market: 'NASDAQ', price: 435.12, pct_change: 5.85 },
  { symbol: 'GOOG', name: 'Alphabet', market: 'NASDAQ', price: 234.22, pct_change: 6.45 },
];

const newsRows = [
  ['零售销售走弱拖累市场，主要指数回落', '10 分钟前'],
  ['科技龙头财报超预期，股价刷新高点', '2 分钟前'],
  ['热门 IPO 表现不及预期，风险偏好降温', '12 小时前'],
  ['新能源车需求升温，相关股票快速拉升', '22 小时前'],
  ['市场情绪转向谨慎，成长板块承压', '2 小时前'],
  ['芯片供应扰动延续，科技股分化加剧', '3 天前'],
];

const gainers = [
  ['AAPL', '苹果', '125', '6.36%'],
  ['JPM', '摩根大通', '121', '21.75%'],
  ['UBER', '优步', '80', '3.84%'],
  ['NVDA', '英伟达', '435', '5.85%'],
  ['GOOG', '谷歌', '234', '6.45%'],
  ['MSFT', '微软', '436', '9.54%'],
  ['TGT', '塔吉特', '89', '11.85%'],
  ['NFLX', '奈飞', '123', '4.90%'],
  ['AMZN', '亚马逊', '467', '5.98%'],
  ['META', 'Meta', '123', '18.94%'],
];

const candles = [
  340, 358, 346, 372, 366, 351, 330, 338, 362, 357, 389, 381, 407, 392, 365, 350, 343, 334, 348,
  361, 354, 377, 394, 415, 431, 405, 392, 380, 401, 418, 427, 411, 421, 435, 428,
];

const heatMapTiles = [
  ['AAPL', 18, 18, 'up'], ['MSFT', 16, 10, 'up'], ['NVDA', 13, 18, 'up'], ['ADBE', 8, 8, 'up'],
  ['INTC', 8, 7, 'flat'], ['CSCO', 8, 7, 'flat'], ['CRM', 8, 7, 'flat'], ['JPM', 12, 10, 'down'],
  ['ABBV', 8, 7, 'down'], ['TMO', 7, 8, 'flat'], ['UNH', 12, 10, 'down'], ['DHR', 8, 8, 'down'],
  ['AMZN', 13, 11, 'flat'], ['HD', 8, 8, 'down'], ['MCD', 8, 8, 'down'], ['NKE', 8, 8, 'flat'],
  ['TSLA', 18, 10, 'down'], ['PYPL', 8, 8, 'up'], ['QCOM', 8, 8, 'flat'], ['AMD', 7, 7, 'down'],
];

function MiniLineChart() {
  const points = candles.map((value, index) => {
    const x = 18 + index * (664 / (candles.length - 1));
    const y = 300 - ((value - 320) / 125) * 245;
    return `${x},${y}`;
  }).join(' ');

  return (
    <div className="terminal-chart">
      <div className="chart-grid" />
      <svg viewBox="0 0 720 330" preserveAspectRatio="none" aria-hidden="true">
        <polyline className="chart-ma chart-ma-blue" points="20,240 110,230 200,236 290,226 380,214 470,188 560,158 700,116" />
        <polyline className="chart-ma chart-ma-yellow" points="20,270 130,262 240,258 350,252 460,246 560,205 700,178" />
        <polyline className="chart-price" points={points} />
        {candles.map((value, index) => {
          const x = 18 + index * (664 / (candles.length - 1));
          const y = 300 - ((value - 320) / 125) * 245;
          const up = index === 0 || value >= candles[index - 1];
          return (
            <g key={`${value}-${index}`} className={up ? 'candle up' : 'candle down'}>
              <line x1={x} x2={x} y1={Math.max(22, y - 21)} y2={Math.min(300, y + 24)} />
              <rect x={x - 3.5} y={Math.min(y, y + (up ? 14 : -14))} width="7" height="24" rx="1" />
            </g>
          );
        })}
      </svg>
      <div className="volume-bars">
        {candles.map((value, index) => (
          <span
            key={`${value}-${index}`}
            className={index % 3 === 0 ? 'down' : 'up'}
            style={{ height: `${18 + (value % 38)}px` }}
          />
        ))}
      </div>
    </div>
  );
}

function StockDashboardPage() {
  const [loading, setLoading] = useState(false);
  const [account, setAccount] = useState(null);
  const [positions, setPositions] = useState([]);
  const [watchlist, setWatchlist] = useState([]);
  const [moduleErrors, setModuleErrors] = useState([]);
  const [keyword, setKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [searchResults, setSearchResults] = useState([]);

  const loadDashboard = async () => {
    setLoading(true);
    setModuleErrors([]);
    try {
      const [accountRes, positionsRes, watchlistRes] = await Promise.allSettled([
        stockAPI.paperAccount(),
        stockAPI.paperPositions(),
        stockAPI.watchlistSnapshot('default'),
      ]);
      const errors = [];

      if (accountRes.status === 'fulfilled') {
        setAccount(accountRes.value.data?.item || accountRes.value.data || null);
      } else {
        setAccount(null);
        errors.push(`模拟账户：${accountRes.reason?.response?.data?.message || '加载失败'}`);
      }

      if (positionsRes.status === 'fulfilled') {
        setPositions(positionsRes.value.data?.items || []);
      } else {
        setPositions([]);
        errors.push(`模拟持仓：${positionsRes.reason?.response?.data?.message || '加载失败'}`);
      }

      if (watchlistRes.status === 'fulfilled') {
        setWatchlist(watchlistRes.value.data?.items || []);
      } else {
        setWatchlist([]);
        errors.push(`股票自选：${watchlistRes.reason?.response?.data?.message || '加载失败'}`);
      }

      setModuleErrors(errors);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadDashboard();
  }, []);

  const handleSearch = async () => {
    const q = keyword.trim();
    if (!q) {
      setSearchResults([]);
      return;
    }
    setSearching(true);
    try {
      const { data } = await stockAPI.search(q);
      setSearchResults(data.items || []);
    } catch (error) {
      message.error(error.response?.data?.message || '搜索标的失败');
    } finally {
      setSearching(false);
    }
  };

  const handleAddWatchlist = async (item) => {
    try {
      await stockAPI.addWatchlistItem({
        group_id: 'default',
        symbol: item.symbol,
        name: item.name || item.symbol,
        market: item.market || item.board || 'CN-A',
      });
      message.success('已加入股票自选');
      loadDashboard();
    } catch (error) {
      message.error(error.response?.data?.message || '加入自选失败');
    }
  };

  const accountStats = useMemo(() => {
    const cash = Number(account?.cash || 122912.5);
    const equity = Number(account?.equity || account?.total_equity || cash + 500185.67);
    const totalReturn = account?.total_return ?? account?.return_rate ?? 2.14;
    return { cash, equity, totalReturn };
  }, [account]);

  const displayWatchlist = watchlist.length > 0 ? watchlist : fallbackWatchlist;
  const featured = displayWatchlist[0] || fallbackWatchlist[0];
  const featuredPrice = Number(featured.price || 406.32);
  const featuredChange = Number(featured.pct_change ?? 0.26);

  return (
    <div className="stock-terminal-page">
      {moduleErrors.length > 0 && (
        <Alert
          className="terminal-alert"
          type="warning"
          showIcon
          message="股票后端部分模块暂不可用"
          description={moduleErrors.join('；')}
        />
      )}

      <div className="stock-terminal-grid">
        <main className="terminal-main-panel">
          <section className="terminal-ticker-card">
            <div className="terminal-security-tabs">
              {['图表', '期权', '新闻', '财务', '分析师', '风险分析', '公告', '笔记', '资料'].map((item) => (
                <button key={item} type="button" className={item === '图表' ? 'active' : ''}>{item}</button>
              ))}
              <Button className="terminal-refresh" size="small" icon={<ReloadOutlined />} loading={loading} onClick={loadDashboard}>
                刷新
              </Button>
            </div>

            <div className="terminal-stock-info">
              <div className="terminal-stock-lead">
                <div className="terminal-symbol-line">
                  <strong>{featured.symbol || 'MSFT'}</strong>
                  <span>{featured.name || 'Microsoft Corp'} {featured.market || 'NASDAQ'}</span>
                  <button type="button"><BellOutlined /></button>
                  <button type="button"><StockOutlined /></button>
                </div>
                <div className="terminal-price-line">
                  <span className="terminal-price">{featuredPrice.toFixed(2)}</span>
                  <span className={featuredChange >= 0 ? 'terminal-change up' : 'terminal-change down'}>
                    {formatPercent(featuredChange)}
                    <small>{featuredChange >= 0 ? '+2.24' : '-2.24'}</small>
                  </span>
                </div>
                <div className="terminal-after-hours">
                  盘后：<b>406.83</b> <span className="down">-0.27 -0.07%</span> | 19:59 04/26 EDT
                </div>
              </div>
              <div className="terminal-stat-columns">
                {[
                  ['开盘', '401.23'], ['最低', '400.10'], ['最高', '408.36'], ['52 周高', '430.82'], ['52 周低', '273.13'],
                  ['3 月均量', '21.73M'], ['流通股本', '7.43B'], ['市值', '3.02T'], ['股息率', '0.74%'], ['查看全部', '>'],
                ].map(([label, value]) => (
                  <div key={label}>
                    <span>{label}</span>
                    <strong>{value}</strong>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="terminal-chart-card">
            <div className="chart-meta-row">
              <span>开盘 <b>408.36</b></span>
              <span>最高 <b>408.36</b></span>
              <span>最低 <b>408.36</b></span>
              <span>收盘 <b>408.36</b></span>
              <span className="up">+8.90 +2.14%</span>
              <span>成交量 <b>56,254,781</b></span>
            </div>
            <div className="chart-tools-row">
              {[AreaChartOutlined, CloseOutlined, AppstoreOutlined, CameraOutlined, SearchOutlined, ToolOutlined].map((Icon, index) => (
                <button key={index} type="button"><Icon /></button>
              ))}
              <span>MA50:<b className="blue">406.98</b></span>
              <span>MA200:<b className="yellow">400.25</b></span>
            </div>
            <MiniLineChart />
            <div className="rsi-panel">
              <span>RSI (6,14,24)</span>
              <svg viewBox="0 0 700 105" preserveAspectRatio="none" aria-hidden="true">
                <polyline points="0,35 35,18 72,22 104,78 140,86 178,62 214,64 246,88 284,73 326,80 360,92 396,58 432,40 468,50 502,72 540,46 584,65 628,58 674,74 700,66" />
                <line x1="0" y1="66" x2="700" y2="66" />
              </svg>
            </div>
            <div className="timeframe-row">
              <span>周期：</span>
              {['1m', '5m', '15m', '30m', '1h', '2h', '4h', 'D', 'W', 'M', 'All', '2m'].map((item) => (
                <button key={item} type="button" className={item === '1h' ? 'active' : ''}>{item}</button>
              ))}
            </div>
          </section>
        </main>

        <aside className="terminal-right-rail">
          <section className="trade-ticket">
            <header>
              <h3>交易</h3>
              <AppstoreOutlined />
            </header>
            <div className="ticket-toggle">
              <button type="button" className="active">买入</button>
              <button type="button">卖出</button>
            </div>
            <label>
              <span>订单类型</span>
              <select defaultValue="市价单">
                <option>市价单</option>
                <option>限价单</option>
              </select>
            </label>
            <label>
              <span>数量 <b>股</b></span>
              <input defaultValue="100" />
            </label>
            <div className="quantity-buttons">
              {[10, 50, 100, 500].map((value) => <button key={value} type="button">{value}</button>)}
              <ToolOutlined />
            </div>
            <label>
              <span>有效期</span>
              <select defaultValue="当日有效">
                <option>当日有效</option>
                <option>撤销前有效</option>
              </select>
            </label>
            <label className="stop-price">
              <span><i /> 止损价</span>
              <input defaultValue="$ 400.00" />
              <small>预估亏损：<b>12,057.36</b></small>
            </label>
            <div className="ticket-summary">
              <span>购买力 <b>${formatMoney(accountStats.cash)}</b></span>
              <span>交易费用 <b>$4.00</b></span>
              <span>预估总额 <b>$40,000</b></span>
            </div>
            <button className="terminal-cta" type="button">买入 {featured.symbol || 'MSFT'}</button>
            <button className="ticket-link" type="button">免责声明 ›</button>
          </section>

          <section className="time-sales terminal-panel">
            <header>
              <h3>逐笔成交</h3>
              <AppstoreOutlined />
            </header>
            {Array.from({ length: 6 }).map((_, index) => (
              <div key={index} className="time-sales-row">
                <span>16:59:32</span>
                <span>420.56</span>
                <span>25</span>
              </div>
            ))}
          </section>
        </aside>
      </div>

      <section className="market-overview-grid">
        <div className="terminal-panel market-search-panel">
          <header>
            <h3>行情搜索</h3>
            <SearchOutlined />
          </header>
          <Space.Compact className="terminal-search-box">
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              onPressEnter={handleSearch}
              placeholder="输入股票、ETF 代码或名称"
            />
            <Button type="primary" icon={<SearchOutlined />} loading={searching} onClick={handleSearch}>
              搜索
            </Button>
          </Space.Compact>
          <div className="search-result-list">
            {searchResults.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="输入关键词开始搜索" />
            ) : searchResults.map((item) => (
              <div key={`${item.symbol}-${item.market}`} className="search-result-row">
                <span>
                  <b>{item.symbol}</b>
                  <small>{item.name || item.symbol}</small>
                </span>
                <Tag>{item.market || item.board || '-'}</Tag>
                <button type="button" onClick={() => handleAddWatchlist(item)}><PlusOutlined /> 加自选</button>
              </div>
            ))}
          </div>
        </div>

        <div className="terminal-panel heatmap-panel">
          <header>
            <h3>热力图</h3>
            <AppstoreOutlined />
          </header>
          <div className="heatmap-controls">
            <button type="button">热门</button>
            {['D', 'W', 'M', 'Y'].map((item) => <button key={item} type="button">{item}</button>)}
          </div>
          <div className="heatmap-tiles">
            {heatMapTiles.map(([symbol, w, h, trend]) => (
              <span
                key={symbol}
                className={trend}
                style={{ gridColumn: `span ${w}`, gridRow: `span ${h}` }}
              >
                {symbol}
              </span>
            ))}
          </div>
        </div>

        <div className="terminal-panel">
          <header>
            <h3>最新资讯</h3>
            <AppstoreOutlined />
          </header>
          <div className="terminal-list">
            {newsRows.map(([title, time]) => (
              <div key={title}>
                <span>{title}</span>
                <small>{time}</small>
              </div>
            ))}
          </div>
        </div>

        <div className="terminal-panel">
          <header>
            <h3>涨幅榜</h3>
            <AppstoreOutlined />
          </header>
          <table className="terminal-table">
            <thead>
              <tr><th>代码</th><th>名称</th><th>价格</th><th>涨跌幅</th></tr>
            </thead>
            <tbody>
              {gainers.map(([symbol, name, price, change]) => (
                <tr key={`${symbol}-${name}`}>
                  <td>{symbol}</td><td>{name}</td><td>{price}</td><td className="up">{change}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {positions.length > 0 && (
        <section className="terminal-panel positions-strip">
          <header>
            <h3>模拟持仓</h3>
            <LineChartOutlined />
          </header>
          <table className="terminal-table">
            <thead>
              <tr><th>代码</th><th>名称</th><th>数量</th><th>成本</th><th>现价</th><th>盈亏</th></tr>
            </thead>
            <tbody>
              {positions.map((item) => (
                <tr key={item.symbol}>
                  <td>{item.symbol}</td>
                  <td>{item.name || '-'}</td>
                  <td>{formatMoney(item.qty)}</td>
                  <td>{Number(item.avg_cost || 0).toFixed(3)}</td>
                  <td>{item.last_price ? Number(item.last_price).toFixed(3) : '-'}</td>
                  <td>{formatMoney(item.unrealized_pl)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  );
}

export default StockDashboardPage;

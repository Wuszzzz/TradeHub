import { useEffect, useState } from 'react';
import { Button, Empty, Input, Select, Tag, message } from 'antd';
import { PlusOutlined, SearchOutlined } from '@ant-design/icons';
import { stockAPI } from '../api';
import { formatNumber, formatPercent } from '../stock/utils';

function StockSearchPage() {
  const [groups, setGroups] = useState([]);
  const [groupId, setGroupId] = useState('default');
  const [keyword, setKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState([]);

  useEffect(() => {
    stockAPI.watchlistGroups().then(({ data }) => {
      const nextGroups = data.items || [];
      setGroups(nextGroups);
      if (nextGroups[0]) setGroupId(nextGroups[0].group_id);
    }).catch(() => {});
  }, []);

  const handleSearch = async () => {
    const q = keyword.trim();
    if (!q) {
      setResults([]);
      return;
    }
    setSearching(true);
    try {
      const { data } = await stockAPI.search(q);
      setResults(data.items || []);
    } catch (error) {
      message.error(error.response?.data?.message || '搜索失败');
    } finally {
      setSearching(false);
    }
  };

  const handleAdd = async (item) => {
    try {
      await stockAPI.addWatchlistItem({
        group_id: groupId || 'default',
        symbol: item.symbol,
        name: item.name || item.symbol,
        market: item.market || item.board || 'CN-A',
      });
      message.success('已加入股票自选');
    } catch (error) {
      message.error(error.response?.data?.message || '加入自选失败');
    }
  };

  return (
    <div className="stock-page-stack">
      <section className="terminal-panel">
        <header>
          <h3>行情搜索</h3>
          <Select
            value={groupId}
            onChange={setGroupId}
            options={groups.map((item) => ({ value: item.group_id, label: item.name }))}
            className="stock-toolbar-control"
          />
        </header>
        <div className="stock-toolbar">
          <Input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onPressEnter={handleSearch}
            placeholder="输入股票、ETF 代码或名称"
            className="stock-toolbar-control stock-toolbar-control-wide"
          />
          <Button type="primary" icon={<SearchOutlined />} loading={searching} onClick={handleSearch}>
            搜索
          </Button>
        </div>
        {results.length === 0 ? (
          <div className="stock-empty-wrap"><Empty description="输入关键字开始搜索" /></div>
        ) : (
          <table className="terminal-table">
            <thead>
              <tr>
                <th>代码</th>
                <th>名称</th>
                <th>市场</th>
                <th>最新价</th>
                <th>涨跌幅</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {results.map((item) => (
                <tr key={`${item.symbol}-${item.market || item.board}`}>
                  <td>{item.symbol}</td>
                  <td>{item.name || '-'}</td>
                  <td><Tag>{item.market || item.board || '-'}</Tag></td>
                  <td>{formatNumber(item.price || item.latest || item.last_price, 3)}</td>
                  <td className={Number(item.pct_change || item.change_percent) >= 0 ? 'up' : 'down'}>
                    {formatPercent(item.pct_change || item.change_percent)}
                  </td>
                  <td>
                    <button type="button" className="stock-text-button" onClick={() => handleAdd(item)}>
                      <PlusOutlined /> 加入自选
                    </button>
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

export default StockSearchPage;

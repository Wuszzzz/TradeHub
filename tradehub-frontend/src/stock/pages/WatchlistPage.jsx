/**
 * WatchlistPage - 专业级自选股页面
 * 功能：自选分组管理、股票行情看板、搜索添加、告警设置
 */

import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  Layout,
  Card,
  Table,
  Button,
  Input,
  Modal,
  Form,
  Select,
  Tag,
  Space,
  Typography,
  Tooltip,
  Dropdown,
  message,
  Statistic,
  Row,
  Col,
  Badge,
  Divider,
  Alert,
  Popconfirm,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  ReloadOutlined,
  DeleteOutlined,
  EditOutlined,
  StarFilled,
  StarOutlined,
  SortAscendingOutlined,
  SortDescendingOutlined,
  MoreOutlined,
  FundViewOutlined,
  BellOutlined,
  SwapOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import ReactECharts from 'echarts-for-react';
import { watchlistApi, marketApi, instrumentApi, alertApi } from '../api/stockApi';
import {
  formatPrice,
  formatChangePercent,
  formatVolume,
  formatAmount,
  getChangeColor,
  getChangeBgColor,
  getMarketName,
  normalizeSymbol,
  getSideName,
  generateId,
} from '../utils';

const { Text, Title } = Typography;

// 排序配置
const SORT_OPTIONS = [
  { key: 'default', label: '默认排序' },
  { key: 'change_percent_desc', label: '涨幅从高到低' },
  { key: 'change_percent_asc', label: '涨幅从低到高' },
  { key: 'volume_desc', label: '成交额从高到低' },
  { key: 'price_desc', label: '价格从高到低' },
  { key: 'price_asc', label: '价格从低到高' },
  { key: 'turnover_rate_desc', label: '换手率从高到低' },
];

const WatchlistPage = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [selectedGroup, setSelectedGroup] = useState(null);
  const [items, setItems] = useState([]);
  const [snapshots, setSnapshots] = useState({});
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editGroup, setEditGroup] = useState(null);
  const [groupForm] = Form.useForm();
  const [sortKey, setSortKey] = useState('default');
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [alertModalVisible, setAlertModalVisible] = useState(false);
  const [selectedForAlert, setSelectedForAlert] = useState(null);
  const [alertForm] = Form.useForm();
  const refreshTimerRef = useRef(null);

  // 加载分组列表
  const loadGroups = useCallback(async () => {
    try {
      const res = await watchlistApi.getGroups();
      const groupsData = res.items || [];
      setGroups(groupsData);
      if (groupsData.length > 0 && !selectedGroup) {
        setSelectedGroup(groupsData[0].group_id);
      }
    } catch (err) {
      message.error('加载分组失败');
    }
  }, [selectedGroup]);

  // 加载自选快照
  const loadSnapshots = useCallback(async () => {
    if (!selectedGroup) return;
    setLoading(true);
    try {
      const res = await watchlistApi.getSnapshot(selectedGroup);
      const itemsData = res.items || [];
      setItems(itemsData);

      // 提取行情数据
      const snapshotMap = {};
      itemsData.forEach((item) => {
        if (item.quote) {
          snapshotMap[item.symbol] = item.quote;
        }
      });
      setSnapshots(snapshotMap);
    } catch (err) {
      message.error('加载行情失败');
    } finally {
      setLoading(false);
    }
  }, [selectedGroup]);

  // 搜索股票
  const searchStocks = useCallback(
    async (keyword) => {
      if (!keyword || keyword.length < 1) {
        setSearchResults([]);
        return;
      }
      setSearchLoading(true);
      try {
        const res = await instrumentApi.search(keyword);
        setSearchResults(res.data || []);
      } catch (err) {
        console.error('搜索失败', err);
      } finally {
        setSearchLoading(false);
      }
    },
    []
  );

  // 添加自选
  const addToWatchlist = async (stock) => {
    try {
      await watchlistApi.addItem({
        group_id: selectedGroup || 'default',
        symbol: stock.symbol,
        name: stock.name,
        market: stock.market || 'CN-A',
        note: '',
      });
      message.success(`已添加 ${stock.name} 到自选`);
      loadSnapshots();
    } catch (err) {
      message.error('添加失败');
    }
  };

  // 删除自选
  const removeFromWatchlist = async (itemId) => {
    try {
      await watchlistApi.deleteItem(itemId);
      message.success('已移除');
      loadSnapshots();
    } catch (err) {
      message.error('移除失败');
    }
  };

  // 创建/编辑分组
  const handleGroupSubmit = async () => {
    try {
      const values = await groupForm.validateFields();
      await watchlistApi.createGroup({
        group_id: editGroup?.group_id || undefined,
        name: values.name,
        sort_order: values.sort_order || 0,
      });
      message.success(editGroup ? '分组已更新' : '分组已创建');
      setModalVisible(false);
      groupForm.resetFields();
      loadGroups();
    } catch (err) {
      message.error('操作失败');
    }
  };

  // 删除分组
  const deleteGroup = async (groupId) => {
    try {
      await watchlistApi.deleteGroup(groupId);
      message.success('分组已删除');
      if (selectedGroup === groupId) {
        setSelectedGroup(null);
      }
      loadGroups();
    } catch (err) {
      message.error(err.message || '删除失败');
    }
  };

  // 排序处理
  const sortedItems = useCallback(() => {
    let sorted = [...items];
    const withQuote = sorted.map((item) => ({
      ...item,
      quote: snapshots[item.symbol] || {},
    }));

    switch (sortKey) {
      case 'change_percent_desc':
        withQuote.sort((a, b) => (b.quote?.change_percent || 0) - (a.quote?.change_percent || 0));
        break;
      case 'change_percent_asc':
        withQuote.sort((a, b) => (a.quote?.change_percent || 0) - (b.quote?.change_percent || 0));
        break;
      case 'volume_desc':
        withQuote.sort((a, b) => (b.quote?.volume || 0) - (a.quote?.volume || 0));
        break;
      case 'price_desc':
        withQuote.sort((a, b) => (b.quote?.price || 0) - (a.quote?.price || 0));
        break;
      case 'price_asc':
        withQuote.sort((a, b) => (a.quote?.price || 0) - (b.quote?.price || 0));
        break;
      case 'turnover_rate_desc':
        withQuote.sort((a, b) => (b.quote?.turnover_rate || 0) - (a.quote?.turnover_rate || 0));
        break;
      default:
        break;
    }
    return withQuote;
  }, [items, snapshots, sortKey]);

  // 统计信息
  const getStats = () => {
    const quoteList = items.map((item) => snapshots[item.symbol]).filter(Boolean);
    const upCount = quoteList.filter((q) => q.change_percent > 0).length;
    const downCount = quoteList.filter((q) => q.change_percent < 0).length;
    const flatCount = quoteList.length - upCount - downCount;

    return { upCount, downCount, flatCount, total: items.length };
  };

  // 设置告警
  const handleSetAlert = (stock) => {
    setSelectedForAlert(stock);
    alertForm.setFieldsValue({
      symbol: stock.symbol,
      name: stock.name,
      metric: 'price',
      op: 'gt',
      threshold: snapshots[stock.symbol]?.price || 0,
    });
    setAlertModalVisible(true);
  };

  const submitAlert = async () => {
    try {
      const values = await alertForm.validateFields();
      await alertApi.createRule({
        ...values,
        name: `${values.name} ${values.metric === 'price' ? '价格' : '涨跌幅'}告警`,
        enabled: true,
      });
      message.success('告警已设置');
      setAlertModalVisible(false);
      alertForm.resetFields();
    } catch (err) {
      message.error('设置失败');
    }
  };

  // 初始化
  useEffect(() => {
    loadGroups();
  }, []);

  useEffect(() => {
    if (selectedGroup) {
      loadSnapshots();
    }
  }, [selectedGroup, loadSnapshots]);

  // 自动刷新（每30秒）
  useEffect(() => {
    refreshTimerRef.current = setInterval(() => {
      if (selectedGroup) {
        loadSnapshots();
      }
    }, 30000);

    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
      }
    };
  }, [selectedGroup, loadSnapshots]);

  // 表格列定义
  const columns = [
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      fixed: 'left',
      width: 180,
      render: (symbol, record) => (
        <Space direction="vertical" size={0}>
          <Space>
            <Text strong className="stock-symbol">
              {symbol}
            </Text>
            <Text className="stock-name">{record.name}</Text>
            <Tag className="stock-market-tag">{getMarketName(record.market)}</Tag>
          </Space>
          {record.note && (
            <Text type="secondary" style={{ fontSize: 11 }}>
              {record.note}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: '最新价',
      dataIndex: 'price',
      key: 'price',
      width: 100,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text
            strong
            style={{
              color: getChangeColor(quote.change_percent),
              fontSize: 15,
              fontFamily: 'SF Mono, Consolas, monospace',
            }}
          >
            {formatPrice(quote.price)}
          </Text>
        );
      },
    },
    {
      title: '涨跌幅',
      dataIndex: 'change_percent',
      key: 'change_percent',
      width: 100,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        const change = quote.change_percent || 0;
        return (
          <Tag
            style={{
              background: getChangeBgColor(change),
              color: getChangeColor(change),
              border: 'none',
              fontFamily: 'SF Mono, Consolas, monospace',
              padding: '4px 8px',
            }}
          >
            {formatChangePercent(change)}
          </Tag>
        );
      },
    },
    {
      title: '涨跌额',
      dataIndex: 'change',
      key: 'change',
      width: 90,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        const change = quote.change || 0;
        return (
          <Text style={{ color: getChangeColor(change), fontFamily: 'SF Mono, Consolas, monospace' }}>
            {change > 0 ? '+' : ''}{change.toFixed(2)}
          </Text>
        );
      },
    },
    {
      title: '成交量',
      dataIndex: 'volume',
      key: 'volume',
      width: 100,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatVolume(quote.volume)}
          </Text>
        );
      },
    },
    {
      title: '成交额',
      dataIndex: 'amount',
      key: 'amount',
      width: 100,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatAmount(quote.amount)}
          </Text>
        );
      },
    },
    {
      title: '换手率',
      dataIndex: 'turnover_rate',
      key: 'turnover_rate',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {quote.turnover_rate != null ? `${quote.turnover_rate.toFixed(2)}%` : '--'}
          </Text>
        );
      },
    },
    {
      title: '振幅',
      dataIndex: 'amplitude',
      key: 'amplitude',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {quote.amplitude != null ? `${quote.amplitude.toFixed(2)}%` : '--'}
          </Text>
        );
      },
    },
    {
      title: '量比',
      dataIndex: 'volume_ratio',
      key: 'volume_ratio',
      width: 70,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {quote.volume_ratio != null ? quote.volume_ratio.toFixed(2) : '--'}
          </Text>
        );
      },
    },
    {
      title: '最高',
      dataIndex: 'high',
      key: 'high',
      width: 90,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ color: '#ee4444', fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatPrice(quote.high)}
          </Text>
        );
      },
    },
    {
      title: '最低',
      dataIndex: 'low',
      key: 'low',
      width: 90,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ color: '#00a54c', fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatPrice(quote.low)}
          </Text>
        );
      },
    },
    {
      title: '今开',
      dataIndex: 'open',
      key: 'open',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatPrice(quote.open)}
          </Text>
        );
      },
    },
    {
      title: '昨收',
      dataIndex: 'prev_close',
      key: 'prev_close',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ color: '#888', fontFamily: 'SF Mono, Consolas, monospace' }}>
            {formatPrice(quote.prev_close)}
          </Text>
        );
      },
    },
    {
      title: '市值(亿)',
      dataIndex: 'market_cap',
      key: 'market_cap',
      width: 100,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {quote.market_cap ? (quote.market_cap / 100000000).toFixed(2) : '--'}
          </Text>
        );
      },
    },
    {
      title: '市盈率',
      dataIndex: 'pe',
      key: 'pe',
      width: 80,
      align: 'right',
      render: (_, record) => {
        const quote = snapshots[record.symbol] || {};
        return (
          <Text style={{ fontFamily: 'SF Mono, Consolas, monospace' }}>
            {quote.pe != null ? quote.pe.toFixed(2) : '--'}
          </Text>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<FundViewOutlined />}
            onClick={() => navigate(`/stock/kline?symbol=${record.symbol}`)}
          >
            K线
          </Button>
          <Button
            type="link"
            size="small"
            icon={<SwapOutlined />}
            onClick={() => navigate(`/stock/paper?symbol=${record.symbol}`)}
          >
            交易
          </Button>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'alert',
                  icon: <BellOutlined />,
                  label: '设置告警',
                  onClick: () => handleSetAlert(record),
                },
                {
                  key: 'remove',
                  icon: <DeleteOutlined />,
                  label: '移出自选',
                  danger: true,
                  onClick: () => removeFromWatchlist(record.item_id),
                },
              ],
            }}
          >
            <Button type="link" size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    },
  ];

  const stats = getStats();
  const displayItems = sortedItems();

  return (
    <div className="stock-page stock-layout">
      {/* 页面头部 */}
      <div className="stock-page-header">
        <div className="stock-flex-between" style={{ flex: 1 }}>
          <Space size="large">
            <Title level={4} style={{ margin: 0, color: '#122033' }}>
              自选股
            </Title>
            {/* 分组切换 */}
            <Select
              placeholder="选择分组"
              style={{ width: 160 }}
              value={selectedGroup}
              onChange={setSelectedGroup}
              options={groups.map((g) => ({
                value: g.group_id,
                label: (
                  <Space>
                    <StarFilled style={{ color: '#1890ff', fontSize: 12 }} />
                    {g.name}
                    <Tag>{g.item_count}</Tag>
                  </Space>
                ),
              }))}
              dropdownRender={(menu) => (
                <>
                  {menu}
                  <Divider style={{ margin: '8px 0' }} />
                  <Space style={{ padding: '0 8px' }}>
                    <Button
                      type="link"
                      size="small"
                      icon={<PlusOutlined />}
                      onClick={() => {
                        setEditGroup(null);
                        groupForm.resetFields();
                        setModalVisible(true);
                      }}
                    >
                      新建分组
                    </Button>
                  </Space>
                </>
              )}
            />
          </Space>

          {/* 统计信息 */}
          <Space size="middle">
            <Badge count={stats.upCount} showZero color="#ee4444">
              <Tag className="stock-tag-up" style={{ padding: '4px 12px' }}>
                涨 {stats.upCount}
              </Tag>
            </Badge>
            <Badge count={stats.flatCount} showZero color="#888">
              <Tag className="stock-tag-flat" style={{ padding: '4px 12px' }}>
                平 {stats.flatCount}
              </Tag>
            </Badge>
            <Badge count={stats.downCount} showZero color="#00a54c">
              <Tag className="stock-tag-down" style={{ padding: '4px 12px' }}>
                跌 {stats.downCount}
              </Tag>
            </Badge>
          </Space>
        </div>

        <Space>
          {/* 搜索添加 */}
          <Input.Search
            placeholder="搜索股票代码/名称"
            style={{ width: 200 }}
            loading={searchLoading}
            onSearch={searchStocks}
            onChange={(e) => {
              setSearchKeyword(e.target.value);
              if (e.target.value.length >= 1) {
                searchStocks(e.target.value);
              } else {
                setSearchResults([]);
              }
            }}
            dropdownRender={(menu) => (
              <div>
                {searchResults.length > 0 ? (
                  <div style={{ maxHeight: 300, overflow: 'auto' }}>
                    {searchResults.slice(0, 10).map((stock) => (
                      <div
                        key={stock.symbol}
                        style={{
                          padding: '8px 12px',
                          cursor: 'pointer',
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                        }}
                        onClick={() => addToWatchlist(stock)}
                        onMouseEnter={(e) => (e.currentTarget.style.background = '#f5f8ff')}
                        onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                      >
                        <Space>
                          <Text className="stock-symbol">{stock.symbol}</Text>
                          <Text>{stock.name}</Text>
                        </Space>
                        <Button type="link" size="small" icon={<PlusOutlined />}>
                          添加
                        </Button>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div style={{ padding: '12px', textAlign: 'center', color: '#888' }}>
                    输入股票代码或名称搜索
                  </div>
                )}
              </div>
            )}
          />

          {/* 排序 */}
          <Select
            placeholder="排序"
            style={{ width: 140 }}
            value={sortKey}
            onChange={setSortKey}
            options={SORT_OPTIONS}
          />

          {/* 刷新 */}
          <Button icon={<ReloadOutlined />} onClick={loadSnapshots} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {/* 内容区 */}
      <div className="stock-content">
        {/* 行情表格 */}
        <Card
          className="stock-card"
          bodyStyle={{ padding: 0 }}
          style={{ marginBottom: 16 }}
        >
          <Table
            className="stock-table"
            columns={columns}
            dataSource={displayItems}
            rowKey="item_id"
            loading={loading}
            scroll={{ x: 1600 }}
            pagination={{
              pageSize: 20,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `共 ${total} 只股票`,
            }}
            locale={{
              emptyText: (
                <div className="stock-empty">
                  <FundViewOutlined style={{ fontSize: 48, color: '#333', marginBottom: 16 }} />
                  <div>暂无自选股票</div>
                  <div style={{ color: '#666', marginTop: 8 }}>
                    在上方搜索框中输入股票代码或名称添加
                  </div>
                </div>
              ),
            }}
          />
        </Card>
      </div>

      {/* 分组编辑弹窗 */}
      <Modal
        title={editGroup ? '编辑分组' : '新建分组'}
        open={modalVisible}
        onOk={handleGroupSubmit}
        onCancel={() => {
          setModalVisible(false);
          groupForm.resetFields();
        }}
        okText="确定"
        cancelText="取消"
      >
        <Form form={groupForm} layout="vertical" initialValues={{ sort_order: 0 }}>
          <Form.Item
            name="name"
            label="分组名称"
            rules={[{ required: true, message: '请输入分组名称' }]}
          >
            <Input placeholder="例如：我的自选、ETF重点" />
          </Form.Item>
          <Form.Item name="sort_order" label="排序">
            <Input type="number" placeholder="数字越小越靠前" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 告警设置弹窗 */}
      <Modal
        title={`设置告警 - ${selectedForAlert?.name || ''}`}
        open={alertModalVisible}
        onOk={submitAlert}
        onCancel={() => {
          setAlertModalVisible(false);
          alertForm.resetFields();
        }}
        okText="确定"
        cancelText="取消"
      >
        <Form form={alertForm} layout="vertical">
          <Form.Item name="symbol" label="股票代码">
            <Input disabled />
          </Form.Item>
          <Form.Item name="name" label="股票名称">
            <Input disabled />
          </Form.Item>
          <Form.Item name="metric" label="告警指标">
            <Select
              options={[
                { value: 'price', label: '价格' },
                { value: 'change_percent', label: '涨跌幅' },
                { value: 'volume', label: '成交量' },
              ]}
            />
          </Form.Item>
          <Form.Item name="op" label="条件">
            <Select
              options={[
                { value: 'gt', label: '大于' },
                { value: 'gte', label: '大于等于' },
                { value: 'lt', label: '小于' },
                { value: 'lte', label: '小于等于' },
              ]}
            />
          </Form.Item>
          <Form.Item name="threshold" label="阈值" rules={[{ required: true }]}>
            <Input type="number" placeholder="请输入告警阈值" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default WatchlistPage;

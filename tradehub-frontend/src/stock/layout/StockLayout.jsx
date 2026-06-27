import React from 'react';
import { ConfigProvider, App as AntApp, Dropdown } from 'antd';
import {
  AccountBookOutlined,
  BarChartOutlined,
  BellOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  FundOutlined,
  GlobalOutlined,
  HistoryOutlined,
  LineChartOutlined,
  OrderedListOutlined,
  SafetyOutlined,
  SearchOutlined,
  SettingOutlined,
  StockOutlined,
  SwapOutlined,
} from '@ant-design/icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import zhCN from 'antd/locale/zh_CN';
import ProductShell, { findShellItem, resolveShellSelectedKey } from '../../components/ProductShell';
import { useAuth } from '../../contexts/AuthContext';
import './StockLayout.less';

const WORKSPACE_STORAGE_KEY = 'tradehubWorkspace';

const menuItems = [
  {
    key: 'overview',
    type: 'group',
    label: '看盘',
    children: [
      { key: 'dashboard', icon: <DashboardOutlined />, label: '工作台', path: '/stock/dashboard' },
      { key: 'watchlist', icon: <FundOutlined />, label: '自选股', path: '/stock/watchlist' },
      { key: 'market', icon: <LineChartOutlined />, label: '市场行情', path: '/stock/market' },
      { key: 'realtime', icon: <LineChartOutlined />, label: '实时盘口', path: '/stock/realtime' },
      { key: 'kline', icon: <BarChartOutlined />, label: 'K线分析', path: '/stock/kline' },
      { key: 'history', icon: <HistoryOutlined />, label: '历史回放', path: '/stock/history' },
      { key: 'global', icon: <GlobalOutlined />, label: '全球市场', path: '/stock/global' },
    ],
  },
  {
    key: 'analysis',
    type: 'group',
    label: '研究',
    children: [
      { key: 'screener', icon: <SearchOutlined />, label: '选股', path: '/stock/screener' },
      { key: 'backtest', icon: <ExperimentOutlined />, label: '策略回测', path: '/stock/backtest' },
      { key: 'etf', icon: <SafetyOutlined />, label: 'ETF风控', path: '/stock/etf' },
    ],
  },
  {
    key: 'trading',
    type: 'group',
    label: '交易',
    children: [
      { key: 'paper', icon: <SwapOutlined />, label: '模拟交易', path: '/stock/paper' },
      { key: 'orders', icon: <OrderedListOutlined />, label: '委托记录', path: '/stock/orders' },
      { key: 'account', icon: <AccountBookOutlined />, label: '账户持仓', path: '/stock/account' },
      { key: 'alerts', icon: <BellOutlined />, label: '告警中心', path: '/stock/alerts' },
    ],
  },
  {
    key: 'data',
    type: 'group',
    label: '数据',
    children: [
      { key: 'search', icon: <SearchOutlined />, label: '标的搜索', path: '/stock/search' },
      { key: 'datacenter', icon: <DatabaseOutlined />, label: '数据中心', path: '/stock/datacenter' },
      { key: 'ingestion', icon: <DatabaseOutlined />, label: '采集任务', path: '/stock/ingestion' },
    ],
  },
  {
    key: 'system',
    type: 'group',
    label: '系统',
    children: [
      { key: 'settings', icon: <SettingOutlined />, label: '设置', path: '/stock/settings' },
    ],
  },
];

const StockLayout = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuth();

  const selectedKey = resolveShellSelectedKey(location.pathname, menuItems, 'dashboard');
  const selectedItem = findShellItem(menuItems, selectedKey);

  const handleNavigate = (path) => {
    if (path) navigate(path);
  };

  const switchWorkspace = (workspace, path) => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(WORKSPACE_STORAGE_KEY, workspace);
    }
    navigate(path);
  };

  const userMenuItems = [
    { key: 'profile', label: '个人资料', onClick: () => navigate('/dashboard/profile') },
    { key: 'settings', label: '设置', onClick: () => navigate('/stock/settings') },
    { type: 'divider' },
    { key: 'logout', label: '退出登录', onClick: () => { logout(); navigate('/'); } },
  ];

  return (
    <ConfigProvider locale={zhCN}>
      <AntApp>
        <ProductShell
          brandTitle="TradeHub Stock"
          brandSubtitle="研究 · 看盘 · 回测"
          brandIcon={<StockOutlined />}
          menuGroups={menuItems}
          selectedKey={selectedKey}
          pageTitle={selectedItem?.label || '股票'}
          pageSubtitle="股票研究工作台"
          activeWorkspace="stock"
          onNavigate={handleNavigate}
          onSwitchWorkspace={switchWorkspace}
          headerExtra={(
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <button className="product-shell-account" type="button">
                <span className="product-shell-avatar">{(user?.username || 'T').slice(0, 1).toUpperCase()}</span>
                <span className="product-shell-account-copy">
                  <span>{user?.username || 'TradeHub 用户'}</span>
                  <small>账户：4453728992</small>
                </span>
                <span className="product-shell-chevron">⌄</span>
              </button>
            </Dropdown>
          )}
        >
          <Outlet />
        </ProductShell>
      </AntApp>
    </ConfigProvider>
  );
};

export default StockLayout;

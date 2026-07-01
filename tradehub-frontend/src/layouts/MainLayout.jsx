import { Grid, Layout } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  AccountBookOutlined,
  AppstoreOutlined,
  BarChartOutlined,
  BellOutlined,
  DatabaseOutlined,
  FundOutlined,
  ExperimentOutlined,
  GlobalOutlined,
  HistoryOutlined,
  LineChartOutlined,
  LogoutOutlined,
  MailOutlined,
  QuestionCircleOutlined,
  SafetyOutlined,
  SettingOutlined,
  StarOutlined,
  StockOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useAuth } from '../contexts/AuthContext';
import ProductShell, { findShellItem, flattenShellItems, resolveShellSelectedKey } from '../components/ProductShell';
import Footer from '../components/Footer';

const { Header, Content } = Layout;
const { useBreakpoint } = Grid;
const WORKSPACE_STORAGE_KEY = 'tradehubWorkspace';
const appSwitchItems = [
  { key: 'fund', label: '基金', icon: <FundOutlined />, path: '/dashboard/watchlists' },
  { key: 'stock', label: '股票', icon: <StockOutlined />, path: '/stock/dashboard' },
];
const fundRouteKeys = new Set([
  '/dashboard',
  '/dashboard/watchlists',
  '/dashboard/market',
  '/dashboard/positions',
  '/dashboard/portfolio-health',
  '/dashboard/compare',
  '/dashboard/accounts',
  '/dashboard/funds',
  '/dashboard/profile',
  '/dashboard/settings',
  '/dashboard/rankings',
  '/dashboard/fund-sectors',
  '/dashboard/fund-data',
  '/dashboard/fund-research',
]);
const stockRouteKeys = new Set([
  '/dashboard/stock',
  '/dashboard/stock/watchlists',
  '/dashboard/stock/global',
  '/dashboard/stock/search',
  '/dashboard/stock/orders',
  '/dashboard/stock/account',
  '/dashboard/stock/alerts',
  '/dashboard/stock/realtime',
  '/dashboard/stock/history',
  '/dashboard/stock/etf',
  '/dashboard/stock/ingestion',
  // Professional Stock Layout
  '/stock',
]);

const getStoredWorkspace = () => {
  if (typeof window === 'undefined') return 'fund';
  return window.localStorage.getItem(WORKSPACE_STORAGE_KEY) || 'fund';
};

const pathMatches = (pathname, routeKeys) => {
  for (const key of routeKeys) {
    if (pathname === key || pathname.startsWith(`${key}/`)) {
      return true;
    }
  }
  return false;
};

const MainLayout = ({ children }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuth();
  const screens = useBreakpoint();
  const isMobile = !screens.md;

  const isExplicitStockRoute = pathMatches(location.pathname, stockRouteKeys);
  const isDashboardRoute = location.pathname === '/dashboard' || location.pathname.startsWith('/dashboard/');
  const currentWorkspace = isExplicitStockRoute
    ? 'stock'
    : pathMatches(location.pathname, fundRouteKeys) || isDashboardRoute
      ? 'fund'
      : getStoredWorkspace();
  const isStockRoute = currentWorkspace === 'stock';

  const handleLogout = () => {
    logout();
    navigate('/');
  };

  const switchWorkspace = (workspace, path) => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(WORKSPACE_STORAGE_KEY, workspace);
    }
    navigate(path);
  };

  const userMenuItems = [
    { key: 'profile', icon: <UserOutlined />, label: '个人资料', onClick: () => navigate('/dashboard/profile') },
    { key: 'settings', icon: <SettingOutlined />, label: '设置', onClick: () => navigate('/dashboard/settings') },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: handleLogout },
  ];

  const fundSideItems = [
    {
      key: 'fund-overview',
      label: '基金',
      children: [
        { key: 'watchlists', path: '/dashboard/watchlists', label: '自选', icon: <StarOutlined /> },
        { key: 'market', path: '/dashboard/market', label: '市场', icon: <FundOutlined /> },
        { key: 'funds', path: '/dashboard/funds', label: '基金库', icon: <DatabaseOutlined /> },
        { key: 'rankings', path: '/dashboard/rankings', label: '评估排行', icon: <BarChartOutlined /> },
        { key: 'fund-sectors', path: '/dashboard/fund-sectors', label: '板块', icon: <LineChartOutlined /> },
        { key: 'positions', path: '/dashboard/positions', label: '持仓', icon: <AccountBookOutlined /> },
        { key: 'portfolio-health', path: '/dashboard/portfolio-health', label: '体检', icon: <SafetyOutlined /> },
        { key: 'compare', path: '/dashboard/compare', label: '对比', icon: <GlobalOutlined /> },
        { key: 'accounts', path: '/dashboard/accounts', label: '账户', icon: <AppstoreOutlined /> },
        { key: 'fund-data', path: '/dashboard/fund-data', label: '数据中心', icon: <DatabaseOutlined /> },
        { key: 'fund-research', path: '/dashboard/fund-research', label: '投研实验室', icon: <ExperimentOutlined /> },
      ],
    },
  ];
  const stockSideItems = [
    { key: 'stock-overview', path: '/dashboard/stock', label: '概览', icon: <StockOutlined /> },
    { key: 'stock-watchlists', path: '/dashboard/stock/watchlists', label: '自选', icon: <StarOutlined /> },
    { key: 'stock-global', path: '/dashboard/stock/global', label: '全球行情', icon: <GlobalOutlined /> },
    { key: 'stock-realtime', path: '/dashboard/stock/realtime', label: '实盘', icon: <LineChartOutlined /> },
    { key: 'stock-history', path: '/dashboard/stock/history', label: '历史', icon: <HistoryOutlined /> },
    { key: 'stock-etf', path: '/dashboard/stock/etf', label: 'ETF风控', icon: <SafetyOutlined /> },
    { key: 'stock-ingestion', path: '/dashboard/stock/ingestion', label: '数据采集', icon: <DatabaseOutlined /> },
    { key: 'stock-search', path: '/dashboard/stock/search', label: '搜索', icon: <GlobalOutlined /> },
    { key: 'stock-orders', path: '/dashboard/stock/orders', label: '委托', icon: <BarChartOutlined /> },
    { key: 'stock-account', path: '/dashboard/stock/account', label: '账户', icon: <AccountBookOutlined /> },
    { key: 'stock-alerts', path: '/dashboard/stock/alerts', label: '告警', icon: <BellOutlined /> },
  ];

  const bottomItems = [
    { key: 'profile', path: '/dashboard/profile', label: '个人资料', icon: <UserOutlined /> },
    { key: 'settings', path: '/dashboard/settings', label: '设置', icon: <SettingOutlined /> },
    { key: 'mail', path: '', label: '消息', icon: <MailOutlined /> },
    { key: 'help', path: '', label: '帮助', icon: <QuestionCircleOutlined /> },
  ];

  const navigateShellItem = (path) => {
    if (path) navigate(path);
  };

  const visibleSideGroups = isStockRoute ? [{ key: 'stock-legacy', children: stockSideItems }] : fundSideItems;
  const allShellGroups = bottomItems.length > 0
    ? [...visibleSideGroups, { key: 'shell-bottom', children: bottomItems }]
    : visibleSideGroups;
  const visibleSideItems = flattenShellItems(visibleSideGroups);
  const selectedSideKey = resolveShellSelectedKey(location.pathname, allShellGroups, visibleSideItems[0]?.key || '');
  const selectedSideItem = findShellItem(allShellGroups, selectedSideKey);
  const workspaceTitle = selectedSideItem?.label || (isStockRoute ? '股票' : '基金');
  const workspaceSubtitle = isStockRoute ? '股票研究工作台' : '基金投资工作台';
  if (isMobile) {
    return (
      <Layout className="trade-mobile-layout">
        <Header className="trade-mobile-header">
          <div className="trade-logo-button">
            <span>TradeHub</span>
            <div className="trade-logo-switcher" aria-label="项目切换">
              {appSwitchItems.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className={(currentWorkspace === item.key) ? 'active' : ''}
                  onClick={() => switchWorkspace(item.key, item.path)}
                  aria-label={item.label}
                >
                  {item.icon}
                </button>
              ))}
            </div>
          </div>
          <button className="trade-user-chip" type="button" onClick={() => navigate('/dashboard/profile')}>
            <UserOutlined />
            <span>{user?.username || 'Trader'}</span>
          </button>
        </Header>
        <Content className="trade-mobile-content">
          <div className="trade-mobile-switch">
            <button type="button" className={!isStockRoute ? 'active' : ''} onClick={() => switchWorkspace('fund', '/dashboard/watchlists')}>
              基金
            </button>
            <button type="button" className={isStockRoute ? 'active' : ''} onClick={() => switchWorkspace('stock', '/stock/dashboard')}>
              股票
            </button>
          </div>
          {children}
          <Footer />
        </Content>
        <nav className="trade-mobile-nav">
          {visibleSideItems.map((item) => (
            <button
              key={item.key}
              type="button"
              className={selectedSideKey === item.key ? 'active' : ''}
              onClick={() => navigateShellItem(item.path)}
            >
              {item.icon}
              <span>{item.label}</span>
            </button>
          ))}
        </nav>
      </Layout>
    );
  }

  return (
    <ProductShell
      brandTitle="TradeHub Fund"
      brandSubtitle="自选 · 市场 · 持仓"
      brandIcon={<FundOutlined />}
      menuGroups={visibleSideGroups}
      selectedKey={selectedSideKey}
      pageTitle={workspaceTitle}
      pageSubtitle={workspaceSubtitle}
      activeWorkspace={currentWorkspace}
      onNavigate={navigateShellItem}
      onSwitchWorkspace={switchWorkspace}
      bottomItems={bottomItems}
      headerExtra={(
        <button className="product-shell-account" type="button" onClick={() => navigate('/dashboard/profile')}>
          <span className="product-shell-avatar">{(user?.username || 'T').slice(0, 1).toUpperCase()}</span>
          <span className="product-shell-account-copy">
            <span>{user?.username || 'TradeHub 用户'}</span>
            <small>账户：4453728992</small>
          </span>
          <span className="product-shell-chevron">⌄</span>
        </button>
      )}
    >
      {children}
      {!isStockRoute && <Footer />}
    </ProductShell>
  );
};

export default MainLayout;

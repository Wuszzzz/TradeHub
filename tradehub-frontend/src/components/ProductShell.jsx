import { Layout } from 'antd';
import { FundOutlined, StockOutlined } from '@ant-design/icons';
import './ProductShell.css';

const { Header, Content } = Layout;

const workspaceItems = [
  { key: 'fund', label: '基金', icon: <FundOutlined />, path: '/dashboard/watchlists' },
  { key: 'stock', label: '股票', icon: <StockOutlined />, path: '/stock/dashboard' },
];

export const flattenShellItems = (groups = []) => groups.flatMap((group) => group.children || []);

export const resolveShellSelectedKey = (pathname, groups = [], fallback = '') => {
  let match = null;
  for (const item of flattenShellItems(groups)) {
    if (!item.path) continue;
    if (pathname === item.path || pathname.startsWith(`${item.path}/`)) {
      if (!match || item.path.length > match.path.length) {
        match = item;
      }
    }
  }
  return match?.key || fallback;
};

export const findShellItem = (groups = [], key) => flattenShellItems(groups).find((item) => item.key === key);

const ProductShell = ({
  brandTitle,
  brandSubtitle,
  brandIcon,
  menuGroups,
  selectedKey,
  pageTitle,
  pageSubtitle,
  activeWorkspace,
  onNavigate,
  onSwitchWorkspace,
  bottomItems = [],
  contentClassName = '',
  headerExtra = null,
  children,
}) => {
  const todayLabel = new Date().toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  });

  return (
    <Layout className="product-shell">
      <aside className="product-shell-sider">
        <div className="product-shell-brand">
          <span className="product-shell-brand-icon">{brandIcon}</span>
          <div>
            <div className="product-shell-brand-title">{brandTitle}</div>
            <div className="product-shell-brand-subtitle">{brandSubtitle}</div>
          </div>
        </div>

        <nav className="product-shell-menu" aria-label={`${brandTitle}导航`}>
          {menuGroups.map((group) => (
            <div className="product-shell-menu-group" key={group.key}>
              {group.label && <div className="product-shell-menu-group-title">{group.label}</div>}
              {(group.children || []).map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className={selectedKey === item.key ? 'product-shell-menu-item active' : 'product-shell-menu-item'}
                  onClick={() => onNavigate(item.path)}
                  aria-label={item.label}
                >
                  {item.icon}
                  <span>{item.label}</span>
                </button>
              ))}
            </div>
          ))}
        </nav>

        {bottomItems.length > 0 && (
          <div className="product-shell-bottom">
            {bottomItems.map((item) => (
              <button
                key={item.key}
                type="button"
                className={selectedKey === item.key ? 'product-shell-menu-item active' : 'product-shell-menu-item'}
                onClick={() => onNavigate(item.path)}
                aria-label={item.label}
              >
                {item.icon}
                <span>{item.label}</span>
              </button>
            ))}
          </div>
        )}
      </aside>

      <Layout className="product-shell-main">
        <Header className="product-shell-header">
          <div className="product-shell-header-copy">
            <div className="product-shell-header-title">{pageTitle}</div>
            <div className="product-shell-header-subtitle">{pageSubtitle}</div>
          </div>
          <div className="product-shell-header-actions">
            <div className="product-shell-workspace-switch" aria-label="项目切换">
              {workspaceItems.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  className={activeWorkspace === item.key ? 'active' : ''}
                  onClick={() => onSwitchWorkspace(item.key, item.path)}
                  aria-label={item.label}
                  title={item.label}
                >
                  {item.icon}
                </button>
              ))}
            </div>
            <div className="product-shell-header-meta">{todayLabel}</div>
            {headerExtra}
          </div>
        </Header>

        <Content className={`product-shell-content ${contentClassName}`.trim()}>
          {children}
        </Content>
      </Layout>
    </Layout>
  );
};

export default ProductShell;

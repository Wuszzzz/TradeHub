import { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import InitializePage from './pages/InitializePage';
import MainLayout from './layouts/MainLayout';
import FundsPage from './pages/FundsPage';
import FundDetailPage from './pages/FundDetailPage';
import AccountsPage from './pages/AccountsPage';
import PositionsPage from './pages/PositionsPage';
import WatchlistsPage from './pages/WatchlistsPage';
import SettingsPage from './pages/SettingsPage';
import AdminPage from './pages/AdminPage';
import ComparePage from './pages/ComparePage';
import RankingsPage from './pages/RankingsPage';
import MarketPage from './pages/MarketPage';
import ProfilePage from './pages/ProfilePage';
import FundDataCenterPage from './pages/FundDataCenterPage';
import FundSectorsPage from './pages/FundSectorsPage';
import StockDashboardPage from './pages/StockDashboardPage';
import StockWatchlistsPage from './pages/StockWatchlistsPage';
import StockSearchPage from './pages/StockSearchPage';
import StockOrdersPage from './pages/StockOrdersPage';
import StockAccountPage from './pages/StockAccountPage';
import StockAlertsPage from './pages/StockAlertsPage';
import StockRealtimePage from './pages/StockRealtimePage';
import StockHistoryPage from './pages/StockHistoryPage';
import StockETFRiskPage from './pages/StockETFRiskPage';
import StockIngestionPage from './pages/StockIngestionPage';
import StockGlobalMarketPage from './pages/StockGlobalMarketPage';
// Professional Stock Pages
import StockLayout from './stock/layout/StockLayout';
import StockDashboard from './stock/pages/DashboardPage';
import StockWatchlist from './stock/pages/WatchlistPage';
import StockKLine from './stock/pages/KLinePage';
import StockScreener from './stock/pages/ScreenerPage';
import StockPaper from './stock/pages/PaperTradingPage';
import StockAlerts from './stock/pages/AlertsPage';
import StockBacktest from './stock/pages/BacktestPage';
import StockMarket from './stock/pages/MarketPage';
import StockSettings from './stock/pages/SettingsPage';
import StockDataCenter from './stock/pages/DataCenterPage';
import { isAuthenticated, getUser } from './utils/auth';
import { AuthProvider } from './contexts/AuthContext';
import { AccountProvider } from './contexts/AccountContext';
import { PreferenceProvider } from './contexts/PreferenceContext';
import './stock/layout/StockLayout.less';

function PrivateRoute({ children }) {
  return isAuthenticated() ? children : <Navigate to="/" />;
}

// 检查是否在桌面/移动应用中运行
export const isNativeApp = () => {
  // 检查 Tauri API
  if (window.__TAURI__ !== undefined) return true;

  // 检查 Capacitor API
  if (window.Capacitor !== undefined) return true;

  // 检查 Tauri 特有的环境变量
  if (window.__TAURI_INTERNALS__ !== undefined) return true;

  // 检查 user agent 中是否包含 Tauri
  if (navigator.userAgent.includes('Tauri')) return true;

  return false;
};

function AppInner() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#0f6fff',
          colorBgBase: '#ffffff',
          colorBgLayout: '#f7f9fc',
          colorBgContainer: '#ffffff',
          colorBorder: '#d9e2ef',
          colorBorderSecondary: '#edf1f7',
          colorText: '#111827',
          colorTextSecondary: '#667085',
          borderRadius: 10,
          fontFamily: '"Golos Text", Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
        components: {
          Card: {
            headerBg: '#ffffff',
          },
          Table: {
            headerBg: '#f8fafc',
            rowHoverBg: '#f3f7ff',
          },
        },
      }}
    >
      <Router>
              <Routes>
              <Route
                path="/"
                element={
                  isAuthenticated() ? (
                    <Navigate to="/dashboard/watchlists" />
                  ) : (
                    <Navigate to="/login" />
                  )
                }
              />
              <Route path="/login" element={<LoginPage />} />
              <Route path="/admin" element={<Navigate to="/dashboard/admin" />} />
              <Route path="/register" element={<RegisterPage />} />
              <Route path="/initialize" element={<InitializePage />} />
              <Route
                path="/dashboard"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <Navigate to="/dashboard/watchlists" />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/funds"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <FundsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/funds/:code"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <FundDetailPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/accounts"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <AccountsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/positions"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <PositionsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/watchlists"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <WatchlistsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/settings"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <SettingsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/market"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <MarketPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/profile"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <ProfilePage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/rankings"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <RankingsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/fund-data"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <FundDataCenterPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/fund-sectors"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <FundSectorsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/compare"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <ComparePage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockDashboardPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/watchlists"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockWatchlistsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/global"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockGlobalMarketPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/search"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockSearchPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/orders"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockOrdersPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/account"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockAccountPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/alerts"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockAlertsPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/realtime"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockRealtimePage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/history"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockHistoryPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/etf"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockETFRiskPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              <Route
                path="/dashboard/stock/ingestion"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <StockIngestionPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
              {/* Professional Stock Routes */}
              <Route
                path="/stock"
                element={
                  <PrivateRoute>
                    <StockLayout />
                  </PrivateRoute>
                }
              >
                <Route index element={<Navigate to="/stock/dashboard" />} />
                <Route path="dashboard" element={<StockDashboard />} />
                <Route path="watchlist" element={<StockWatchlist />} />
                <Route path="search" element={<StockSearchPage />} />
                <Route path="orders" element={<StockOrdersPage />} />
                <Route path="account" element={<StockAccountPage />} />
                <Route path="realtime" element={<StockRealtimePage />} />
                <Route path="history" element={<StockHistoryPage />} />
                <Route path="etf" element={<StockETFRiskPage />} />
                <Route path="ingestion" element={<StockIngestionPage />} />
                <Route path="global" element={<StockGlobalMarketPage />} />
                <Route path="kline" element={<StockKLine />} />
                <Route path="screener" element={<StockScreener />} />
                <Route path="backtest" element={<StockBacktest />} />
                <Route path="paper" element={<StockPaper />} />
                <Route path="alerts" element={<StockAlerts />} />
                <Route path="market" element={<StockMarket />} />
                <Route path="settings" element={<StockSettings />} />
                <Route path="datacenter" element={<StockDataCenter />} />
              </Route>
              <Route
                path="/dashboard/admin"
                element={
                  <PrivateRoute>
                    <MainLayout>
                      <AdminPage />
                    </MainLayout>
                  </PrivateRoute>
                }
              />
            </Routes>
          </Router>
    </ConfigProvider>
  );
}

function App() {
  return (
    <AuthProvider>
      <AccountProvider>
        <PreferenceProvider>
          <AppInner />
        </PreferenceProvider>
      </AccountProvider>
    </AuthProvider>
  );
}

export default App;

import { useEffect, useState } from 'react';
import { Button, Form, Input, Layout, Modal, Switch, Typography, message } from 'antd';
import {
  ArrowRightOutlined,
  LineChartOutlined,
  LockOutlined,
  MoonOutlined,
  SettingOutlined,
  StockOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { Link, useNavigate } from 'react-router-dom';
import { login } from '../api';
import { setToken } from '../utils/auth';
import { useAuth } from '../contexts/AuthContext';
import { isNativeApp } from '../App';

const { Content, Footer } = Layout;
const { Text } = Typography;

function LoginPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [serverModalVisible, setServerModalVisible] = useState(false);
  const [serverUrl, setServerUrl] = useState(localStorage.getItem('apiBaseUrl') || '');
  const [testingConnection, setTestingConnection] = useState(false);
  const [isNative, setIsNative] = useState(isNativeApp());
  const { login: authLogin } = useAuth();

  useEffect(() => {
    const checkNative = () => {
      setIsNative(isNativeApp());
    };

    checkNative();
    const timer = setTimeout(checkNative, 100);
    return () => clearTimeout(timer);
  }, []);

  const onFinish = async (values) => {
    setLoading(true);
    try {
      const response = await login(values.username, values.password);
      const { access_token, refresh_token, user } = response.data;

      setToken(access_token, refresh_token);
      authLogin(user);
      message.success(`欢迎回来，${user.username}！`);

      navigate('/dashboard');
    } catch (error) {
      message.error(error.response?.data?.error || '登录失败');
    } finally {
      setLoading(false);
    }
  };

  const handleServerConfig = async () => {
    if (!serverUrl.trim()) {
      message.error('请输入服务器地址');
      return;
    }

    if (!serverUrl.startsWith('http://') && !serverUrl.startsWith('https://')) {
      message.error('服务器地址必须以 http:// 或 https:// 开头');
      return;
    }

    setTestingConnection(true);
    try {
      const response = await fetch(`${serverUrl}/api/health/`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
      });

      if (response.ok) {
        localStorage.setItem('apiBaseUrl', serverUrl);
        message.success('服务器配置成功！');
        setServerModalVisible(false);
        window.location.reload();
      } else {
        message.error('无法连接到服务器，请检查地址是否正确');
      }
    } catch (error) {
      message.error(`连接失败: ${error.message}`);
    } finally {
      setTestingConnection(false);
    }
  };

  return (
    <Layout className="auth-shell auth-shell-terminal">
      <Content className="auth-terminal-content">
        <section className="auth-market-hero">
          <div className="auth-hero-brand">
            <span className="auth-hero-logo"><StockOutlined /></span>
            <span>TradeHub</span>
          </div>
          <div className="auth-hero-copy">
            <h1>自信驾驭市场</h1>
            <div className="auth-tags">
              <span>高级技术分析与图表工具</span>
              <span>市场资讯流</span>
              <span>基金与股票统一工作台</span>
              <span>可定制交易视图</span>
            </div>
          </div>
          <div className="auth-testimonial">
            <div className="quote-mark">“</div>
            <p>
              一套真正适合日常盯盘的软件，帮助我快速 <b>分析市场趋势</b>，并且 <b>做出更稳的决策</b>
            </p>
            <div className="auth-person">
              <span>TH</span>
              <strong>TradeHub 用户</strong>
              <small>专业账户</small>
            </div>
          </div>
        </section>

        <section className="auth-terminal-panel">
          <div className="auth-mode-toggle">
            <MoonOutlined />
            <span>深色模式</span>
            <Switch size="small" checked />
          </div>
          {isNative && (
            <button className="auth-server-button" type="button" onClick={() => setServerModalVisible(true)}>
              <SettingOutlined />
              <span>服务器</span>
            </button>
          )}

          <Form
            name="login"
            onFinish={onFinish}
            autoComplete="off"
            layout="vertical"
            size="large"
            className="auth-terminal-form"
          >
            <Form.Item
              label="用户名"
              name="username"
              rules={[{ required: true, message: '请输入用户名' }]}
            >
              <Input
                prefix={<UserOutlined />}
                placeholder="用户名"
                autoComplete="username"
              />
            </Form.Item>

            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, message: '请输入密码' }]}
              extra="使用本地 TradeHub 账户登录。"
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="密码"
                autoComplete="current-password"
              />
            </Form.Item>

            <label className="auth-remember">
              <input type="checkbox" />
              <span>记住这台设备</span>
            </label>

            <Button
              className="auth-submit"
              type="primary"
              htmlType="submit"
              loading={loading}
              block
              size="large"
              icon={<ArrowRightOutlined />}
            >
              登录
            </Button>

            <div className="auth-card-foot">
              <Text>
                还没有账号？ <Link to="/register">立即注册</Link>
              </Text>
            </div>
          </Form>
        </section>
      </Content>

      <Footer className="auth-footer">
        <Text>© 2026 TradeHub</Text>
      </Footer>

      <Modal
        title="服务器配置"
        open={serverModalVisible}
        onOk={handleServerConfig}
        onCancel={() => setServerModalVisible(false)}
        confirmLoading={testingConnection}
        okText="保存"
        cancelText="取消"
      >
        <Form layout="vertical">
          <Form.Item
            label="服务器地址"
            extra="例如: http://192.168.1.100:8000 或 https://tradehub.example.com"
          >
            <Input
              prefix={<LineChartOutlined />}
              placeholder="http://your-server:8000"
              value={serverUrl}
              onChange={(e) => setServerUrl(e.target.value)}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  );
}

export default LoginPage;

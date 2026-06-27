import { Layout, Typography, theme } from 'antd';

const { Footer: AntFooter } = Layout;
const { Text } = Typography;

const Footer = () => {
  const { token } = theme.useToken();
  return (
    <AntFooter
      style={{
        textAlign: 'center',
        padding: '16px 24px',
        background: token.colorFillAlter,
        borderTop: `1px solid ${token.colorBorderSecondary}`,
      }}
    >
      <Text type="secondary" style={{ fontSize: 13 }}>
        © 2024-2026 tradehub
      </Text>
    </AntFooter>
  );
};

export default Footer;

import { Layout, theme } from 'antd';

const { Footer: AntFooter } = Layout;

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
      <span style={{ color: token.colorTextSecondary, fontSize: 13 }}>
        © 2024-2026 tradehub
      </span>
    </AntFooter>
  );
};

export default Footer;

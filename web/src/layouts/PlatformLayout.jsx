import { Layout, Menu, Typography, Button, Space } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { CrownOutlined, SafetyOutlined, LogoutOutlined } from '@ant-design/icons'
import { auth } from '../api/client.js'

const { Header, Sider, Content } = Layout
const { Title } = Typography

// PlatformLayout: the separate platform console for managing tenants and the
// global role/permission catalog. Business modules are not shown here.
export default function PlatformLayout() {
  const nav = useNavigate()
  const loc = useLocation()
  const user = auth.user()

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider theme="dark" width={200}>
        <div style={{ color: '#fff', padding: 16, fontSize: 16, fontWeight: 600 }}>
          平台管理控制台
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[loc.pathname]}
          items={[
            { key: '/admin/tenants', icon: <CrownOutlined />, label: '租户管理' },
            { key: '/admin/roles', icon: <SafetyOutlined />, label: '角色与权限' },
          ]}
          onClick={(e) => nav(e.key)}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Title level={4} style={{ margin: 0 }}>平台管理</Title>
          <Space>
            <span style={{ color: '#666' }}>{user?.name || user?.username || ''}</span>
            <Button type="link" icon={<LogoutOutlined />} onClick={() => { auth.clear(); nav('/login') }}>退出</Button>
          </Space>
        </Header>
        <Content style={{ margin: 16 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}

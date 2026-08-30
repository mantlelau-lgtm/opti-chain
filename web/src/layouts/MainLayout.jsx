import { Layout, Menu, Typography, Button, Space } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  DatabaseOutlined, TeamOutlined, ContainerOutlined,
  BranchesOutlined, ShoppingCartOutlined, InboxOutlined,
  FundOutlined, UserOutlined, ShopOutlined, LogoutOutlined,
} from '@ant-design/icons'
import { auth } from '../api/client.js'

const { Header, Sider, Content } = Layout
const { Title } = Typography

// Navigation mirrors the four backend modules.
const items = [
  { key: '/materials', icon: <DatabaseOutlined />, label: '物料', group: '基础数据' },
  { key: '/suppliers', icon: <TeamOutlined />, label: '供应商', group: '基础数据' },
  { key: '/customers', icon: <UserOutlined />, label: '客户', group: '基础数据' },
  { key: '/warehouses', icon: <ContainerOutlined />, label: '仓库', group: '基础数据' },
  { key: '/locations', icon: <BranchesOutlined />, label: '库位', group: '基础数据' },
  { key: '/purchase-orders', icon: <ShoppingCartOutlined />, label: '采购订单', group: '采购' },
  { key: '/sales-orders', icon: <ShopOutlined />, label: '销售订单', group: '销售' },
  { key: '/stock', icon: <InboxOutlined />, label: '实时库存', group: '仓储' },
  { key: '/inventory', icon: <ContainerOutlined />, label: '出入库', group: '仓储' },
  { key: '/planning', icon: <FundOutlined />, label: '计划/MRP', group: '计划' },
]

export default function MainLayout() {
  const nav = useNavigate()
  const loc = useLocation()
  // Group menu items by module for a tidy sidebar.
  const groups = items.reduce((acc, it) => {
    (acc[it.group] = acc[it.group] || []).push({ key: it.key, icon: it.icon, label: it.label })
    return acc
  }, {})

  return (
     <Layout style={{ minHeight: '100vh' }}>
       <Sider theme="dark" width={200}>
         <div style={{ color: '#fff', padding: 16, fontSize: 16, fontWeight: 600 }}>
           SCM · 供应链
         </div>
         <Menu
           theme="dark"
           mode="inline"
           selectedKeys={[loc.pathname]}
           items={Object.entries(groups).map(([g, children]) => ({
             key: g,
             label: g,
             children,
           }))}
           onClick={(e) => nav(e.key)}
         />
       </Sider>
       <Layout>
         <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
           <Title level={4} style={{ margin: 0 }}>轻量级供应链管理系统</Title>
           <Space>
             <span style={{ color: '#666' }}>{auth.user()?.name || auth.user()?.username || ''}</span>
             <Button
              type="link"
              icon={<LogoutOutlined />}
              onClick={() => { auth.clear(); nav('/login') }}
             >
              退出
             </Button>
           </Space>
         </Header>
         <Content style={{ margin: 16 }}>
           <Outlet />
         </Content>
       </Layout>
     </Layout>
  )
}

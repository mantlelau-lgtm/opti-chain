import { useEffect, useState } from 'react'
import { Layout, Menu, Typography, Button, Space } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  DatabaseOutlined, TeamOutlined, ContainerOutlined,
  BranchesOutlined, ShoppingCartOutlined, InboxOutlined,
  FundOutlined, UserOutlined, ShopOutlined, LogoutOutlined,
  SettingOutlined, ExperimentOutlined, ApartmentOutlined, FileSearchOutlined, AuditOutlined,
  KeyOutlined,
} from '@ant-design/icons'
import { auth } from '../api/client.js'
import { authApi } from '../api/index.js'

const { Header, Sider, Content } = Layout
const { Title } = Typography

// Menu entries carry the permission that gates them; the sidebar renders only
// what the actor holds (permission catalog lives in DB tables, surfaced via
// /auth/me).
const items = [
  { key: '/approvals', icon: <AuditOutlined />, label: '审批列表', group: '工作台', perm: 'approval:view' },
  { key: '/api-keys', icon: <KeyOutlined />, label: '密钥签发', group: '工作台', perm: '' },
  { key: '/boms', icon: <ExperimentOutlined />, label: 'BOM 管理', group: '研发', perm: 'bom:view' },
  { key: '/materials', icon: <DatabaseOutlined />, label: '物料', group: '基础数据', perm: 'material:view' },
  { key: '/suppliers', icon: <TeamOutlined />, label: '供应商', group: '基础数据', perm: 'supplier:view' },
  { key: '/supplier-material', icon: <ApartmentOutlined />, label: '供应关系', group: '基础数据', perm: 'supplier:view' },
  { key: '/customers', icon: <UserOutlined />, label: '客户', group: '基础数据', perm: 'customer:view' },
  { key: '/warehouses', icon: <ContainerOutlined />, label: '仓库', group: '基础数据', perm: 'warehouse:view' },
  { key: '/locations', icon: <BranchesOutlined />, label: '库位', group: '基础数据', perm: 'warehouse:view' },
  { key: '/purchase-orders', icon: <ShoppingCartOutlined />, label: '采购订单', group: '采购', perm: 'po:view' },
  { key: '/sales-orders', icon: <ShopOutlined />, label: '销售订单', group: '销售', perm: 'so:view' },
  { key: '/stock', icon: <InboxOutlined />, label: '实时库存', group: '仓储', perm: 'stock:view' },
  { key: '/inventory', icon: <ContainerOutlined />, label: '出入库', group: '仓储', perm: 'inv:move' },
  { key: '/planning', icon: <FundOutlined />, label: '计划/MRP', group: '计划', perm: 'demand:view' },
  { key: '/users', icon: <SettingOutlined />, label: '用户管理', group: '系统', perm: 'user:manage' },
  { key: '/approval-groups', icon: <SettingOutlined />, label: '审批组管理', group: '系统', perm: 'approval:manage' },
  { key: '/operation-logs', icon: <FileSearchOutlined />, label: '操作日志', group: '系统', perm: 'audit:view' },
]

export default function MainLayout() {
  const nav = useNavigate()
  const loc = useLocation()
  const [perms, setPerms] = useState(auth.perms())

  // Refresh the permission codes so menu gating stays in sync with the DB.
  useEffect(() => {
    authApi.me().then((me) => {
      auth.setPerms(me.perms || [])
      setPerms(me.perms || [])
    }).catch(() => {})
  }, [])

  // Items without a perm requirement (e.g. personal key issuance) show for
  // every authenticated user; the rest are gated by the actor's permissions.
  const visible = items.filter((it) => !it.perm || perms.includes(it.perm))
  const groups = visible.reduce((acc, it) => {
    (acc[it.group] = acc[it.group] || []).push({ key: it.key, icon: it.icon, label: it.label })
    return acc
  }, {})

  const user = auth.user()

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
           defaultOpenKeys={['工作台']}
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
             <span style={{ color: '#666' }}>{user?.tenant ? `${user.tenant} · ` : ''}{user?.name || user?.username || ''}</span>
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

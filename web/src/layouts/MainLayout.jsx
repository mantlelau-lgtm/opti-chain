import { useEffect, useState } from 'react'
import { Layout, Menu, Typography, Button, Space, Modal, Form, Input, message, Dropdown } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  DatabaseOutlined, TeamOutlined, ContainerOutlined,
  BranchesOutlined, ShoppingCartOutlined, InboxOutlined,
  FundOutlined, UserOutlined, ShopOutlined, LogoutOutlined,
  SettingOutlined, ExperimentOutlined, ApartmentOutlined, FileSearchOutlined, AuditOutlined,
  KeyOutlined, RobotOutlined,
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
  { key: '/assistant', icon: <RobotOutlined />, label: '智能助手', group: '工作台', perm: '' },
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
  const [pwdOpen, setPwdOpen] = useState(false)
  const [pwdForm] = Form.useForm()

  // Refresh the permission codes so menu gating stays in sync with the DB.
  useEffect(() => {
    authApi.me().then((me) => {
      auth.setPerms(me.perms || [])
      setPerms(me.perms || [])
    }).catch(() => {})
  }, [])

  const handleChangePassword = async () => {
    try {
      const values = await pwdForm.validateFields()
      if (values.new_password !== values.confirm_password) {
        message.error('两次输入的新密码不一致')
        return
      }
      await authApi.changePassword({ old_password: values.old_password, new_password: values.new_password })
      message.success('密码修改成功')
      setPwdOpen(false)
    } catch (e) {
      if (e.errorFields) return
      message.error(e.message || '修改失败')
    }
  }

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
           <Dropdown
             menu={{
               items: [
                 { key: 'pwd', label: '修改密码', onClick: () => { pwdForm.resetFields(); setPwdOpen(true) } },
                 { key: 'logout', label: '退出登录', danger: true,
                   onClick: () => {
                     Modal.confirm({ title: '确认退出登录？', onOk: () => { auth.clear(); nav('/login') } })
                   },
                 },
               ],
             }}
             trigger={['hover']}
           >
             <span style={{ color: '#1677ff', cursor: 'pointer' }}>
               {user?.tenant ? `${user.tenant} · ` : ''}{user?.name || user?.username || ''}
             </span>
           </Dropdown>
         </Header>
         <Content style={{ margin: 16 }}>
           <Outlet />
         </Content>

         <Modal title="修改密码" open={pwdOpen} onOk={handleChangePassword} onCancel={() => setPwdOpen(false)} destroyOnClose>
           <Form form={pwdForm} layout="vertical">
             <Form.Item name="old_password" label="旧密码" rules={[{ required: true, message: '请输入旧密码' }]}>
               <Input.Password />
             </Form.Item>
             <Form.Item name="new_password" label="新密码" rules={[{ required: true, min: 6, message: '至少 6 位' }]}>
               <Input.Password />
             </Form.Item>
             <Form.Item name="confirm_password" label="确认新密码" rules={[{ required: true, message: '请再次输入新密码' }]}>
               <Input.Password />
             </Form.Item>
           </Form>
         </Modal>
       </Layout>
     </Layout>
  )
}

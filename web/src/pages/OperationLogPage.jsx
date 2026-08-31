import { useState, useEffect, useCallback } from 'react'
import { Table, Select, Input, DatePicker, Space, Button, Tag, Card } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { operationLogApi, authApi, tenantApi } from '../api/index.js'
import { auth } from '../api/client.js'

const MODULES = ['基础数据', '采购', '销售', '仓储', '计划', '研发', '系统']
const ACTIONS = ['创建', '更新', '删除', '审批', '收货', '发布', '取消', '审核', '状态变更', '权限配置', '拆单下单', '运算']
const { RangePicker } = DatePicker

// OperationLogPage: audit trail. Business tenants see their own logs; the
// platform console shows all tenants with an extra tenant filter.
export default function OperationLogPage() {
  const isPlatform = auth.user()?.tenant === 'platform'
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(20)

  const [tenants, setTenants] = useState([])
  const [roles, setRoles] = useState([])
  const [fTenant, setFTenant] = useState(undefined)
  const [fUser, setFUser] = useState('')
  const [fRole, setFRole] = useState(undefined)
  const [fModule, setFModule] = useState(undefined)
  const [fAction, setFAction] = useState(undefined)
  const [fRange, setFRange] = useState(null)

  useEffect(() => {
    if (isPlatform) tenantApi.list({ page: 1, size: 200 }).then((r) => setTenants(r.list || [])).catch(() => {})
    authApi.catalog().then((c) => setRoles(c.roles || [])).catch(() => {})
  }, [isPlatform])

  const load = useCallback(() => {
    const params = { page, size }
    if (isPlatform && fTenant) params.tenant_id = fTenant
    if (fUser) params.user = fUser
    if (fRole) params.role = fRole
    if (fModule) params.module = fModule
    if (fAction) params.action = fAction
    if (fRange && fRange[0]) params.date_from = fRange[0].format('YYYY-MM-DD')
    if (fRange && fRange[1]) params.date_to = fRange[1].format('YYYY-MM-DD')
    setLoading(true)
    operationLogApi.list(params)
      .then((r) => { setData(r.list || []); setTotal(r.total || 0) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [page, size, isPlatform, fTenant, fUser, fRole, fModule, fAction, fRange])

  useEffect(() => { load() }, [load])

  const tenantName = (id) => tenants.find((t) => t.id === id)?.name || id
  const roleOpts = roles.map((r) => ({ label: r.name, value: r.code }))

  const cols = [
    { title: '时间', dataIndex: 'created_at', width: 160,
      render: (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-') },
    { title: '操作人', dataIndex: 'username', width: 120 },
    { title: '角色', dataIndex: 'roles', width: 160, render: (v) => v ? <Tag>{v}</Tag> : '-' },
    { title: '模块', dataIndex: 'module', width: 90 },
    { title: '动作', dataIndex: 'action', width: 100,
      render: (v) => <Tag color="blue">{v}</Tag> },
    { title: '摘要', dataIndex: 'summary' },
  ]
  if (isPlatform) {
    cols.splice(2, 0, { title: '租户', dataIndex: 'tenant_id', width: 140, render: (v) => tenantName(v) })
  }

  return (
    <Card title="操作日志" extra={
      <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
    }>
      <Space wrap style={{ marginBottom: 16 }}>
        {isPlatform && (
          <Select style={{ width: 160 }} allowClear placeholder="租户" value={fTenant}
            onChange={(v) => { setFTenant(v); setPage(1) }}
            options={tenants.map((t) => ({ label: t.name, value: t.id }))} showSearch optionFilterProp="label" />
        )}
        <Input style={{ width: 140 }} allowClear placeholder="人员" value={fUser}
          onChange={(e) => { setFUser(e.target.value); setPage(1) }} />
        <Select style={{ width: 140 }} allowClear placeholder="角色" value={fRole}
          onChange={(v) => { setFRole(v); setPage(1) }} options={roleOpts} />
        <Select style={{ width: 130 }} allowClear placeholder="模块" value={fModule}
          onChange={(v) => { setFModule(v); setPage(1) }}
          options={MODULES.map((m) => ({ label: m, value: m }))} />
        <Select style={{ width: 130 }} allowClear placeholder="类型(动作)" value={fAction}
          onChange={(v) => { setFAction(v); setPage(1) }}
          options={ACTIONS.map((a) => ({ label: a, value: a }))} />
        <RangePicker value={fRange} onChange={(v) => { setFRange(v); setPage(1) }} />
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={data}
        columns={cols}
        scroll={{ x: 'max-content' }}
        pagination={{
          current: page, pageSize: size, total, showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, s) => { setPage(p); setSize(s) },
        }}
      />
    </Card>
  )
}

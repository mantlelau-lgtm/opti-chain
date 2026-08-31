import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Input, Space, Modal, Form, message, Select, Tag, Popconfirm } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { userApi, authApi } from '../api/index.js'

// UsersPage: per-tenant user provisioning with role assignment (admin only).
export default function UsersPage() {
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [roles, setRoles] = useState([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form] = Form.useForm()

  useEffect(() => {
    authApi.catalog().then((c) => setRoles(c.roles || [])).catch(() => {})
  }, [])

  const load = useCallback(() => {
    setLoading(true)
    userApi.list({ page, size, keyword })
      .then((res) => { setData(res.list); setTotal(res.total) })
      .catch((e) => message.error(e.message))
      .finally(() => setLoading(false))
  }, [page, size, keyword])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ role_codes: ['procurement_assistant'] })
    setOpen(true)
  }
  const openEdit = (record) => {
    setEditing(record)
    form.resetFields()
    form.setFieldsValue({
      username: record.username, name: record.name, status: record.status,
      role_codes: [],
    })
    setOpen(true)
  }
  const submit = async () => {
    try {
      const values = await form.validateFields()
      const payload = {
        username: values.username,
        password: values.password || '',
        name: values.name,
        status: values.status,
        role_codes: values.role_codes || [],
      }
      if (editing) await userApi.update(editing.id, payload)
      else await userApi.create(payload)
      message.success('已保存')
      setOpen(false)
      load()
    } catch (e) {
      if (e.errorFields) return
      message.error(e.message || '操作失败')
    }
  }
  const remove = async (id) => {
    try {
      await userApi.remove(id)
      message.success('已删除')
      load()
    } catch (e) { message.error(e.message || '删除失败') }
  }

  const roleOpts = roles.map((r) => ({ label: r.name, value: r.code }))

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Input.Search placeholder="用户名搜索" allowClear style={{ width: 240 }}
          onSearch={(v) => { setKeyword(v); setPage(1) }} />
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建用户</Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={data}
        pagination={{
          current: page, pageSize: size, total, showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, s) => { setPage(p); setSize(s) },
        }}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '用户名', dataIndex: 'username' },
          { title: '姓名', dataIndex: 'name' },
          { title: '状态', dataIndex: 'status', width: 80,
            render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
          { title: '创建时间', dataIndex: 'created_at', width: 160 },
          {
            title: '操作', key: 'action', width: 160,
            render: (_, record) => (
              <Space>
                <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
                {record.username !== 'admin' && (
                  <Popconfirm title="确认删除？" onConfirm={() => remove(record.id)}>
                    <Button type="link" danger size="small">删除</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />
      <Modal title={editing ? '编辑用户' : '新建用户'} open={open} onOk={submit}
        onCancel={() => setOpen(false)} destroyOnClose width={520}>
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入' }]}>
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="password" label={editing ? '密码（留空不修改）' : '密码'} rules={editing ? [] : [{ required: true, message: '请输入' }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="name" label="姓名"><Input /></Form.Item>
          <Form.Item name="role_codes" label="角色" rules={[{ required: true, message: '至少选择一个角色' }]}>
            <Select mode="multiple" options={roleOpts} placeholder="选择角色" />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

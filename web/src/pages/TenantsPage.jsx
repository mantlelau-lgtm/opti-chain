import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Input, Space, Modal, Form, message, Select, Tag, Popconfirm } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { tenantApi } from '../api/index.js'

// TenantsPage: platform scope — tenant lifecycle (create / suspend / activate).
export default function TenantsPage() {
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const load = useCallback(() => {
    setLoading(true)
    tenantApi.list({ page, size, keyword })
      .then((res) => { setData(res.list); setTotal(res.total) })
      .catch((e) => message.error(e.message))
      .finally(() => setLoading(false))
  }, [page, size, keyword])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
    form.resetFields()
    form.setFieldsValue({ plan: 'FREE', status: 'ACTIVE' })
    setOpen(true)
  }
  const submit = async () => {
    try {
      const values = await form.validateFields()
      await tenantApi.create(values)
      message.success('租户已创建')
      setOpen(false)
      load()
    } catch (e) {
      if (e.errorFields) return
      message.error(e.message || '操作失败')
    }
  }
  const setStatus = async (record, status) => {
    try {
      await tenantApi.update(record.id, { ...record, status })
      message.success(status === 'SUSPENDED' ? '租户已停用' : '租户已启用')
      load()
    } catch (e) { message.error(e.message || '操作失败') }
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Input.Search placeholder="编码/名称搜索" allowClear style={{ width: 240 }}
          onSearch={(v) => { setKeyword(v); setPage(1) }} />
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建租户</Button>
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
          { title: '租户编码', dataIndex: 'code' },
          { title: '名称', dataIndex: 'name' },
          { title: '套餐', dataIndex: 'plan', width: 100,
            render: (v) => <Tag>{v}</Tag> },
          { title: '状态', dataIndex: 'status', width: 100,
            render: (v) => <Tag color={v === 'ACTIVE' ? 'green' : 'red'}>{v === 'ACTIVE' ? '启用' : '停用'}</Tag> },
          { title: '到期时间', dataIndex: 'expires_at', width: 160,
            render: (v) => v || '-' },
          {
            title: '操作', key: 'action', width: 120,
            render: (_, record) => record.code !== 'platform' && record.code !== 'demo' ? (
              record.status === 'ACTIVE'
                ? <Popconfirm title="停用后该租户全部用户无法登录，确认？" onConfirm={() => setStatus(record, 'SUSPENDED')}>
                    <Button type="link" danger size="small">停用</Button>
                  </Popconfirm>
                : <Button type="link" size="small" onClick={() => setStatus(record, 'ACTIVE')}>启用</Button>
            ) : <span style={{ color: '#bbb' }}>内置</span>,
          },
        ]}
      />
      <Modal title="新建租户" open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="code" label="租户编码" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="如 acme" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="plan" label="套餐">
            <Select options={[{ label: 'FREE', value: 'FREE' }, { label: 'PRO', value: 'PRO' }]} />
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select options={[{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'SUSPENDED' }]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

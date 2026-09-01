import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Tag, message, Popconfirm } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { approvalApi, userApi } from '../api/index.js'

const ORDER_TYPES = [
  { label: '采购单', value: 'PO' },
  { label: '销售单', value: 'SO' },
]
const MODES = [
  { label: '会签（全员通过）', value: 'ALL' },
  { label: '或签（任一通过）', value: 'ANY' },
]

// ApprovalGroupPage: per-order-type approval groups (tenant scope).
export default function ApprovalGroupPage() {
  const [list, setList] = useState([])
  const [users, setUsers] = useState([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form] = Form.useForm()

  const load = useCallback(() => {
    approvalApi.groups({ page: 1, size: 200 }).then((r) => setList(r.list || [])).catch(() => {})
  }, [])

  useEffect(() => {
    load()
    userApi.list({ page: 1, size: 500 }).then((r) => setUsers(r.list || [])).catch(() => {})
  }, [load])

  const openCreate = () => { setEditing(null); form.resetFields(); form.setFieldsValue({ mode: 'ALL' }); setOpen(true) }
  const openEdit = (r) => {
    setEditing(r)
    form.resetFields()
    form.setFieldsValue({
      name: r.name, order_type: r.order_type, mode: r.mode,
      member_ids: (r.members || []).map((m) => m.user_id),
    })
    setOpen(true)
  }
  const submit = async () => {
    try {
      const values = await form.validateFields()
      const payload = {
        name: values.name, order_type: values.order_type, mode: values.mode,
        members: (values.member_ids || []).map((uid) => ({ user_id: uid })),
      }
      if (editing) await approvalApi.updateGroup(editing.id, payload)
      else await approvalApi.createGroup(payload)
      message.success('已保存')
      setOpen(false)
      load()
    } catch (e) { if (!e.errorFields) message.error(e.message || '操作失败') }
  }
  const remove = async (id) => {
    try { await approvalApi.removeGroup(id); message.success('已删除'); load() }
    catch (e) { message.error(e.message || '删除失败') }
  }

  const userOpts = users.map((u) => ({ label: `${u.name || u.username}（${u.username}）`, value: u.id }))

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <span style={{ color: '#666' }}>按订单类型定义审批组；提交审批后组内成员在「审批工作台」处理。</span>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建审批组</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={list}
        pagination={false}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          { title: '组名', dataIndex: 'name' },
          { title: '订单类型', dataIndex: 'order_type', width: 110, render: (v) => <Tag color="blue">{ORDER_TYPES.find((o) => o.value === v)?.label || v}</Tag> },
          { title: '审批模式', dataIndex: 'mode', width: 140, render: (v) => <Tag>{MODES.find((m) => m.value === v)?.label || v}</Tag> },
          {
            title: '审批成员', dataIndex: 'members',
            render: (members) => (members || []).map((m) => users.find((u) => u.id === m.user_id)?.name || m.user_id).join('、'),
          },
          {
            title: '操作', key: 'a', width: 140,
            render: (_, r) => (
              <Space>
                <Button type="link" size="small" onClick={() => openEdit(r)}>编辑</Button>
                <Popconfirm title="确认删除？" onConfirm={() => remove(r.id)}>
                  <Button type="link" danger size="small">删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑审批组' : '新建审批组'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="组名" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="如 采购单审批组" />
          </Form.Item>
          <Form.Item name="order_type" label="订单类型" rules={[{ required: true, message: '请选择' }]}>
            <Select options={ORDER_TYPES} disabled={!!editing} />
          </Form.Item>
          <Form.Item name="mode" label="审批模式" rules={[{ required: true, message: '请选择' }]}>
            <Select options={MODES} />
          </Form.Item>
          <Form.Item name="member_ids" label="审批成员" rules={[{ required: true, message: '请至少选一人' }]}>
            <Select mode="multiple" options={userOpts} placeholder="选择审批人" showSearch optionFilterProp="label" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Input, Space, Modal, Form, message, Select, Tag, Popconfirm, Typography, Tooltip } from 'antd'
import { PlusOutlined, CopyOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { apiKeyApi } from '../api/index.js'

const { Text } = Typography

// ApiKeyPage: every user issues their own AK/SK for agent / MCP access. The
// key is bound to the user and its permission set is derived from the user's
// roles automatically (never chosen by hand). The SK is shown exactly once.
export default function ApiKeyPage() {
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [open, setOpen] = useState(false)
  const [issued, setIssued] = useState(null) // { ak, sk, name } shown once
  const [form] = Form.useForm()

  const load = useCallback(() => {
    setLoading(true)
    apiKeyApi.list({ page, size, keyword })
      .then((res) => { setData(res.list); setTotal(res.total) })
      .catch((e) => message.error(e.message))
      .finally(() => setLoading(false))
  }, [page, size, keyword])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
    form.resetFields()
    setOpen(true)
  }

  const submit = async () => {
    try {
      const values = await form.validateFields()
      const payload = {
        name: values.name,
        expires_at: values.expires_days ? dayjs().add(values.expires_days, 'day').toISOString() : '',
      }
      const res = await apiKeyApi.create(payload)
      message.success('密钥已签发')
      setOpen(false)
      setIssued({ ak: res.key.ak, sk: res.sk, name: res.key.name })
      load()
    } catch (e) {
      if (e.errorFields) return
      message.error(e.message || '签发失败')
    }
  }

  const toggle = async (record) => {
    try {
      if (record.status === 1) await apiKeyApi.disable(record.id)
      else await apiKeyApi.enable(record.id)
      message.success('已更新')
      load()
    } catch (e) { message.error(e.message || '操作失败') }
  }
  const remove = async (id) => {
    try {
      await apiKeyApi.remove(id)
      message.success('已删除')
      load()
    } catch (e) { message.error(e.message || '删除失败') }
  }

  const copy = async (text) => {
    try {
      await navigator.clipboard.writeText(text)
      message.success('已复制')
    } catch { message.error('复制失败，请手动复制') }
  }

  // Permissions are auto-derived from the user's roles at issuance time.
  const permName = (codes) => {
    if (!codes) return <Tag color="gold">全部权限</Tag>
    const list = codes.split(',').filter(Boolean)
    if (list.length === 0) return <Tag color="gold">全部权限</Tag>
    return (
      <>
        <Tag color="blue">随角色授予</Tag>
        {list.map((c) => <Tag key={c}>{c}</Tag>)}
      </>
    )
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Input.Search placeholder="名称 / AK 搜索" allowClear style={{ width: 260 }}
          onSearch={(v) => { setKeyword(v); setPage(1) }} />
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>签发密钥</Button>
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
          { title: '名称', dataIndex: 'name', width: 140 },
          {
            title: 'Access Key', dataIndex: 'ak', width: 220,
            render: (v) => <Text code copyable={{ text: v }}>{v}</Text>,
          },
          { title: '权限范围', dataIndex: 'permissions', render: (v) => permName(v) },
          {
            title: '状态', dataIndex: 'status', width: 80,
            render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag>,
          },
          {
            title: '过期时间', dataIndex: 'expires_at', width: 160,
            render: (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm') : <Text type="secondary">永久</Text>),
          },
          { title: '创建时间', dataIndex: 'created_at', width: 160,
            render: (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-') },
          {
            title: '操作', key: 'action', width: 160,
            render: (_, record) => (
              <Space>
                <Button type="link" size="small" onClick={() => toggle(record)}>
                  {record.status === 1 ? '禁用' : '启用'}
                </Button>
                <Popconfirm title="确认删除？删除后该密钥立即失效" onConfirm={() => remove(record.id)}>
                  <Button type="link" danger size="small">删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal title="签发密钥" open={open} onOk={submit}
        onCancel={() => setOpen(false)} destroyOnClose width={480}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入密钥名称' }]}>
            <Input placeholder="例如：采购助手、MCP agent" />
          </Form.Item>
          <Form.Item name="expires_days" label="过期时间（留空 = 永久有效）">
            <Select
              allowClear
              placeholder="选择有效期"
              options={[
                { label: '7 天', value: 7 },
                { label: '30 天', value: 30 },
                { label: '90 天', value: 90 },
                { label: '1 年', value: 365 },
              ]}
            />
          </Form.Item>
          <Text type="secondary">权限范围将自动随您当前的角色权限授予，无需手动选择。</Text>
        </Form>
      </Modal>

      <Modal
        title="密钥签发成功"
        open={!!issued}
        footer={null}
        onCancel={() => setIssued(null)}
      >
        <p>请立即保存 Secret Key，<Text type="danger">关闭后无法再次查看</Text>。</p>
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>
            <Text strong>名称：</Text>{issued?.name}
          </div>
          <div>
            <Text strong>Access Key：</Text>
            <Text code>{issued?.ak}</Text>
            <Tooltip title="复制 AK"><Button type="link" size="small" icon={<CopyOutlined />} onClick={() => copy(issued.ak)} /></Tooltip>
          </div>
          <div>
            <Text strong>Secret Key：</Text>
            <Text code copyable={{ text: issued?.sk }}>{issued?.sk}</Text>
          </div>
        </Space>
      </Modal>
    </div>
  )
}

import { useState, useEffect, useCallback, useRef } from 'react'
import { Table, Button, Input, Space, Modal, Form, message, Tag, Select, Progress, Card, Alert, Popconfirm } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { storageApi } from '../api/index.js'

const STATUS_LABEL = { idle: '空闲', running: '进行中', done: '完成', failed: '失败' }
const STATUS_COLOR = { idle: 'default', running: 'processing', done: 'success', failed: 'error' }

// 隐藏 DSN 中的密码，仅用于展示。
function maskDsn(driver, dsn) {
  if (!dsn) return '-'
  if (driver === 'mysql') return dsn.replace(/^([^:]+):[^@]+@/, '$1:***@')
  if (driver === 'postgres') return dsn.replace(/password=\S+/, 'password=***')
  return dsn
}

// StoragePage (platform console): configure target data sources and migrate the
// live database into one of them, with progress tracking. After a successful
// migration, restart the server with the matching SCMDB_DRIVER/SCMDB_DSN env
// vars to switch to the new storage.
export default function StoragePage() {
  const [list, setList] = useState([])
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()
  const [progress, setProgress] = useState(null)
  const [current, setCurrent] = useState(null)
  const timerRef = useRef(null)

  const load = useCallback(() => {
    storageApi.dataSources({ page: 1, size: 200 }).then((r) => setList(r.list || [])).catch(() => {})
  }, [])

  useEffect(() => {
    load()
    storageApi.current().then((c) => setCurrent(c)).catch(() => {})
  }, [load])

  const poll = useCallback(() => {
    storageApi.status().then((s) => {
      setProgress(s)
      if (s.status === 'running') {
        timerRef.current = setTimeout(poll, 1000)
      }
    }).catch(() => {})
  }, [])

  useEffect(() => {
    poll()
    return () => clearTimeout(timerRef.current)
  }, [poll])

  const submit = async () => {
    try {
      const values = await form.validateFields()
      await storageApi.createDataSource(values)
      message.success('数据源已保存')
      setOpen(false)
      form.resetFields()
      load()
    } catch (e) { if (!e.errorFields) message.error(e.message || '保存失败') }
  }

  const test = async (record) => {
    try {
      await storageApi.testConnection({ driver: record.driver, dsn: record.dsn })
      message.success('连接成功')
    } catch (e) { message.error(e.message || '连接失败') }
  }

  const migrate = async (record) => {
    try {
      await storageApi.migrate(record.id)
      message.success('迁移已启动')
      poll()
    } catch (e) { message.error(e.message || '启动迁移失败') }
  }

  const remove = async (id) => {
    try {
      await storageApi.removeDataSource(id)
      message.success('已删除')
      load()
    } catch (e) { message.error(e.message || '删除失败') }
  }

  const pct = progress && progress.total_tables > 0
    ? Math.round(progress.done_tables / progress.total_tables * 100) : 0

  return (
    <div>
      {current && (
        <Alert type="info" showIcon style={{ marginBottom: 16 }}
          message={`当前数据源：${current.driver === 'mysql' ? 'MySQL' : current.driver === 'postgres' ? 'PostgreSQL' : current.driver}`}
          description={maskDsn(current.driver, current.dsn)} />
      )}

      <Card title="数据源配置" style={{ marginBottom: 16 }}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); form.setFieldsValue({ driver: 'mysql' }); setOpen(true) }}>新增数据源</Button>}>
        <Table
          rowKey="id"
          dataSource={list}
          pagination={false}
          locale={{ emptyText: '暂无数据源，点击右上角新增' }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 60 },
            { title: '名称', dataIndex: 'name' },
            { title: '驱动', dataIndex: 'driver', width: 100,
              render: (v) => <Tag color={v === 'mysql' ? 'blue' : 'purple'}>{v === 'mysql' ? 'MySQL' : 'PostgreSQL'}</Tag> },
            { title: 'DSN', dataIndex: 'dsn', ellipsis: true },
            {
              title: '操作', key: 'action', width: 240,
              render: (_, record) => (
                <Space>
                  <Button type="link" size="small" onClick={() => test(record)}>测试连接</Button>
                  {record.driver === current?.driver
                    ? <Tag color="green">当前类型</Tag>
                    : <Button type="link" size="small" onClick={() => migrate(record)}>一键迁移</Button>}
                  <Popconfirm title="确认删除？" onConfirm={() => remove(record.id)}>
                    <Button type="link" danger size="small">删除</Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      {progress && (
        <Card title="迁移进度" extra={<Button icon={<ReloadOutlined />} onClick={poll}>刷新</Button>}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space>
              <Tag color={STATUS_COLOR[progress.status]}>{STATUS_LABEL[progress.status] || progress.status}</Tag>
              {progress.target_name && <span>目标：{progress.target_name}</span>}
              {progress.current_table && progress.status === 'running' && <span>当前表：{progress.current_table}</span>}
            </Space>
            <Progress percent={pct} status={progress.status === 'failed' ? 'exception' : progress.status === 'done' ? 'success' : 'active'} />
            <span style={{ color: '#666' }}>
              表 {progress.done_tables}/{progress.total_tables} · 行 {progress.done_rows}/{progress.total_rows}
            </span>
            {progress.error && <Alert type="error" message={progress.error} />}
            {progress.status === 'done' && (
              <Alert type="success" showIcon message="迁移完成"
                description="请修改 .env 中的 SCMDB_DRIVER / SCMDB_DSN 为目标数据源，然后执行 ./restart.sh 切换存储。" />
            )}
          </Space>
        </Card>
      )}

      <Modal title="新增数据源" open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="如 生产 MySQL" />
          </Form.Item>
          <Form.Item name="driver" label="驱动" rules={[{ required: true, message: '请选择' }]}>
            <Select options={[{ label: 'MySQL', value: 'mysql' }, { label: 'PostgreSQL', value: 'postgres' }]} />
          </Form.Item>
          <Form.Item name="dsn" label="DSN" rules={[{ required: true, message: '请输入' }]}>
            <Input.TextArea rows={2}
              placeholder="MySQL: user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True
PostgreSQL: host=127.0.0.1 user=postgres password=postgres123 dbname=scm port=5432" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

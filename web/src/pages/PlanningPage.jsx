import { useState, useEffect, useCallback } from 'react'
import {
  Tabs, Table, Button, Input, Space, Modal, Form, message,
  Tag, Select, InputNumber, DatePicker, Popconfirm,
} from 'antd'
import { PlusOutlined, ThunderboltOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { planningApi, materialApi } from '../api/index.js'

const SOURCES = [
    { label: '预测', value: 'FORECAST' },
    { label: '销售订单', value: 'SALES_ORDER' },
]
const sourceLabel = (s) => SOURCES.find((o) => o.value === s)?.label || s

// PlanningPage has two tabs:
//   1) 需求 (plan_demand) — full CRUD.
//   2) MRP结果 — run a compute batch, then view generated suggestions.
export default function PlanningPage() {
  // ---- demand state ----
  const [demands, setDemands] = useState([])
  const [dTotal, setDTotal] = useState(0)
  const [dPage, setDPage] = useState(1)
  const [dSize, setDSize] = useState(10)
  const [dKeyword, setDKeyword] = useState('')
  const [dLoading, setDLoading] = useState(false)
  const [dOpen, setDOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [dForm] = Form.useForm()

  // ---- mrp state ----
  const [mrp, setMrp] = useState([])
  const [mTotal, setMTotal] = useState(0)
  const [mPage, setMPage] = useState(1)
  const [mSize, setMSize] = useState(10)
  const [mLoading, setMLoading] = useState(false)
  const [computing, setComputing] = useState(false)

  const [materials, setMaterials] = useState([])

  useEffect(() => {
     let mounted = true
     materialApi.list({ page: 1, size: 1000 })
        .then((r) => { if (mounted) setMaterials(r.list || []) })
        .catch(() => {})
     return () => { mounted = false }
      }, [])

  const matName = (id) => materials.find((m) => m.id === id)?.name || id
  const matOpts = materials.map((m) => ({ label: `${m.sku_code} ${m.name}`, value: m.id }))

  // ---- demand CRUD ----
  const loadDemands = useCallback(() => {
     setDLoading(true)
     planningApi.demands({ page: dPage, size: dSize, keyword: dKeyword })
          .then((res) => { setDemands(res.list); setDTotal(res.total) })
          .catch((e) => message.error(e.message))
          .finally(() => setDLoading(false))
       }, [dPage, dSize, dKeyword])

  useEffect(() => { loadDemands() }, [loadDemands])

  const openCreate = () => {
     setEditing(null)
     dForm.resetFields()
     dForm.setFieldsValue({ demand_date: dayjs(), source_type: 'FORECAST', demand_qty: 1 })
     setDOpen(true)
     }

  const openEdit = (record) => {
     setEditing(record)
     dForm.setFieldsValue({
        demand_number: record.demand_number,
        material_id: record.material_id,
        demand_qty: Number(record.demand_qty),
        demand_date: record.demand_date ? dayjs(record.demand_date) : undefined,
        source_type: record.source_type,
       })
     setDOpen(true)
     }

  const submitDemand = async () => {
     try {
        const values = await dForm.validateFields()
        const payload = {
          demand_number: values.demand_number,
          material_id: values.material_id,
          demand_qty: String(values.demand_qty),
          demand_date: values.demand_date?.format ? values.demand_date.format('YYYY-MM-DD') : undefined,
          source_type: values.source_type,
         }
        if (editing) {
           await planningApi.updateDemand(editing.id, payload)
           message.success('已更新')
           } else {
           await planningApi.createDemand(payload)
           message.success('已创建')
           }
        setDOpen(false)
        loadDemands()
         } catch (e) {
        if (e.errorFields) return
        message.error(e.message || '操作失败')
         }
     }

  const removeDemand = async (id) => {
     try {
        await planningApi.removeDemand(id)
        message.success('已删除')
        loadDemands()
         } catch (e) { message.error(e.message || '删除失败') }
     }

  // ---- mrp ----
  const loadMrp = useCallback(() => {
     setMLoading(true)
     planningApi.mrp({ page: mPage, size: mSize })
          .then((res) => { setMrp(res.list); setMTotal(res.total) })
          .catch((e) => message.error(e.message))
          .finally(() => setMLoading(false))
       }, [mPage, mSize])

  useEffect(() => { loadMrp() }, [loadMrp])

  const compute = async () => {
     setComputing(true)
     try {
        const res = await planningApi.compute()
        message.success(`MRP 计算完成，批次 ${res.mrp_number}`)
        setMPage(1)
        loadMrp()
         } catch (e) {
        message.error(e.message || '计算失败')
         } finally {
        setComputing(false)
         }
     }

  const removeMrp = async (id) => {
     try {
        await planningApi.removeMrp(id)
        message.success('已删除')
        loadMrp()
         } catch (e) { message.error(e.message || '删除失败') }
     }

  const demandCols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
     { title: '需求单号', dataIndex: 'demand_number', width: 160 },
      { title: '物料', dataIndex: 'material_id', render: (v) => matName(v) },
      { title: '需求数量', dataIndex: 'demand_qty' },
      { title: '需求日期', dataIndex: 'demand_date',
       render: (v) => v ? dayjs(v).format('YYYY-MM-DD') : '-' },
      { title: '来源', dataIndex: 'source_type',
       render: (v) => sourceLabel(v) },
      {
       title: '状态', dataIndex: 'status',
       render: (v) => <Tag color={v === 'GENERATED' ? 'success' : 'blue'}>{v}</Tag>,
       },
      {
       title: '操作', key: 'action', fixed: 'right', width: 140,
       render: (_, record) => (
          <Space>
            <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
             <Popconfirm title="确认删除？" onConfirm={() => removeDemand(record.id)}>
               <Button type="link" danger size="small">删除</Button>
             </Popconfirm>
           </Space>
        ),
       },
     ]

  const mrpCols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
      { title: '批次', dataIndex: 'mrp_number', width: 160 },
      { title: '物料', dataIndex: 'material_id', render: (v) => matName(v) },
      { title: '现有库存', dataIndex: 'current_stock' },
      { title: '在途数量', dataIndex: 'on_order_qty' },
      { title: '总需求', dataIndex: 'gross_demand' },
      { title: '建议采购量', dataIndex: 'suggested_po_qty' },
      { title: '建议日期', dataIndex: 'suggested_po_date',
       render: (v) => v ? dayjs(v).format('YYYY-MM-DD') : '-' },
      {
       title: '状态', dataIndex: 'status',
       render: (v) => <Tag color={v === 'CONVERTED' ? 'success' : 'processing'}>{v}</Tag>,
       },
      {
       title: '操作', key: 'action', fixed: 'right', width: 100,
       render: (_, record) => (
           <Popconfirm title="确认删除？" onConfirm={() => removeMrp(record.id)}>
            <Button type="link" danger size="small">删除</Button>
           </Popconfirm>
        ),
       },
     ]

  const items = [
      {
       key: 'demands',
       label: '需求',
       children: (
          <div>
            <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
              <Input.Search
              placeholder="需求单号搜索"
              allowClear
              style={{ width: 240 }}
              onSearch={(v) => { setDKeyword(v); setDPage(1) }}
              />
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建需求</Button>
            </Space>
            <Table
            rowKey="id"
            loading={dLoading}
            dataSource={demands}
            columns={demandCols}
            scroll={{ x: 'max-content' }}
            pagination={{
              current: dPage, pageSize: dSize, total: dTotal,
              showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
              onChange: (p, s) => { setDPage(p); setDSize(s) },
             }}
            />
          </div>
        ),
      },
      {
       key: 'mrp',
       label: 'MRP结果',
       children: (
          <div>
            <Space style={{ marginBottom: 16, justifyContent: 'flex-end', width: '100%' }}>
              <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              loading={computing}
              onClick={compute}
              >
              运行 MRP 计算
              </Button>
            </Space>
            <Table
            rowKey="id"
            loading={mLoading}
            dataSource={mrp}
            columns={mrpCols}
            scroll={{ x: 'max-content' }}
            pagination={{
              current: mPage, pageSize: mSize, total: mTotal,
              showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
              onChange: (p, s) => { setMPage(p); setMSize(s) },
             }}
            />
          </div>
        ),
      },
    ]

  return (
    <div>
        <Tabs items={items} />
        <Modal
        title={editing ? '编辑需求' : '新建需求'}
        open={dOpen}
        onOk={submitDemand}
        onCancel={() => setDOpen(false)}
        destroyOnClose
        width={560}
        >
          <Form form={dForm} layout="vertical">
            <Form.Item name="demand_number" label="需求单号" rules={[{ required: true, message: '请输入' }]}>
              <Input placeholder="如 DEMAND-0001" disabled={!!editing} />
            </Form.Item>
            <Form.Item name="material_id" label="物料" rules={[{ required: true, message: '请选择' }]}>
              <Select options={matOpts} placeholder="选择物料" showSearch optionFilterProp="label" />
            </Form.Item>
            <Space size="large" align="start">
              <Form.Item name="demand_qty" label="需求数量" rules={[{ required: true, message: '请输入' }]}>
                <InputNumber min={0} step={0.0001} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="demand_date" label="需求日期">
                <DatePicker />
              </Form.Item>
              <Form.Item name="source_type" label="来源" rules={[{ required: true, message: '请选择' }]}>
                <Select options={SOURCES} style={{ width: 140 }} />
              </Form.Item>
            </Space>
          </Form>
        </Modal>
      </div>
    )
}

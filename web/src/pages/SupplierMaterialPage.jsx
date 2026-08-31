import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Input, Space, Modal, Form, message, Tag, Select, InputNumber, Popconfirm, Switch } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useSearchParams } from 'react-router-dom'
import { supplierMaterialApi, supplierApi, materialApi } from '../api/index.js'

// SupplierMaterialPage: supplier↔material supply relationships with prices.
// Filter by supplier OR material; each row binds a pair with price + lead time.
export default function SupplierMaterialPage() {
  const [sp] = useSearchParams()
  const [suppliers, setSuppliers] = useState([])
  const [materials, setMaterials] = useState([])
  const [supplierId, setSupplierId] = useState(sp.get('supplier_id') ? Number(sp.get('supplier_id')) : undefined)
  const [materialId, setMaterialId] = useState(sp.get('material_id') ? Number(sp.get('material_id')) : undefined)
  const [data, setData] = useState([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form] = Form.useForm()

  useEffect(() => {
    Promise.all([
      supplierApi.list({ page: 1, size: 200 }),
      materialApi.list({ page: 1, size: 1000 }),
    ]).then(([s, m]) => {
      setSuppliers(s.list || [])
      setMaterials(m.list || [])
    }).catch(() => {})
  }, [])

  const load = useCallback(() => {
    const params = {}
    if (supplierId) params.supplier_id = supplierId
    if (materialId) params.material_id = materialId
    setLoading(true)
    supplierMaterialApi.list(params)
      .then((list) => setData(list || []))
      .catch((e) => message.error(e.message))
      .finally(() => setLoading(false))
  }, [supplierId, materialId])

  useEffect(() => { load() }, [load])

  const openBind = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ lead_time_days: 0, is_preferred: false })
    setOpen(true)
  }
  const openEdit = (record) => {
    setEditing(record)
    form.resetFields()
    form.setFieldsValue({
      supplier_id: record.supplier_id, material_id: record.material_id,
      unit_price: String(record.unit_price), lead_time_days: record.lead_time_days,
      is_preferred: record.is_preferred,
    })
    setOpen(true)
  }
  const submit = async () => {
    try {
      const values = await form.validateFields()
      const payload = {
        supplier_id: values.supplier_id, material_id: values.material_id,
        unit_price: String(values.unit_price), lead_time_days: values.lead_time_days ?? 0,
        is_preferred: values.is_preferred ?? false,
      }
      if (editing) await supplierMaterialApi.update(editing.id, payload)
      else await supplierMaterialApi.bind(payload)
      message.success(editing ? '已更新' : '已绑定')
      setOpen(false)
      load()
    } catch (e) { if (!e.errorFields) message.error(e.message || '操作失败') }
  }
  const remove = async (id) => {
    try {
      await supplierMaterialApi.remove(id)
      message.success('已解绑')
      load()
    } catch (e) { message.error(e.message || '解绑失败') }
  }

  const supplierName = (id) => suppliers.find((s) => s.id === id)?.name || id
  const materialName = (id) => materials.find((m) => m.id === id)?.name || id
  const supplierOpts = suppliers.map((s) => ({ label: `${s.supplier_code} ${s.name}`, value: s.id }))
  const materialOpts = materials.map((m) => ({ label: `${m.sku_code} ${m.name}`, value: m.id }))

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Space>
          <span>供应商</span>
          <Select style={{ width: 240 }} options={supplierOpts} value={supplierId} allowClear
            onChange={(v) => { setSupplierId(v); setMaterialId(undefined) }} placeholder="按供应商筛选" showSearch optionFilterProp="label" />
          <span>物料</span>
          <Select style={{ width: 240 }} options={materialOpts} value={materialId} allowClear
            onChange={(v) => { setMaterialId(v); setSupplierId(undefined) }} placeholder="按物料筛选" showSearch optionFilterProp="label" />
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={openBind}>绑定供应关系</Button>
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={data}
        pagination={false}
        locale={{ emptyText: '选择供应商或物料查看供应关系，或点右上角绑定' }}
        columns={[
          { title: '供应商', dataIndex: 'supplier_id', render: (v) => supplierName(v) },
          { title: '物料', dataIndex: 'material_id', render: (v) => materialName(v) },
          { title: '供应单价', dataIndex: 'unit_price', width: 120 },
          { title: '交期(天)', dataIndex: 'lead_time_days', width: 100 },
          { title: '首选', dataIndex: 'is_preferred', width: 80,
            render: (v) => (v ? <Tag color="blue">首选</Tag> : '-') },
          {
            title: '操作', key: 'action', width: 160,
            render: (_, record) => (
              <Space>
                <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
                <Popconfirm title="确认解绑？" onConfirm={() => remove(record.id)}>
                  <Button type="link" danger size="small">解绑</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑供应关系' : '绑定供应关系'} open={open} onOk={submit}
        onCancel={() => setOpen(false)} destroyOnClose width={520}>
        <Form form={form} layout="vertical">
          <Form.Item name="supplier_id" label="供应商" rules={[{ required: true, message: '请选择' }]}>
            <Select options={supplierOpts} showSearch optionFilterProp="label" disabled={!!editing} placeholder="选择供应商" />
          </Form.Item>
          <Form.Item name="material_id" label="物料" rules={[{ required: true, message: '请选择' }]}>
            <Select options={materialOpts} showSearch optionFilterProp="label" disabled={!!editing} placeholder="选择物料" />
          </Form.Item>
          <Form.Item name="unit_price" label="供应单价" rules={[{ required: true, message: '请输入' }]}>
            <InputNumber min={0} step={0.0001} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="lead_time_days" label="交期(天)">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="is_preferred" label="设为首选供应商" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

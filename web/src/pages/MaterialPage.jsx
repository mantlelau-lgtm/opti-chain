import { useState } from 'react'
import CrudTable from '../components/CrudTable.jsx'
import { Input, InputNumber, Select, Tag, Button, Modal, Table, Space, message, Form, Popconfirm, InputNumber as AntInputNumber, Switch } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { materialApi, supplierApi, supplierMaterialApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]

export default function MaterialPage() {
  const [relOpen, setRelOpen] = useState(false)
  const [currentMat, setCurrentMat] = useState(null)
  const [relData, setRelData] = useState([])
  const [relLoading, setRelLoading] = useState(false)
  const [relSuppliers, setRelSuppliers] = useState([])
  const [relEdit, setRelEdit] = useState(null)
  const [relFormOpen, setRelFormOpen] = useState(false)
  const [form] = Form.useForm()

  const openRels = async (record) => {
    setCurrentMat(record)
    setRelOpen(true)
    setRelLoading(true)
    try {
      const [rels, sups] = await Promise.all([
        supplierMaterialApi.list({ material_id: record.id }),
        supplierApi.list({ page: 1, size: 500 }),
      ])
      setRelData(rels || [])
      setRelSuppliers(sups.list || [])
    } catch (e) {
      message.error(e.message || '加载失败')
    } finally {
      setRelLoading(false)
    }
  }

  const refreshRels = async () => {
    if (!currentMat) return
    setRelLoading(true)
    try {
      const rels = await supplierMaterialApi.list({ material_id: currentMat.id })
      setRelData(rels || [])
    } catch (e) {
      message.error(e.message || '加载失败')
    } finally {
      setRelLoading(false)
    }
  }

  const openBind = () => {
    setRelEdit(null)
    form.resetFields()
    form.setFieldsValue({ lead_time_days: 0, is_preferred: false })
    setRelFormOpen(true)
  }

  const openRelEdit = (r) => {
    setRelEdit(r)
    form.resetFields()
    form.setFieldsValue({
      supplier_id: r.supplier_id,
      unit_price: Number(r.unit_price),
      lead_time_days: r.lead_time_days,
      is_preferred: r.is_preferred,
    })
    setRelFormOpen(true)
  }

  const submitRel = async () => {
    try {
      const values = await form.validateFields()
      const payload = {
        material_id: currentMat.id,
        supplier_id: values.supplier_id,
        unit_price: String(values.unit_price),
        lead_time_days: values.lead_time_days ?? 0,
        is_preferred: values.is_preferred ?? false,
      }
      if (relEdit) {
        await supplierMaterialApi.update(relEdit.id, payload)
        message.success('已更新')
      } else {
        await supplierMaterialApi.bind(payload)
        message.success('已绑定')
      }
      setRelFormOpen(false)
      refreshRels()
    } catch (e) {
      if (!e.errorFields) message.error(e.message || '操作失败')
    }
  }

  const unbind = async (id) => {
    try {
      await supplierMaterialApi.remove(id)
      message.success('已解绑')
      refreshRels()
    } catch (e) {
      message.error(e.message || '解绑失败')
    }
  }

  const supplierName = (id) => relSuppliers.find((s) => s.id === id)?.name || id
  const supplierOpts = relSuppliers.map((s) => ({
    label: `${s.supplier_code} ${s.name}${s.audit_status !== 'APPROVED' ? `（${s.audit_status}）` : ''}`,
    value: s.id,
  }))

  const resource = {
    title: '物料',
    api: materialApi,
    columns: [
       { title: 'ID', dataIndex: 'id', width: 60 },
       { title: 'SKU编码', dataIndex: 'sku_code' },
       { title: '名称', dataIndex: 'name' },
       { title: '分类', dataIndex: 'category' },
       { title: '单位', dataIndex: 'unit', width: 80 },
       { title: '安全下限', dataIndex: 'min_stock' },
       { title: '安全上限', dataIndex: 'max_stock' },
       {
         title: '状态',
         dataIndex: 'status',
         render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag>,
       },
      ],
    fields: [
       { name: 'sku_code', label: 'SKU编码', rules: [{ required: true, message: '请输入' }] },
       { name: 'name', label: '名称', rules: [{ required: true, message: '请输入' }] },
       { name: 'category', label: '分类', rules: [{ required: true, message: '请输入' }] },
       {
         name: 'unit', label: '基本单位', rules: [{ required: true, message: '请输入' }],
         render: () => <Input placeholder="个/kg/箱" />,
       },
       { name: 'min_stock', label: '安全库存下限', valuePropName: 'value',
         render: () => <InputNumber style={{ width: '100%' }} min={0} step={0.01} /> },
       { name: 'max_stock', label: '安全库存上限', valuePropName: 'value',
         render: () => <InputNumber style={{ width: '100%' }} min={0} step={0.01} /> },
       { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
         render: () => <Select options={STATUS} /> },
      ],
    makeFormValues: (r) => r ? {
       sku_code: r.sku_code, name: r.name, category: r.category, unit: r.unit,
       min_stock: Number(r.min_stock), max_stock: Number(r.max_stock), status: r.status,
      } : { status: 1, min_stock: 0, max_stock: 0 },
    extraActions: (record) => (
      <Button type="link" size="small" onClick={() => openRels(record)}>查看供应关系</Button>
    ),
  }

  return (
    <>
      <CrudTable resource={resource} />

      <Modal
        title={currentMat ? `供应关系 · ${currentMat.sku_code} ${currentMat.name}` : '供应关系'}
        open={relOpen}
        onCancel={() => setRelOpen(false)}
        footer={null}
        destroyOnClose
        width={820}
      >
        <Space style={{ marginBottom: 12, width: '100%', justifyContent: 'flex-end' }}>
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openBind}>绑定供应商</Button>
        </Space>
        <Table
          rowKey="id"
          size="small"
          loading={relLoading}
          dataSource={relData}
          pagination={false}
          locale={{ emptyText: '该物料尚未绑定任何供应商，点击右上角绑定' }}
          columns={[
            { title: '供应商', dataIndex: 'supplier_id', render: (v) => supplierName(v) },
            { title: '供应单价', dataIndex: 'unit_price', width: 120 },
            { title: '交期(天)', dataIndex: 'lead_time_days', width: 100 },
            { title: '首选', dataIndex: 'is_preferred', width: 80,
              render: (v) => v ? <Tag color="blue">首选</Tag> : '-' },
            { title: '操作', key: 'act', width: 160,
              render: (_, r) => (
                <Space>
                  <Button type="link" size="small" onClick={() => openRelEdit(r)}>编辑</Button>
                  <Popconfirm title="确认解绑？" onConfirm={() => unbind(r.id)}>
                    <Button type="link" danger size="small">解绑</Button>
                  </Popconfirm>
                </Space>
              )
            },
          ]}
        />
      </Modal>

      <Modal
        title={relEdit ? '编辑供应关系' : '绑定供应商'}
        open={relFormOpen}
        onOk={submitRel}
        onCancel={() => setRelFormOpen(false)}
        destroyOnClose
        width={480}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="supplier_id" label="供应商" rules={[{ required: true, message: '请选择供应商' }]}>
            <Select options={supplierOpts} disabled={!!relEdit} showSearch optionFilterProp="label" placeholder="选择供应商" />
          </Form.Item>
          <Form.Item name="unit_price" label="供应单价" rules={[{ required: true, message: '请输入单价' }]}>
            <AntInputNumber style={{ width: '100%' }} min={0} step={0.0001} />
          </Form.Item>
          <Form.Item name="lead_time_days" label="交期(天)">
            <AntInputNumber style={{ width: '100%' }} min={0} />
          </Form.Item>
          <Form.Item name="is_preferred" label="设为首选供应商" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

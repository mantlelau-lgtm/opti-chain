import { useState } from 'react'
import CrudTable from '../components/CrudTable.jsx'
import { Button, Select, Tag, message, Modal, Table, Space, Form, Popconfirm, InputNumber, Switch } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { supplierApi, materialApi, supplierMaterialApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]
const AUDIT = [
  { label: '待审核', value: 'PENDING' },
  { label: '已核准', value: 'APPROVED' },
  { label: '已驳回', value: 'REJECTED' },
]
const auditColor = (s) => ({ PENDING: 'orange', APPROVED: 'green', REJECTED: 'red' }[s] || 'orange')
const auditLabel = (s) => AUDIT.find((o) => o.value === s)?.label || s || '待审核'

const setAudit = async (record, status, reload) => {
  try {
    await supplierApi.setAudit(record.id, status)
    message.success('审核状态已更新')
    reload()
   } catch (e) { message.error(e.message || '操作失败') }
}

export default function SupplierPage() {
  const [relOpen, setRelOpen] = useState(false)
  const [currentSup, setCurrentSup] = useState(null)
  const [relData, setRelData] = useState([])
  const [relLoading, setRelLoading] = useState(false)
  const [relMaterials, setRelMaterials] = useState([])
  const [relEdit, setRelEdit] = useState(null)
  const [relFormOpen, setRelFormOpen] = useState(false)
  const [form] = Form.useForm()

  const openRels = async (record) => {
    setCurrentSup(record)
    setRelOpen(true)
    setRelLoading(true)
    try {
      const [rels, mats] = await Promise.all([
        supplierMaterialApi.list({ supplier_id: record.id }),
        materialApi.list({ page: 1, size: 1000 }),
      ])
      setRelData(rels || [])
      setRelMaterials(mats.list || [])
    } catch (e) {
      message.error(e.message || '加载失败')
    } finally {
      setRelLoading(false)
    }
  }

  const refreshRels = async () => {
    if (!currentSup) return
    setRelLoading(true)
    try {
      const rels = await supplierMaterialApi.list({ supplier_id: currentSup.id })
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
      material_id: r.material_id,
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
        supplier_id: currentSup.id,
        material_id: values.material_id,
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

  const materialName = (id) => {
    const m = relMaterials.find((x) => x.id === id)
    return m ? `${m.sku_code} ${m.name}` : id
  }
  const materialOpts = relMaterials.map((m) => ({
    label: `${m.sku_code} ${m.name}`, value: m.id,
  }))

  const resource = {
    title: '供应商',
    api: supplierApi,
    columns: [
        { title: 'ID', dataIndex: 'id', width: 60 },
        { title: '供应商编号', dataIndex: 'supplier_code' },
        { title: '名称', dataIndex: 'name' },
        { title: '联系人', dataIndex: 'contact_person' },
        { title: '电话', dataIndex: 'phone' },
        { title: '地址', dataIndex: 'address' },
        { title: '准入状态', dataIndex: 'audit_status', width: 100,
          render: (v) => <Tag color={auditColor(v)}>{auditLabel(v)}</Tag> },
        { title: '状态', dataIndex: 'status',
          render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
       ],
    fields: [
        { name: 'supplier_code', label: '供应商编号', rules: [{ required: true, message: '请输入' }] },
        { name: 'name', label: '名称', rules: [{ required: true, message: '请输入' }] },
        { name: 'contact_person', label: '联系人' },
        { name: 'phone', label: '联系电话' },
        { name: 'address', label: '地址' },
        { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
         render: () => <Select options={STATUS} /> },
       ],
    extraActions: (record, reload) => (
      <>
        <Button type="link" size="small" onClick={() => openRels(record)}>供应物料</Button>
        {record.audit_status === 'APPROVED'
          ? <Button type="link" size="small" onClick={() => setAudit(record, 'REJECTED', reload)}>驳回</Button>
          : <Button type="link" size="small" onClick={() => setAudit(record, 'APPROVED', reload)}>核准</Button>}
      </>
     ),
  }

  return (
    <>
      <CrudTable resource={resource} />

      <Modal
        title={currentSup ? `供应物料 · ${currentSup.supplier_code} ${currentSup.name}` : '供应物料'}
        open={relOpen}
        onCancel={() => setRelOpen(false)}
        footer={null}
        destroyOnClose
        width={820}
      >
        <Space style={{ marginBottom: 12, width: '100%', justifyContent: 'flex-end' }}>
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openBind}
            disabled={currentSup?.audit_status !== 'APPROVED'}
            title={currentSup?.audit_status !== 'APPROVED' ? '供应商尚未核准，不能绑定物料' : ''}>
            绑定物料
          </Button>
        </Space>
        <Table
          rowKey="id"
          size="small"
          loading={relLoading}
          dataSource={relData}
          pagination={false}
          locale={{ emptyText: '该供应商尚未绑定任何物料，点击右上角绑定' }}
          columns={[
            { title: '物料', dataIndex: 'material_id', render: (v) => materialName(v) },
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
        title={relEdit ? '编辑供应关系' : '绑定物料'}
        open={relFormOpen}
        onOk={submitRel}
        onCancel={() => setRelFormOpen(false)}
        destroyOnClose
        width={480}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="material_id" label="物料" rules={[{ required: true, message: '请选择物料' }]}>
            <Select options={materialOpts} disabled={!!relEdit} showSearch optionFilterProp="label" placeholder="选择物料" />
          </Form.Item>
          <Form.Item name="unit_price" label="供应单价" rules={[{ required: true, message: '请输入单价' }]}>
            <InputNumber style={{ width: '100%' }} min={0} step={0.0001} />
          </Form.Item>
          <Form.Item name="lead_time_days" label="交期(天)">
            <InputNumber style={{ width: '100%' }} min={0} />
          </Form.Item>
          <Form.Item name="is_preferred" label="首选供应商（该物料采购默认这家）" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

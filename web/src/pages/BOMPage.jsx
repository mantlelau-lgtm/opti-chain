import { useState, useEffect, useCallback } from 'react'
import {
  Table, Button, Input, Space, Modal, Form, message, Tag, Select, InputNumber, Popconfirm,
} from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { productApi, bomApi, materialApi } from '../api/index.js'

const BOM_STATUS = {
  DRAFT: { color: 'default', label: '草稿' },
  RELEASED: { color: 'green', label: '已发布' },
  OBSOLETE: { color: 'red', label: '已作废' },
}

// BOMPage: R&D products + versioned BOMs. A product has a list of BOM
// versions; one RELEASED version is the effective default.
export default function BOMPage() {
  const [products, setProducts] = useState([])
  const [materials, setMaterials] = useState([])
  const [productId, setProductId] = useState(undefined)
  const [boms, setBoms] = useState([])
  const [loading, setLoading] = useState(false)

  const [prodOpen, setProdOpen] = useState(false)
  const [prodForm] = Form.useForm()
  const [bomOpen, setBomOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [bomForm] = Form.useForm()

  useEffect(() => {
    Promise.all([
      productApi.list({ page: 1, size: 500 }),
      materialApi.list({ page: 1, size: 1000 }),
    ]).then(([p, m]) => {
      setProducts(p.list || [])
      setMaterials(m.list || [])
    }).catch(() => {})
  }, [])

  const loadBoms = useCallback(() => {
    if (!productId) { setBoms([]); return }
    setLoading(true)
    bomApi.byProduct(productId)
      .then((list) => setBoms(list || []))
      .catch((e) => message.error(e.message))
      .finally(() => setLoading(false))
  }, [productId])

  useEffect(() => { loadBoms() }, [loadBoms])

  const reloadProducts = () => {
    productApi.list({ page: 1, size: 500 }).then((p) => setProducts(p.list || [])).catch(() => {})
  }

  const submitProduct = async () => {
    try {
      const values = await prodForm.validateFields()
      await productApi.create(values)
      message.success('产品已创建')
      setProdOpen(false)
      prodForm.resetFields()
      reloadProducts()
    } catch (e) { if (!e.errorFields) message.error(e.message || '操作失败') }
  }

  const openCreateBOM = () => {
    setEditing(null)
    bomForm.resetFields()
    bomForm.setFieldsValue({ unit_qty: '1', details: [{ qty_per_unit: '1', scrap_rate: '0' }] })
    setBomOpen(true)
  }
  const openEditBOM = (record) => {
    setEditing(record)
    bomForm.resetFields()
    bomForm.setFieldsValue({
      bom_no: record.bom_no, unit_qty: String(record.unit_qty), remark: record.remark,
      details: (record.details || []).map((d) => ({
        component_id: d.component_id, qty_per_unit: String(d.qty_per_unit),
        scrap_rate: String(d.scrap_rate), remark: d.remark,
      })),
    })
    setBomOpen(true)
  }
  const submitBOM = async () => {
    try {
      const values = await bomForm.validateFields()
      const payload = {
        bom_no: values.bom_no, product_id: productId,
        unit_qty: String(values.unit_qty), remark: values.remark,
        details: (values.details || []).map((d) => ({
          component_id: d.component_id, qty_per_unit: String(d.qty_per_unit),
          scrap_rate: String(d.scrap_rate ?? 0), remark: d.remark,
        })),
      }
      if (editing) await bomApi.update(editing.id, payload)
      else await bomApi.create(payload)
      message.success('已保存')
      setBomOpen(false)
      loadBoms()
    } catch (e) { if (!e.errorFields) message.error(e.message || '操作失败') }
  }
  const release = async (record) => {
    try {
      await bomApi.release(record.id)
      message.success('已发布，成为生效版本')
      loadBoms()
    } catch (e) { message.error(e.message || '发布失败') }
  }
  const remove = async (id) => {
    try {
      await bomApi.remove(id)
      message.success('已删除')
      loadBoms()
    } catch (e) { message.error(e.message || '删除失败') }
  }

  const productOpts = products.map((p) => ({ label: `${p.product_code} ${p.name}`, value: p.id }))
  const materialOpts = materials.map((m) => ({ label: `${m.sku_code} ${m.name}`, value: m.id }))
  const materialName = (id) => materials.find((m) => m.id === id)?.name || id

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Space>
          <span>产品</span>
          <Select
            style={{ width: 260 }}
            options={productOpts}
            value={productId}
            onChange={setProductId}
            placeholder="选择产品"
            showSearch
            optionFilterProp="label"
          />
        </Space>
        <Space>
          <Button onClick={() => { prodForm.resetFields(); setProdOpen(true) }}>新建产品</Button>
          <Button type="primary" icon={<PlusOutlined />} disabled={!productId} onClick={openCreateBOM}>
            新建 BOM 版本
          </Button>
        </Space>
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={boms}
        pagination={false}
        locale={{ emptyText: '请先选择产品；产品无 BOM 时可新建版本' }}
        expandable={{
          expandedRowRender: (record) => (
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={record.details || []}
              columns={[
                { title: '组件物料', dataIndex: 'component_id', render: (v) => materialName(v) },
                { title: '单耗', dataIndex: 'qty_per_unit' },
                { title: '损耗率%', dataIndex: 'scrap_rate' },
                { title: '备注', dataIndex: 'remark' },
              ]}
            />
          ),
        }}
        columns={[
          { title: 'BOM 单号', dataIndex: 'bom_no' },
          { title: '版本', dataIndex: 'version', width: 80, render: (v) => `v${v}` },
          { title: '状态', dataIndex: 'status', width: 100,
            render: (v) => <Tag color={BOM_STATUS[v]?.color}>{BOM_STATUS[v]?.label || v}</Tag> },
          { title: '生效版本', dataIndex: 'is_default', width: 90,
            render: (v) => (v ? <Tag color="blue">默认</Tag> : '-') },
          { title: '基准量', dataIndex: 'unit_qty', width: 90 },
          {
            title: '操作', key: 'action', width: 220,
            render: (_, record) => record.status === 'DRAFT' ? (
              <Space>
                <Button type="link" size="small" onClick={() => openEditBOM(record)}>编辑</Button>
                <Popconfirm title="发布后成为该产品生效版本，旧默认版将作废，确认？" onConfirm={() => release(record)}>
                  <Button type="link" size="small">发布</Button>
                </Popconfirm>
                <Popconfirm title="确认删除？" onConfirm={() => remove(record.id)}>
                  <Button type="link" danger size="small">删除</Button>
                </Popconfirm>
              </Space>
            ) : <span style={{ color: '#bbb' }}>不可编辑</span>,
          },
        ]}
      />

      <Modal title="新建产品" open={prodOpen} onOk={submitProduct} onCancel={() => setProdOpen(false)} destroyOnClose>
        <Form form={prodForm} layout="vertical">
          <Form.Item name="product_code" label="产品编码" rules={[{ required: true, message: '请输入' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="spec" label="规格型号"><Input /></Form.Item>
          <Form.Item name="unit" label="单位" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="如 台 / 件" />
          </Form.Item>
          <Form.Item name="cost_price" label="参考成本"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>

      <Modal
        title={editing ? `编辑 BOM v${editing.version}` : '新建 BOM 版本'}
        open={bomOpen}
        onOk={submitBOM}
        onCancel={() => setBomOpen(false)}
        destroyOnClose
        width={820}
      >
        <Form form={bomForm} layout="vertical">
          <Space size="large" align="start">
            <Form.Item name="bom_no" label="BOM 单号" rules={[{ required: true, message: '请输入' }]}>
              <Input style={{ width: 240 }} />
            </Form.Item>
            <Form.Item name="unit_qty" label="基准产出量" rules={[{ required: true, message: '请输入' }]}>
              <InputNumber min={0} step={0.0001} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name="remark" label="备注"><Input style={{ width: 200 }} /></Form.Item>
          </Space>
          <Form.Item label="组件明细" required>
            <Form.List name="details">
              {(fields, { add, remove: rm }) => (
                <div>
                  {fields.map((f) => (
                    <Space key={f.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                      <Form.Item name={[f.name, 'component_id']} rules={[{ required: true, message: '选组件' }]}>
                        <Select style={{ width: 240 }} options={materialOpts} placeholder="组件物料" showSearch optionFilterProp="label" />
                      </Form.Item>
                      <Form.Item name={[f.name, 'qty_per_unit']} rules={[{ required: true, message: '单耗' }]}>
                        <InputNumber min={0} step={0.0001} style={{ width: 110 }} placeholder="单耗" />
                      </Form.Item>
                      <Form.Item name={[f.name, 'scrap_rate']}>
                        <InputNumber min={0} step={0.01} style={{ width: 100 }} placeholder="损耗%" />
                      </Form.Item>
                      <Form.Item name={[f.name, 'remark']}>
                        <Input style={{ width: 140 }} placeholder="备注" />
                      </Form.Item>
                      <Button type="text" danger icon={<DeleteOutlined />} onClick={() => rm(f.name)} />
                    </Space>
                  ))}
                  <Button type="dashed" block icon={<PlusOutlined />} onClick={() => add({ qty_per_unit: '1', scrap_rate: '0' })}>
                    添加组件
                  </Button>
                </div>
              )}
            </Form.List>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

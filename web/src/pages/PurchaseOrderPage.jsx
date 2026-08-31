import { useState, useEffect, useCallback } from 'react'
import {
  Table, Button, Input, Space, Modal, Form, message, Tag,
  Select, InputNumber, DatePicker, Popconfirm, Radio,
} from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { poApi, supplierApi, materialApi, locationApi, warehouseApi, productApi, bomApi, supplierMaterialApi } from '../api/index.js'

// PO statuses mirror the backend constants (model/procurement.go).
const PO_STATUS = [
   { label: '草稿', value: 'DRAFT' },
   { label: '已审批', value: 'APPROVED' },
   { label: '进行中', value: 'IN_PROGRESS' },
   { label: '已完成', value: 'COMPLETED' },
   { label: '已取消', value: 'CANCELLED' },
]
const statusColor = (s) => ({
  DRAFT: 'default', APPROVED: 'blue', IN_PROGRESS: 'processing',
  COMPLETED: 'success', CANCELLED: 'error',
}[s] || 'default')
const statusLabel = (s) => PO_STATUS.find((o) => o.value === s)?.label || s

// PurchaseOrderPage is a bespoke CRUD: the create/edit modal carries a dynamic
// detail-line table (material + qty + unit price) that is not expressible by the
// generic CrudTable, so it lives here.
export default function PurchaseOrderPage() {
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [suppliers, setSuppliers] = useState([])
  const [materials, setMaterials] = useState([])
  const [locations, setLocations] = useState([])
  const [warehouses, setWarehouses] = useState([])
  const [form] = Form.useForm()

  // ---- BOM-based ordering ----
  const [orderMode, setOrderMode] = useState('manual') // manual | bom
  const [products, setProducts] = useState([])
  const [bomProductId, setBomProductId] = useState(undefined)
  const [bomQty, setBomQty] = useState(1)
  const [supPrices, setSupPrices] = useState({}) // material_id -> unit_price

  // ---- receiving (receive goods against a PO, with QC rejection) ----
  const [receiving, setReceiving] = useState(null)
  const [recvWh, setRecvWh] = useState(undefined)
  const [recvRemark, setRecvRemark] = useState('')
  const [recvRows, setRecvRows] = useState([])
  // ---- receipt history ----
  const [receiptsOf, setReceiptsOf] = useState(null)
  const [receipts, setReceipts] = useState([])

  useEffect(() => {
     let mounted = true
     Promise.all([
       supplierApi.list({ page: 1, size: 200 }),
       materialApi.list({ page: 1, size: 1000 }),
       locationApi.list({ page: 1, size: 1000 }),
       warehouseApi.list({ page: 1, size: 200 }),
       productApi.list({ page: 1, size: 500 }),
       ]).then(([s, m, l, w, p]) => {
       if (!mounted) return
       setSuppliers(s.list || [])
       setMaterials(m.list || [])
       setLocations(l.list || [])
       setWarehouses(w.list || [])
       setProducts(p.list || [])
       }).catch(() => {})
     return () => { mounted = false }
     }, [])

  const load = useCallback(() => {
     setLoading(true)
     poApi.list({ page, size, keyword })
         .then((res) => { setData(res.list); setTotal(res.total) })
         .catch((e) => message.error(e.message))
         .finally(() => setLoading(false))
     }, [page, size, keyword])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
     setEditing(null)
     setOrderMode('manual')
     setBomProductId(undefined)
     setBomQty(1)
     setSupPrices({})
     form.resetFields()
     form.setFieldsValue({
        order_date: dayjs(),
        details: [{ material_id: undefined, order_qty: 1, unit_price: 0, location_id: undefined }],
      })
     setOpen(true)
   }

  // 选择供应商后，拉取该供应商的物料供应价，用于 BOM 展开时自动带出单价。
  const onSupplierChange = (sid) => {
     if (!sid) { setSupPrices({}); return }
     supplierMaterialApi.list({ supplier_id: sid }).then((list) => {
        const m = {}
        ;(list || []).forEach((r) => { m[r.material_id] = r.unit_price })
        setSupPrices(m)
      }).catch(() => setSupPrices({}))
   }

  // 按产品默认 BOM 展开为采购明细（数量 = 单耗 × 产品数量，单价取供应关系）。
  const expandBom = async () => {
     if (!bomProductId) { message.warning('请选择产品'); return }
     if (!bomQty || Number(bomQty) <= 0) { message.warning('请输入产品数量'); return }
     try {
        const versions = await bomApi.byProduct(bomProductId)
        const def = (versions || []).find((b) => b.is_default)
        if (!def) { message.warning('该产品无已发布的 BOM，请先在 BOM 管理页发布'); return }
        const details = (def.details || []).map((d) => ({
          material_id: d.component_id,
          order_qty: Number(d.qty_per_unit) * Number(bomQty),
          unit_price: Number(supPrices[d.component_id] ?? 0),
          location_id: undefined,
        }))
        form.setFieldsValue({ details })
        message.success(`已按 BOM 展开 ${details.length} 行明细，请补充确认单价`)
      } catch (e) { message.error(e.message || '展开失败') }
   }

  // Editing only touches the header; details are view-only after creation.
  const openEdit = (record) => {
     setEditing(record)
     form.setFieldsValue({
        po_number: record.po_number,
        supplier_id: record.supplier_id,
        order_date: record.order_date ? dayjs(record.order_date) : undefined,
        expected_delivery: record.expected_delivery_date ? dayjs(record.expected_delivery_date) : undefined,
        created_by: record.created_by,
      })
     setOpen(true)
   }

  const submit = async () => {
     try {
       const values = await form.validateFields()
       if (editing) {
          await poApi.update(editing.id, {
            po_number: values.po_number,
            supplier_id: values.supplier_id,
            order_date: values.order_date?.format ? values.order_date.format('YYYY-MM-DD') : values.order_date,
            expected_delivery: values.expected_delivery?.format ? values.expected_delivery.format('YYYY-MM-DD') : undefined,
            created_by: values.created_by,
          })
          message.success('已更新')
          setOpen(false)
          load()
          return
       }
       const payload = {
          po_number: values.po_number,
          supplier_id: values.supplier_id,
          order_date: values.order_date?.format ? values.order_date.format('YYYY-MM-DD') : undefined,
          expected_delivery: values.expected_delivery?.format ? values.expected_delivery.format('YYYY-MM-DD') : undefined,
          created_by: values.created_by,
          details: (values.details || []).map((d) => ({
            material_id: d.material_id,
            order_qty: String(d.order_qty),
            unit_price: String(d.unit_price),
            location_id: d.location_id || 0,
          })),
       }
       await poApi.create(payload)
       message.success('已创建')
       setOpen(false)
       load()
     } catch (e) {
        if (e.errorFields) return
        message.error(e.message || '操作失败')
      }
   }

  const changeStatus = async (record, status) => {
     try {
        await poApi.setStatus(record.id, status)
        message.success('状态已更新')
        load()
       } catch (e) { message.error(e.message || '操作失败') }
   }

  const remove = async (id) => {
     try {
        await poApi.remove(id)
        message.success('已删除')
        load()
       } catch (e) { message.error(e.message || '删除失败') }
   }

  // ---- receiving ----
  const openReceive = (record) => {
     setReceiving(record)
     setRecvWh(undefined)
     setRecvRemark('')
     setRecvRows((record.details || []).map((d) => {
        const remaining = Number(d.order_qty) - Number(d.received_qty)
        return {
          po_detail_id: d.id,
          material_id: d.material_id,
          order_qty: Number(d.order_qty),
          received_qty: Number(d.received_qty),
          remaining,
          passed_qty: remaining,
          rejected_qty: 0,
          reject_reason: '',
        }
      }))
   }

  const patchRecvRow = (i, patch) => {
     setRecvRows((rows) => rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
   }

  // Submit one receiving round: passed qty goes to stock, rejected qty stays
  // out of stock and requires a reason (supplier return evidence).
  const submitReceive = async () => {
     if (!recvWh) { message.warning('请选择收货仓库'); return }
     const rows = recvRows.filter((r) => r.passed_qty > 0 || r.rejected_qty > 0)
     if (rows.length === 0) { message.warning('请至少填写一行收货数量'); return }
     for (const r of rows) {
        if (r.passed_qty > r.remaining) { message.warning('合格数量不能超过待收数量'); return }
        if (r.rejected_qty > 0 && !r.reject_reason.trim()) { message.warning('拒收必须填写原因'); return }
      }
     try {
        await poApi.receive(receiving.id, {
          warehouse_id: recvWh,
          remark: recvRemark,
          details: rows.map((r) => ({
            po_detail_id: r.po_detail_id,
            passed_qty: String(r.passed_qty),
            rejected_qty: String(r.rejected_qty),
            reject_reason: r.reject_reason,
          })),
        })
        message.success('收货成功')
        setReceiving(null)
        load()
      } catch (e) { message.error(e.message || '收货失败') }
   }

  const openReceipts = async (record) => {
     setReceiptsOf(record)
     setReceipts([])
     try {
        const list = await poApi.receipts(record.id)
        setReceipts(list || [])
      } catch (e) { message.error(e.message || '查询收货记录失败') }
   }

  const supplierOpts = suppliers.map((s) => ({ label: s.name, value: s.id }))
  const materialOpts = materials.map((m) => ({ label: `${m.sku_code} ${m.name}`, value: m.id }))
  const productOpts = products.map((p) => ({ label: `${p.product_code} ${p.name}`, value: p.id }))
  const locationOpts = locations.map((l) => ({ label: l.location_code, value: l.id }))
  const warehouseOpts = warehouses.map((w) => ({ label: w.name, value: w.id }))
  const materialName = (id) => materials.find((m) => m.id === id)?.name || id

  const cols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
     { title: '订单号', dataIndex: 'po_number', width: 160 },
     {
       title: '供应商', dataIndex: 'supplier_id',
       render: (v) => suppliers.find((s) => s.id === v)?.name || v,
     },
     { title: '下单日期', dataIndex: 'order_date', render: (v) => v ? dayjs(v).format('YYYY-MM-DD') : '-' },
     {
       title: '预计到货', dataIndex: 'expected_delivery_date',
       render: (v) => v ? dayjs(v).format('YYYY-MM-DD') : '-',
     },
     { title: '金额', dataIndex: 'total_amount', width: 120 },
     {
       title: '状态', dataIndex: 'status', width: 110,
       render: (v) => <Tag color={statusColor(v)}>{statusLabel(v)}</Tag>,
     },
     {
       title: '操作', key: 'action', fixed: 'right', width: 330,
       render: (_, record) => (
          <Space>
            <Select
             size="small"
             style={{ width: 110 }}
             value={record.status}
             options={PO_STATUS}
             onChange={(s) => changeStatus(record, s)}
            />
            {(record.status === 'APPROVED' || record.status === 'IN_PROGRESS') && (
              <Button type="link" size="small" onClick={() => openReceive(record)}>收货</Button>
            )}
            <Button type="link" size="small" onClick={() => openReceipts(record)}>收货记录</Button>
            <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
            <Popconfirm title="确认删除？" onConfirm={() => remove(record.id)}>
              <Button type="link" danger size="small">删除</Button>
            </Popconfirm>
          </Space>
       ),
     },
   ]

  return (
     <div>
       <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
         <Input.Search
          placeholder="订单号/关键字搜索"
          allowClear
          style={{ width: 240 }}
          onSearch={(v) => { setKeyword(v); setPage(1) }}
         />
         <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建采购订单</Button>
       </Space>

       <Table
        rowKey="id"
        loading={loading}
        dataSource={data}
        columns={cols}
        scroll={{ x: 'max-content' }}
        expandable={{
          expandedRowRender: (record) => (
            <Table
             rowKey="id"
             size="small"
             pagination={false}
             dataSource={record.details || []}
             columns={[
                { title: '物料', dataIndex: 'material_id',
                  render: (v) => materials.find((m) => m.id === v)?.name || v },
                { title: '数量', dataIndex: 'order_qty' },
                { title: '单价', dataIndex: 'unit_price' },
                { title: '小计', dataIndex: 'total_price' },
                { title: '已收货', dataIndex: 'received_qty' },
                { title: '库位', dataIndex: 'location_id' },
             ]}
            />
          ),
       }}
       pagination={{
          current: page,
          pageSize: size,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, s) => { setPage(p); setSize(s) },
       }}
       />

       <Modal
        title={editing ? '编辑采购订单' : '新建采购订单'}
        open={open}
        onOk={submit}
        onCancel={() => setOpen(false)}
        destroyOnClose
        width={760}
       >
         <Form form={form} layout="vertical">
           <Form.Item name="po_number" label="订单号" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="如 PO-2026-0001" disabled={!!editing} />
           </Form.Item>
           <Form.Item name="supplier_id" label="供应商" rules={[{ required: true, message: '请选择' }]}>
            <Select options={supplierOpts} placeholder="选择供应商" showSearch optionFilterProp="label" onChange={onSupplierChange} />
           </Form.Item>
           <Space size="large" align="start">
             <Form.Item name="order_date" label="下单日期">
              <DatePicker />
             </Form.Item>
             <Form.Item name="expected_delivery" label="预计到货">
              <DatePicker />
             </Form.Item>
             <Form.Item name="created_by" label="创建人">
              <Input placeholder="可选" />
             </Form.Item>
           </Space>
           <Form.Item label="订单明细" required>
            {!editing && (
              <Space style={{ marginBottom: 12 }} wrap>
                <span>下单方式</span>
                <Radio.Group value={orderMode} onChange={(e) => setOrderMode(e.target.value)}>
                  <Radio.Button value="manual">手动明细</Radio.Button>
                  <Radio.Button value="bom">基于 BOM</Radio.Button>
                </Radio.Group>
                {orderMode === 'bom' && (
                  <>
                    <Select style={{ width: 220 }} value={bomProductId} onChange={setBomProductId}
                      options={productOpts} placeholder="选择产品（需已发布 BOM）" showSearch optionFilterProp="label" />
                    <InputNumber min={0} step={0.0001} value={bomQty} onChange={(x) => setBomQty(x ?? 1)} style={{ width: 120 }} placeholder="产品数量" />
                    <Button onClick={expandBom}>展开明细</Button>
                  </>
                )}
              </Space>
            )}
            <Form.List name="details">
             {(fields, { add, remove }) => (
               <div>
                 {fields.map((f) => (
                    <Space key={f.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                      <Form.Item
                       name={[f.name, 'material_id']}
                       rules={[{ required: true, message: '选物料' }]}
                      >
                        <Select
                         style={{ width: 220 }}
                         options={materialOpts}
                         placeholder="物料"
                         showSearch
                         optionFilterProp="label"
                        />
                      </Form.Item>
                      <Form.Item name={[f.name, 'order_qty']} rules={[{ required: true, message: '数量' }]}>
                       <InputNumber min={0} step={0.0001} style={{ width: 120 }} placeholder="数量" />
                      </Form.Item>
                      <Form.Item name={[f.name, 'unit_price']} rules={[{ required: true, message: '单价' }]}>
                       <InputNumber min={0} step={0.01} style={{ width: 120 }} placeholder="单价" />
                      </Form.Item>
                      <Form.Item name={[f.name, 'location_id']}>
                       <Select
                        style={{ width: 140 }}
                        allowClear
                        options={locationOpts}
                        placeholder="库位"
                        showSearch
                        optionFilterProp="label"
                       />
                      </Form.Item>
                      <Button
                       type="text"
                       danger
                       icon={<DeleteOutlined />}
                       onClick={() => remove(f.name)}
                      />
                    </Space>
                 ))}
                 <Button
                  type="dashed"
                  block
                  icon={<PlusOutlined />}
                  onClick={() => add({ order_qty: 1, unit_price: 0 })}
                 >
                  添加明细
                 </Button>
               </div>
             )}
            </Form.List>
           </Form.Item>
         </Form>
       </Modal>

       <Modal
        title={`收货 - ${receiving?.po_number || ''}`}
        open={!!receiving}
        onOk={submitReceive}
        onCancel={() => setReceiving(null)}
        destroyOnClose
        width={860}
        okText="确认收货"
       >
         <Space style={{ marginBottom: 12 }} wrap>
           <span>收货仓库</span>
           <Select
            style={{ width: 200 }}
            options={warehouseOpts}
            value={recvWh}
            onChange={setRecvWh}
            placeholder="选择仓库"
            showSearch
            optionFilterProp="label"
           />
           <Input
            style={{ width: 260 }}
            placeholder="备注（可选）"
            value={recvRemark}
            onChange={(e) => setRecvRemark(e.target.value)}
           />
         </Space>
         <Table
          rowKey="po_detail_id"
          size="small"
          pagination={false}
          dataSource={recvRows}
          columns={[
             { title: '物料', dataIndex: 'material_id', render: (v) => materialName(v) },
             { title: '订购', dataIndex: 'order_qty', width: 80 },
             { title: '已收', dataIndex: 'received_qty', width: 80 },
             { title: '待收', dataIndex: 'remaining', width: 80 },
             {
               title: '合格入库', dataIndex: 'passed_qty', width: 130,
               render: (v, _, i) => (
                 <InputNumber
                  min={0}
                  step={0.0001}
                  style={{ width: 110 }}
                  value={v}
                  onChange={(x) => patchRecvRow(i, { passed_qty: x ?? 0 })}
                 />
               ),
             },
             {
               title: '拒收', dataIndex: 'rejected_qty', width: 130,
               render: (v, _, i) => (
                 <InputNumber
                  min={0}
                  step={0.0001}
                  style={{ width: 110 }}
                  value={v}
                  onChange={(x) => patchRecvRow(i, { rejected_qty: x ?? 0 })}
                 />
               ),
             },
             {
               title: '拒收原因', dataIndex: 'reject_reason',
               render: (v, r, i) => (r.rejected_qty > 0
                  ? (
                    <Input
                     status={v.trim() ? undefined : 'error'}
                     placeholder="必填（退换货依据）"
                     value={v}
                     onChange={(e) => patchRecvRow(i, { reject_reason: e.target.value })}
                    />
                  )
                  : <span style={{ color: '#bbb' }}>-</span>),
             },
           ]}
         />
         <p style={{ marginTop: 8, color: '#999', fontSize: 12 }}>
           合格数量入库存并累计到已收货；拒收数量不入库存，保留为在途量等待供应商补货。
         </p>
       </Modal>

       <Modal
        title={`收货记录 - ${receiptsOf?.po_number || ''}`}
        open={!!receiptsOf}
        footer={null}
        onCancel={() => setReceiptsOf(null)}
        width={860}
       >
         <Table
          rowKey="id"
          size="small"
          pagination={false}
          dataSource={receipts}
          locale={{ emptyText: '暂无收货记录' }}
          columns={[
             { title: '收货单号', dataIndex: 'receipt_number' },
             {
               title: '收货日期', dataIndex: 'receipt_date', width: 150,
               render: (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-'),
             },
             { title: '备注', dataIndex: 'remark' },
           ]}
          expandable={{
             expandedRowRender: (rc) => (
               <Table
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={rc.details || []}
                columns={[
                   { title: '物料', dataIndex: 'material_id', render: (v) => materialName(v) },
                   { title: '到货', dataIndex: 'received_qty', width: 90 },
                   { title: '合格入库', dataIndex: 'passed_qty', width: 90 },
                   {
                     title: '拒收', dataIndex: 'rejected_qty', width: 90,
                     render: (v) => (Number(v) > 0 ? <Tag color="error">{v}</Tag> : v),
                   },
                   { title: '拒收原因', dataIndex: 'reject_reason' },
                ]}
               />
             ),
          }}
         />
       </Modal>
     </div>
   )
}

import { useState, useEffect, useCallback } from 'react'
import {
  Tabs, Table, Button, Input, Space, Modal, Form, message,
  Tag, Select, InputNumber, Popconfirm,
} from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { inventoryApi, warehouseApi, materialApi, locationApi } from '../api/index.js'

const ORDER_TYPES = [
    { label: '采购入库', value: 'PURCHASE_IN' },
    { label: '销售出库', value: 'SALE_OUT' },
    { label: '调拨', value: 'TRANSFER' },
]
const typeLabel = (t) => ORDER_TYPES.find((o) => o.value === t)?.label || t

// InventoryPage drives stock via movements. It has three tabs:
//  1) 出入库订单 (created orders + delete)
//  2) 操作日志 (audit trail, read-only)
//  3) 快捷操作 (move-in / move-out forms that create an order + adjust stock atomically)
export default function InventoryPage() {
  const [orders, setOrders] = useState([])
  const [ordersTotal, setOrdersTotal] = useState(0)
  const [logs, setLogs] = useState([])
  const [logsTotal, setLogsTotal] = useState(0)
  const [oPage, setOPage] = useState(1)
  const [oSize, setOSize] = useState(10)
  const [lPage, setLPage] = useState(1)
  const [lSize, setLSize] = useState(10)
  const [loading, setLoading] = useState(false)
  const [warehouses, setWarehouses] = useState([])
  const [materials, setMaterials] = useState([])
  const [locations, setLocations] = useState([])
  const [moveOpen, setMoveOpen] = useState(false)
  const [moveDir, setMoveDir] = useState('in')
  const [form] = Form.useForm()

  useEffect(() => {
     let mounted = true
     Promise.all([
       warehouseApi.list({ page: 1, size: 200 }),
       materialApi.list({ page: 1, size: 1000 }),
       locationApi.list({ page: 1, size: 1000 }),
        ]).then(([w, m, l]) => {
       if (!mounted) return
       setWarehouses(w.list || [])
       setMaterials(m.list || [])
       setLocations(l.list || [])
        }).catch(() => {})
     return () => { mounted = false }
      }, [])

  const loadOrders = useCallback(() => {
     setLoading(true)
     inventoryApi.orders({ page: oPage, size: oSize })
         .then((res) => { setOrders(res.list); setOrdersTotal(res.total) })
         .catch((e) => message.error(e.message))
         .finally(() => setLoading(false))
      }, [oPage, oSize])

  const loadLogs = useCallback(() => {
     inventoryApi.logs({ page: lPage, size: lSize })
         .then((res) => { setLogs(res.list); setLogsTotal(res.total) })
         .catch((e) => message.error(e.message))
      }, [lPage, lSize])

  useEffect(() => { loadOrders() }, [loadOrders])
  useEffect(() => { loadLogs() }, [loadLogs])

  const whOpts = warehouses.map((w) => ({ label: w.name, value: w.id }))
  const matOpts = materials.map((m) => ({ label: `${m.sku_code} ${m.name}`, value: m.id }))
  const locOpts = locations.map((l) => ({ label: l.location_code, value: l.id }))

  const openMove = (dir) => {
     setMoveDir(dir)
     form.resetFields()
     form.setFieldsValue({
        order_number: '',
        warehouse_id: undefined,
        details: [{ material_id: undefined, location_id: undefined, qty: 1 }],
       })
     setMoveOpen(true)
    }

  const submitMove = async () => {
     try {
        const values = await form.validateFields()
        const payload = {
          order_number: values.order_number,
          warehouse_id: values.warehouse_id,
          details: (values.details || []).map((d) => ({
            material_id: d.material_id,
            location_id: d.location_id || 0,
            qty: String(d.qty),
           })),
        }
        if (moveDir === 'in') {
           await inventoryApi.moveIn(payload)
          } else {
           await inventoryApi.moveOut(payload)
          }
        message.success(moveDir === 'in' ? '已入库' : '已出库')
        setMoveOpen(false)
        loadOrders()
        loadLogs()
        } catch (e) {
        if (e.errorFields) return
        message.error(e.message || '操作失败')
        }
    }

  const removeOrder = async (id) => {
     try {
        await inventoryApi.removeOrder(id)
        message.success('已删除')
        loadOrders()
        } catch (e) { message.error(e.message || '删除失败') }
    }

  const orderCols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
     { title: '单据号', dataIndex: 'order_number', width: 160 },
     { title: '类型', dataIndex: 'order_type', render: (v) => typeLabel(v) },
     { title: '关联单号', dataIndex: 'ref_order_number', render: (v) => v || '-' },
     { title: '仓库', dataIndex: 'warehouse_id',
       render: (v) => warehouses.find((w) => w.id === v)?.name || v },
      {
       title: '状态', dataIndex: 'status',
       render: (v) => <Tag color={v === 'COMPLETED' ? 'success' : 'processing'}>{v}</Tag>,
      },
      {
       title: '操作', key: 'action', fixed: 'right', width: 100,
       render: (_, record) => (
           <Popconfirm title="确认删除该单据？" onConfirm={() => removeOrder(record.id)}>
            <Button type="link" danger size="small">删除</Button>
           </Popconfirm>
        ),
      },
    ]

  const logCols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
      { title: '物料', dataIndex: 'material_id',
       render: (v) => materials.find((m) => m.id === v)?.name || v },
      { title: '仓库', dataIndex: 'warehouse_id',
       render: (v) => warehouses.find((w) => w.id === v)?.name || v },
      {
       title: '动作', dataIndex: 'action_type', width: 90,
       render: (v) => <Tag color={v === 'IN' ? 'green' : 'red'}>{v}</Tag>,
      },
      { title: '变动量', dataIndex: 'change_qty' },
      { title: '变动前', dataIndex: 'before_qty' },
      { title: '变动后', dataIndex: 'after_qty' },
      { title: '关联单号', dataIndex: 'ref_order_number', render: (v) => v || '-' },
      { title: '时间', dataIndex: 'created_at',
       render: (v) => v ? new Date(v).toLocaleString() : '-' },
    ]

  const items = [
     {
       key: 'orders',
       label: '出入库订单',
       children: (
         <Table
          rowKey="id"
          loading={loading}
          dataSource={orders}
          columns={orderCols}
          scroll={{ x: 'max-content' }}
          pagination={{
            current: oPage, pageSize: oSize, total: ordersTotal,
            showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
            onChange: (p, s) => { setOPage(p); setOSize(s) },
          }}
         />
       ),
     },
     {
       key: 'logs',
       label: '操作日志',
       children: (
         <Table
          rowKey="id"
          dataSource={logs}
          columns={logCols}
          scroll={{ x: 'max-content' }}
          pagination={{
            current: lPage, pageSize: lSize, total: logsTotal,
            showSizeChanger: true, showTotal: (t) => `共 ${t} 条`,
            onChange: (p, s) => { setLPage(p); setLSize(s) },
          }}
         />
       ),
     },
   ]

  return (
    <div>
       <Space style={{ marginBottom: 16, justifyContent: 'flex-end', width: '100%' }}>
         <Button type="primary" icon={<PlusOutlined />} onClick={() => openMove('in')}>入库</Button>
         <Button icon={<PlusOutlined />} onClick={() => openMove('out')}>出库</Button>
       </Space>

       <Tabs items={items} />

       <Modal
        title={moveDir === 'in' ? '新建入库单' : '新建出库单'}
        open={moveOpen}
        onOk={submitMove}
        onCancel={() => setMoveOpen(false)}
        destroyOnClose
        width={720}
        >
         <Form form={form} layout="vertical">
           <Form.Item name="order_number" label="单据号" rules={[{ required: true, message: '请输入' }]}>
             <Input placeholder="如 IN-2026-0001" />
           </Form.Item>
           <Form.Item name="warehouse_id" label="仓库" rules={[{ required: true, message: '请选择' }]}>
             <Select options={whOpts} placeholder="选择仓库" />
           </Form.Item>
           <Form.Item label="明细" required>
             <Form.List name="details">
              {(fields, { add, remove }) => (
                <div>
                  {fields.map((f) => (
                     <Space key={f.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                       <Form.Item
                       name={[f.name, 'material_id']}
                       rules={[{ required: true, message: '选物料' }]}>
                        <Select
                        style={{ width: 220 }}
                        options={matOpts}
                        placeholder="物料"
                        showSearch
                        optionFilterProp="label"
                        />
                       </Form.Item>
                       <Form.Item name={[f.name, 'location_id']}>
                        <Select
                        style={{ width: 140 }}
                        allowClear
                        options={locOpts}
                        placeholder="库位"
                        showSearch
                        optionFilterProp="label"
                        />
                       </Form.Item>
                       <Form.Item name={[f.name, 'qty']} rules={[{ required: true, message: '数量' }]}>
                        <InputNumber min={0} step={0.0001} style={{ width: 120 }} placeholder="数量" />
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
                  onClick={() => add({ qty: 1 })}
                  >
                  添加明细
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

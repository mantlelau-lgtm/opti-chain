import { useState, useEffect, useCallback } from 'react'
import {
  Table, Button, Input, Space, Modal, Form, message, Tag,
  Select, InputNumber, DatePicker, Popconfirm,
} from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { soApi, customerApi, materialApi } from '../api/index.js'

// SO statuses mirror the backend constants (model/sales.go).
const SO_STATUS = [
  { label: '草稿', value: 'DRAFT' },
  { label: '已审批', value: 'APPROVED' },
  { label: '发货中', value: 'IN_SHIPPING' },
  { label: '已完成', value: 'COMPLETED' },
  { label: '已取消', value: 'CANCELLED' },
]
const statusColor = (s) => ({
  DRAFT: 'default', APPROVED: 'blue', IN_SHIPPING: 'processing',
  COMPLETED: 'success', CANCELLED: 'error',
}[s] || 'default')
const statusLabel = (s) => SO_STATUS.find((o) => o.value === s)?.label || s

// SalesOrderPage: create SOs (DRAFT), approve to lock available stock +
// consume customer credit, cancel to release both. Approval is where the
// anti-oversell guard fires, so backend errors are surfaced verbatim.
export default function SalesOrderPage() {
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [open, setOpen] = useState(false)
  const [customers, setCustomers] = useState([])
  const [materials, setMaterials] = useState([])
  const [form] = Form.useForm()

  useEffect(() => {
     let mounted = true
     Promise.all([
       customerApi.list({ page: 1, size: 200 }),
       materialApi.list({ page: 1, size: 1000 }),
       ]).then(([c, m]) => {
       if (!mounted) return
       setCustomers(c.list || [])
       setMaterials(m.list || [])
       }).catch(() => {})
     return () => { mounted = false }
     }, [])

  const load = useCallback(() => {
     setLoading(true)
     soApi.list({ page, size, keyword })
         .then((res) => { setData(res.list); setTotal(res.total) })
         .catch((e) => message.error(e.message))
         .finally(() => setLoading(false))
     }, [page, size, keyword])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
     form.resetFields()
     form.setFieldsValue({
        order_date: dayjs(),
        details: [{ material_id: undefined, qty: 1, unit_price: 0 }],
      })
     setOpen(true)
   }

  const submit = async () => {
     try {
       const values = await form.validateFields()
       await soApi.create({
          so_number: values.so_number,
          customer_id: values.customer_id,
          order_date: values.order_date?.format ? values.order_date.format('YYYY-MM-DD') : undefined,
          created_by: values.created_by,
          details: (values.details || []).map((d) => ({
            material_id: d.material_id,
            qty: String(d.qty),
            unit_price: String(d.unit_price),
          })),
       })
       message.success('已创建（草稿）')
       setOpen(false)
       load()
     } catch (e) {
        if (e.errorFields) return
        message.error(e.message || '操作失败')
      }
   }

  // Approve locks available stock and consumes credit; backend errors
  // (insufficient stock / credit exceeded) arrive as message text.
  const approve = async (record) => {
     try {
        await soApi.approve(record.id)
        message.success('已审批，库存已锁定')
        load()
      } catch (e) { message.error(e.message || '审批失败') }
   }

  const cancel = async (record) => {
     try {
        await soApi.cancel(record.id)
        message.success('已取消，锁定已释放')
        load()
      } catch (e) { message.error(e.message || '取消失败') }
   }

  const remove = async (id) => {
     try {
        await soApi.remove(id)
        message.success('已删除')
        load()
      } catch (e) { message.error(e.message || '删除失败') }
   }

  const customerOpts = customers.map((c) => ({ label: c.name, value: c.id }))
  const materialOpts = materials.map((m) => ({ label: `${m.sku_code} ${m.name}`, value: m.id }))
  const customerName = (id) => customers.find((c) => c.id === id)?.name || id
  const materialName = (id) => materials.find((m) => m.id === id)?.name || id

  const cols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
     { title: '订单号', dataIndex: 'so_number', width: 160 },
     { title: '客户', dataIndex: 'customer_id', render: (v) => customerName(v) },
     { title: '下单日期', dataIndex: 'order_date', render: (v) => v ? dayjs(v).format('YYYY-MM-DD') : '-' },
     { title: '金额', dataIndex: 'total_amount', width: 120 },
     {
       title: '状态', dataIndex: 'status', width: 110,
       render: (v) => <Tag color={statusColor(v)}>{statusLabel(v)}</Tag>,
     },
     {
       title: '操作', key: 'action', fixed: 'right', width: 240,
       render: (_, record) => (
          <Space>
            {record.status === 'DRAFT' && (
              <Popconfirm title="审批将锁定可用库存并占用信用额度，确认？" onConfirm={() => approve(record)}>
                <Button type="link" size="small">审批</Button>
              </Popconfirm>
            )}
            {(record.status === 'DRAFT' || record.status === 'APPROVED') && (
              <Popconfirm title="确认取消？" onConfirm={() => cancel(record)}>
                <Button type="link" size="small">取消</Button>
              </Popconfirm>
            )}
            {record.status === 'DRAFT' && (
              <Popconfirm title="确认删除？" onConfirm={() => remove(record.id)}>
                <Button type="link" danger size="small">删除</Button>
              </Popconfirm>
            )}
          </Space>
       ),
     },
   ]

  return (
     <div>
       <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
         <Input.Search
          placeholder="订单号搜索"
          allowClear
          style={{ width: 240 }}
          onSearch={(v) => { setKeyword(v); setPage(1) }}
         />
         <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建销售订单</Button>
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
                { title: '物料', dataIndex: 'material_id', render: (v) => materialName(v) },
                { title: '数量', dataIndex: 'qty' },
                { title: '单价', dataIndex: 'unit_price' },
                { title: '小计', dataIndex: 'total_price' },
                { title: '已发货', dataIndex: 'shipped_qty' },
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
        title="新建销售订单"
        open={open}
        onOk={submit}
        onCancel={() => setOpen(false)}
        destroyOnClose
        width={760}
       >
         <Form form={form} layout="vertical">
           <Form.Item name="so_number" label="订单号" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="如 SO-2026-0001" />
           </Form.Item>
           <Form.Item name="customer_id" label="客户" rules={[{ required: true, message: '请选择' }]}>
            <Select options={customerOpts} placeholder="选择客户" showSearch optionFilterProp="label" />
           </Form.Item>
           <Space size="large" align="start">
             <Form.Item name="order_date" label="下单日期">
              <DatePicker />
             </Form.Item>
             <Form.Item name="created_by" label="创建人">
              <Input placeholder="可选" />
             </Form.Item>
           </Space>
           <Form.Item label="订单明细" required>
            <Form.List name="details">
             {(fields, { add, remove: rm }) => (
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
                      <Form.Item name={[f.name, 'qty']} rules={[{ required: true, message: '数量' }]}>
                       <InputNumber min={0} step={0.0001} style={{ width: 120 }} placeholder="数量" />
                      </Form.Item>
                      <Form.Item name={[f.name, 'unit_price']} rules={[{ required: true, message: '单价' }]}>
                       <InputNumber min={0} step={0.01} style={{ width: 120 }} placeholder="单价" />
                      </Form.Item>
                      <Button
                       type="text"
                       danger
                       icon={<DeleteOutlined />}
                       onClick={() => rm(f.name)}
                      />
                    </Space>
                 ))}
                 <Button
                  type="dashed"
                  block
                  icon={<PlusOutlined />}
                  onClick={() => add({ qty: 1, unit_price: 0 })}
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

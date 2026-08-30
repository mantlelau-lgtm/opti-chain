import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Space, message, Select } from 'antd'
import { ReloadOutlined, InboxOutlined } from '@ant-design/icons'
import { inventoryApi, warehouseApi, materialApi } from '../api/index.js'

// StockPage shows real-time on-hand inventory (inv_stock). Stock is derived
// from inventory movements, so this view is read-only (no direct create/edit).
export default function StockPage() {
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [warehouseId, setWarehouseId] = useState(undefined)
  const [warehouses, setWarehouses] = useState([])
  const [materials, setMaterials] = useState([])

  useEffect(() => {
     let mounted = true
     Promise.all([
       warehouseApi.list({ page: 1, size: 200 }),
       materialApi.list({ page: 1, size: 1000 }),
      ]).then(([w, m]) => {
       if (!mounted) return
       setWarehouses(w.list || [])
       setMaterials(m.list || [])
      }).catch(() => {})
     return () => { mounted = false }
    }, [])

  const whName = (id) => warehouses.find((w) => w.id === id)?.name || id
  const matName = (id) => materials.find((m) => m.id === id)?.name || id

  const load = useCallback(() => {
     setLoading(true)
     inventoryApi.stock({ page, size, warehouse_id: warehouseId })
        .then((res) => { setData(res.list); setTotal(res.total) })
        .catch((e) => message.error(e.message))
        .finally(() => setLoading(false))
    }, [page, size, warehouseId])

  useEffect(() => { load() }, [load])

  const whOptions = warehouses.map((w) => ({ label: w.name, value: w.id }))

  const cols = [
     { title: 'ID', dataIndex: 'id', width: 60 },
     { title: '仓库', dataIndex: 'warehouse_id', render: (v) => whName(v) },
     { title: '库位', dataIndex: 'location_id', render: (v) => v || '-' },
     { title: '物料', dataIndex: 'material_id', render: (v) => matName(v) },
     { title: '库存量', dataIndex: 'quantity' },
     { title: '锁定量', dataIndex: 'locked_quantity' },
   ]

  return (
     <div>
       <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
         <Space>
           <InboxOutlined style={{ fontSize: 18 }} />
           <span style={{ fontSize: 16, fontWeight: 600 }}>实时库存</span>
         </Space>
         <Space>
           <Select
            allowClear
            placeholder="按仓库筛选"
            style={{ width: 180 }}
            options={whOptions}
            value={warehouseId}
            onChange={(v) => { setWarehouseId(v); setPage(1) }}
           />
           <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
         </Space>
       </Space>
       <Table
        rowKey="id"
        loading={loading}
        dataSource={data}
        columns={cols}
        scroll={{ x: 'max-content' }}
        pagination={{
          current: page,
          pageSize: size,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, s) => { setPage(p); setSize(s) },
         }}
       />
     </div>
   )
}

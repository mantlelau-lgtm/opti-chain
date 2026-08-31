import { useState, useEffect } from 'react'
import CrudTable from '../components/CrudTable.jsx'
import { Select, Tag } from 'antd'
import { locationApi, warehouseApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]

// LocationPage needs a warehouse dropdown, so it wires the warehouse list in.
export default function LocationPage() {
   const [warehouses, setWarehouses] = useState([])
   useEffect(() => {
    let mounted = true
    warehouseApi.list({ page: 1, size: 200 }).then((r) => { if (mounted) setWarehouses(r.list) }).catch(() => {})
    return () => { mounted = false }
     }, [])
   const whOptions = warehouses.map((w) => ({ label: w.name, value: w.id }))

   const resource = {
     title: '库位',
     api: locationApi,
     columns: [
        { title: 'ID', dataIndex: 'id', width: 60 },
        { title: '仓库ID', dataIndex: 'warehouse_id' },
        { title: '库位编码', dataIndex: 'location_code' },
        { title: '名称', dataIndex: 'name' },
        { title: '状态', dataIndex: 'status',
          render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
      ],
     fields: [
        { name: 'warehouse_id', label: '所属仓库', rules: [{ required: true, message: '请选择' }],
          valuePropName: 'value',
          render: () => <Select options={whOptions} placeholder="选择仓库" /> },
        { name: 'location_code', label: '库位编码', rules: [{ required: true, message: '请输入' }],
          placeholder: '如 A-01-101' },
        { name: 'name', label: '名称' },
        { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
          render: () => <Select options={STATUS} /> },
      ],
    }
   return <CrudTable resource={resource} />
}

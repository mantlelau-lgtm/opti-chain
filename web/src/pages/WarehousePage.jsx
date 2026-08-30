import CrudTable from '../components/CrudTable.jsx'
import { Select, Tag } from 'antd'
import { warehouseApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]

const resource = {
  title: '仓库',
  api: warehouseApi,
  columns: [
      { title: 'ID', dataIndex: 'id', width: 60 },
      { title: '仓库编码', dataIndex: 'warehouse_code' },
      { title: '名称', dataIndex: 'name' },
      { title: '地址', dataIndex: 'address' },
      { title: '状态', dataIndex: 'status',
        render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
     ],
  fields: [
      { name: 'warehouse_code', label: '仓库编码', rules: [{ required: true, message: '请输入' }] },
      { name: 'name', label: '名称', rules: [{ required: true, message: '请输入' }] },
      { name: 'address', label: '地址' },
      { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
       render: () => <Select options={STATUS} /> },
     ],
}

export default function WarehousePage() {
   return <CrudTable resource={resource} />
}

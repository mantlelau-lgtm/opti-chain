import CrudTable from '../components/CrudTable.jsx'
import { Input, InputNumber, Select, Tag } from 'antd'
import { materialApi } from '../api/index.js'

// Material page: base-data CRUD with safety stock bounds and status.
const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]

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
  // Backend stores DECIMAL as string; keep them as strings for lossless round-trip.
  makeFormValues: (r) => r ? {
     sku_code: r.sku_code, name: r.name, category: r.category, unit: r.unit,
     min_stock: Number(r.min_stock), max_stock: Number(r.max_stock), status: r.status,
    } : { status: 1, min_stock: 0, max_stock: 0 },
}

export default function MaterialPage() {
   return <CrudTable resource={resource} />
}

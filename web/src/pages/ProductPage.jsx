import CrudTable from '../components/CrudTable.jsx'
import { Select, Tag, InputNumber } from 'antd'
import { productApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]

const resource = {
  title: '产品列表',
  api: productApi,
  columns: [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '产品编码', dataIndex: 'product_code' },
    { title: '名称', dataIndex: 'name' },
    { title: '规格型号', dataIndex: 'spec' },
    { title: '单位', dataIndex: 'unit' },
    { title: '参考成本', dataIndex: 'cost_price' },
    { title: '状态', dataIndex: 'status',
      render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
  ],
  fields: [
    { name: 'product_code', label: '产品编码', rules: [{ required: true, message: '请输入' }] },
    { name: 'name', label: '名称', rules: [{ required: true, message: '请输入' }] },
    { name: 'spec', label: '规格型号' },
    { name: 'unit', label: '单位', rules: [{ required: true, message: '请输入' }] },
    { name: 'cost_price', label: '参考成本',
      render: () => <InputNumber min={0} step={0.01} style={{ width: '100%' }} /> },
    { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
      render: () => <Select options={STATUS} /> },
  ],
}

export default function ProductPage() {
  return <CrudTable resource={resource} />
}

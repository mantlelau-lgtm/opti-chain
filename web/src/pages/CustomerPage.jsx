import CrudTable from '../components/CrudTable.jsx'
import { InputNumber, Select, Tag } from 'antd'
import { customerApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]
const AUDIT = [
  { label: '待审核', value: 'PENDING' },
  { label: '已核准', value: 'APPROVED' },
  { label: '已驳回', value: 'REJECTED' },
]
const auditColor = (s) => ({ PENDING: 'orange', APPROVED: 'green', REJECTED: 'red' }[s] || 'default')
const auditLabel = (s) => AUDIT.find((o) => o.value === s)?.label || s || '待审核'

const resource = {
  title: '客户',
  api: customerApi,
  columns: [
      { title: 'ID', dataIndex: 'id', width: 60 },
      { title: '客户编号', dataIndex: 'customer_code' },
      { title: '名称', dataIndex: 'name' },
      { title: '联系人', dataIndex: 'contact_person' },
      { title: '电话', dataIndex: 'phone' },
      { title: '信用额度', dataIndex: 'credit_limit', width: 100 },
      { title: '已用额度', dataIndex: 'used_credit', width: 100 },
      { title: '准入状态', dataIndex: 'audit_status', width: 100,
        render: (v) => <Tag color={auditColor(v)}>{auditLabel(v)}</Tag> },
      { title: '状态', dataIndex: 'status', width: 80,
        render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
     ],
  fields: [
      { name: 'customer_code', label: '客户编号', rules: [{ required: true, message: '请输入' }] },
      { name: 'name', label: '名称', rules: [{ required: true, message: '请输入' }] },
      { name: 'contact_person', label: '联系人' },
      { name: 'phone', label: '联系电话' },
      { name: 'credit_limit', label: '信用额度（0 = 不启用信用控制）',
       render: () => <InputNumber min={0} step={100} style={{ width: '100%' }} /> },
      { name: 'audit_status', label: '准入状态', initialValue: 'APPROVED', valuePropName: 'value',
       render: () => <Select options={AUDIT} /> },
      { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
       render: () => <Select options={STATUS} /> },
     ],
}

export default function CustomerPage() {
   return <CrudTable resource={resource} />
}

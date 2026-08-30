import CrudTable from '../components/CrudTable.jsx'
import { Button, Select, Tag, message } from 'antd'
import { supplierApi } from '../api/index.js'

const STATUS = [{ label: '启用', value: 1 }, { label: '禁用', value: 0 }]
const AUDIT = [
  { label: '待审核', value: 'PENDING' },
  { label: '已核准', value: 'APPROVED' },
  { label: '已驳回', value: 'REJECTED' },
]
const auditColor = (s) => ({ PENDING: 'orange', APPROVED: 'green', REJECTED: 'red' }[s] || 'orange')
const auditLabel = (s) => AUDIT.find((o) => o.value === s)?.label || s || '待审核'

// 准入管控：只有 APPROVED 供应商能上采购订单（后端强校验）。
const setAudit = async (record, status, reload) => {
  try {
    await supplierApi.setAudit(record.id, status)
    message.success('审核状态已更新')
    reload()
   } catch (e) { message.error(e.message || '操作失败') }
}

const resource = {
  title: '供应商',
  api: supplierApi,
  columns: [
      { title: 'ID', dataIndex: 'id', width: 60 },
      { title: '供应商编号', dataIndex: 'supplier_code' },
      { title: '名称', dataIndex: 'name' },
      { title: '联系人', dataIndex: 'contact_person' },
      { title: '电话', dataIndex: 'phone' },
      { title: '地址', dataIndex: 'address' },
      { title: '准入状态', dataIndex: 'audit_status', width: 100,
        render: (v) => <Tag color={auditColor(v)}>{auditLabel(v)}</Tag> },
      { title: '状态', dataIndex: 'status',
        render: (v) => <Tag color={v === 1 ? 'green' : 'red'}>{v === 1 ? '启用' : '禁用'}</Tag> },
     ],
  fields: [
      { name: 'supplier_code', label: '供应商编号', rules: [{ required: true, message: '请输入' }] },
      { name: 'name', label: '名称', rules: [{ required: true, message: '请输入' }] },
      { name: 'contact_person', label: '联系人' },
      { name: 'phone', label: '联系电话' },
      { name: 'address', label: '地址' },
      { name: 'status', label: '状态', initialValue: 1, valuePropName: 'value',
       render: () => <Select options={STATUS} /> },
     ],
  extraActions: (record, reload) => (
    record.audit_status === 'APPROVED'
      ? <Button type="link" size="small" onClick={() => setAudit(record, 'REJECTED', reload)}>驳回</Button>
      : <Button type="link" size="small" onClick={() => setAudit(record, 'APPROVED', reload)}>核准</Button>
   ),
}

export default function SupplierPage() {
   return <CrudTable resource={resource} />
}

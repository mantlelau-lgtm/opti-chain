import { useState, useEffect, useCallback } from 'react'
import { Tabs, Table, Tag, Button, Space, Modal, Input, message, Descriptions } from 'antd'
import dayjs from 'dayjs'
import { approvalApi, poApi, soApi } from '../api/index.js'

const TYPE_LABEL = { PO: '采购单', SO: '销售单' }
const STATUS = {
  PENDING: { color: 'processing', label: '审批中' },
  APPROVED: { color: 'success', label: '已通过' },
  REJECTED: { color: 'error', label: '已驳回' },
}
const MEMBER_STATUS = {
  PENDING: { color: 'default', label: '待审批' },
  APPROVED: { color: 'success', label: '已通过' },
  REJECTED: { color: 'error', label: '已驳回' },
}

// ApprovalWorkbenchPage: lists the orders the current user must approve, with
// detail + approve/reject in a modal. Also shows processed & submitted items.
export default function ApprovalWorkbenchPage() {
  const [pending, setPending] = useState([])
  const [processed, setProcessed] = useState([])
  const [submitted, setSubmitted] = useState([])
  const [active, setActive] = useState('pending')

  const [view, setView] = useState(null) // task being viewed
  const [order, setOrder] = useState(null)
  const [comment, setComment] = useState('')
  const [acting, setActing] = useState(false)

  const load = useCallback(() => {
    approvalApi.pending().then((l) => setPending(l || [])).catch(() => {})
    approvalApi.processed().then((l) => setProcessed(l || [])).catch(() => {})
    approvalApi.submitted().then((l) => setSubmitted(l || [])).catch(() => {})
  }, [])

  useEffect(() => { load() }, [load])

  const openView = async (task) => {
    setView(task)
    setOrder(null)
    setComment('')
    try {
      const o = task.order_type === 'PO'
        ? await poApi.get(task.order_id)
        : await soApi.get(task.order_id)
      setOrder(o)
    } catch { setOrder(null) }
  }

  const act = async (action) => {
    setActing(true)
    try {
      await approvalApi.act(view.id, { action, comment })
      message.success(action === 'APPROVED' ? '已通过' : '已驳回')
      setView(null)
      load()
    } catch (e) { message.error(e.message || '操作失败') } finally { setActing(false) }
  }

  const cols = (showAction) => [
    { title: '单号', dataIndex: 'order_number', width: 180 },
    { title: '类型', dataIndex: 'order_type', width: 90, render: (v) => <Tag>{TYPE_LABEL[v] || v}</Tag> },
    { title: '提交人', dataIndex: 'submitter_name', width: 120 },
    { title: '提交时间', dataIndex: 'created_at', width: 160, render: (v) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-' },
    { title: '状态', dataIndex: 'status', width: 100, render: (v) => <Tag color={STATUS[v]?.color}>{STATUS[v]?.label || v}</Tag> },
    ...(showAction ? [{
      title: '操作', key: 'a', width: 90, fixed: 'right',
      render: (_, r) => <Button type="link" size="small" onClick={() => openView(r)}>审批</Button>,
    }] : []),
  ]

  return (
    <div>
      <Tabs
        activeKey={active}
        onChange={setActive}
        items={[
          { key: 'pending', label: `待我审批（${pending.length}）`, children: <Table rowKey="id" dataSource={pending} columns={cols(true)} pagination={false} /> },
          { key: 'processed', label: '我已处理', children: <Table rowKey="id" dataSource={processed} columns={cols(false)} pagination={false} /> },
          { key: 'submitted', label: '我提交的', children: <Table rowKey="id" dataSource={submitted} columns={cols(false)} pagination={false} /> },
        ]}
      />

      <Modal
        title={`审批：${view?.order_number || ''}`}
        open={!!view}
        onCancel={() => setView(null)}
        width={720}
        footer={view && view.status === 'PENDING' && view.members?.some((m) => m.status === 'PENDING' && m.user_id === authUserId()) ? (
          <Space>
            <Input placeholder="审批意见（驳回时建议填写）" value={comment} onChange={(e) => setComment(e.target.value)} style={{ width: 280 }} />
            <Button danger loading={acting} onClick={() => act('REJECTED')}>驳回</Button>
            <Button type="primary" loading={acting} onClick={() => act('APPROVED')}>通过</Button>
          </Space>
        ) : null}
      >
        <Descriptions size="small" column={2} bordered style={{ marginBottom: 12 }}>
          <Descriptions.Item label="类型">{TYPE_LABEL[view?.order_type] || '-'}</Descriptions.Item>
          <Descriptions.Item label="提交人">{view?.submitter_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态"><Tag color={STATUS[view?.status]?.color}>{STATUS[view?.status]?.label}</Tag></Descriptions.Item>
          <Descriptions.Item label="金额">{order?.total_amount ?? '-'}</Descriptions.Item>
        </Descriptions>

        <h4>审批进度</h4>
        <Table
          rowKey="id" size="small" pagination={false}
          dataSource={view?.members || []}
          columns={[
            { title: '审批人', dataIndex: 'user_name' },
            { title: '状态', dataIndex: 'status', width: 100, render: (v) => <Tag color={MEMBER_STATUS[v]?.color}>{MEMBER_STATUS[v]?.label || v}</Tag> },
            { title: '意见', dataIndex: 'comment', render: (v) => v || '-' },
            { title: '时间', dataIndex: 'approved_at', width: 160, render: (v) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-' },
          ]}
        />

        {order && (
          <>
            <h4 style={{ marginTop: 12 }}>单据明细</h4>
            <Table
              rowKey="id" size="small" pagination={false}
              dataSource={order.details || []}
              columns={[
                { title: '物料', dataIndex: 'material_id' },
                { title: '数量', dataIndex: view?.order_type === 'PO' ? 'order_qty' : 'qty' },
                { title: '单价', dataIndex: 'unit_price' },
              ]}
            />
          </>
        )}
      </Modal>
    </div>
  )
}

// 当前登录用户 id（用于判断是否是待审批成员）。
function authUserId() {
  try { return JSON.parse(localStorage.getItem('scm_user'))?.id } catch { return null }
}

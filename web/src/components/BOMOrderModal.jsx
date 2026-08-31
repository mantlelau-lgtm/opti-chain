import { useState, useEffect } from 'react'
import { Modal, Select, InputNumber, Button, Space, message, Table, Tag, Alert, Divider } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { productApi, bomOrderApi } from '../api/index.js'

// BOMOrderModal: the "order by BOM" flow, rendered as a modal from the
// purchase-order page. Pick several products + quantities, preview the
// supplier split (aggregated), then confirm (with a confirmation dialog) to
// create one PO per supplier.
export default function BOMOrderModal({ open, onClose, onCreated }) {
  const [products, setProducts] = useState([])
  const [lines, setLines] = useState([{ key: 0, product_id: undefined, qty: 1 }])
  const [plan, setPlan] = useState(null)
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    if (open) {
      setPlan(null)
      setLines([{ key: 0, product_id: undefined, qty: 1 }])
      productApi.list({ page: 1, size: 500 }).then((p) => setProducts(p.list || [])).catch(() => {})
    }
  }, [open])

  const patchLine = (key, patch) => {
    setLines((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))
  }
  const addLine = () => setLines((ls) => [...ls, { key: Date.now(), product_id: undefined, qty: 1 }])
  const removeLine = (key) => setLines((ls) => (ls.length > 1 ? ls.filter((l) => l.key !== key) : ls))

  const buildItems = () => lines
    .filter((l) => l.product_id && Number(l.qty) > 0)
    .map((l) => ({ product_id: l.product_id, qty: String(l.qty) }))

  const preview = async () => {
    const items = buildItems()
    if (items.length === 0) { message.warning('请至少选择一个产品并填写数量'); return }
    setLoading(true)
    setPlan(null)
    try {
      setPlan(await bomOrderApi.preview({ items }))
    } catch (e) { message.error(e.message || '预览失败') } finally { setLoading(false) }
  }

  const doConfirm = () => {
    Modal.confirm({
      title: '确认下单？',
      content: (
        <div>
          <p>将按 <b>{plan.groups.length}</b> 家供应商拆成 <b>{plan.groups.length}</b> 张草稿采购单：</p>
          <p>{plan.groups.map((g) => `${g.supplier_name}（${g.items.length} 项）`).join('、')}</p>
        </div>
      ),
      okText: '确认下单',
      cancelText: '取消',
      onOk: async () => {
        setCreating(true)
        try {
          const orders = await bomOrderApi.confirm({ items: buildItems() })
          message.success(`已按 ${orders.length} 家供应商拆单生成采购单`)
          onCreated?.()
        } catch (e) { message.error(e.message || '下单失败'); throw e } finally { setCreating(false) }
      },
    })
  }

  const productOpts = products.map((p) => ({ label: `${p.product_code} ${p.name}`, value: p.id }))

  return (
    <Modal title="基于 BOM 下单（多产品）" open={open} onCancel={onClose} footer={null} width={920}>
      <Divider style={{ marginTop: 0 }} orientation="left">选择产品</Divider>
      <Space direction="vertical" style={{ width: '100%' }}>
        {lines.map((l) => (
          <Space key={l.key} align="baseline">
            <Select style={{ width: 260 }} options={productOpts} value={l.product_id}
              onChange={(v) => { patchLine(l.key, { product_id: v }); setPlan(null) }}
              placeholder="选择产品（需已发布 BOM）" showSearch optionFilterProp="label" />
            <span>×</span>
            <InputNumber min={0} step={0.0001} value={l.qty}
              onChange={(x) => patchLine(l.key, { qty: x ?? 0 })} style={{ width: 140 }} placeholder="数量" />
            <Button type="text" danger icon={<DeleteOutlined />} onClick={() => removeLine(l.key)} />
          </Space>
        ))}
        <Space>
          <Button type="dashed" icon={<PlusOutlined />} onClick={addLine}>添加产品</Button>
          <Button type="primary" loading={loading} onClick={preview}>预览拆单</Button>
        </Space>
      </Space>

      {plan && plan.warnings?.length > 0 && (
        <Alert type="warning" showIcon style={{ marginTop: 16 }}
          message="以下物料无可用供应商，无法下单" description={plan.warnings.join('，')} />
      )}

      {plan && plan.groups?.map((g, gi) => (
        <div key={g.supplier_id} style={{ marginTop: 16 }}>
          <Divider orientation="left">供应商：{g.supplier_name}</Divider>
          <Table
            rowKey="material_id"
            size="small"
            pagination={false}
            dataSource={g.items}
            columns={[
              { title: '物料', dataIndex: 'material_name' },
              { title: '数量', dataIndex: 'qty', width: 140 },
              { title: '单价', dataIndex: 'unit_price', width: 140 },
              { title: '小计', width: 140,
                render: (_, r) => (Number(r.qty) * Number(r.unit_price)).toFixed(2) },
            ]}
            summary={() => (
              <Table.Summary.Row>
                <Table.Summary.Cell index={0} colSpan={4}>
                  <Tag color="blue">拆单 #{gi + 1} · 共 {g.items.length} 项</Tag>
                </Table.Summary.Cell>
              </Table.Summary.Row>
            )}
          />
        </div>
      ))}

      {plan && plan.groups?.length > 0 && plan.warnings?.length === 0 && (
        <Space style={{ marginTop: 16, justifyContent: 'flex-end', width: '100%' }}>
          <Button type="primary" loading={creating} onClick={doConfirm}>
            确认下单（拆成 {plan.groups.length} 张采购单）
          </Button>
        </Space>
      )}
    </Modal>
  )
}

import { useState, useEffect } from 'react'
import { Card, Select, InputNumber, Button, Space, message, Table, Tag, Alert, Divider } from 'antd'
import { productApi, bomOrderApi } from '../api/index.js'

// BOMOrderPage: order by BOM. Pick a product + qty, preview the auto-selected
// supplier split, then confirm to create one PO per supplier.
export default function BOMOrderPage() {
  const [products, setProducts] = useState([])
  const [productId, setProductId] = useState(undefined)
  const [qty, setQty] = useState(1)
  const [plan, setPlan] = useState(null)
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState(null)

  useEffect(() => {
    productApi.list({ page: 1, size: 500 }).then((p) => setProducts(p.list || [])).catch(() => {})
  }, [])

  const preview = async () => {
    if (!productId) { message.warning('请选择产品'); return }
    if (!qty || Number(qty) <= 0) { message.warning('请输入数量'); return }
    setLoading(true)
    setPlan(null)
    setCreated(null)
    try {
      const p = await bomOrderApi.preview({ product_id: productId, qty: String(qty) })
      setPlan(p)
    } catch (e) { message.error(e.message || '预览失败') } finally { setLoading(false) }
  }

  const confirm = async () => {
    setCreating(true)
    try {
      const orders = await bomOrderApi.confirm({ product_id: productId, qty: String(qty) })
      message.success(`已按 ${orders.length} 家供应商拆单生成采购单`)
      setCreated(orders)
    } catch (e) { message.error(e.message || '下单失败') } finally { setCreating(false) }
  }

  const productOpts = products.map((p) => ({ label: `${p.product_code} ${p.name}`, value: p.id }))

  return (
    <div>
      <Card title="基于 BOM 下采购单" style={{ marginBottom: 16 }}>
        <Space wrap>
          <span>产品</span>
          <Select style={{ width: 260 }} options={productOpts} value={productId}
            onChange={(v) => { setProductId(v); setPlan(null); setCreated(null) }}
            placeholder="选择产品（需已发布 BOM）" showSearch optionFilterProp="label" />
          <span>数量</span>
          <InputNumber min={0} step={0.0001} value={qty} onChange={(x) => setQty(x ?? 1)} style={{ width: 140 }} />
          <Button type="primary" loading={loading} onClick={preview}>预览拆单</Button>
        </Space>
        <p style={{ marginTop: 8, color: '#999', fontSize: 12 }}>
          系统按产品默认 BOM 展开物料需求，每个物料自动选择供应商（首选 &gt; 最低价），确认时按供应商拆成多张采购单（草稿）。
        </p>
      </Card>

      {plan && plan.warnings?.length > 0 && (
        <Alert type="warning" showIcon style={{ marginBottom: 16 }}
          message="以下物料无可用供应商（未绑定供应关系或供应商未核准），无法下单" description={plan.warnings.join('，')} />
      )}

      {plan && plan.groups?.map((g, gi) => (
        <Card key={g.supplier_id} title={`供应商：${g.supplier_name}`} size="small" style={{ marginBottom: 12 }}>
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
        </Card>
      ))}

      {plan && plan.groups?.length > 0 && plan.warnings?.length === 0 && (
        <Space style={{ marginTop: 8 }}>
          <Button type="primary" loading={creating} onClick={confirm}>确认下单（拆成 {plan.groups.length} 张采购单）</Button>
        </Space>
      )}

      {created && (
        <Card title="已生成采购单" style={{ marginTop: 16 }}>
          <Table rowKey="id" size="small" pagination={false} dataSource={created}
            columns={[
              { title: '采购单号', dataIndex: 'po_number' },
              { title: '供应商', dataIndex: 'supplier_id' },
              { title: '金额', dataIndex: 'total_amount' },
              { title: '状态', dataIndex: 'status', render: (v) => <Tag>{v}</Tag> },
            ]} />
        </Card>
      )}
    </div>
  )
}

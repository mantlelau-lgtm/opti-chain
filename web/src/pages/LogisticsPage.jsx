import { useState } from 'react'
import { Table, Input, Button, Space, Upload, message, Tag, Card, Timeline } from 'antd'
import { SearchOutlined, UploadOutlined } from '@ant-design/icons'
import { logisticsApi } from '../api/index.js'

export default function LogisticsPage() {
  const [trackingNo, setTrackingNo] = useState('')
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)

  const query = async () => {
    if (!trackingNo.trim()) return
    setLoading(true)
    try {
      const res = await logisticsApi.query(trackingNo.trim())
      setResult(res)
    } catch (e) { message.error(e.message || '查询失败') }
    finally { setLoading(false) }
  }

  const upload = async (file) => {
    const form = new FormData()
    form.append('file', file)
    setLoading(true)
    try {
      const res = await logisticsApi.upload(form)
      if (res?.length) {
        setResult(res[0])
        message.success(`导入成功，共 ${res.length} 单`)
      }
    } catch (e) { message.error(e.message || '导入失败') }
    finally { setLoading(false) }
    return false
  }

  const routes = result?.routes || []

  return (
    <div>
      <Card title="物流跟踪" style={{ marginBottom: 16 }}>
        <Space>
          <Input placeholder="输入顺丰运单号" value={trackingNo} onChange={(e) => setTrackingNo(e.target.value)}
            onPressEnter={query} style={{ width: 260 }} />
          <Button type="primary" icon={<SearchOutlined />} onClick={query} loading={loading}>查询</Button>
          <Upload beforeUpload={upload} showUploadList={false} accept=".txt,.csv,.tsv">
            <Button icon={<UploadOutlined />}>批量导入</Button>
          </Upload>
        </Space>
      </Card>

      {result && (
        <Card title={`运单: ${result.mailNo || trackingNo}`}>
          <p>最新状态: <Tag color="blue">{routes.length > 0 ? routes[routes.length-1].secondaryStatusName || routes[routes.length-1].remark : '暂无'}</Tag></p>
          <Timeline items={routes.map((r) => ({
            children: (
              <div>
                <div style={{ fontWeight: 500 }}>{r.remark || r.secondaryStatusName}</div>
                <div style={{ color: '#999', fontSize: 12 }}>{(r.acceptTime||'').slice(0,19)} {r.acceptAddress||''}</div>
              </div>
            ),
          }))} />
        </Card>
      )}
    </div>
  )
}
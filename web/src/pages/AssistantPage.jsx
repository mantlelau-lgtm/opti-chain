import { useState, useRef, useEffect } from 'react'
import { Input, Button, Typography, Tag, Space, Spin, Popconfirm, message } from 'antd'
import { SendOutlined, RobotOutlined, UserOutlined, DeleteOutlined } from '@ant-design/icons'
import { assistantApi } from '../api/index.js'

const { Text, Paragraph } = Typography

// AssistantPage: in-app intelligent assistant. The backend routes the question
// to the role-appropriate agent and calls internal tools with the user's
// permissions; each tool call is authorized internally.
export default function AssistantPage() {
  const [messages, setMessages] = useState([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const bottomRef = useRef(null)

  useEffect(() => {
    assistantApi.getHistory().then((res) => {
      if (res.history?.length) {
        setMessages(res.history.map((h) => ({
          role: h.role,
          content: h.content,
          toolCalls: h.tool_calls ? h.tool_calls.split(',') : [],
        })))
      }
    }).catch(() => {})
  }, [])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, loading])

  const send = async () => {
    const text = input.trim()
    if (!text || loading) return
    setInput('')
    setMessages((prev) => [...prev, { role: 'user', content: text }])
    setLoading(true)
    try {
      const res = await assistantApi.chat(text)
      setMessages((prev) => [...prev, {
        role: 'assistant',
        content: res.reply,
        agentName: res.agent_name,
        toolCalls: res.tool_calls || [],
      }])
    } catch (e) {
      setMessages((prev) => [...prev, { role: 'assistant', content: `出错了：${e.message || '未知错误'}`, error: true }])
    } finally {
      setLoading(false)
    }
  }

  const clearMemory = async () => {
    try {
      await assistantApi.clearMemory()
      setMessages([])
      message.success('记忆已清除')
    } catch (e) { message.error('清除失败') }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 140px)', maxWidth: 880, margin: '0 auto' }}>
      <div style={{ flex: 1, overflowY: 'auto', padding: '0 8px 16px' }}>
        {messages.length === 0 && (
          <div style={{ textAlign: 'center', color: '#999', marginTop: 80 }}>
            <RobotOutlined style={{ fontSize: 40 }} />
            <p>我是智能助手，可以帮你采购下单、创建/更新物料、创建 BOM 等。</p>
            <p style={{ fontSize: 12 }}>例如：&ldquo;帮我查一下库存里的电子料&rdquo;、&ldquo;给 XX 供应商下一笔 100 个某物料的采购单&rdquo;</p>
          </div>
        )}
        {messages.map((m, i) => (
          <div key={i} style={{ display: 'flex', justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start', marginBottom: 16 }}>
            <div style={{ maxWidth: '78%' }}>
              <div style={{
                display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4,
                justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start',
              }}>
                {m.role === 'user'
                  ? <><Text type="secondary" style={{ fontSize: 12 }}>我</Text><UserOutlined style={{ fontSize: 12, color: '#999' }} /></>
                  : <><RobotOutlined style={{ fontSize: 12, color: '#1890ff' }} /><Text type="secondary" style={{ fontSize: 12 }}>智能助手</Text></>}
              </div>
              <div style={{
                background: m.role === 'user' ? '#1890ff' : (m.error ? '#fff1f0' : '#f5f5f5'),
                color: m.role === 'user' ? '#fff' : (m.error ? '#cf1322' : '#333'),
                padding: '10px 14px', borderRadius: 10, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
              }}>
                <Paragraph style={{ margin: 0 }}>{m.content}</Paragraph>
              </div>
              {m.role === 'assistant' && m.agentName && (
                <Space size={4} style={{ marginTop: 4 }}>
                  <Tag color="blue" style={{ fontSize: 11 }}>{m.agentName}</Tag>
                  {m.toolCalls.map((t) => <Tag key={t} style={{ fontSize: 11 }}>{t}</Tag>)}
                </Space>
              )}
            </div>
          </div>
        ))}
        {loading && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: '#999' }}>
            <Spin size="small" /> 思考中…
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      <div style={{ display: 'flex', gap: 8, padding: '12px 0', borderTop: '1px solid #eee' }}>
        <Popconfirm title="确认清除所有对话记忆？" onConfirm={clearMemory}>
          <Button icon={<DeleteOutlined />} size="small" disabled={loading}>清除记忆</Button>
        </Popconfirm>
        <Input.TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="描述你想做的事，例如：查询物料 / 创建采购单 / 新建 BOM…"
          autoSize={{ minRows: 1, maxRows: 4 }}
          onPressEnter={(e) => { if (!e.shiftKey) { e.preventDefault(); send() } }}
          disabled={loading}
        />
        <Button type="primary" icon={<SendOutlined />} onClick={send} loading={loading} disabled={!input.trim()}>
          发送
        </Button>
      </div>
    </div>
  )
}

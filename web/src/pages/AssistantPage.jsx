import { useState, useRef, useEffect } from 'react'
import { Input, Button, Typography, Tag, Space, Spin, Popconfirm, message, Upload, Image, Tooltip, Badge } from 'antd'
import {
  SendOutlined, RobotOutlined, UserOutlined, DeleteOutlined,
  PaperClipOutlined, FileTextOutlined, FileImageOutlined,
  FilePdfOutlined, FileExcelOutlined, FileWordOutlined,
} from '@ant-design/icons'
import { assistantApi } from '../api/index.js'

const { Text, Paragraph } = Typography

const MAX_SIZE = 5 * 1024 * 1024
const MAX_SIZE_TEXT = '5MB'
const ACCEPT = '.txt,.md,.markdown,.csv,.tsv,.json,.log,.png,.jpg,.jpeg,.webp,.gif,.bmp,.pdf,.docx,.xlsx,.xls'
const IMAGE_EXT = new Set(['.png', '.jpg', '.jpeg', '.webp', '.gif', '.bmp'])
const PDF_EXT = new Set(['.pdf'])
const EXCEL_EXT = new Set(['.xlsx', '.xls', '.csv', '.tsv'])
const WORD_EXT = new Set(['.docx'])

function isImage(name) {
  const ext = (name || '').split('.').pop()?.toLowerCase()
  return IMAGE_EXT.has('.' + ext)
}

function bytesToSize(bytes) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(2) + ' MB'
}

function fileIconColor(name) {
  const ext = '.' + (name || '').split('.').pop()?.toLowerCase()
  if (PDF_EXT.has(ext)) return { color: '#ff4d4f', Icon: FilePdfOutlined }
  if (EXCEL_EXT.has(ext)) return { color: '#52c41a', Icon: FileExcelOutlined }
  if (WORD_EXT.has(ext)) return { color: '#1890ff', Icon: FileWordOutlined }
  if (isImage(name)) return { color: '#722ed1', Icon: FileImageOutlined }
  return { color: '#8c8c8c', Icon: FileTextOutlined }
}

function FileThumb({ file, size = 'm' }) {
  const dims = size === 's' ? [44, 32] : [72, 56]
  const [w, h] = dims
  if (file.url && isImage(file.name)) {
    return (
      <Image
        width={w} height={h}
        style={{ objectFit: 'cover', borderRadius: 6, border: '1px solid #eee', display: 'block' }}
        src={file.url}
        alt={file.name}
        preview={false}
      />
    )
  }
  const { Icon, color } = fileIconColor(file.name)
  return (
    <div style={{
      width: w, height: h, borderRadius: 6,
      border: `1px dashed ${color}50`,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: `${color}10`,
    }}>
      <Icon style={{ fontSize: size === 's' ? 16 : 22, color }} />
    </div>
  )
}

function formatMs(ms) {
  if (!ms || ms < 0) return '-'
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(1) + 's'
}

function formatUsage(u) {
  if (!u) return null
  const parts = []
  if (u.total_tokens || u.total_tokens === 0) {
    parts.push(`T ${u.total_tokens || 0}`)
  }
  if (u.prompt_tokens) parts.push(`P ${u.prompt_tokens}`)
  if (u.completion_tokens) parts.push(`C ${u.completion_tokens}`)
  return parts.join(' · ')
}

export default function AssistantPage() {
  const [messages, setMessages] = useState([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [pendingFiles, setPendingFiles] = useState([])
  const fileInputRef = useRef(null)
  const bottomRef = useRef(null)

  useEffect(() => {
    assistantApi.getHistory().then((res) => {
      if (res.history?.length) {
        setMessages(res.history.map((h) => {
          let attachments = null
          if (h.attachments && Array.isArray(h.attachments)) {
            attachments = h.attachments.map((a) => ({
              uid: a.id || String(Date.now() + Math.random()),
              id: a.id,
              name: a.name,
              size: a.size,
              mime: a.mime,
              storedAt: a.stored_at,
              url: a.stored_at ? assistantApi.attachmentUrl(a.stored_at) : (a.url || null),
            }))
          }
          return {
            role: h.role,
            content: h.content,
            agentName: h.agent_name,
            toolCalls: (() => {
              if (!h.tool_calls) return []
              if (typeof h.tool_calls === 'string' && h.tool_calls.startsWith('[')) {
                try { return JSON.parse(h.tool_calls) } catch { return h.tool_calls.split(',') }
              }
              return typeof h.tool_calls === 'string' ? h.tool_calls.split(',') : (Array.isArray(h.tool_calls) ? h.tool_calls : [])
            })(),
            timestamp: h.timestamp ? new Date(h.timestamp) : null,
            attachments,
            usage: h.usage || null,
          }
        }))
      }
    }).catch(() => {})
  }, [])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, loading])

  const addFiles = (fileList) => {
    if (loading) return
    const added = []
    for (const f of fileList) {
      if (f.size > MAX_SIZE) {
        message.error(`文件 ${f.name} 超过 ${MAX_SIZE_TEXT}，无法上传`)
        continue
      }
      const ext = '.' + (f.name || '').split('.').pop()?.toLowerCase()
      if (!ACCEPT.split(',').includes(ext)) {
        message.error(`不支持的文件类型：${ext || '未知'}`)
        continue
      }
      const localURL = isImage(f.name) ? URL.createObjectURL(f) : null
      added.push({
        uid: f.uid || String(Date.now()) + Math.random(),
        name: f.name,
        size: f.size,
        url: localURL,
        originFileObj: f,
      })
    }
    if (added.length) setPendingFiles((prev) => [...prev, ...added])
  }

  const beforeUpload = (file) => {
    addFiles([file])
    return Upload.LIST_IGNORE
  }

  const removePending = (uid) => {
    setPendingFiles((prev) => {
      const found = prev.find((f) => f.uid === uid)
      if (found?.url) URL.revokeObjectURL(found.url)
      return prev.filter((f) => f.uid !== uid)
    })
  }

  const clearAllPending = () => {
    pendingFiles.forEach((f) => f.url && URL.revokeObjectURL(f.url))
    setPendingFiles([])
  }

  const send = async () => {
    const text = input.trim()
    if (!text && pendingFiles.length === 0) return
    if (loading) return

    const outgoingFiles = pendingFiles.slice()
    clearAllPending()
    setInput('')
    const userMsg = {
      role: 'user',
      content: text,
      timestamp: new Date(),
      attachments: outgoingFiles.map((f) => ({
        uid: f.uid, name: f.name, size: f.size, url: f.url,
      })),
    }
    setMessages((prev) => [...prev, userMsg])
    setLoading(true)
    try {
      const files = outgoingFiles.map((f) => f.originFileObj).filter(Boolean)
      const res = await assistantApi.chat({ message: text, files })
      // Rewrite the just-sent user message with server-stored attachment URLs so
      // they keep working after page reload (instead of the revoked blob URLs).
      if (res.user_attachments?.length) {
        setMessages((prev) => {
          const next = prev.slice()
          for (let i = next.length - 1; i >= 0; i--) {
            if (next[i] === userMsg) {
              const attachments = res.user_attachments.map((a) => ({
                uid: a.id || String(Date.now() + Math.random()),
                id: a.id,
                name: a.name,
                size: a.size,
                mime: a.mime,
                storedAt: a.stored_at,
                url: a.stored_at ? assistantApi.attachmentUrl(a.stored_at) : null,
              }))
              next[i] = { ...userMsg, attachments }
              break
            }
          }
          return next
        })
      }
      setMessages((prev) => [...prev, {
        role: 'assistant',
        content: res.reply,
        timestamp: new Date(),
        agentName: res.agent_name,
        toolCalls: res.tool_calls || [],
        usage: res.usage || null,
      }])
    } catch (e) {
      setMessages((prev) => [...prev, {
        role: 'assistant',
        content: `出错了：${e.message || '未知错误'}`,
        timestamp: new Date(),
        error: true,
      }])
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

  const onPickFiles = () => fileInputRef.current?.click()

  const onInputFilesChange = (e) => {
    addFiles(Array.from(e.target.files || []))
    e.target.value = ''
  }

  return (
    <div style={{
      display: 'flex', flexDirection: 'column',
      height: 'calc(100vh - 140px)',
      maxWidth: 920, margin: '0 auto',
    }}>
      {/* ---- 消息区 ---- */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '0 8px 16px' }}>
        {messages.length === 0 && (
          <div style={{ textAlign: 'center', color: '#999', marginTop: 80 }}>
            <RobotOutlined style={{ fontSize: 40 }} />
            <p>我是智能助手，可以帮你采购下单、创建/更新物料、创建 BOM 等。</p>
            <p style={{ fontSize: 12 }}>
              支持上传文本或图片（{MAX_SIZE_TEXT}/文件），例如：&ldquo;根据这张采购单截图帮我下一笔 PO&rdquo;、&ldquo;读取这个 TXT 里的物料清单汇总一下&rdquo;
            </p>
          </div>
        )}
        {messages.map((m, i) => (
          <div key={i} style={{
            display: 'flex',
            justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start',
            marginBottom: 18,
          }}>
            <div style={{ maxWidth: '78%' }}>
              <div style={{
                display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4,
                justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start',
              }}>
                {m.role === 'user'
                  ? (<>
                    <Text type="secondary" style={{ fontSize: 12 }}>我</Text>
                    <UserOutlined style={{ fontSize: 12, color: '#999' }} />
                  </>)
                  : (<>
                    <RobotOutlined style={{ fontSize: 12, color: '#1890ff' }} />
                    <Text type="secondary" style={{ fontSize: 12 }}>智能助手</Text>
                  </>)}
              </div>

              {m.attachments?.length > 0 && (
                <div style={{
                  display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 6,
                  justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start',
                }}>
                  {m.attachments.map((a) => (
                    <Tooltip key={a.uid || a.name} title={`${a.name}（${bytesToSize(a.size)}）`}>
                      <div style={{
                        padding: 6, background: '#fff',
                        border: '1px solid #eee', borderRadius: 8,
                      }}>
                        <FileThumb file={a} />
                        <div style={{
                          fontSize: 12, color: '#666', marginTop: 4,
                          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                          maxWidth: 90,
                        }}>{a.name}</div>
                      </div>
                    </Tooltip>
                  ))}
                </div>
              )}

              {m.content && (
                <div style={{
                  background: m.role === 'user' ? '#1890ff' : (m.error ? '#fff1f0' : '#f5f5f5'),
                  color: m.role === 'user' ? '#fff' : (m.error ? '#cf1322' : '#333'),
                  padding: '10px 14px', borderRadius: 10,
                  whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                  boxShadow: '0 1px 2px rgba(0,0,0,.04)',
                }}>
                  <Paragraph style={{ margin: 0 }}>{m.content}</Paragraph>
                </div>
              )}

              {m.role === 'assistant' && (m.agentName || m.usage) && (
                <Space size={4} wrap style={{ marginTop: 4 }}>
                  {m.agentName && <Tag color="blue" style={{ fontSize: 11 }}>{m.agentName}</Tag>}
                  {m.toolCalls && m.toolCalls.map((t) => (
                    <Tag key={t} style={{ fontSize: 11 }}>{t}</Tag>
                  ))}
                </Space>
              )}

              {m.role === 'assistant' && m.usage && (
                <div style={{
                  marginTop: 6, padding: '4px 10px',
                  background: '#fafafa',
                  border: '1px solid #f0f0f0',
                  borderRadius: 6,
                  fontSize: 11,
                  color: '#8c8c8c',
                  lineHeight: 1.8,
                  display: 'flex', flexWrap: 'wrap', gap: '6px 12px',
                }}>
                  {m.timestamp && (
                    <span title="响应时间">
                      🕐 {new Date(m.timestamp).toLocaleTimeString('zh-CN', { hour12: false })}
                    </span>
                  )}
                  {m.usage.model && (
                    <span title="模型">🤖 {m.usage.model}</span>
                  )}
                  {formatUsage(m.usage) && (
                    <span title={`提示词 ${m.usage.prompt_tokens || 0} · 生成 ${m.usage.completion_tokens || 0} · 总 ${m.usage.total_tokens || 0}`}>
                      🔢 {formatUsage(m.usage)}
                    </span>
                  )}
                  <span title="耗时">⏱ {formatMs(m.usage.total_ms)}</span>
                  {m.usage.llm_calls > 1 && (
                    <span title="本轮函数调用回合数">🔁 {m.usage.llm_calls} 轮</span>
                  )}
                </div>
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

      {/* ---- 底部交互区 ---- */}
      <div style={{
        borderRadius: 14,
        border: '1px solid #e5e7eb',
        background: '#fff',
        boxShadow: '0 2px 10px rgba(0,0,0,.04)',
      }}>
        {/* 待发送附件 */}
        {pendingFiles.length > 0 && (
          <div style={{
            padding: '10px 14px 10px',
            borderBottom: '1px dashed #f0f0f0',
            background: '#fafbfc',
            borderTopLeftRadius: 14,
            borderTopRightRadius: 14,
          }}>
            <div style={{
              display: 'flex', alignItems: 'center', justifyContent: 'space-between',
              marginBottom: 8,
            }}>
              <Space size={8}>
                <Text type="secondary" style={{ fontSize: 12 }}>待发送</Text>
                <Badge count={pendingFiles.length} size="small" />
              </Space>
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={clearAllPending}
              >
                清空
              </Button>
            </div>
            <div style={{
              display: 'flex', gap: 10, overflowX: 'auto',
              padding: '6px 2px 2px',
            }}>
              {pendingFiles.map((f) => (
                <div key={f.uid} style={{
                  flex: '0 0 auto', position: 'relative', width: 104,
                }}>
                  <div style={{
                    padding: 8, borderRadius: 8, background: '#fff',
                    border: '1px solid #f0f0f0',
                    boxShadow: '0 1px 2px rgba(0,0,0,.04)',
                  }}>
                    <FileThumb file={f} size="m" />
                    <div style={{
                      fontSize: 11, color: '#333', marginTop: 6,
                      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }} title={f.name}>{f.name}</div>
                    <div style={{ fontSize: 10, color: '#999', marginTop: 1 }}>{bytesToSize(f.size)}</div>
                  </div>
                  <button
                    type="button"
                    onClick={() => removePending(f.uid)}
                    title="移除"
                    style={{
                      position: 'absolute', top: 2, right: 2,
                      width: 20, height: 20,
                      borderRadius: '50%',
                      border: '1px solid #ffccc7',
                      background: '#fff',
                      color: '#ff4d4f',
                      cursor: 'pointer',
                      padding: 0,
                      fontSize: 13,
                      fontWeight: 600,
                      lineHeight: 1,
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      boxShadow: '0 1px 2px rgba(0,0,0,.12)',
                      appearance: 'none',
                      WebkitAppearance: 'none',
                      outline: 'none',
                    }}
                  >×</button>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 工具栏 + 输入框 + 发送 */}
        <div style={{
          padding: 10,
          borderBottomLeftRadius: 14,
          borderBottomRightRadius: 14,
          overflow: 'hidden',
        }}>
          <div style={{
            display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6,
            flexWrap: 'wrap',
          }}>
            <Upload.Dragger
              multiple
              accept={ACCEPT}
              showUploadList={false}
              beforeUpload={beforeUpload}
              style={{
                display: 'none',
              }}
              disabled={loading}
            />
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept={ACCEPT}
              disabled={loading}
              style={{ display: 'none' }}
              onChange={onInputFilesChange}
            />
            <Button
              size="small"
              icon={<PaperClipOutlined />}
              onClick={onPickFiles}
              disabled={loading}
            >
              附件
            </Button>
            <Popconfirm title="确认清除所有对话记忆？" onConfirm={clearMemory}>
              <Button size="small" icon={<DeleteOutlined />} disabled={loading}>
                清除记忆
              </Button>
            </Popconfirm>
            <Space size={4} style={{ marginLeft: 'auto' }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Enter 发送
              </Text>
              <Text type="secondary" style={{ fontSize: 11, color: '#bbb' }}>·</Text>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Shift+Enter 换行
              </Text>
              <Text type="secondary" style={{ fontSize: 11, color: '#bbb' }}>·</Text>
              <Text type="secondary" style={{ fontSize: 12 }}>
                ≤{MAX_SIZE_TEXT}/文件
              </Text>
            </Space>
          </div>

          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
            <Input.TextArea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder={
                pendingFiles.length > 0
                  ? `已添加 ${pendingFiles.length} 个文件，可以补充说明要做什么…（也可以只发文件让我自己看）`
                  : `描述你想做的事，或点击"附件"上传文件 / 粘贴图片`
              }
              autoSize={{ minRows: 1, maxRows: 6 }}
              onPressEnter={(e) => {
                if (!e.shiftKey) { e.preventDefault(); send() }
              }}
              disabled={loading}
              style={{
                flex: 1,
                background: '#fafbfc',
                borderRadius: 10,
                resize: 'none',
                border: '1px solid #f0f0f0',
                padding: '10px 12px',
                lineHeight: 1.6,
                fontSize: 14,
              }}
            />
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={send}
              loading={loading}
              disabled={!input.trim() && pendingFiles.length === 0}
              style={{
                height: 44,
                minWidth: 86,
                borderRadius: 10,
                padding: '0 16px',
                fontSize: 14,
                fontWeight: 500,
              }}
            >
              发送
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

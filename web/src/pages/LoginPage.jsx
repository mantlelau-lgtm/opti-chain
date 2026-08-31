import { useState } from 'react'
import { Form, Input, Button, Card, message } from 'antd'
import { UserOutlined, LockOutlined, HomeOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { authApi } from '../api/index.js'
import { auth } from '../api/client.js'

// LoginPage: tenant + credentials -> JWT; perms are refreshed from /auth/me
// so the menu reflects the DB-catalogued permissions immediately.
export default function LoginPage() {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const submit = async (values) => {
    setLoading(true)
    try {
      const data = await authApi.login(values)
      auth.save(data.token, data.user, [])
      try {
        const me = await authApi.me()
        auth.setPerms(me.perms || [])
      } catch { /* token accepted; perms will refresh on next load */ }
      message.success(`欢迎，${data.user.name || data.user.username}`)
      navigate('/', { replace: true })
    } catch (e) {
      message.error(e.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#001529',
      }}
    >
      <Card style={{ width: 380 }} title="SCM · 轻量级供应链系统">
        <Form layout="vertical" onFinish={submit}>
          <Form.Item name="tenant_code" rules={[{ required: true, message: '请输入租户编码' }]}>
            <Input prefix={<HomeOutlined />} placeholder="租户编码（如 demo / platform）" autoFocus />
          </Form.Item>
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form>
      </Card>
    </div>
  )
}

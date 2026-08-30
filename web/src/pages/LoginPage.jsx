import { useState } from 'react'
import { Form, Input, Button, Card, message } from 'antd'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import client, { auth } from '../api/client.js'

// LoginPage: exchanges credentials for a JWT and stores it locally.
export default function LoginPage() {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const submit = async (values) => {
    setLoading(true)
    try {
      const data = await client.post('/auth/login', values)
      auth.save(data.token, data.user)
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
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" autoFocus />
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

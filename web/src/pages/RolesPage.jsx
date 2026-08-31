import { useState, useEffect } from 'react'
import { Card, Select, Checkbox, Button, message, Space, Tag, Divider } from 'antd'
import { authApi } from '../api/index.js'

// RolesPage: platform console — view the six global roles and edit each
// role's permission matrix (roles/permissions are shared across tenants).
export default function RolesPage() {
  const [modules, setModules] = useState([])
  const [permissions, setPermissions] = useState([])
  const [roles, setRoles] = useState([])
  const [rolePerms, setRolePerms] = useState({})
  const [roleCode, setRoleCode] = useState(undefined)
  const [checked, setChecked] = useState([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    authApi.catalog().then((c) => {
      setModules(c.modules || [])
      setPermissions(c.permissions || [])
      setRoles(c.roles || [])
      setRolePerms(c.role_perms || {})
    }).catch((e) => message.error(e.message))
  }, [])

  const selectRole = (code) => {
    setRoleCode(code)
    setChecked(rolePerms[code] || [])
  }

  const save = async () => {
    const role = roles.find((r) => r.code === roleCode)
    if (!role) return
    setSaving(true)
    try {
      await authApi.setRolePermissions(role.id, checked)
      message.success('已保存，权限即时生效')
      setRolePerms((p) => ({ ...p, [roleCode]: checked }))
    } catch (e) { message.error(e.message || '保存失败') } finally { setSaving(false) }
  }

  const roleOpts = roles.map((r) => ({ label: `${r.name}（${r.code}）`, value: r.code }))

  return (
    <div>
      <Card title="角色与权限配置" extra={
        <Space>
          <span>角色</span>
          <Select style={{ width: 260 }} options={roleOpts} value={roleCode} onChange={selectRole} placeholder="选择角色" />
          <Button type="primary" disabled={!roleCode} loading={saving} onClick={save}>保存</Button>
        </Space>
      }>
        {!roleCode
          ? <div style={{ color: '#999' }}>选择角色后，勾选该角色拥有的权限（权限为全局模板，所有租户共用）</div>
          : modules.map((m) => {
              const modPerms = permissions.filter((p) => p.module_id === m.id)
              if (modPerms.length === 0) return null
              return (
                <div key={m.id}>
                  <Divider orientation="left">{m.name}</Divider>
                  <Checkbox.Group
                    options={modPerms.map((p) => ({ label: `${p.name}（${p.code}）`, value: p.code }))}
                    value={checked}
                    onChange={(vals) => setChecked(vals)}
                  />
                </div>
              )
            })}
        {roleCode && (
          <div style={{ marginTop: 16, color: '#999' }}>
            <Tag color="blue">{checked.length} 项权限</Tag>
            提示：admin 角色建议保留全部权限，否则平台/租户管理员可能失去管理入口。
          </div>
        )}
      </Card>
    </div>
  )
}

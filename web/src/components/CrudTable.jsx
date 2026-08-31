import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Input, Space, Modal, Form, message, Popconfirm } from 'antd'
import { PlusOutlined } from '@ant-design/icons'

// CrudTable is a generic, reusable CRUD UI. Each page passes a `resource`
// descriptor (columns, form fields, api) so the page itself stays tiny.
//
// resource = {
//   title, api: {list,create,update,remove}, columns, fields,
//   makeFormValues(record|null), toPayload(formValues),
// }
export default function CrudTable({ resource }) {
  const { title, api, columns, fields, makeFormValues, toPayload } = resource
  const [data, setData] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form] = Form.useForm()

  const load = useCallback(() => {
     setLoading(true)
     api.list({ page, size, keyword })
       .then((res) => { setData(res.list); setTotal(res.total) })
       .catch((e) => message.error(e.message))
       .finally(() => setLoading(false))
   }, [api, page, size, keyword])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
     setEditing(null)
     form.resetFields()
     form.setFieldsValue(makeFormValues ? makeFormValues(null) : {})
     setOpen(true)
   }

  const openEdit = (record) => {
     setEditing(record)
     form.setFieldsValue(makeFormValues ? makeFormValues(record) : record)
     setOpen(true)
   }

  const submit = async () => {
    try {
      const values = await form.validateFields()
      const payload = toPayload ? toPayload(values) : values
      if (editing) {
         await api.update(editing.id, payload)
        message.success('已更新')
       } else {
         await api.create(payload)
        message.success('已创建')
       }
      setOpen(false)
      load()
      } catch (e) {
      if (e.errorFields) return // validation error already surfaced by the form
      message.error(e.message || '操作失败')
      }
   }

  const remove = async (id) => {
    try {
      await api.remove(id)
      message.success('已删除')
      load()
      } catch (e) {
      message.error(e.message || '删除失败')
      }
   }

  const fmtTime = (v) => (v ? String(v).slice(0, 19).replace('T', ' ') : '-')

  const cols = [
     ...columns,
     { title: '创建人', dataIndex: 'created_by', width: 90, render: (v) => v || '-' },
     { title: '创建时间', dataIndex: 'created_at', width: 160, render: fmtTime },
     { title: '更新人', dataIndex: 'updated_by', width: 90, render: (v) => v || '-' },
     { title: '更新时间', dataIndex: 'updated_at', width: 160, render: fmtTime },
     {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: resource.extraActions ? 240 : 160,
      render: (_, record) => (
          <Space>
            {resource.extraActions?.(record, load)}
            <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
            <Popconfirm title="确认删除？" onConfirm={() => remove(record.id)}>
              <Button type="link" danger size="small">删除</Button>
            </Popconfirm>
          </Space>
        ),
     },
   ]

  return (
       <div>
         <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
           <Input.Search
             placeholder="关键字搜索"
             allowClear
             style={{ width: 240 }}
             onSearch={(v) => { setKeyword(v); setPage(1) }}
           />
           <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
           新建{title}
           </Button>
         </Space>

         <Table
         rowKey="id"
         loading={loading}
         dataSource={data}
         columns={cols}
         scroll={{ x: 'max-content' }}
         pagination={{
           current: page,
           pageSize: size,
           total,
           showSizeChanger: true,
           showTotal: (t) => `共 ${t} 条`,
           onChange: (p, s) => { setPage(p); setSize(s) },
           }}
         />

         <Modal
          title={editing ? `编辑${title}` : `新建${title}`}
          open={open}
          onOk={submit}
          onCancel={() => setOpen(false)}
          destroyOnClose
          width={560}
         >
           <Form form={form} layout="vertical" preserve={false}>
             {fields.map((f) => (
               <Form.Item
                 key={f.name}
                 name={f.name}
                 label={f.label}
                 rules={f.rules || []}
                 valuePropName={f.valuePropName}
               >
                 {f.render ? f.render() : <Input placeholder={f.placeholder} />}
               </Form.Item>
            ))}
           </Form>
         </Modal>
       </div>
    )
}

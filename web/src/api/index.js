import client, { pageRes } from './client'

// Generic REST helpers used by every resource module. Each returns the
// unwrapped payload, so pages never touch axios or the envelope directly.
const api = {
  list: (path, params) => client.get(path, { params }).then(pageRes),
  get: (path) => client.get(path),
  create: (path, data) => client.post(path, data),
  update: (path, data) => client.put(path, data),
  remove: (path) => client.delete(path),
}

// ---- Base data ----
export const materialApi = {
  list: (p) => api.list('/materials', p),
  get: (id) => api.get(`/materials/${id}`),
  create: (d) => api.create('/materials', d),
  update: (id, d) => api.update(`/materials/${id}`, d),
  remove: (id) => api.remove(`/materials/${id}`),
}
export const supplierApi = {
  list: (p) => api.list('/suppliers', p),
  get: (id) => api.get(`/suppliers/${id}`),
  create: (d) => api.create('/suppliers', d),
  update: (id, d) => api.update(`/suppliers/${id}`, d),
  remove: (id) => api.remove(`/suppliers/${id}`),
  setAudit: (id, auditStatus) => api.update(`/suppliers/${id}/audit`, { audit_status: auditStatus }),
}
export const warehouseApi = {
  list: (p) => api.list('/warehouses', p),
  get: (id) => api.get(`/warehouses/${id}`),
  create: (d) => api.create('/warehouses', d),
  update: (id, d) => api.update(`/warehouses/${id}`, d),
  remove: (id) => api.remove(`/warehouses/${id}`),
}
export const locationApi = {
  list: (p) => api.list('/locations', p),
  get: (id) => api.get(`/locations/${id}`),
  create: (d) => api.create('/locations', d),
  update: (id, d) => api.update(`/locations/${id}`, d),
  remove: (id) => api.remove(`/locations/${id}`),
}

// ---- Procurement ----
export const poApi = {
  list: (p) => api.list('/po', p),
  get: (id) => api.get(`/po/${id}`),
  create: (d) => api.create('/po', d),
  update: (id, d) => api.update(`/po/${id}`, d),
  setStatus: (id, status) => api.update(`/po/${id}/status`, { status }),
  remove: (id) => api.remove(`/po/${id}`),
  receive: (id, d) => client.post(`/po/${id}/receive`, d),
  receipts: (id) => client.get(`/po/${id}/receipts`),
}

// ---- Sales ----
export const customerApi = {
  list: (p) => api.list('/customers', p),
  get: (id) => api.get(`/customers/${id}`),
  create: (d) => api.create('/customers', d),
  update: (id, d) => api.update(`/customers/${id}`, d),
  remove: (id) => api.remove(`/customers/${id}`),
}
export const soApi = {
  list: (p) => api.list('/so', p),
  get: (id) => api.get(`/so/${id}`),
  create: (d) => api.create('/so', d),
  approve: (id) => api.update(`/so/${id}/approve`),
  cancel: (id) => api.update(`/so/${id}/cancel`),
  remove: (id) => api.remove(`/so/${id}`),
}

// ---- Inventory ----
export const inventoryApi = {
  stock: (p) => api.list('/inventory/stock', p),
  moveIn: (d) => client.post('/inventory/move-in', d),
  moveOut: (d) => client.post('/inventory/move-out', d),
  orders: (p) => api.list('/inventory/orders', p),
  removeOrder: (id) => api.remove(`/inventory/orders/${id}`),
  logs: (p) => api.list('/inventory/logs', p),
}

// ---- Planning ----
export const planningApi = {
  demands: (p) => api.list('/planning/demands', p),
  createDemand: (d) => api.create('/planning/demands', d),
  updateDemand: (id, d) => api.update(`/planning/demands/${id}`, d),
  removeDemand: (id) => api.remove(`/planning/demands/${id}`),
  mrp: (p) => api.list('/planning/mrp', p),
  removeMrp: (id) => api.remove(`/planning/mrp/${id}`),
  compute: () => client.post('/planning/mrp/compute'),
  convert: (id, poNumber) => client.post(`/planning/mrp/${id}/convert`, { po_number: poNumber || '' }),
}

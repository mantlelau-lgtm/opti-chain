import axios from 'axios'

// Single axios instance. All requests go through /api/v1 (proxied by Vite to
// the Go backend), keeping the frontend fully decoupled from the backend host.
const client = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

// Unwrap the standard {code,message,data} envelope and reject on non-zero code.
client.interceptors.response.use(
  (res) => {
     const body = res.data
     if (body && typeof body.code === 'number' && body.code !== 0) {
        return Promise.reject(new Error(body.message || 'request failed'))
     }
     return body ? body.data : undefined
  },
  (err) => {
     const msg = err?.response?.data?.message || err.message || 'network error'
     return Promise.reject(new Error(msg))
  },
)

export const pageRes = (data) => ({ total: data?.total || 0, list: data?.list || [] })
export default client

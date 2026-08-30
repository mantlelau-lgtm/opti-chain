import axios from 'axios'

// Single axios instance. All requests go through /api/v1 (proxied by Vite to
// the Go backend), keeping the frontend fully decoupled from the backend host.
const client = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

const TOKEN_KEY = 'scm_token'
const USER_KEY = 'scm_user'

// auth persists the JWT + a safe user view in localStorage.
export const auth = {
  token: () => localStorage.getItem(TOKEN_KEY),
  user: () => {
    try { return JSON.parse(localStorage.getItem(USER_KEY)) } catch { return null }
  },
  save: (token, user) => {
    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(USER_KEY, JSON.stringify(user))
  },
  clear: () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
  },
}

// Attach the bearer token to every request.
client.interceptors.request.use((cfg) => {
  const t = auth.token()
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

// Unwrap the standard {code,message,data} envelope and reject on non-zero
// code. A 401 clears the session and bounces to /login (unless already there).
client.interceptors.response.use(
  (res) => {
     const body = res.data
     if (body && typeof body.code === 'number' && body.code !== 0) {
        return Promise.reject(new Error(body.message || 'request failed'))
     }
     return body ? body.data : undefined
  },
  (err) => {
     if (err?.response?.status === 401 && !window.location.pathname.startsWith('/login')) {
        auth.clear()
        window.location.href = '/login'
     }
     const msg = err?.response?.data?.message || err.message || 'network error'
     return Promise.reject(new Error(msg))
  },
)

export const pageRes = (data) => ({ total: data?.total || 0, list: data?.list || [] })
export default client

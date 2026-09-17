import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

// 请求拦截：附加 token
// Authorization 之外同时发送 X-Api-Key：部分托管平台网关会改写
// Authorization 头，双发保证在被代理环境下依然可用
http.interceptors.request.use((config) => {
  const token = localStorage.getItem('domhub_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
    config.headers['X-Api-Key'] = token
  }
  return config
})

// 响应拦截：统一处理 401 与业务错误
http.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const status = error.response?.status
    const message = error.response?.data?.message || '请求失败，请稍后重试'
    if (status === 401) {
      localStorage.removeItem('domhub_token')
      document.cookie = 'domhub_token=; path=/; max-age=0'
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login')
      }
    }
    // 登录接口的错误由页面自己展示，这里不弹
    if (!error.config?.url?.includes('/auth/login')) {
      ElMessage.error(message)
    }
    return Promise.reject(error)
  }
)

export default http

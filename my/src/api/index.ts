import type { AxiosRequestConfig } from 'axios'
import axios from 'axios'
import { toast } from 'vue-sonner'

import router from '@/router'
import { useUserStore } from '@/stores/user'

/** 后端统一响应结构：{ code, message, data }，code === 0 为成功 */
export interface ApiResult<T = unknown> {
  code: number
  message: string
  data: T
}

const MAX_RETRY_COUNT = 2
const RETRY_DELAY = 1000

declare module 'axios' {
  export interface AxiosRequestConfig {
    retry?: boolean
    retryCount?: number
  }
}

// 开启 Mock 时强制走 /mock-api/v1（vite-plugin-fake-server 拦截）；
// 关闭 Mock 时走 VITE_API_BASE_URL（真实后端，经 vite proxy）。
// 这样切换 VITE_APP_MOCK 一个开关即可，无需再手动改 baseURL。
const baseURL = import.meta.env.VITE_APP_MOCK === 'true'
  ? '/mock-api/v1'
  : (import.meta.env.VITE_API_BASE_URL || '/api/v1')

const api = axios.create({
  baseURL,
  timeout: 1000 * 60,
  responseType: 'json',
})

// 请求拦截：注入 JWT
api.interceptors.request.use((request) => {
  const userStore = useUserStore()
  if (request.headers && userStore.isLogin) {
    request.headers.Authorization = `Bearer ${userStore.token}`
  }
  return request
})

function handleError(error: any) {
  let message = error.message || '请求出错'
  if (message === 'Network Error') {
    message = '网络错误'
  }
  else if (message.includes('timeout')) {
    message = '接口请求超时'
  }
  else if (message.includes('Request failed with status code')) {
    const response = error.response
    message = response?.data?.message || `接口异常 ${message.slice(-3)}`
  }
  toast.error('Error', { description: message })
  if (error.response?.status === 401) {
    // 登录/注册/登出接口自身的 401 是业务结果（凭证错误 / 未登录），
    // 不能再触发“会话失效→登出”处理——否则登出接口的 401 会无限递归调用自己。
    const url: string = error.config?.url || ''
    const isAuthEndpoint = /auth\/(login|register|logout)/.test(url)
    if (!isAuthEndpoint) {
      const userStore = useUserStore()
      if (userStore.isLogin) {
        // 仅清本地登录态（不发网络登出请求），并跳转登录页
        userStore.clear()
        if (router.currentRoute.value.name !== 'login') {
          router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
        }
      }
    }
    throw error
  }
  return Promise.reject(error)
}

// 响应拦截：解包 + 业务错误 toast
api.interceptors.response.use(
  (response) => {
    const res = response.data as ApiResult
    if (res && typeof res.code === 'number' && res.code !== 0) {
      if (res.message) {
        toast.warning('Warning', { description: res.message })
      }
      return Promise.reject(res)
    }
    return response.data
  },
  async (error) => {
    const config = error.config
    if (!config || !config.retry) {
      return handleError(error)
    }
    config.retryCount = config.retryCount || 0
    if (config.retryCount >= MAX_RETRY_COUNT) {
      return handleError(error)
    }
    config.retryCount += 1
    await new Promise(resolve => setTimeout(resolve, RETRY_DELAY))
    return api(config)
  },
)

/** 发起请求，返回已解包的 ApiResult（含 code/message/data） */
export function request<T = unknown>(config: AxiosRequestConfig) {
  return api.request<unknown, ApiResult<T>>(config)
}

export default api

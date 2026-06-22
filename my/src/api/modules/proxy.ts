import type {
  ProbeOutcome,
  Proxy,
  ProxyCreate,
  ProxyListParams,
  ProxyListResult,
  ProxyUpdate,
} from '@/types/proxy'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 我的代理（SOCKS5）：归属当前登录前台用户，需登录，完整增删改查
export default {
  list: (params: ProxyListParams) =>
    api.get<unknown, R<ProxyListResult>>('proxy/list', { params }),

  // 测试代理：经 SOCKS5 实测连通性/延迟/出口IP + 自动识别归属，返回更新后的记录
  test: (id: number) => api.post<unknown, R<Proxy>>(`proxy/${id}/test`),

  // 即时探测（添加/编辑表单里测试用，按参数探测、不落库）
  probe: (payload: { host: string, port: number, username?: string, password?: string }) =>
    api.post<unknown, R<ProbeOutcome>>('proxy/probe', payload),

  detail: (id: number) => api.get<unknown, R<Proxy>>(`proxy/${id}`),

  create: (data: ProxyCreate) => api.post<unknown, R<Proxy>>('proxy/create', data),

  // 批量导入（前端已解析成结构化条目）
  batch: (proxies: ProxyCreate[]) =>
    api.post<unknown, R<{ created: number, total: number }>>('proxy/batch', { proxies }),

  update: (id: number, data: ProxyUpdate) =>
    api.put<unknown, R<Proxy>>(`proxy/update/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`proxy/delete/${id}`),
}

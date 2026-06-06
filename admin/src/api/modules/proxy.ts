import type { Proxy, ProxyListParams, ProxyListResult } from '@/types/proxy'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  adminList: (params: ProxyListParams) =>
    api.get<unknown, R<ProxyListResult>>('admin/proxies/list', { params }),

  adminDetail: (id: number) =>
    api.get<unknown, R<Proxy>>(`admin/proxies/${id}`),

  adminDelete: (id: number) =>
    api.delete<unknown, R<null>>(`admin/proxies/delete/${id}`),
}

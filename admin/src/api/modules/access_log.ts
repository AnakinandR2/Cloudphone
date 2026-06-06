import type {
  AccessLog,
  AccessLogListParams,
  AccessLogListResult,
} from '@/types/access_log'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  list: (params: AccessLogListParams) =>
    api.get<unknown, R<AccessLogListResult>>('access-log/list', { params }),

  detail: (id: number) => api.get<unknown, R<AccessLog>>(`access-log/${id}`),
}

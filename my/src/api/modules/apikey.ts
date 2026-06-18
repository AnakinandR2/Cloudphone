import type { ApiKey, ApiKeyCreated } from '@/types/apikey'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// API 密钥：归属当前登录前台用户，需登录。
export default {
  list: () => api.get<unknown, R<ApiKey[]>>('user/api-keys'),
  create: (name: string) => api.post<unknown, R<ApiKeyCreated>>('user/api-keys', { name }),
  // 重复查看：解密返回完整明文。
  reveal: (id: number) => api.get<unknown, R<{ fullKey: string }>>(`user/api-keys/${id}/reveal`),
  revoke: (id: number) => api.post<unknown, R<null>>(`user/api-keys/${id}/revoke`),
}

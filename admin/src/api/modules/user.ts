import type { User, UserListParams, UserListResult } from '@/types/user'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  list: (params: UserListParams) =>
    api.get<unknown, R<UserListResult>>('admin/users/list', { params }),

  detail: (id: number) => api.get<unknown, R<User>>(`admin/users/${id}`),

  setStatus: (id: number, isActive: boolean) =>
    api.put<unknown, R<User>>(`admin/users/${id}/status`, { is_active: isActive }),
}

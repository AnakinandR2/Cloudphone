import type {
  PermissionGroup,
  Role,
  RoleCreate,
  RoleListParams,
  RoleListResult,
  RoleUpdate,
} from '@/types/role'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  list: (params: RoleListParams) =>
    api.get<unknown, R<RoleListResult>>('role/list', { params }),

  detail: (id: number) => api.get<unknown, R<Role>>(`role/${id}`),

  create: (data: RoleCreate) => api.post<unknown, R<Role>>('role/create', data),

  update: (id: number, data: RoleUpdate) =>
    api.put<unknown, R<Role>>(`role/update/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`role/delete/${id}`),

  /** 所有可分配权限（按模块分组） */
  permissions: () => api.get<unknown, R<PermissionGroup[]>>('role/permissions'),

  /** 全部角色（用于用户表单的角色多选，不分页） */
  all: () =>
    api.get<unknown, R<RoleListResult>>('role/list', {
      params: { page: 1, size: 999 },
    }),
}

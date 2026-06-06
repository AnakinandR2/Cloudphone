/** 角色简要信息（用户上的角色标签） */
export interface RoleBrief {
  id: number
  name: string
}

/** 角色列表项 */
export interface RoleListItem {
  id: number
  name: string
  description: string
  is_builtin: boolean
  permission_count: number
  user_count: number
  created_at: string
  updated_at: string
}

/** 角色详情（含权限码列表） */
export interface Role {
  id: number
  name: string
  description: string
  is_builtin: boolean
  permissions: string[]
  created_at: string
  updated_at: string
}

export interface RoleCreate {
  name: string
  description?: string
  permissions?: string[]
}

export interface RoleUpdate {
  name: string
  description?: string
  permissions?: string[]
}

export interface RoleListParams {
  page: number
  size: number
  name?: string
}

export interface RoleListResult {
  list: RoleListItem[]
  total: number
}

export interface Permission {
  key: string
  label: string
}

export interface PermissionGroup {
  module: string
  module_key: string
  permissions: Permission[]
}

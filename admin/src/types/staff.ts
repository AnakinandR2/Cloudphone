import type { RoleBrief } from './role'

/** 当前登录管理员的信息与权限（登录态，主体仍称 user） */
export interface UserInfo {
  account: string
  name: string
  avatar: string
  is_superuser: boolean
  permissions: string[]
}

/** 登录结果（账号密码 / SSO 共用） */
export interface LoginResult {
  account: string
  token: string
  avatar: string
  is_superuser: boolean
}

/** 被管理的管理员实体 */
export interface Staff {
  id: number
  username: string
  /** 展示姓名；为空时界面用 username 代替 */
  name: string
  is_active: boolean
  is_superuser: boolean
  avatar: string
  roles: RoleBrief[]
  created_at: string
  updated_at: string
}

export interface StaffCreate {
  username: string
  name?: string
  password: string
  is_active: boolean
  is_superuser: boolean
  avatar?: string
  role_ids?: number[]
}

export interface StaffUpdate {
  username?: string
  name?: string
  password?: string
  is_active?: boolean
  is_superuser?: boolean
  avatar?: string
  role_ids?: number[]
}

export interface StaffListParams {
  page: number
  size: number
  username?: string
}

export interface StaffListResult {
  list: Staff[]
  total: number
}

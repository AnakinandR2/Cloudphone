/** 前台用户（C 端自助注册的手机号用户），admin 只读 + 改状态 */
export interface User {
  id: number
  phone: string
  nickname: string
  avatar: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface UserListParams {
  page: number
  size: number
  phone?: string
  is_active?: boolean
}

export interface UserListResult {
  list: User[]
  total: number
}

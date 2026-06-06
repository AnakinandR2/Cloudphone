/** 前台用户（user） */
export interface User {
  id: number
  phone: string
  nickname: string
  avatar: string
  is_active: boolean
  created_at: string
  updated_at: string
}

/** 登录/注册响应 */
export interface AuthResult {
  id: number
  phone: string
  nickname: string
  avatar: string
  token: string
}

export interface LoginRequest {
  phone: string
  password: string
}

export interface RegisterRequest {
  phone: string
  password: string
  nickname?: string
}

import type {
  AuthResult,
  LoginRequest,
  RegisterRequest,
  User,
} from '@/types/user'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  /** 前台用户登录（手机号 + 密码） */
  login: (data: LoginRequest) =>
    api.post<unknown, R<AuthResult>>('user/auth/login', data),

  /** 前台用户注册 */
  register: (data: RegisterRequest) =>
    api.post<unknown, R<AuthResult>>('user/auth/register', data),

  /** 登出 */
  logout: () => api.post<unknown, R<null>>('user/auth/logout'),

  /** 当前登录用户信息 */
  me: () => api.get<unknown, R<User>>('user/me'),
}

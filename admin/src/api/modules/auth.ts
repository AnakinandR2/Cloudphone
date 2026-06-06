import type { LoginResult, UserInfo } from '@/types/staff'
import api from '../index'

export default {
  /** 账号密码登录 */
  login: (data: { account: string, password: string }) =>
    api.post<unknown, { code: number, message: string, data: LoginResult }>(
      'staff/auth/login',
      data,
    ),

  /** 小西通行证（SSO）登录：用 SSO 回调 token 换取本系统登录态 */
  ssoLogin: (data: { token: string }) =>
    api.post<unknown, { code: number, message: string, data: LoginResult }>(
      'staff/auth/sso-login',
      data,
    ),

  /** 获取当前用户信息与权限 */
  me: () =>
    api.get<unknown, { code: number, message: string, data: UserInfo }>('staff/auth/me'),
}

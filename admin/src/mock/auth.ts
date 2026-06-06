import { defineFakeRoute } from 'vite-plugin-fake-server/client'

/**
 * 登录鉴权 Mock。统一响应 { code, message, data }，code === 0 成功。
 * 内置账号（密码任意）：
 *   admin —— 超级管理员（permissions: ['*']，拥有全部权限）
 *   test  —— 普通用户（仅 dashboard:view / staff:view）
 */
const USERS: Record<
  string,
  { name: string, is_superuser: boolean, permissions: string[] }
> = {
  admin: {
    name: '超级管理员',
    is_superuser: true,
    permissions: ['*'],
  },
  test: {
    name: '测试用户',
    is_superuser: false,
    permissions: ['dashboard:view', 'staff:view'],
  },
}

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

export default defineFakeRoute([
  {
    url: '/v1/staff/auth/login',
    method: 'post',
    response: ({ body }) => {
      const account = body?.account ?? 'admin'
      const profile = USERS[account]
      if (!profile) {
        return { code: 1, message: '账号或密码错误', data: null }
      }
      return ok({
        account,
        // token 以 account 开头，便于 /staff/auth/me 据此返回对应权限
        token: `${account}.${Math.random().toString(36).slice(2)}.mocktoken`,
        // mock 不返回头像地址，前端用 shadcn 头像占位（User 图标）
        avatar: '',
        is_superuser: profile.is_superuser,
      })
    },
  },
  {
    // 小西通行证（SSO）登录 Mock：任意 token 均视为以 admin 身份登录，便于本地联调
    url: '/v1/staff/auth/sso-login',
    method: 'post',
    response: ({ body }) => {
      const token = (body?.token as string) || ''
      if (!token) {
        return { code: 1, message: '缺少 SSO Token', data: null }
      }
      const account = 'admin'
      const profile = USERS[account]
      return ok({
        account,
        token: `${account}.${Math.random().toString(36).slice(2)}.ssomocktoken`,
        avatar: '',
        is_superuser: profile.is_superuser,
      })
    },
  },
  {
    url: '/v1/staff/auth/me',
    method: 'get',
    response: ({ headers }) => {
      const auth = (headers.authorization as string) || ''
      const token = auth.replace(/^Bearer\s+/i, '')
      const account = token.split('.')[0] || 'admin'
      const profile = USERS[account] ?? USERS.admin
      return ok({
        account,
        name: profile.name,
        avatar: '',
        is_superuser: profile.is_superuser,
        permissions: profile.permissions,
      })
    },
  },
])

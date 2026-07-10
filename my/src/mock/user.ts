import { defineFakeRoute } from 'vite-plugin-fake-server/client'
import { DEMO_TEST_ACCOUNT } from '@/constants/demoAccount'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function fail(message: string) {
  return { code: 1, message, data: null }
}
function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}

interface UserRec {
  id: number
  phone: string
  nickname: string
  avatar: string
  is_active: boolean
  created_at: string
  updated_at: string
  /** 仅内置测试账号校验；其它演示账号不强制 */
  password?: string
}

// 内存前台用户表：内置全量测试账号；其它手机号仍可随意登录（演示）
const users: UserRec[] = [
  {
    id: 1,
    phone: DEMO_TEST_ACCOUNT.phone,
    nickname: DEMO_TEST_ACCOUNT.nickname,
    avatar: `https://api.dicebear.com/7.x/initials/svg?seed=${DEMO_TEST_ACCOUNT.phone}`,
    is_active: true,
    password: DEMO_TEST_ACCOUNT.password,
    created_at: '2026-01-01 09:00:00',
    updated_at: '2026-01-01 09:00:00',
  },
]
let seq = 1

function ensureUser(phone: string): UserRec {
  const existing = users.find(x => x.phone === phone)
  if (existing) {
    return existing
  }
  const rec: UserRec = {
    id: ++seq,
    phone,
    nickname: `用户${phone.slice(-4)}`,
    avatar: `https://api.dicebear.com/7.x/initials/svg?seed=${phone}`,
    is_active: true,
    created_at: now(),
    updated_at: now(),
  }
  users.push(rec)
  return rec
}

function findByToken(headers: Record<string, string>): UserRec | undefined {
  const token = (headers.authorization || '').replace(/^Bearer\s+/i, '')
  const phone = token.split('.')[0]
  return users.find(u => u.phone === phone)
}

function authResult(u: UserRec) {
  return {
    id: u.id,
    phone: u.phone,
    nickname: u.nickname,
    avatar: u.avatar,
    token: `${u.phone}.${Math.random().toString(36).slice(2)}.mocktoken`,
  }
}

export default defineFakeRoute([
  {
    url: '/v1/user/auth/login',
    method: 'post',
    response: ({ body }) => {
      const phone = String(body?.phone ?? '').trim()
      const password = String(body?.password ?? '')
      if (!phone) {
        return fail('请填写手机号')
      }
      // 内置全量测试账号：必须匹配密码
      if (phone === DEMO_TEST_ACCOUNT.phone) {
        if (password !== DEMO_TEST_ACCOUNT.password) {
          return fail('手机号或密码错误')
        }
        return ok(authResult(ensureUser(phone)))
      }
      // 其它号码：演示用，密码不校验
      return ok(authResult(ensureUser(phone)))
    },
  },
  {
    url: '/v1/user/auth/register',
    method: 'post',
    response: ({ body }) => {
      const phone = body?.phone ?? ''
      if (!phone) {
        return fail('请填写手机号')
      }
      if (users.some(x => x.phone === phone)) {
        return fail('该手机号已注册')
      }
      const rec: UserRec = {
        id: ++seq,
        phone,
        nickname: body?.nickname || `用户${phone.slice(-4)}`,
        avatar: `https://api.dicebear.com/7.x/initials/svg?seed=${phone}`,
        is_active: true,
        created_at: now(),
        updated_at: now(),
      }
      users.push(rec)
      return ok(authResult(rec))
    },
  },
  {
    url: '/v1/user/auth/logout',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/user/me',
    method: 'get',
    response: ({ headers }) => {
      const u = findByToken(headers as Record<string, string>) ?? users[0]
      return ok(u)
    },
  },
])

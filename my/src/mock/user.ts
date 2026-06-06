import { defineFakeRoute } from 'vite-plugin-fake-server/client'

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
}

// 内存前台用户表（演示用，默认内置一个账号：13800138000 / 任意密码）
const users: UserRec[] = [
  {
    id: 1,
    phone: '13800138000',
    nickname: '小明',
    avatar: 'https://api.dicebear.com/7.x/initials/svg?seed=13800138000',
    is_active: true,
    created_at: '2026-01-01 09:00:00',
    updated_at: '2026-01-01 09:00:00',
  },
]
let seq = 1

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
      const phone = body?.phone ?? ''
      const u = users.find(x => x.phone === phone)
      if (!u) {
        return fail('手机号或密码错误')
      }
      return ok(authResult(u))
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

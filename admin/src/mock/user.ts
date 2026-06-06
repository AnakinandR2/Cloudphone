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

/** 前台用户（C 端自助注册的手机号用户） */
interface UserRec {
  id: number
  phone: string
  nickname: string
  avatar: string
  is_active: boolean
  created_at: string
  updated_at: string
}

const users: UserRec[] = [
  { id: 1, phone: '13800000001', nickname: '小明', is_active: true },
  { id: 2, phone: '13900000002', nickname: '阿强', is_active: true },
  { id: 3, phone: '13700000003', nickname: '', is_active: false },
  { id: 4, phone: '15800000004', nickname: 'Lily', is_active: true },
  { id: 5, phone: '15900000005', nickname: '老王', is_active: false },
  { id: 6, phone: '18600000006', nickname: 'Tom', is_active: true },
  { id: 7, phone: '13600000007', nickname: '芳芳', is_active: true },
].map((u, i) => ({
  ...u,
  avatar: `https://api.dicebear.com/7.x/initials/svg?seed=${u.nickname || u.phone}`,
  created_at: `2026-0${(i % 5) + 1}-1${i} 09:0${i}:00`,
  updated_at: `2026-0${(i % 5) + 1}-1${i} 09:0${i}:00`,
}))

export default defineFakeRoute([
  {
    url: '/v1/admin/users/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const phone = (query.phone as string) || ''
      const isActive = String(query.is_active ?? '')
      let list = users
      if (phone) {
        list = list.filter(u => u.phone.includes(phone))
      }
      if (isActive !== '') {
        const want = isActive === 'true' || isActive === '1'
        list = list.filter(u => u.is_active === want)
      }
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/admin/users/:id',
    method: 'get',
    response: ({ params }) => {
      const u = users.find(x => x.id === Number(params.id))
      return u ? ok(u) : fail('用户不存在')
    },
  },
  {
    url: '/v1/admin/users/:id/status',
    method: 'put',
    response: ({ params, body }) => {
      const u = users.find(x => x.id === Number(params.id))
      if (!u) return fail('用户不存在')
      u.is_active = !!body.is_active
      u.updated_at = now()
      return ok(u)
    },
  },
])

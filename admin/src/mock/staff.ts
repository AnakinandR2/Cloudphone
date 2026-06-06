import { faker } from '@faker-js/faker'
import { defineFakeRoute } from 'vite-plugin-fake-server/client'

import { roleBriefs } from './role'

faker.seed(2026)

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
  username: string
  name: string
  is_active: boolean
  is_superuser: boolean
  avatar: string
  roles: { id: number, name: string }[]
  created_at: string
  updated_at: string
}

const briefs = roleBriefs()

const users: UserRec[] = Array.from({ length: 57 }).map((_, i) => {
  const username = faker.internet.username().toLowerCase()
  const created = faker.date
    .past({ years: 1 })
    .toISOString()
    .slice(0, 19)
    .replace('T', ' ')
  return {
    id: i + 1,
    username,
    name: faker.person.fullName(),
    is_active: i % 5 !== 0,
    is_superuser: i === 0,
    avatar: `https://api.dicebear.com/7.x/initials/svg?seed=${username}`,
    roles: i === 0 ? [briefs[0]] : i % 3 === 0 ? [] : [briefs[(i % (briefs.length - 1)) + 1]],
    created_at: created,
    updated_at: created,
  }
})
let userSeq = users.length

function mapRoles(roleIds: number[] = []) {
  return briefs.filter(b => roleIds.includes(b.id))
}

export default defineFakeRoute([
  {
    url: '/v1/staff/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const keyword = (query.username as string) || ''
      let list = users
      if (keyword) {
        list = list.filter(
          u => u.username.includes(keyword) || u.name.includes(keyword),
        )
      }
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/staff/:id',
    method: 'get',
    response: ({ params }) => {
      const u = users.find(x => x.id === Number(params.id))
      return u ? ok(u) : fail('用户不存在')
    },
  },
  {
    url: '/v1/staff/create',
    method: 'post',
    response: ({ body }) => {
      if (users.some(u => u.username === body.username)) {
        return fail('用户名已存在')
      }
      const rec: UserRec = {
        id: ++userSeq,
        username: body.username,
        name: body.name || '',
        is_active: body.is_active ?? true,
        is_superuser: body.is_superuser ?? false,
        avatar:
          body.avatar
          || `https://api.dicebear.com/7.x/initials/svg?seed=${body.username}`,
        roles: mapRoles(body.role_ids),
        created_at: now(),
        updated_at: now(),
      }
      users.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/staff/update/:id',
    method: 'put',
    response: ({ params, body }) => {
      const u = users.find(x => x.id === Number(params.id))
      if (!u) return fail('用户不存在')
      u.username = body.username ?? u.username
      u.name = body.name ?? u.name
      if (body.is_active !== undefined) u.is_active = body.is_active
      if (body.is_superuser !== undefined) u.is_superuser = body.is_superuser
      if (body.avatar !== undefined) u.avatar = body.avatar
      if (body.role_ids) u.roles = mapRoles(body.role_ids)
      u.updated_at = now()
      return ok(u)
    },
  },
  {
    url: '/v1/staff/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = users.findIndex(x => x.id === Number(params.id))
      if (idx === -1) return fail('用户不存在')
      users.splice(idx, 1)
      return ok(null)
    },
  },
])

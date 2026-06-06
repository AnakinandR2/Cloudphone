import { faker } from '@faker-js/faker'
import { defineFakeRoute } from 'vite-plugin-fake-server/client'

faker.seed(99)

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function fail(message: string) {
  return { code: 1, message, data: null }
}

const METHODS = ['GET', 'GET', 'GET', 'POST', 'PUT', 'DELETE', 'PATCH']
const PATHS = [
  '/api/v1/staff/list',
  '/api/v1/staff/create',
  '/api/v1/role/list',
  '/api/v1/staff/auth/login',
  '/api/v1/staff/auth/me',
  '/api/v1/example/list',
  '/api/v1/access-log/list',
  '/api/v1/dashboard/stats',
]
const USERNAMES = ['admin', 'test', 'editor01', 'viewer02', '']

function statusFor(i: number) {
  if (i % 11 === 0) return 500
  if (i % 7 === 0) return 404
  if (i % 13 === 0) return 401
  return 200
}

const logs = Array.from({ length: 120 }).map((_, i) => {
  const method = METHODS[i % METHODS.length]
  const path = PATHS[i % PATHS.length]
  const username = USERNAMES[i % USERNAMES.length]
  const status = statusFor(i)
  const created = faker.date
    .recent({ days: 14 })
    .toISOString()
    .slice(0, 19)
    .replace('T', ' ')
  return {
    id: 1000 - i,
    user_id: username ? (i % 9) + 1 : 0,
    username,
    method,
    path,
    status_code: status,
    latency_ms: faker.number.int({ min: 3, max: 1200 }),
    client_ip: faker.internet.ipv4(),
    created_at: created,
    user_agent: faker.internet.userAgent(),
    request_headers: JSON.stringify(
      { 'Content-Type': 'application/json', 'Authorization': 'Bearer ***' },
    ),
    request_body: method === 'GET' ? '' : JSON.stringify({ page: 1, size: 10 }),
    response_headers: JSON.stringify({ 'Content-Type': 'application/json' }),
    response_body: JSON.stringify({ code: status === 200 ? 0 : 1, message: status === 200 ? '成功' : '错误' }),
  }
})

function statusGroupMatch(code: number, group: string) {
  if (group === '2xx') return code >= 200 && code < 300
  if (group === '4xx') return code >= 400 && code < 500
  if (group === '5xx') return code >= 500
  return true
}

export default defineFakeRoute([
  {
    url: '/v1/access-log/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      let list = logs.slice()
      const { username, method, path, status_group, start_time, end_time } = query as Record<string, string>
      if (username) list = list.filter(l => l.username.includes(username))
      if (method) list = list.filter(l => l.method === method)
      if (path) list = list.filter(l => l.path.includes(path))
      if (status_group) list = list.filter(l => statusGroupMatch(l.status_code, status_group))
      if (start_time) list = list.filter(l => l.created_at >= start_time)
      if (end_time) list = list.filter(l => l.created_at <= `${end_time} 23:59:59`)
      const total = list.length
      const start = (page - 1) * size
      // 返回完整记录（含头/体），便于行展开直接展示，无需二次请求
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/access-log/:id',
    method: 'get',
    response: ({ params }) => {
      const log = logs.find(l => l.id === Number(params.id))
      return log ? ok(log) : fail('日志不存在')
    },
  },
])

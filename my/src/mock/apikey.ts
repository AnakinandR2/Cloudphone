import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}

interface KeyRec {
  id: number
  name: string
  masked: string
  status: string
  lastUsedAt: string | null
  createdAt: string
  full: string // 演示用：完整明文（真实后端为加密存储）
}

let seq = 2
const keys: KeyRec[] = [
  { id: 1, name: '默认密钥', masked: 'gp_live_3f2a••••a17c', status: 'active', lastUsedAt: '2026-06-05 14:02', createdAt: '2026-05-12 09:20', full: 'gp_live_3f2a000000000000000000000000000000000000000000a17c' },
]

function rand(n: number) {
  let s = ''
  const cs = '0123456789abcdef'
  for (let i = 0; i < n; i++) s += cs[Math.floor(Math.random() * cs.length)]
  return s
}

export default defineFakeRoute([
  { url: '/v1/user/api-keys', method: 'get', response: () => ok(keys) },
  {
    url: '/v1/user/api-keys',
    method: 'post',
    response: ({ body }) => {
      const raw = rand(48)
      const full = `gp_live_${raw}`
      const rec: KeyRec = {
        id: ++seq,
        name: body?.name || '密钥',
        masked: `gp_live_${raw.slice(0, 4)}••••${raw.slice(-4)}`,
        status: 'active',
        lastUsedAt: null,
        createdAt: now(),
        full,
      }
      keys.unshift(rec)
      return ok({ ...rec, fullKey: full })
    },
  },
  {
    url: '/v1/user/api-keys/:id/reveal',
    method: 'get',
    response: ({ params }) => {
      const k = keys.find(x => x.id === Number(params.id))
      return ok({ fullKey: k?.full ?? '' })
    },
  },
  {
    url: '/v1/user/api-keys/:id/revoke',
    method: 'post',
    response: ({ params }) => {
      const k = keys.find(x => x.id === Number(params.id))
      if (k)
        k.status = 'revoked'
      return ok(null)
    },
  },
])

import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}

interface Rec {
  id: number
  store: boolean
  scriptId: number
  name: string
  description: string
  version: string
  luaContent: string
  fileName: string
  status: string
  uploaderId: number
  createTime: string
  updateTime: string
}

let seq = 10
const scripts: Rec[] = [
  { id: 1, store: true, scriptId: 200, name: 'Hello World', description: '连通性测试脚本', version: '1.0.0', luaContent: '--[[\n{ "name": { "desc": "名字", "type": "string", "required": true, "default": "world", "uiType": "string", "label": "名字" } }\n]]\nlocal name = \'${name}\'\nlog("hello " .. name)', fileName: 'hello.lua', status: 'enabled', uploaderId: 0, createTime: '2026-06-01 09:00:00', updateTime: '2026-06-01 09:00:00' },
  { id: 5, store: false, scriptId: 305, name: '某用户的养号脚本', description: '', version: '1.0.0', luaContent: 'log("warm")', fileName: 'warm.lua', status: 'enabled', uploaderId: 12, createTime: '2026-06-12 14:00:00', updateTime: '2026-06-12 14:00:00' },
]

export default defineFakeRoute([
  { url: '/v1/admin/automation/store', method: 'get', response: () => ok(scripts.filter(s => s.store)) },
  {
    url: '/v1/admin/automation/store',
    method: 'post',
    response: ({ body }) => {
      const rec: Rec = { id: ++seq, store: true, scriptId: 400 + seq, name: body?.name, description: body?.description ?? '', version: '1.0.0', luaContent: body?.luaContent ?? '', fileName: body?.fileName ?? '', status: 'enabled', uploaderId: 0, createTime: now(), updateTime: now() }
      scripts.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/admin/automation/store/:id',
    method: 'put',
    response: ({ params, body }) => {
      const s = scripts.find(x => x.id === Number(params.id))
      if (s)
        Object.assign(s, { name: body?.name, description: body?.description, luaContent: body?.luaContent, updateTime: now() })
      return ok(s)
    },
  },
  { url: '/v1/admin/automation/store/:id/toggle', method: 'post', response: ({ params, body }) => { const s = scripts.find(x => x.id === Number(params.id)); if (s) s.status = body?.enabled ? 'enabled' : 'disabled'; return ok(null) } },
  { url: '/v1/admin/automation/store/:id', method: 'delete', response: ({ params }) => { const i = scripts.findIndex(x => x.id === Number(params.id)); if (i >= 0) scripts.splice(i, 1); return ok(null) } },

  { url: '/v1/admin/automation/user-scripts', method: 'get', response: () => ok(scripts.filter(s => !s.store)) },
  { url: '/v1/admin/automation/user-scripts/:id/toggle', method: 'post', response: ({ params, body }) => { const s = scripts.find(x => x.id === Number(params.id)); if (s) s.status = body?.enabled ? 'enabled' : 'disabled'; return ok(null) } },
  { url: '/v1/admin/automation/user-scripts/:id', method: 'delete', response: ({ params }) => { const i = scripts.findIndex(x => x.id === Number(params.id)); if (i >= 0) scripts.splice(i, 1); return ok(null) } },
])

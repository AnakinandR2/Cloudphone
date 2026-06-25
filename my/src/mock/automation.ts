import { defineFakeRoute } from 'vite-plugin-fake-server/client'
import { extractSchemaComment, parseSchema } from '@/utils/paramsComment'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}
// 仿后端：schema 真源 = 脚本顶部注释；归一化成我们的数组 JSON 存库。
function deriveSchema(lua: string, fallback?: string): string {
  const inner = extractSchemaComment(lua || '')
  const specs = inner ? parseSchema(inner) : (fallback ? parseSchema(fallback) : [])
  return specs.length ? JSON.stringify(specs) : ''
}

interface ScriptRec {
  id: number
  store: boolean
  scriptId: number
  name: string
  description: string
  version: string
  luaContent: string
  paramsSchema: string
  fileName: string
  status: string
  createTime: string
  updateTime: string
}

let scriptSeq = 10
const scripts: ScriptRec[] = [
  { id: 1, store: false, scriptId: 100, name: '每日签到', description: '自动签到', version: '1.0.0', luaContent: 'log("hi")\nreport_result(\'{"ok":true}\')', paramsSchema: '', fileName: 'checkin.lua', status: 'enabled', createTime: '2026-06-10 10:00:00', updateTime: '2026-06-10 10:00:00' },
  { id: 2, store: true, scriptId: 200, name: 'Hello World', description: '连通性测试脚本', version: '1.0.0', luaContent: '--[[\n{ "name": { "desc": "名字", "type": "string", "required": true, "default": "world", "uiType": "string", "label": "名字" } }\n]]\nlocal name = \'${name}\'\nlog("hello " .. name)', paramsSchema: '[{"key":"name","label":"名字","type":"string","required":true,"default":"world"}]', fileName: 'hello.lua', status: 'enabled', createTime: '2026-06-01 09:00:00', updateTime: '2026-06-01 09:00:00' },
]

interface PlanRec {
  id: number
  planId: number
  planUid: string
  scriptId: number
  scriptName: string
  name: string
  frequency: string
  intervalValue: number
  executionTime: string
  startTime: string
  endTime: string
  cpIds: string[]
  status: string
  createTime: string
  updateTime: string
}
let planSeq = 1
const plans: PlanRec[] = []

interface TaskRec {
  id: number
  midTaskId: number
  taskNo: string
  scriptId: number
  scriptName: string
  planId: number
  cpId: string
  taskName: string
  trigger: string
  status: string
  runStart: string
  runEnd: string
  createTime: string
}
let taskSeq = 1
let midSeq = 9000
const tasks: TaskRec[] = []
// 任务详情轮询：midTaskId → 已轮询次数（第二次起返回完成）
const pollCount: Record<number, number> = {}

export default defineFakeRoute([
  // ---- 脚本 ----
  { url: '/v1/automation/scripts', method: 'get', response: () => ok(scripts.filter(s => !s.store)) },
  { url: '/v1/automation/scripts/store', method: 'get', response: () => ok(scripts.filter(s => s.store)) },
  { url: '/v1/automation/scripts/usable', method: 'get', response: () => ok(scripts.filter(s => s.status === 'enabled')) },
  {
    url: '/v1/automation/scripts',
    method: 'post',
    response: ({ body }) => {
      const rec: ScriptRec = {
        id: ++scriptSeq, store: false, scriptId: 300 + scriptSeq,
        name: body?.name ?? '脚本', description: body?.description ?? '', version: '1.0.0',
        luaContent: body?.luaContent ?? '', paramsSchema: deriveSchema(body?.luaContent ?? '', body?.paramsSchema),
        fileName: body?.fileName ?? '', status: 'enabled',
        createTime: now(), updateTime: now(),
      }
      scripts.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/automation/scripts/:id',
    method: 'put',
    response: ({ params, body }) => {
      const s = scripts.find(x => x.id === Number(params.id))
      if (s) {
        Object.assign(s, { name: body?.name, description: body?.description, luaContent: body?.luaContent, paramsSchema: deriveSchema(body?.luaContent ?? '', body?.paramsSchema), updateTime: now() })
      }
      return ok(s)
    },
  },
  {
    url: '/v1/automation/scripts/:id/toggle',
    method: 'post',
    response: ({ params, body }) => {
      const s = scripts.find(x => x.id === Number(params.id))
      if (s)
        s.status = body?.enabled ? 'enabled' : 'disabled'
      return ok(null)
    },
  },
  {
    url: '/v1/automation/scripts/:id',
    method: 'delete',
    response: ({ params }) => {
      const i = scripts.findIndex(x => x.id === Number(params.id))
      if (i >= 0)
        scripts.splice(i, 1)
      return ok(null)
    },
  },

  // ---- 计划 ----
  { url: '/v1/automation/plans', method: 'get', response: () => ok(plans) },
  {
    url: '/v1/automation/plans',
    method: 'post',
    response: ({ body }) => {
      const sc = scripts.find(x => x.id === body?.scriptId)
      const rec: PlanRec = {
        id: ++planSeq, planId: 500 + planSeq, planUid: `plan-${planSeq}`,
        scriptId: body?.scriptId, scriptName: sc?.name ?? '', name: body?.name,
        frequency: body?.frequency, intervalValue: body?.intervalValue ?? 0,
        executionTime: body?.executionTime ?? '', startTime: body?.startTime ?? '', endTime: body?.endTime ?? '',
        cpIds: body?.cpIds ?? [], status: 'NOT_STARTED', createTime: now(), updateTime: now(),
      }
      plans.unshift(rec)
      return ok(rec)
    },
  },
  { url: '/v1/automation/plans/:id/start', method: 'post', response: ({ params }) => { const p = plans.find(x => x.id === Number(params.id)); if (p) p.status = 'ENABLING'; return ok(null) } },
  { url: '/v1/automation/plans/:id/pause', method: 'post', response: ({ params }) => { const p = plans.find(x => x.id === Number(params.id)); if (p) p.status = 'PAUSED'; return ok(null) } },
  { url: '/v1/automation/plans/:id', method: 'delete', response: ({ params }) => { const i = plans.findIndex(x => x.id === Number(params.id)); if (i >= 0) plans.splice(i, 1); return ok(null) } },

  // ---- 任务 ----
  {
    url: '/v1/automation/tasks/run',
    method: 'post',
    response: ({ body }) => {
      const sc = scripts.find(x => x.id === body?.scriptId)
      const out: TaskRec[] = (body?.cpIds ?? []).map((cp: string) => {
        const mid = ++midSeq
        const rec: TaskRec = {
          id: ++taskSeq, midTaskId: mid, taskNo: `T-${mid}`,
          scriptId: body?.scriptId, scriptName: sc?.name ?? '', planId: 0,
          cpId: cp, taskName: body?.taskName || sc?.name || '任务', trigger: 'manual',
          status: 'WAITING_PUBLISH', runStart: '', runEnd: '', createTime: now(),
        }
        tasks.unshift(rec)
        return rec
      })
      return ok(out)
    },
  },
  {
    url: '/v1/automation/tasks',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      let list = tasks
      if (query.status)
        list = list.filter(t => t.status === query.status)
      const total = list.length
      return ok({ list: list.slice((page - 1) * size, page * size), total })
    },
  },
  {
    url: '/v1/automation/tasks/:midTaskId',
    method: 'get',
    response: ({ params }) => {
      const mid = Number(params.midTaskId)
      pollCount[mid] = (pollCount[mid] ?? 0) + 1
      const done = pollCount[mid] >= 2
      const tk = tasks.find(t => t.midTaskId === mid)
      if (tk)
        tk.status = done ? 'COMPLETED' : 'EXECUTING'
      return ok({
        midTaskId: mid, taskNo: `T-${mid}`,
        status: done ? 'COMPLETED' : 'EXECUTING', statusDesc: done ? '任务完成' : '正在执行',
        terminal: done, runDurationMs: done ? 1234 : 0,
        runLog: done ? '[INFO] hello world from glory cloud phone\n#RESULT#{"ok":true}#RESULT#' : '',
        result: done ? '{"ok":true}' : '',
        screenshotUrl: '',
      })
    },
  },
])

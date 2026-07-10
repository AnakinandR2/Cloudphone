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
function normalizeIds(raw: unknown): number[] {
  if (!Array.isArray(raw))
    return []
  return raw.map(id => Number(id)).filter(id => Number.isFinite(id) && id > 0)
}
function parseBody(body: unknown): Record<string, any> {
  if (body == null)
    return {}
  if (typeof body === 'string') {
    try {
      return JSON.parse(body) as Record<string, any>
    }
    catch {
      return {}
    }
  }
  if (typeof body === 'object')
    return body as Record<string, any>
  return {}
}

interface TagRec {
  name: string
  color: string
}

interface PhoneRec {
  id: number
  user_id: number
  cp_id: string
  name: string
  status: string
  vm_id: string
  image_id: string
  proxy_id: number
  remark: string
  tags: TagRec[]
  created_at: string
  updated_at: string
}

// 内存云手机表（演示用，归属 user 1）
const phones: PhoneRec[] = Array.from({ length: 4 }).map((_, i) => ({
  id: 4 - i,
  user_id: 1,
  cp_id: `CP-${1000 + (4 - i)}`,
  name: `云手机 ${4 - i}`,
  status: ['RUNNING', 'STOPPED', 'CREATED', 'CREATE_FAILED'][i % 4],
  vm_id: `vm-${4 - i}`,
  image_id: `img-android13`,
  proxy_id: i % 2 === 0 ? (4 - i) : 0,
  remark: '',
  tags: [],
  created_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
  updated_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
}))
let seq = 3000

// Root 演示态：phone id → 是否已 root
const rootState: Record<number, boolean> = {}

// 脚本任务演示态：轮询次数计数（达到阈值后返回完成）
let scriptPolls = 0

// ADB 演示态：phone id → { enabled, 白名单 IP }
const adbState: Record<number, { enabled: boolean, whiteIp: string[] }> = {}
function adbInfo(id: number) {
  const st = adbState[id]
  const enabled = st?.enabled ?? false
  return {
    enabled,
    adbAddress: enabled ? `10.30.${id % 255}.${(id * 7) % 255}:5555` : '',
    adbToken: enabled ? `tok-${(id * 8123).toString(36)}` : '',
    adbTokenExpiredAt: enabled ? '2026-07-06 10:00:00' : '',
    status: 'NORMAL',
  }
}

export default defineFakeRoute([
  {
    url: '/v1/phone/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const kw = (query.kw as string) || ''
      const status = (query.status as string) || ''
      const tag = (query.tag as string) || ''
      let list = phones
      if (kw) list = list.filter(p => p.name.includes(kw) || p.cp_id.includes(kw))
      if (status) list = list.filter(p => p.status === status)
      if (tag) list = list.filter(p => p.tags.some(tg => tg.name === tag))
      const total = list.length
      const start = (page - 1) * size
      const items = list.slice(start, start + size).map(p => ({
        ...p,
        adb_enabled: adbState[p.id]?.enabled ?? false,
        rooted: rootState[p.id] ?? false,
      }))
      return ok({ list: items, total })
    },
  },
  // 须在 /v1/phone/:id 之前注册，否则 /phone/tags 会被当成 id=tags
  {
    url: '/v1/phone/tags',
    method: 'get',
    response: () => {
      const seen = new Map<string, TagRec>()
      for (const p of phones) {
        for (const tg of p.tags) {
          if (!seen.has(tg.name))
            seen.set(tg.name, { ...tg })
        }
      }
      return ok(Array.from(seen.values()))
    },
  },
  {
    url: '/v1/phone/tags',
    method: 'post',
    response: ({ body }) => {
      const payload = parseBody(body)
      const idSet = new Set(normalizeIds(payload.ids))
      const tags = (Array.isArray(payload.tags) ? payload.tags : []) as TagRec[]
      if (!idSet.size)
        return fail('请选择云手机')
      const normalized = tags
        .map(tg => ({ name: String(tg?.name ?? '').trim(), color: tg?.color || 'green' }))
        .filter(tg => tg.name)
      let hit = 0
      for (const p of phones) {
        if (idSet.has(p.id)) {
          p.tags = normalized.map(tg => ({ ...tg }))
          p.updated_at = now()
          hit++
        }
      }
      if (!hit)
        return fail('云手机不存在')
      return ok(null)
    },
  },
  {
    url: '/v1/phone/:id',
    method: 'get',
    response: ({ params }) => {
      const p = phones.find(x => x.id === Number(params.id))
      return p ? ok(p) : fail('云手机不存在')
    },
  },
  {
    // 远控真实开机时长：mock 返回一个已运行约 2 小时的运行中会话。
    url: '/v1/phone/:id/runtime',
    method: 'get',
    response: ({ params }) => {
      const p = phones.find(x => x.id === Number(params.id))
      if (!p) return fail('云手机不存在')
      const uptime = 7235 // ~2h
      const onAt = new Date(Date.now() - uptime * 1000).toISOString()
      return ok({ running: true, power_on_at: onAt, uptime_seconds: uptime })
    },
  },
  {
    url: '/v1/phone/create',
    method: 'post',
    response: ({ body }) => {
      if (!body?.name) return fail('请填写名称')
      const id = ++seq
      const rec: PhoneRec = {
        id,
        user_id: 1,
        cp_id: `CP-${id}`,
        name: body.name,
        status: 'CREATING',
        vm_id: `vm-${id}`,
        image_id: body.image_id || 'img-android13',
        proxy_id: Number(body.proxy_id) || 0,
        remark: body.remark || '',
        tags: [],
        created_at: now(),
        updated_at: now(),
      }
      phones.unshift(rec)
      // 模拟异步收敛：创建中 → 3s 后创建成功
      setTimeout(() => {
        const p = phones.find(x => x.id === id)
        if (p && p.status === 'CREATING')
          p.status = 'CREATED'
      }, 3000)
      return ok(rec)
    },
  },
  {
    url: '/v1/phone/update/:id',
    method: 'put',
    response: ({ params, body }) => {
      const p = phones.find(x => x.id === Number(params.id))
      if (!p) return fail('云手机不存在')
      if (body?.name !== undefined && body.name !== '') p.name = body.name
      if (body?.status !== undefined && body.status !== '') p.status = body.status
      if (body?.image_id !== undefined) p.image_id = body.image_id
      if (body?.proxy_id !== undefined) p.proxy_id = Number(body.proxy_id) || 0
      if (body?.remark !== undefined) p.remark = body.remark
      p.updated_at = now()
      return ok(p)
    },
  },
  {
    url: '/v1/phone/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = phones.findIndex(x => x.id === Number(params.id))
      if (idx === -1) return fail('云手机不存在')
      phones.splice(idx, 1)
      return ok(null)
    },
  },

  // ---- 实例操作 ----
  {
    url: '/v1/phone/:id/power',
    method: 'post',
    response: ({ params, body }) => {
      const p = phones.find(x => x.id === Number(params.id))
      if (!p) return fail('云手机不存在')
      if (body?.operation === '开机') {
        if (p.status !== 'CREATED' && p.status !== 'STOPPED') return fail('当前状态不可开机')
        p.status = 'STARTING'
        // 模拟异步收敛：开机中 → 3s 后运行中
        setTimeout(() => {
          const x = phones.find(y => y.id === p.id)
          if (x && x.status === 'STARTING') x.status = 'RUNNING'
        }, 3000)
      }
      else if (body?.operation === '关机') {
        if (p.status !== 'RUNNING') return fail('当前状态不可关机')
        p.status = 'STOPPED'
      }
      return ok(null)
    },
  },
  {
    url: '/v1/phone/:id/restart',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/reset',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/destroy',
    method: 'post',
    response: ({ params }) => {
      const idx = phones.findIndex(x => x.id === Number(params.id))
      if (idx !== -1) phones.splice(idx, 1)
      return ok(null)
    },
  },

  // ---- 远程控制（WebRTC）----
  {
    url: '/v1/phone/:id/webrtc-auth',
    method: 'post',
    response: ({ params }) => ok({
      cpId: `CP-${params.id}`,
      vmId: `vm-${params.id}`,
      zoneId: 'cn-hk-1',
      pushStreamUrl: `webrtc://push.example.com/live/vm-${params.id}`,
      signalUrl: `wss://signal.example.com/ws/vm-${params.id}`,
      authToken: `mock-token-${params.id}-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789`,
    }),
  },
  {
    url: '/v1/phone/:id/webrtc-state',
    method: 'get',
    response: () => ok({ in_webrtc: true }),
  },
  {
    url: '/v1/phone/:id/volume',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/rotate',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/shake',
    method: 'post',
    response: () => ok(null),
  },

  // ---- 应用管理 ----
  {
    url: '/v1/phone/:id/apps',
    method: 'get',
    response: () => ok([
      { id: 101, packageName: 'com.tencent.mm', version: '8.0.49', md5: 'a1b2c3', appName: '微信', iconPath: '', fileSize: 268435456 },
      { id: 102, packageName: 'com.taobao.taobao', version: '10.30.0', md5: 'd4e5f6', appName: '淘宝', iconPath: '', fileSize: 188743680 },
      { id: 103, packageName: 'com.android.chrome', version: '125.0', md5: '7g8h9i', appName: 'Chrome', iconPath: '', fileSize: 134217728 },
    ]),
  },
  {
    url: '/v1/phone/:id/apps/install',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/apps/uninstall',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/apps/start',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/apps/stop',
    method: 'post',
    response: () => ok(null),
  },
  {
    url: '/v1/phone/:id/apps/kill-all',
    method: 'post',
    response: () => ok(null),
  },

  // ---- Root 开关（演示用内存态：按 phone id 记 isRooted）----
  {
    url: '/v1/phone/:id/root',
    method: 'post',
    response: ({ params, body }) => {
      rootState[Number(params.id)] = body?.enable === true
      return ok(null)
    },
  },

  // ---- 自动化脚本最小闭环（演示：下发即返回 taskId；查询第二次起返回完成）----
  {
    url: '/v1/phone/:id/script/hello',
    method: 'post',
    response: () => {
      scriptPolls = 0
      return ok({ task_id: 9001, task_no: 'T-9001' })
    },
  },
  {
    url: '/v1/phone/:id/script/task/:taskId',
    method: 'get',
    response: ({ params }) => {
      scriptPolls++
      const done = scriptPolls >= 2 // 第一次执行中，之后完成
      return ok({
        task_id: Number(params.taskId),
        task_no: 'T-9001',
        status: done ? 'COMPLETED' : 'EXECUTING',
        status_desc: done ? '任务完成' : '正在执行',
        exec_result: done ? 1 : null,
        terminal: done,
        run_duration_ms: done ? 1234 : 0,
        run_log: done ? '[INFO] hello world from glory cloud phone\n#RESULT#{"msg":"hello world from glory","ok":true}#RESULT#' : '',
        result: done ? '{"msg":"hello world from glory","ok":true}' : '',
        screenshot_url: '',
      })
    },
  },

  // ---- ADB（演示用内存态：按 phone id 记 enabled + 白名单）----
  {
    url: '/v1/phone/:id/adb',
    method: 'get',
    response: ({ params }) => ok(adbInfo(Number(params.id))),
  },
  {
    url: '/v1/phone/:id/adb/enable',
    method: 'post',
    response: ({ params, body }) => {
      const id = Number(params.id)
      adbState[id] = { enabled: true, whiteIp: (body?.whiteIp as string[]) ?? [] }
      return ok(adbInfo(id))
    },
  },
  {
    url: '/v1/phone/:id/adb/disable',
    method: 'post',
    response: ({ params }) => {
      const id = Number(params.id)
      adbState[id] = { enabled: false, whiteIp: adbState[id]?.whiteIp ?? [] }
      return ok(null)
    },
  },
  {
    url: '/v1/phone/:id/adb/whitelist',
    method: 'get',
    response: ({ params }) => {
      const id = Number(params.id)
      const ips = adbState[id]?.whiteIp ?? []
      return ok(ips.map((ip, i) => ({
        id: i + 1,
        cpId: `CP-${1000 + id}`,
        vmId: `vm-${id}`,
        ipAddress: ip,
        status: 1,
        statusDesc: '生效',
        expired: false,
        createTime: now(),
      })))
    },
  },
  {
    url: '/v1/phone/:id/adb/whitelist',
    method: 'post',
    response: ({ params, body }) => {
      const id = Number(params.id)
      adbState[id] = { enabled: adbState[id]?.enabled ?? true, whiteIp: (body?.whiteIp as string[]) ?? [] }
      return ok(null)
    },
  },
])

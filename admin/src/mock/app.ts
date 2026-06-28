import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

type ParseStatus = 'parsing' | 'ready' | 'failed'

interface MarketRec {
  id: number
  app_name: string
  package_name: string
  version: string
  icon_url: string
  size_bytes: number
  parse_status: ParseStatus
  parse_error?: string
  created_at: string
}

interface OpsRec {
  file_id: number
  app_name: string
  package_name: string
  version: string
  size_bytes: number
  parse_status: ParseStatus
  parse_error?: string
  icon_url: string
  user_phone: string
  user_nickname: string
  created_at: string
}

// 内联图标（mock dev 下让图标列可见）：小尺寸 SVG data URI。
const ICON_GREEN = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="%2307c160"/><text x="16" y="22" font-size="16" text-anchor="middle" fill="white" font-family="sans-serif">微</text></svg>'
const ICON_ORANGE = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="%23ff5000"/><text x="16" y="22" font-size="16" text-anchor="middle" fill="white" font-family="sans-serif">淘</text></svg>'
const ICON_BLACK = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="%23161823"/><text x="16" y="22" font-size="14" text-anchor="middle" fill="white" font-family="sans-serif">抖</text></svg>'

// 内存应用市场表（admin 上传 → my 端「应用市场」可见）
const market: MarketRec[] = [
  { id: 201, app_name: '微信', package_name: 'com.tencent.mm', version: '8.0.49', icon_url: ICON_GREEN, size_bytes: 268435456, parse_status: 'ready', created_at: '2026-01-10 10:00:00' },
  { id: 202, app_name: '淘宝', package_name: 'com.taobao.taobao', version: '10.30.0', icon_url: ICON_ORANGE, size_bytes: 188743680, parse_status: 'ready', created_at: '2026-01-11 10:00:00' },
  { id: 203, app_name: 'Chrome', package_name: 'com.android.chrome', version: '125.0', icon_url: '', size_bytes: 134217728, parse_status: 'failed', parse_error: 'manifest 解析失败', created_at: '2026-01-12 10:00:00' },
]
let marketSeq = 300

// 内存「用户上传应用」表（跨用户治理视图）
const userApps: OpsRec[] = [
  { file_id: 1001, app_name: '抖音', package_name: 'com.ss.android.ugc.aweme', version: '28.5.0', size_bytes: 209715200, parse_status: 'ready', icon_url: ICON_BLACK, user_phone: '13800138000', user_nickname: '张三', created_at: '2026-02-01 09:00:00' },
  { file_id: 1002, app_name: '小红书', package_name: 'com.xingin.xhs', version: '8.2.0', size_bytes: 157286400, parse_status: 'parsing', icon_url: '', user_phone: '13900139000', user_nickname: '李四', created_at: '2026-02-02 09:00:00' },
]

export default defineFakeRoute([
  // ── 应用市场 ──
  {
    url: '/v1/admin/apps/market',
    method: 'get',
    response: () => ok(market),
  },
  {
    url: '/v1/admin/apps/market/upload',
    method: 'post',
    response: () => {
      const id = marketSeq++
      const rec: MarketRec = {
        id,
        app_name: `上传应用 ${id}`,
        package_name: `com.demo.app${id}`,
        version: '1.0.0',
        icon_url: '',
        size_bytes: 20971520,
        parse_status: 'ready',
        created_at: new Date().toISOString().slice(0, 19).replace('T', ' '),
      }
      market.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/admin/apps/market/batch-delete',
    method: 'post',
    response: ({ body }) => {
      const ids: number[] = body?.ids ?? []
      for (const id of ids) {
        const idx = market.findIndex(a => a.id === id)
        if (idx !== -1)
          market.splice(idx, 1)
      }
      return ok(null)
    },
  },

  // ── 用户上传应用治理 ──
  {
    url: '/v1/admin/apps',
    method: 'get',
    response: () => ok(userApps),
  },
  {
    url: '/v1/admin/apps/batch-delete',
    method: 'post',
    response: ({ body }) => {
      const fileIds: number[] = body?.file_ids ?? []
      for (const id of fileIds) {
        const idx = userApps.findIndex(a => a.file_id === id)
        if (idx !== -1)
          userApps.splice(idx, 1)
      }
      return ok(null)
    },
  },
])

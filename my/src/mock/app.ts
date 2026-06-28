import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

// 内联图标（mock dev 下让图标列可见）：小尺寸 SVG data URI。
const ICON_LEDGER = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="%232563eb"/><text x="16" y="22" font-size="16" text-anchor="middle" fill="white" font-family="sans-serif">账</text></svg>'
const ICON_BETA = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="%2316a34a"/><text x="16" y="21" font-size="11" text-anchor="middle" fill="white" font-family="sans-serif">β</text></svg>'

// 我的应用（素材库 app 文件 + app_user_meta；含解析中/就绪/失败）。
const mine = [
  { file_id: 1, app_name: '我的记账本', package_name: 'com.demo.ledger', version: '1.2.0', icon_url: ICON_LEDGER, size_bytes: 19293798, parse_status: 'ready', created_at: '2026-06-01T09:12:00Z' },
  { file_id: 2, app_name: '内测 Demo', package_name: 'com.demo.beta', version: '0.9.1', icon_url: ICON_BETA, size_bytes: 54525952, parse_status: 'ready', created_at: '2026-06-05T14:30:00Z' },
  { file_id: 3, app_name: '坏包样例', package_name: '', version: '', icon_url: '', size_bytes: 1024, parse_status: 'failed', parse_error: 'manifest 缺失', created_at: '2026-06-06T10:00:00Z' },
]

// 应用市场（平台资产，公有桶）。
const market = [
  { id: 201, app_name: '微信', package_name: 'com.tencent.mm', version: '8.0.49', icon_url: '', size_bytes: 268435456, parse_status: 'ready', created_at: '2026-01-10T10:00:00Z' },
  { id: 202, app_name: '淘宝', package_name: 'com.taobao.taobao', version: '10.30.0', icon_url: '', size_bytes: 188743680, parse_status: 'ready', created_at: '2026-01-11T10:00:00Z' },
  { id: 203, app_name: 'Chrome', package_name: 'com.android.chrome', version: '125.0', icon_url: '', size_bytes: 134217728, parse_status: 'ready', created_at: '2026-01-12T10:00:00Z' },
  { id: 204, app_name: '抖音', package_name: 'com.ss.android.ugc.aweme', version: '27.5.0', icon_url: '', size_bytes: 230686720, parse_status: 'ready', created_at: '2026-01-13T10:00:00Z' },
]

export default defineFakeRoute([
  {
    url: '/v1/app/user',
    method: 'get',
    response: () => ok(mine),
  },
  {
    url: '/v1/app/market',
    method: 'get',
    response: () => ok(market),
  },
  {
    url: '/v1/phone/apps/install-by-url',
    method: 'post',
    response: ({ body }: { body: { phone_ids?: number[] } }) =>
      ok({
        task_info_list: (body?.phone_ids ?? [0]).map((_, i) => ({
          task_id: `mock-task-${i + 1}`,
          instance_id: `mock-instance-${i + 1}`,
        })),
      }),
  },
])

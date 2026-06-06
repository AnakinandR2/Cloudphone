import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

// 我的应用（我上传的，含创建中/正常状态）
const mine = [
  { id: 1, cpAppId: 9001, appMd5: 'mine-md5-1', appName: '我的记账本', packageName: 'com.demo.ledger', version: '1.2.0', fileSize: '18.4 MB', iconPath: '', status: 'NORMAL', createTime: '2026-06-01 09:12:00' },
  { id: 2, cpAppId: 9002, appMd5: 'mine-md5-2', appName: '内测 Demo', packageName: 'com.demo.beta', version: '0.9.1', fileSize: '52.0 MB', iconPath: '', status: 'CREATING', createTime: '2026-06-05 14:30:00' },
]

// 应用市场（admin 上传的「应用商店」应用；结构与「我的应用」一致，含 cpAppId）
const market = [
  { id: 201, cpAppId: 5101, appMd5: 'mk-201', appName: '微信', packageName: 'com.tencent.mm', version: '8.0.49', fileSize: '256MB', iconPath: '', status: 'NORMAL', createTime: '2026-01-10 10:00:00' },
  { id: 202, cpAppId: 5102, appMd5: 'mk-202', appName: '淘宝', packageName: 'com.taobao.taobao', version: '10.30.0', fileSize: '180MB', iconPath: '', status: 'NORMAL', createTime: '2026-01-11 10:00:00' },
  { id: 203, cpAppId: 5103, appMd5: 'mk-203', appName: 'Chrome', packageName: 'com.android.chrome', version: '125.0', fileSize: '128MB', iconPath: '', status: 'NORMAL', createTime: '2026-01-12 10:00:00' },
  { id: 204, cpAppId: 5104, appMd5: 'mk-204', appName: '抖音', packageName: 'com.ss.android.ugc.aweme', version: '27.5.0', fileSize: '220MB', iconPath: '', status: 'NORMAL', createTime: '2026-01-13 10:00:00' },
]

export default defineFakeRoute([
  {
    url: '/v1/app/list',
    method: 'get',
    response: () => ok(mine),
  },
  {
    url: '/v1/app/market',
    method: 'get',
    response: () => ok(market),
  },
])

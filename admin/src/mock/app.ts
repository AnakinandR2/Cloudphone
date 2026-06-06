import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

interface StoreRec {
  id: number
  cpAppId: number
  appMd5: string
  appName: string
  packageName: string
  version: string
  fileSize: string
  iconPath: string
  status: 'CREATING' | 'NORMAL'
  store: boolean
  createTime: string
}

// 内存应用商店表（admin 上传 → my 端「应用市场」可见）
const store: StoreRec[] = [
  { id: 201, cpAppId: 5101, appMd5: 'mk-201', appName: '微信', packageName: 'com.tencent.mm', version: '8.0.49', fileSize: '256MB', iconPath: '', status: 'NORMAL', store: true, createTime: '2026-01-10 10:00:00' },
  { id: 202, cpAppId: 5102, appMd5: 'mk-202', appName: '淘宝', packageName: 'com.taobao.taobao', version: '10.30.0', fileSize: '180MB', iconPath: '', status: 'NORMAL', store: true, createTime: '2026-01-11 10:00:00' },
  { id: 203, cpAppId: 5103, appMd5: 'mk-203', appName: 'Chrome', packageName: 'com.android.chrome', version: '125.0', fileSize: '128MB', iconPath: '', status: 'NORMAL', store: true, createTime: '2026-01-12 10:00:00' },
]
let seq = 300

export default defineFakeRoute([
  {
    url: '/v1/admin/apps/store',
    method: 'get',
    response: () => ok(store),
  },
  {
    url: '/v1/admin/apps/store/upload',
    method: 'post',
    response: () => {
      const id = seq++
      const rec: StoreRec = {
        id,
        cpAppId: 5000 + id,
        appMd5: `mk-${id}`,
        appName: `上传应用 ${id}`,
        packageName: `com.demo.app${id}`,
        version: '1.0.0',
        fileSize: '20MB',
        iconPath: '',
        status: 'NORMAL',
        store: true,
        createTime: new Date().toISOString().slice(0, 19).replace('T', ' '),
      }
      store.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/admin/apps/store/batch-delete',
    method: 'post',
    response: ({ body }) => {
      const ids: number[] = body?.ids ?? []
      for (const id of ids) {
        const idx = store.findIndex(a => a.id === id)
        if (idx !== -1)
          store.splice(idx, 1)
      }
      return ok(null)
    },
  },
])

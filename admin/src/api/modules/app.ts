import type { AdminAppItem, StoreAppItem } from '@/types/app'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 运营侧应用管理（跨用户，需 staff 权限 app:view / app:manage）
export default {
  list: () => api.get<unknown, R<AdminAppItem[]>>('admin/apps'),
  batchDelete: (ids: number[]) =>
    api.post<unknown, R<null>>('admin/apps/batch-delete', { ids }),

  // ---- 应用商店（admin 上传的应用即「应用市场」内容；my 端读 /app/market）----
  storeList: () => api.get<unknown, R<StoreAppItem[]>>('admin/apps/store'),

  // 上传 APK：multipart（后端落临时文件后整链上传到中台）。不手动设 Content-Type，由 axios 补 boundary。
  storeUpload: (file: File, appName?: string, appDesc?: string, onProgress?: (percent: number) => void) => {
    const fd = new FormData()
    fd.append('file', file)
    if (appName)
      fd.append('appName', appName)
    if (appDesc)
      fd.append('appDesc', appDesc)
    return api.post<unknown, R<StoreAppItem>>('admin/apps/store/upload', fd, {
      timeout: 300000,
      onUploadProgress: (e) => {
        if (onProgress && e.total)
          onProgress(Math.round((e.loaded / e.total) * 100))
      },
    })
  },

  storeBatchDelete: (ids: number[]) =>
    api.post<unknown, R<null>>('admin/apps/store/batch-delete', { ids }),
}

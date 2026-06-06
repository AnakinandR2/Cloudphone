import type { AppItem, MarketApp } from '@/types/app'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 应用管理（「我上传的应用」，按用户隔离，需登录）
export default {
  // 返回当前用户上传的应用（含创建中/正常状态）
  list: () => api.get<unknown, R<AppItem[]>>('app/list'),

  // 应用市场：admin 上传的「应用商店」应用，供浏览可安装的应用
  market: () => api.get<unknown, R<MarketApp[]>>('app/market'),

  // 上传 APK：multipart（后端落临时文件后整链上传到中台）。不手动设 Content-Type，由 axios 补 boundary。
  upload: (file: File, appName?: string, appDesc?: string, onProgress?: (percent: number) => void) => {
    const fd = new FormData()
    fd.append('file', file)
    if (appName)
      fd.append('appName', appName)
    if (appDesc)
      fd.append('appDesc', appDesc)
    return api.post<unknown, R<AppItem>>('app/upload', fd, {
      timeout: 300000,
      onUploadProgress: (e) => {
        if (onProgress && e.total)
          onProgress(Math.round((e.loaded / e.total) * 100))
      },
    })
  },

  // 批量删除（传本地绑定 id）
  batchDelete: (ids: number[]) =>
    api.post<unknown, R<null>>('app/batch-delete', { ids }),
}

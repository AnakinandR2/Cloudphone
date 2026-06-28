import type { MarketApp, OpsUserApp } from '@/types/app'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 运营侧应用管理（跨用户，需 staff 权限 app:view / app:manage）
export default {
  // ---- 应用市场（admin 维护的平台资产，存公有桶，自解析）----
  marketList: () => api.get<unknown, R<MarketApp[]>>('admin/apps/market'),

  // 上传 APK/XAPK：multipart（后端落临时文件 → 解析 → 存公有桶）。字段名 'file'，不手动设 Content-Type。
  marketUpload: (file: File, onProgress?: (percent: number) => void) => {
    const fd = new FormData()
    fd.append('file', file)
    return api.post<unknown, R<MarketApp>>('admin/apps/market/upload', fd, {
      timeout: 300000,
      onUploadProgress: (e) => {
        if (onProgress && e.total)
          onProgress(Math.round((e.loaded / e.total) * 100))
      },
    })
  },

  marketBatchDelete: (ids: number[]) =>
    api.post<unknown, R<null>>('admin/apps/market/batch-delete', { ids }),

  // ---- 用户应用治理（跨用户：素材库 app 文件 + 上传者 + 解析元数据）----
  opsList: () => api.get<unknown, R<OpsUserApp[]>>('admin/apps'),
  opsBatchDelete: (fileIds: number[]) =>
    api.post<unknown, R<null>>('admin/apps/batch-delete', { file_ids: fileIds }),
}

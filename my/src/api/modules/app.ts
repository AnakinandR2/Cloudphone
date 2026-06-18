import type { AppItem, MarketApp, ParsedAppInfo, UploadInitiateResp, UploadStatus } from '@/types/app'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 分片上传相关接口超时（APK 较大、合并/解析耗时）。
const UPLOAD_TIMEOUT = 5 * 60 * 1000

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

  // ── 浏览器驱动的分片上传流水线（参考 mcn）：initiate → part → complete → parse → create → status ──

  // §1.3 启动分片上传会话；命中秒传时返回 uploadSuccess=true + appInfo（可跳过分片直接创建）。
  uploadInitiate: (data: { fileName: string, fileSize: number, contentMd5: string }) =>
    api.post<unknown, R<UploadInitiateResp>>('app/upload/initiate', data, { timeout: UPLOAD_TIMEOUT }),

  // §1.4 上传单个分片：uploadId/partNumber/contentMd5 走 query，二进制走 form 字段 file。
  uploadPart: (
    fd: FormData,
    params: { uploadId: string, partNumber: number, contentMd5: string },
    onProgress?: (percent: number) => void,
  ) =>
    api.post<unknown, R<unknown>>('app/upload/part', fd, {
      params,
      timeout: UPLOAD_TIMEOUT,
      onUploadProgress: (e) => {
        if (onProgress && e.total)
          onProgress(Math.round((e.loaded / e.total) * 100))
      },
    }),

  // §1.5 完成分片合并。
  uploadComplete: (uploadId: string) =>
    api.post<unknown, R<{ downloadUrl: string }>>('app/upload/complete', { uploadId }, { timeout: UPLOAD_TIMEOUT }),

  // §1.6 解析已上传 APK 元信息。
  uploadParse: (uploadId: string) =>
    api.post<unknown, R<ParsedAppInfo>>('app/upload/parse', { uploadId }, { timeout: UPLOAD_TIMEOUT }),

  // §1.8 由已上传文件创建应用并落本地绑定（秒传时 uploadId 传空串）。
  uploadCreate: (data: {
    uploadId: string
    appName: string
    packageName?: string
    version?: string
    iconPath?: string
    fileSize?: string
    md5?: string
  }) => api.post<unknown, R<AppItem>>('app/upload/create', data, { timeout: UPLOAD_TIMEOUT }),

  // §1.9 查询上传任务处理状态（OSS_UPLOADING / OSS_SUCCESS / OSS_FAILED）。
  uploadStatus: (uploadId: string) =>
    api.get<unknown, R<{ status: UploadStatus }>>('app/upload/status', { params: { uploadId } }),

  // 批量删除（传本地绑定 id）
  batchDelete: (ids: number[]) =>
    api.post<unknown, R<null>>('app/batch-delete', { ids }),
}

import type {
  ConfirmUploadReq,
  FileListQuery,
  LibraryFile,
  LibraryFolder,
  LibraryOrder,
  LibraryOverview,
  LibraryTag,
  OrderCreateResult,
  PackageOrderCreateReq,
  PackageParams,
  PackageQuoteResult,
  PresignUploadReq,
  PresignUploadResult,
  UpdateFileReq,
} from '@/types/library'
import axios from 'axios'
import { hashFile } from '@/utils/filehash'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

// 直传 S3 用裸 axios（不经项目 api 实例，避免注入 Authorization 头污染预签名）。
const rawHttp = axios.create({ timeout: 1000 * 60 * 10 })

const libraryApi = {
  // ===== 概览 =====
  // GET /library/overview — 用量 + 订阅 + 档位目录 + 免费额度 + 时长选项 + notice。
  overview: () => api.get<unknown, R<LibraryOverview>>('library/overview'),

  // ===== 上传（两段式 presigned 直传） =====
  // 1) 申请预签名 PUT 地址 + 占位 file_id（含配额预占）。
  presign: (req: PresignUploadReq) =>
    api.post<unknown, R<PresignUploadResult>>('library/upload/presign', req),
  // 2) 浏览器用返回的 url 直传 S3（PUT，带 Content-Type；不带项目 auth 头）。
  putToS3: (url: string, file: File | Blob, contentType: string, onProgress?: (pct: number) => void) =>
    rawHttp.put(url, file, {
      headers: { 'Content-Type': contentType || 'application/octet-stream' },
      onUploadProgress: (e) => {
        if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
      },
    }),
  // 3) 确认上传（HeadObject 校验 + 修正真实大小 + 置 active + 累加用量 + 可选绑标签）。
  confirm: (req: ConfirmUploadReq) =>
    api.post<unknown, R<LibraryFile>>('library/upload/confirm', req),

  // ===== 文件 =====
  listFiles: (params: FileListQuery) =>
    api.get<unknown, R<Page<LibraryFile>>>('library/files', { params }),
  // 下载：返回 presigned GET（后端做超额锁定校验）。
  downloadUrl: (id: number) =>
    api.get<unknown, R<{ url: string }>>(`library/files/${id}/download`),
  updateFile: (id: number, req: UpdateFileReq) =>
    api.put<unknown, R<LibraryFile>>(`library/files/${id}`, req),
  deleteFile: (id: number) => api.delete<unknown, R<null>>(`library/files/${id}`),
  getFileTags: (id: number) =>
    api.get<unknown, R<{ tag_ids: number[] }>>(`library/files/${id}/tags`),
  setFileTags: (id: number, tag_ids: number[]) =>
    api.put<unknown, R<null>>(`library/files/${id}/tags`, { tag_ids }),

  // ===== 文件夹 =====
  listFolders: () => api.get<unknown, R<LibraryFolder[]>>('library/folders'),
  createFolder: (name: string, parent_id = 0) =>
    api.post<unknown, R<LibraryFolder>>('library/folders', { name, parent_id }),
  renameFolder: (id: number, name: string) =>
    api.put<unknown, R<LibraryFolder>>(`library/folders/${id}`, { name }),
  deleteFolder: (id: number) => api.delete<unknown, R<null>>(`library/folders/${id}`),
  moveFolder: (id: number, parent_id: number) =>
    api.post<unknown, R<LibraryFolder>>(`library/folders/${id}/move`, { parent_id }),

  // ===== 标签 =====
  listTags: () => api.get<unknown, R<LibraryTag[]>>('library/tags'),
  createTag: (name: string, color: string) =>
    api.post<unknown, R<LibraryTag>>('library/tags', { name, color }),
  updateTag: (id: number, req: { name?: string, color?: string }) =>
    api.put<unknown, R<LibraryTag>>(`library/tags/${id}`, req),
  deleteTag: (id: number) => api.delete<unknown, R<null>>(`library/tags/${id}`),

  // ===== 套餐购买 =====
  // POST /library/package/quote — rich 预览（折后价/补差价明细/新到期预览）。
  // params 直接作为 body（后端 QuotePackage 读 json.RawMessage）。
  quotePackage: (params: PackageParams) =>
    api.post<unknown, R<PackageQuoteResult>>('library/package/quote', params),
  // 下单/支付直接打 billing（biz_type=lib_* + params）。
  createPackageOrder: (req: PackageOrderCreateReq) =>
    api.post<unknown, R<OrderCreateResult>>('billing/orders', req),
  payOrder: (id: number) =>
    api.post<unknown, R<OrderCreateResult>>(`billing/orders/${id}/pay`),
  // 套餐订单历史：billing 订单列表按 biz_type 过滤（后端支持 biz_type 过滤参数）。
  packageOrders: (params: { page?: number, size?: number, biz_type?: string }) =>
    api.get<unknown, R<Page<LibraryOrder>>>('billing/orders', { params }),
}

// ===== 共享上传流程（含全局去重 / 秒传） =====
// hash（全量 md5 + slice_md5）→ presign → instant 命中则跳过 PUT/confirm，
// 否则照常 PUT 直传 S3 → confirm。两处上传入口（素材 / 应用）复用。
export interface UploadResult {
  fileId: number
  instant: boolean // true=秒传命中（已 active，无需 PUT/confirm）
}

export interface UploadOptions {
  folderId?: number
  tagIds?: number[]
  // 计算校验值进度回调（流式 hash 阶段，pct 仅在上传阶段语义化，这里给个开始信号）。
  onHashing?: () => void
  // S3 直传进度（0-100）。
  onProgress?: (pct: number) => void
}

export async function uploadLibraryFile(file: File, opts: UploadOptions = {}): Promise<UploadResult> {
  // 1) 流式算 md5（全量）+ slice_md5（前 256KB）+ size。
  opts.onHashing?.()
  const { md5, sliceMd5, size } = await hashFile(file)
  // 2) presign（含配额预占 + 秒传查重）。
  const pre = await libraryApi.presign({
    name: file.name,
    size_bytes: size,
    mime: file.type || 'application/octet-stream',
    folder_id: opts.folderId ?? 0,
    md5,
    slice_md5: sliceMd5,
  })
  // 3) 秒传命中：后端已建立引用并置 active，跳过 PUT/confirm。
  if (pre.data.instant) {
    return { fileId: pre.data.file_id, instant: true }
  }
  // 4) 直传 S3 → confirm。
  await libraryApi.putToS3(pre.data.upload_url!, file, file.type, opts.onProgress)
  await libraryApi.confirm({ file_id: pre.data.file_id, tag_ids: opts.tagIds })
  return { fileId: pre.data.file_id, instant: false }
}

export default libraryApi

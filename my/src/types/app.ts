/** 应用本地状态：创建中（中台异步处理）/ 正常（已就绪） */
export type AppStatus = 'CREATING' | 'NORMAL'

/** 「我上传的应用」一条（后端本地绑定 + 中台同步后的展示字段） */
export interface AppItem {
  /** 本地绑定 id（删除按它） */
  id: number
  /** 中台 app_info 主键 */
  cpAppId: number
  appMd5?: string
  appName: string
  packageName: string
  version: string
  /** 已格式化字符串，如 "39.53 MB" */
  fileSize: string
  iconPath: string
  status: AppStatus
  createTime: string
  updateTime?: string
}

/**
 * 应用市场一条：由 admin 上传、面向全部用户的「应用商店」应用。
 * 后端 /app/market 返回与「我的应用」相同的本地绑定结构（含 cpAppId，安装时用它）。
 */
export type MarketApp = AppItem

// ── 分片上传流水线（参考 mcn） ──

/** 秒传命中 / 解析返回的应用元信息（确认面板展示用）。 */
export interface UploadAppInfo {
  appName: string
  packageName: string
  version: string
  iconPath: string
  fileSize: string
  md5: string
}

/** §1.3 initiate 响应：协商分片参数，或命中秒传（uploadSuccess + appInfo）。 */
export interface UploadInitiateResp {
  /** 上传会话 id（字符串透传；秒传命中时为空串）。 */
  uploadId: string
  partSize: number
  totalParts: number
  uploadSuccess: boolean
  appInfo?: UploadAppInfo
}

/** §1.6 parse 响应：解析出的 APK 元信息。 */
export interface ParsedAppInfo {
  uploadId: string
  appName: string
  packageName: string
  version: string
  iconPath: string
  fileSize: string
  md5: string
}

/** §1.9 上传任务处理状态。 */
export type UploadStatus = 'OSS_UPLOADING' | 'OSS_SUCCESS' | 'OSS_FAILED' | ''

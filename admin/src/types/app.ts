/** 应用本地状态：创建中（中台异步处理）/ 正常（已就绪） */
export type AppStatus = 'CREATING' | 'NORMAL'

/** 应用商店一条（admin 上传、面向全部用户；本地绑定 + 中台同步字段） */
export interface StoreAppItem {
  /** 本地绑定 id（删除按它） */
  id: number
  /** 中台 app_info 主键 */
  cpAppId: number
  appMd5?: string
  appName: string
  packageName: string
  version: string
  fileSize: string
  iconPath: string
  status: AppStatus
  store?: boolean
  createTime: string
  updateTime?: string
}

/** 运营视角的「用户上传应用」一条（本地绑定 + 所属用户信息） */
export interface AdminAppItem {
  id: number
  cpAppId: number
  appMd5?: string
  appName: string
  packageName: string
  version: string
  fileSize: string
  iconPath: string
  status: AppStatus
  createTime: string
  userPhone: string
  userNickname: string
}

/** 应用解析状态：解析中 / 就绪 / 失败（服务端自解析 APK/XAPK 元数据） */
export type ParseStatus = 'parsing' | 'ready' | 'failed'

/** 应用市场一条（admin 上传、面向全部用户的平台资产，存公有桶，不计配额） */
export interface MarketApp {
  /** 市场应用主键（删除按它） */
  id: number
  app_name: string
  package_name: string
  version: string
  /** 公有桶图标 URL */
  icon_url: string
  /** 文件大小（字节） */
  size_bytes: number
  /** 解析状态 */
  parse_status: ParseStatus
  /** 解析失败原因 */
  parse_error?: string
  created_at: string
}

/** 运营视角的「用户上传应用」一条（素材库 app 文件 + 上传者信息 + 解析元数据） */
export interface OpsUserApp {
  /** 素材库文件 id（删除按它） */
  file_id: number
  app_name: string
  package_name: string
  version: string
  /** 文件大小（字节） */
  size_bytes: number
  /** 解析状态 */
  parse_status: ParseStatus
  /** 解析失败原因 */
  parse_error?: string
  /** 公有桶图标 URL */
  icon_url: string
  /** 上传者手机号 */
  user_phone: string
  /** 上传者昵称 */
  user_nickname: string
  created_at: string
}

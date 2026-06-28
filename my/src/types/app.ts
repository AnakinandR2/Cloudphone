// 应用解析状态（服务端 apkparse）：解析中 / 就绪 / 失败。
export type ParseStatus = 'parsing' | 'ready' | 'failed'

/**
 * 「我的应用」一条 = 素材库 app 类型文件 + app_user_meta（LEFT JOIN）。
 * 删除按 file_id（走素材库删除，释放配额）。
 */
export interface UserApp {
  /** = library_files.id（删除/安装均按它） */
  file_id: number
  app_name: string
  package_name: string
  version: string
  icon_url: string
  size_bytes: number
  parse_status: ParseStatus
  /** 解析失败原因（parse_status=failed 时） */
  parse_error?: string
  created_at: string
}

/** 应用市场一条 = 平台资产（公有桶，不计配额）。 */
export interface MarketApp {
  id: number
  app_name: string
  package_name: string
  version: string
  icon_url: string
  size_bytes: number
  parse_status: ParseStatus
  created_at: string
}

/** 安装载荷引用：来源 + 主键（user=library_file_id / market=app_market.id）。 */
export interface AppRef {
  source: 'user' | 'market'
  id: number
}

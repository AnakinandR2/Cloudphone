// 素材库（网盘 + 容量套餐）前端类型。
// 契约源：backend/modules/library/internal/{overview.go, files.go, folders.go, tags.go,
//   pricingconfig_model.go, service.go}、billing 的 bizorder_model.go（params）。
// 金额一律 cents（分）；时间一律 RFC3339 字符串；bps：10000=原价无折扣，9500=9.5折。

// ---- 文件类型枚举（后端 classifyFileType） ----
export type FileType = 'image' | 'video' | 'audio' | 'document' | 'app' | 'other'
export type FileStatus = 'uploading' | 'active' | 'deleted'

// ---- 套餐动作（后端 ActionXxx / biz_type lib_*） ----
export type PackageAction = 'new' | 'renew' | 'upgrade' | 'downgrade'
export type LibraryBizType = 'lib_new' | 'lib_renew' | 'lib_upgrade' | 'lib_downgrade'

// ---- 定价配置档位（pricingconfig_model.go） ----
export interface TierCfg {
  code: string // t50 / t100 / t500
  name: string
  capacity_gb: number
  monthly_price_cents: number
  tier_discount_bps: number // 10000=原价
  enabled: boolean
  sort: number
}
export interface CustomTierCfg {
  enabled: boolean
  min_capacity_gb: number
  price_per_gb_month_cents: number
  tier_discount_bps: number
}
export interface DurationOptCfg {
  days: number
  discount_bps: number
}

// ---- GET /library/overview ----
export interface OverviewUsage {
  used_bytes: number
  capacity_bytes: number
  locked: boolean // used_bytes > capacity_bytes → 超额锁定
}
export interface OverviewSubscription {
  tier_code: string // free 或档位码
  capacity_bytes: number
  monthly_price_cents: number
  expire_at: string | null // 免费=null 永不过期
  is_free: boolean
}
export interface LibraryOverview {
  usage: OverviewUsage
  subscription: OverviewSubscription
  free_quota_bytes: number
  tiers: TierCfg[]
  custom_tier: CustomTierCfg
  duration_options: DurationOptCfg[]
  notice: string
  billing_note: string
}

// ---- 文件 ----
export interface LibraryFile {
  id: number
  folder_id: number // 0=根
  name: string
  ext: string
  mime_type: string
  file_type: FileType
  size_bytes: number
  status: FileStatus
  md5: string
  created_at: string
  updated_at: string
  // ListFiles 随单返回标签 ID 列表（FileDTO）。
  tag_ids?: number[]
}

// ---- 文件夹 ----
export interface LibraryFolder {
  id: number
  parent_id: number // 0=根
  name: string
  created_at: string
  updated_at: string
}

// ---- 标签 ----
export interface LibraryTag {
  id: number
  name: string
  color: string
  created_at: string
}

// ---- 上传两段式 ----
export interface PresignUploadReq {
  name: string
  size_bytes: number
  mime: string
  folder_id: number
  md5: string // 全量 md5（全局去重键之一）
  slice_md5: string // 前 256KB 的 md5（秒传持有证明）
}
export interface PresignUploadResult {
  file_id: number
  // 秒传命中（instant=true）时后端不返回直传地址，前端跳过 PUT/confirm。
  upload_url?: string
  s3_key?: string
  expires_in?: number // 秒
  instant: boolean // true=秒传命中，文件已 active
}
export interface ConfirmUploadReq {
  file_id: number
  tag_ids?: number[]
}

// ---- 改名 / 移动 ----
export interface UpdateFileReq {
  name?: string
  folder_id?: number
}

// ---- 文件列表查询 ----
export interface FileListQuery {
  folder_id?: number
  file_type?: FileType | ''
  tag_id?: number
  keyword?: string
  page?: number
  size?: number
}

// ---- 套餐报价（POST /library/package/quote） ----
// 请求载荷（service.go PackageParams）：预设档带 tier_code；自定义档带 capacity_gb；
// upgrade/downgrade 不带 days。action 可留空，由服务端按月价自动判定。
export interface PackageParams {
  action?: PackageAction | ''
  tier_code?: string
  capacity_gb?: number
  days?: number
}
// 响应（service.go QuoteResult）。
export interface PackageQuoteResult {
  action: PackageAction
  tier_code: string
  tier_name: string
  capacity_bytes: number
  days: number
  monthly_price_cents: number
  tier_discount_bps: number
  duration_discount_bps: number
  remaining_days: number
  total_cents: number
  new_expire_at: string | null
}

// ---- 下单 / 支付（直接打 billing，携带 params） ----
export type OrderStatus = 'unpaid' | 'paid' | 'expired'
export interface PackageOrderCreateReq {
  biz_type: LibraryBizType
  pay_method: string
  params: PackageParams
}
export interface LibraryOrder {
  id: number
  biz_type: string
  status: OrderStatus
  total_cents: number
  fee_cents: number
  pay_method: string
  created_at: string
  paid_at: string | null
}
export interface PayInfo { status: OrderStatus, [k: string]: unknown }
export interface OrderCreateResult { order: LibraryOrder, pay: PayInfo }

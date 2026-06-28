// ───────────────────────────────────────────────────────────────────────────
// 素材库定价配置（admin）— 对应 backend modules/library/internal/pricingconfig_model.go
// 金额单位 cents（分）；容量单位 bytes（字节，1 GiB = 1024^3）；bps 折扣：10000=全价。
// ───────────────────────────────────────────────────────────────────────────

/** 1 GiB 字节数（与后端 GiB 常量一致：1024^3） */
export const GIB = 1024 * 1024 * 1024

/** 预设容量档位（后端 TierCfg） */
export interface TierCfg {
  code: string
  name: string
  capacity_gb: number
  monthly_price_cents: number
  /** 档位折扣基点，10000=原价 */
  tier_discount_bps: number
  enabled: boolean
  sort: number
}

/** 自定义档（按 GiB 计价，后端 CustomTierCfg） */
export interface CustomTierCfg {
  enabled: boolean
  min_capacity_gb: number
  price_per_gb_month_cents: number
  tier_discount_bps: number
}

/** 时长选项（天 + 折扣，后端 DurationOptCfg） */
export interface DurationOptCfg {
  days: number
  discount_bps: number
}

/** GET/PUT /admin/library/pricing 整体形状（后端 LibraryPricingConfigData） */
export interface LibraryPricingConfig {
  /** 免费额度（字节）；前端以 GiB 编辑 */
  free_quota_bytes: number
  tiers: TierCfg[]
  custom_tier: CustomTierCfg
  duration_options: DurationOptCfg[]
  notice: string
  billing_note: string
}

// ── 后台查/调单个用户用量与订阅（gap-7，可选）─────────────────────────────

/** 素材库用量（后端 OverviewUsage） */
export interface LibraryUsage {
  used_bytes: number
  capacity_bytes: number
  locked: boolean
}

/** 素材库订阅（后端 OverviewSubscription） */
export interface LibrarySubscriptionView {
  tier_code: string
  capacity_bytes: number
  monthly_price_cents: number
  expire_at: string | null
  is_free: boolean
}

/** GET /admin/library/users/:userId（后端 AdminUserView） */
export interface AdminLibraryUserView {
  user_id: number
  usage: LibraryUsage
  subscription: LibrarySubscriptionView
}

/** POST /admin/library/users/:userId/grant（后端 AdminGrantRequest） */
export interface AdminLibraryGrantRequest {
  /** 预设档码；free 或留空且 capacity_gb<=0 → 重置免费态 */
  tier_code?: string
  /** 直给容量（GiB）；>0 时优先生效 */
  capacity_gb?: number
  /** >0 设到期为 now+days；<=0 永不过期 */
  days?: number
}

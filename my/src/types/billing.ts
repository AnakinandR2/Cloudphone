// 账户
export interface BillingAccount {
  id: number
  user_id: number
  balance_cents: number
  created_at: string
  updated_at: string
}

// 流水
export interface LedgerEntry {
  id: number
  user_id: number
  subject: string // balance/instance_seat/boot_seat/runtime_minute
  type: string // topup/consume/adjust_grant/adjust_deduct/purchase/trial
  delta: number
  balance_after: number
  reason: string
  order_id: number
  operator: string
  created_at: string
}

// ============================================================================
// 购买与费用重构（2026-06-21）— 新模型类型
// 契约：docs/superpowers/specs/2026-06-21-购买与费用重构-接口契约.md §1
// 金额一律 cents（分）；时间一律 RFC3339 字符串；bps：10000=全价，8500=8.5折。
// kind ∈ {seat, boot_slot}；biz_type ∈ {recharge, seat_new, seat_renew,
//   boot_slot_new, boot_slot_renew, runtime_pack}。
// ============================================================================

export type LicenseKind = 'seat' | 'boot_slot'
export type BizType = 'recharge' | 'seat_new' | 'seat_renew' | 'boot_slot_new' | 'boot_slot_renew' | 'runtime_pack'
export type OrderStatus2 = 'unpaid' | 'paid' | 'expired'
export type QuotaType = 'boot_slot' | 'temp' | 'capped_free' | 'mixed'

/** GET /billing/overview — KPI 概览 */
export interface BillingOverview {
  balance_cents: number
  seat: { total: number, used: number }
  boot_slot: { total: number, in_use: number }
  runtime_minutes_remaining: number
}

/** GET /billing/purchase-config — 渲染拉条/按钮组/须知/支付方式 */
export interface PaymentMethod {
  code: string // wechat/alipay/balance
  name: string
  enabled: boolean
  sort: number
  /** 比例手续费，基点(200=2%)；0=无该项；余额方式恒为 0 */
  fee_percent_bps: number
  /** 固定手续费，分(100=¥1)；0=无该项；余额方式恒为 0 */
  fee_fixed_cents: number
  /** 渠道 logo URL（上传或手填）；空=无，前端回退首字方块 */
  logo_url: string
  /** 订单(加费前)金额≥此值免手续费，分；0=永不免；余额方式恒为 0 */
  fee_free_threshold_cents: number
}
export interface QtyTier {
  min_quantity: number
  discount_bps: number
}
export interface DurationOption {
  value: number
  discount_bps: number
}
export interface KindConfig {
  unit_price_cents: number
  unit_label: string
  qty_tiers: QtyTier[]
  duration_unit: 'month' | 'day'
  duration_options: DurationOption[]
  notice: string
  billing_note: string
}
export interface RuntimePackConfig {
  unit_price_cents_per_minute: number
  min_minutes: number
  packs: { minutes: number, discount_bps: number }[]
  notice: string
  daily_cap_minutes: number
}
export interface PurchaseConfig {
  payment_methods: PaymentMethod[]
  recharge_presets_cents: number[]
  kinds: Record<LicenseKind, KindConfig>
  runtime_pack: RuntimePackConfig
}

/** POST /billing/quote — 服务端权威计价 */
export interface QuoteReq2 {
  biz_type: BizType
  quantity?: number
  duration_value?: number
  minutes?: number
}
export interface QuoteResult2 {
  quantity: number
  duration_value: number
  unit_price_cents: number
  billing_units: number
  original_cents: number
  qty_discount_bps: number
  duration_discount_bps: number
  payable_cents: number
  /** 席位新购/续费赠送的临时开机时长（分钟）；其它资源为 0 */
  gift_runtime_minutes: number
}

/** 授权单元上占用的实例摘要（后端填充；空闲单元为 null） */
export interface LicenseUnitInstance {
  cp_id: string
  name: string
  status: string
}
/** GET /billing/license-units — 续费 tab 用（后端 LicenseUnitView） */
export interface LicenseUnit {
  id: number
  kind: LicenseKind
  created_at: string
  expire_at: string
  /** 当前占用该单元的实例 cpId；空串表示空闲 */
  current_instance_id: string
  /** 占用实例摘要（名称/状态/cpId）；空闲单元为 null */
  instance?: LicenseUnitInstance | null
}

/** Order（新形状，契约 §1.5） */
export interface Order2 {
  id: number
  biz_type: BizType
  status: OrderStatus2
  total_cents: number
  /** 实收手续费（分）；历史订单为 0 */
  fee_cents: number
  /** 下单时配置快照：比例手续费基点 */
  fee_percent_bps: number
  /** 下单时配置快照：固定手续费（分） */
  fee_fixed_cents: number
  pay_method: string
  created_at: string
  paid_at: string | null
  expired_at: string | null
  /** 履约时实际赠送的临时开机时长（分钟，仅席位新购/续费 > 0） */
  gift_runtime_minutes: number
  /** 订单历史列表随单返回的订单项明细（派生摘要 + 行展开） */
  items?: OrderItem2[]
}
export interface OrderItem2 {
  id: number
  order_id: number
  target_kind: string
  quantity: number
  duration_value: number
  duration_unit: string
  unit_price_cents: number
  qty_discount_bps: number
  duration_discount_bps: number
  amount_cents: number
}
/** GET /billing/orders/:id — 后端返回扁平订单字段 + items（非 { order, items } 包裹） */
export interface OrderDetail2 extends Order2 { items: OrderItem2[] }

/** POST /billing/orders 请求体 */
export interface OrderCreateReq2 {
  biz_type: BizType
  quantity?: number
  duration_value?: number
  unit_ids?: number[]
  minutes?: number
  amount_cents?: number
  pay_method: string
  /** 已注册业务类型（如 library 套餐 lib_*）的不透明请求载荷；内置类型不传。 */
  params?: Record<string, unknown>
}
export interface PayInfo { status: OrderStatus2, [k: string]: unknown }
export interface OrderCreateResult { order: Order2, pay: PayInfo }

/** GET /billing/runtime/log — 聚合到开机会话的费用记录 */
export interface RuntimeLogSegment {
  quota_type: QuotaType
  minutes: number
  from: string
  to: string
  /** 该段计费的具体原因（后端落账时生成）；旧数据可能为空，前端回退到按类型的通用说明。 */
  reason?: string
}
export interface RuntimeLogEntry {
  cp_id: string
  instance_name: string
  power_on_at: string
  power_off_at: string | null
  quota_type: QuotaType
  temp_minutes_charged: number
  running: boolean
  segments: RuntimeLogSegment[]
}
export interface RuntimeLogResult {
  items: RuntimeLogEntry[]
  total: number
  daily_cap_minutes: number
}

// 试用
export interface TrialPolicyItem {
  id: number
  policy_id: number
  subject: string
  quantity: number
  expire_days: number // 0 = 永久
}
export interface TrialPolicy {
  id: number
  code: string
  name: string
  enabled: boolean
  per_user_limit: number
  allow_new_user: boolean
  invite_code: string // 前台恒为空(后端抹除)
  items: TrialPolicyItem[]
}
export interface ClaimableItem {
  policy: TrialPolicy
  claimable: boolean
  need_invite: boolean
  claimed_count: number
  reason: string
}

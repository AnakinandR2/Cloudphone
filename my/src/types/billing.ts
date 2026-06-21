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

// SKU + 折扣阶梯
// 订单（旧列表形状，BillingOrdersView 仍在用）
export interface Order {
  id: number
  order_no: string
  user_id: number
  status: string // pending/paid/cancelled
  pay_method: string // balance/wechat/alipay
  total_cents: number
  paid_at: string | null
  created_at: string
  updated_at: string
}

// 权益/资源
export interface CapacitySnapshot { instance_seat: number, boot_seat: number, runtime_minute: number }
export interface EntitlementBatch {
  id: number
  user_id: number
  subject: string
  quantity: number
  used: number
  expire_at: string | null
  source: string
  source_ref: string
  created_at: string
  updated_at: string
}
export interface EntitlementsResult { capacities: CapacitySnapshot, batches: EntitlementBatch[] }

/** 时长费用量切片（一次结算一用户） */
export interface RuntimeUsageSlice {
  id: number
  window_start: string
  window_end: string
  billable_unit_minutes: number
  covered_seat_minutes: number
  charged_pack_minutes: number
  charged_balance_cents: number
  unfunded_minutes: number
  unit_price_cents: number
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
  qty_options: number[]
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
  pay_method: string
  created_at: string
  paid_at: string | null
  expired_at: string | null
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
}
export interface PayInfo { status: OrderStatus2, [k: string]: unknown }
export interface OrderCreateResult { order: Order2, pay: PayInfo }

/** GET /billing/runtime/log — 聚合到开机会话的费用记录 */
export interface RuntimeLogSegment {
  quota_type: QuotaType
  minutes: number
  from: string
  to: string
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

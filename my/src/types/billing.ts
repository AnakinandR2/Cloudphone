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
  type: string    // topup/consume/adjust_grant/adjust_deduct/purchase/trial
  delta: number
  balance_after: number
  reason: string
  order_id: number
  operator: string
  created_at: string
}

// SKU + 折扣阶梯
export interface DiscountTier {
  id: number
  sku_id: number
  cycle_months: number
  min_quantity: number
  discount_bps: number
}
export interface Sku {
  id: number
  code: string
  category: string // instance_fee/boot_pack/time_pack
  name: string
  description: string
  unit_price_cents: number
  unit: string
  listed: boolean
  sort: number
}
export interface SkuWithTiers { sku: Sku, tiers: DiscountTier[] }

// 计价
export interface QuoteRequest { sku_code: string, cycle_months: number, quantity: number }
export interface QuoteResult {
  sku_code: string
  sku_name: string
  category: string
  cycle_months: number
  quantity: number
  unit_price_cents: number
  billing_units: number
  original_cents: number
  discount_bps: number
  payable_cents: number
}

// 订单
export interface OrderItem {
  id: number
  order_id: number
  sku_code: string
  sku_name: string
  category: string
  cycle_months: number
  quantity: number
  unit_price_cents: number
  discount_bps: number
  original_cents: number
  payable_cents: number
}
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
export interface OrderDetail { order: Order, items: OrderItem[] }
export interface OrderItemReq { sku_code: string, cycle_months: number, quantity: number }
export interface OrderCreateReq { items: OrderItemReq[], pay_method: string }

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

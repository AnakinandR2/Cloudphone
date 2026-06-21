export interface LedgerEntry { id: number, user_id: number, subject: string, type: string, delta: number, balance_after: number, reason: string, order_id: number, operator: string, created_at: string }
export interface Account { id: number, user_id: number, balance_cents: number, created_at: string, updated_at: string }
export interface CapacitySnapshot { instance_seat: number, boot_seat: number, runtime_minute: number }
/** 新模型容量快照（按 V2 科目命名：seat/boot_slot/runtime_minute）。 */
export interface CapacitySnapshotV2 { seat: number, boot_slot: number, runtime_minute: number }
export interface AccountView { account: Account, ledger: LedgerEntry[], ledger_total: number, capacities: CapacitySnapshot, capacities_v2: CapacitySnapshotV2 }

export interface TrialPolicyItem { id: number, policy_id: number, subject: string, quantity: number, expire_days: number }
export interface TrialPolicyItemInput { subject: string, quantity: number, expire_days: number }
export interface TrialPolicy { id: number, code: string, name: string, enabled: boolean, per_user_limit: number, allow_new_user: boolean, invite_code: string, items: TrialPolicyItem[], created_at: string, updated_at: string }
export interface TrialPolicyCreate { code: string, name: string, items: TrialPolicyItemInput[], per_user_limit?: number, allow_new_user?: boolean, invite_code?: string, enabled?: boolean }
export interface TrialPolicyUpdate { name?: string, items?: TrialPolicyItemInput[], per_user_limit?: number, allow_new_user?: boolean, invite_code?: string, enabled?: boolean }
export interface TrialGrant { id: number, claim_id: number, policy_id: number, user_id: number, subject: string, quantity: number, created_at: string }

// ───────────────────────────────────────────────────────────────────────────
// 购买与费用重构（2026-06-21 接口契约 §2 后台端点）
// 金额单位均为 cents（分，int64）；bps 折扣：10000=全价无折扣，8500=8.5折。
// kind ∈ { seat | boot_slot }。
// ───────────────────────────────────────────────────────────────────────────

export type BillingKind = 'seat' | 'boot_slot'

/** 数量阶梯：达到 min_quantity 起享 discount_bps */
export interface QtyTier { min_quantity: number, discount_bps: number }
/** 时长选项：value=月数(席位)/天数(包月数)，对应 discount_bps */
export interface DurationOption { value: number, discount_bps: number }

/** 按 kind 的定价配置（后端 KindPricing，无 kind 字段） */
export interface KindPricing {
  unit_price_cents: number
  unit_label: string
  qty_options: number[]
  qty_tiers: QtyTier[]
  duration_unit: 'month' | 'day'
  duration_options: DurationOption[]
  notice: string
  billing_note: string
}
/** GET/PUT /admin/billing/pricing 整体形状：{ kinds: { seat, boot_slot } } */
export interface PricingConfig {
  kinds: Record<BillingKind, KindPricing>
}

/** 临时时长包预设 */
export interface RuntimePack { minutes: number, discount_bps: number }
/**
 * GET/PUT /admin/billing/runtime-config（后端 RuntimePackCfg，扁平）。
 * notice 由「须知文案」端点维护，这里透传以免 PUT 覆盖时被清空。
 */
export interface RuntimeBillingConfig {
  unit_price_cents_per_minute: number
  packs: RuntimePack[]
  min_minutes: number
  daily_cap_minutes: number
  recycle_retention_days: number
  notice?: string
}

/** 支付方式（GET/PUT /admin/billing/payment-methods 的单项） */
export interface PaymentMethod { code: string, name: string, enabled: boolean, sort: number }
/** GET/PUT /admin/billing/payment-methods 整体形状：{ payment_methods } */
export interface PaymentMethodsResp { payment_methods: PaymentMethod[] }

/** POST /admin/billing/accounts/:userId/adjust-resource（V2 统一履约） */
export interface AdjustResourceBody {
  subject: 'seat' | 'boot_slot' | 'runtime_minute'
  quantity?: number
  duration_value?: number
  minutes?: number
  reason: string
}

/** GET/PUT /admin/billing/recharge-presets */
export interface RechargePresets { presets_cents: number[] }

/** runtime_pack 的须知文案 */
export interface RuntimeNotice { notice: string }
/** GET/PUT /admin/billing/notices（各 kind 的 notice/billing_note + runtime_pack notice） */
export interface NoticesConfig {
  seat: { notice: string, billing_note: string }
  boot_slot: { notice: string, billing_note: string }
  runtime_pack: RuntimeNotice
}

/** 新订单形状（契约 §1.5）。status ∈ { unpaid | paid | expired } */
export interface BillingOrder {
  id: number
  user_id: number
  biz_type: string
  status: string
  total_cents: number
  pay_method: string
  created_at: string
  paid_at: string | null
  expired_at: string | null
}
export interface BillingOrderItem {
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
  renew_unit_ids: number[] | null
}
export interface BillingOrderDetail extends BillingOrder { items: BillingOrderItem[] }

export interface Sku { id: number, code: string, category: string, name: string, description: string, unit_price_cents: number, unit: string, listed: boolean, sort: number }

/** 时长费配置（单行） */
export interface RuntimeConfig { unit_price_cents_per_minute: number, low_balance_alert_cents: number }
export interface DiscountTier { id: number, sku_id: number, cycle_months: number, min_quantity: number, discount_bps: number }
export interface SkuCreate { code: string, category: string, name: string, description?: string, unit_price_cents: number, unit?: string, listed?: boolean, sort?: number }
export interface SkuUpdate { name?: string, description?: string, unit_price_cents?: number, unit?: string, listed?: boolean, sort?: number }
export interface TierCreate { cycle_months: number, min_quantity: number, discount_bps: number }
export interface TierUpdate { cycle_months?: number, min_quantity?: number, discount_bps?: number }

export interface Order { id: number, order_no: string, user_id: number, status: string, pay_method: string, total_cents: number, paid_at: string | null, created_at: string, updated_at: string }
export interface OrderItem { id: number, order_id: number, sku_code: string, sku_name: string, category: string, cycle_months: number, quantity: number, unit_price_cents: number, discount_bps: number, original_cents: number, payable_cents: number }
export interface OrderDetail { order: Order, items: OrderItem[] }

export interface LedgerEntry { id: number, user_id: number, subject: string, type: string, delta: number, balance_after: number, reason: string, order_id: number, operator: string, created_at: string }
export interface Account { id: number, user_id: number, balance_cents: number, created_at: string, updated_at: string }
export interface CapacitySnapshot { instance_seat: number, boot_seat: number, runtime_minute: number }
export interface AccountView { account: Account, ledger: LedgerEntry[], ledger_total: number, capacities: CapacitySnapshot }

export interface TrialPolicy { id: number, code: string, name: string, enabled: boolean, grant_subject: string, grant_quantity: number, grant_expire_days: number, per_user_limit: number, allow_new_user: boolean, invite_code: string, created_at: string, updated_at: string }
export interface TrialPolicyCreate { code: string, name: string, grant_subject: string, grant_quantity: number, grant_expire_days?: number, per_user_limit?: number, allow_new_user?: boolean, invite_code?: string, enabled?: boolean }
export interface TrialPolicyUpdate { name?: string, grant_quantity?: number, grant_expire_days?: number, per_user_limit?: number, allow_new_user?: boolean, invite_code?: string, enabled?: boolean }
export interface TrialGrant { id: number, policy_id: number, user_id: number, subject: string, quantity: number, created_at: string }

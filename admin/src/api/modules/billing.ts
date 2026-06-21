import type {
  Account,
  AccountView,
  AdjustResourceBody,
  BillingOrder,
  BillingOrderDetail,
  DiscountTier,
  NoticesConfig,
  Order,
  OrderDetail,
  PaymentMethod,
  PaymentMethodsResp,
  PricingConfig,
  RechargePresets,
  RuntimeBillingConfig,
  RuntimeConfig,
  Sku,
  SkuCreate,
  SkuUpdate,
  TierCreate,
  TierUpdate,
  TrialGrant,
  TrialPolicy,
  TrialPolicyCreate,
  TrialPolicyUpdate,
} from '@/types/billing'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

export default {
  // 定价
  listSkus: () => api.get<unknown, R<Sku[]>>('admin/billing/skus'),
  createSku: (d: SkuCreate) => api.post<unknown, R<Sku>>('admin/billing/skus', d),
  updateSku: (id: number, d: SkuUpdate) => api.put<unknown, R<Sku>>(`admin/billing/skus/${id}`, d),
  deleteSku: (id: number) => api.delete<unknown, R<null>>(`admin/billing/skus/${id}`),
  listTiers: (skuId: number) => api.get<unknown, R<DiscountTier[]>>(`admin/billing/skus/${skuId}/tiers`),
  createTier: (skuId: number, d: TierCreate) => api.post<unknown, R<DiscountTier>>(`admin/billing/skus/${skuId}/tiers`, d),
  updateTier: (tierId: number, d: TierUpdate) => api.put<unknown, R<DiscountTier>>(`admin/billing/tiers/${tierId}`, d),
  deleteTier: (tierId: number) => api.delete<unknown, R<null>>(`admin/billing/tiers/${tierId}`),

  // 时长费配置
  getRuntimeConfig: () => api.get<unknown, R<RuntimeConfig>>('admin/billing/runtime-config'),
  saveRuntimeConfig: (d: RuntimeConfig) => api.put<unknown, R<RuntimeConfig>>('admin/billing/runtime-config', d),

  // 订单
  orders: (params: { page?: number, size?: number, userId?: number, status?: string }) =>
    api.get<unknown, R<Page<Order>>>('admin/billing/orders', { params }),
  markPaid: (id: number) => api.post<unknown, R<OrderDetail>>(`admin/billing/orders/${id}/mark-paid`),

  // 账户/资源调整
  account: (userId: number, params?: { page?: number, size?: number }) =>
    api.get<unknown, R<AccountView>>(`admin/billing/accounts/${userId}`, { params }),
  adjustBalance: (userId: number, delta_cents: number, reason: string) =>
    api.post<unknown, R<Account>>(`admin/billing/accounts/${userId}/adjust`, { delta_cents, reason }),
  // 资源调整（V2 统一履约）：seat/boot_slot 需 quantity + duration_value；runtime_minute 用 minutes
  adjustResource: (userId: number, body: AdjustResourceBody) =>
    api.post<unknown, R<null>>(`admin/billing/accounts/${userId}/adjust-resource`, body),

  // 试用
  listTrials: () => api.get<unknown, R<TrialPolicy[]>>('admin/billing/trials'),
  createTrial: (d: TrialPolicyCreate) => api.post<unknown, R<TrialPolicy>>('admin/billing/trials', d),
  updateTrial: (id: number, d: TrialPolicyUpdate) => api.put<unknown, R<TrialPolicy>>(`admin/billing/trials/${id}`, d),
  deleteTrial: (id: number) => api.delete<unknown, R<null>>(`admin/billing/trials/${id}`),
  grantEligibility: (id: number, user_id: number) => api.post<unknown, R<null>>(`admin/billing/trials/${id}/eligibility`, { user_id }),
  trialGrants: (id: number) => api.get<unknown, R<TrialGrant[]>>(`admin/billing/trials/${id}/grants`),

  // ── 购买与费用重构 · 配置后台（契约 §2）─────────────────────────────────
  // 定价（按 kind：unit_price_cents、qty_tiers、duration_options、notice、billing_note）
  getPricing: () => api.get<unknown, R<PricingConfig>>('admin/billing/pricing'),
  savePricing: (d: PricingConfig) => api.put<unknown, R<PricingConfig>>('admin/billing/pricing', d),
  // 临时时长（per-minute 单价、时长包预设、手输最低值、200/天封顶值、回收站保留天数）
  getRuntimeBillingConfig: () => api.get<unknown, R<RuntimeBillingConfig>>('admin/billing/runtime-config'),
  saveRuntimeBillingConfig: (d: RuntimeBillingConfig) => api.put<unknown, R<RuntimeBillingConfig>>('admin/billing/runtime-config', d),
  // 支付方式（开关 + 排序）。后端 GET/PUT 均以 { payment_methods } 包裹。
  getPaymentMethods: () => api.get<unknown, R<PaymentMethodsResp>>('admin/billing/payment-methods'),
  savePaymentMethods: (payment_methods: PaymentMethod[]) => api.put<unknown, R<PaymentMethodsResp>>('admin/billing/payment-methods', { payment_methods }),
  // 充值预设（金额档位）
  getRechargePresets: () => api.get<unknown, R<RechargePresets>>('admin/billing/recharge-presets'),
  saveRechargePresets: (presets_cents: number[]) => api.put<unknown, R<RechargePresets>>('admin/billing/recharge-presets', { presets_cents }),
  // 须知文案（各 kind 的 notice/billing_note + runtime_pack notice）。GET/PUT 均为扁平 map。
  getNotices: () => api.get<unknown, R<NoticesConfig>>('admin/billing/notices'),
  saveNotices: (d: NoticesConfig) => api.put<unknown, R<unknown>>('admin/billing/notices', d),
  // 新模型订单（biz-orders，status ∈ unpaid/paid/expired）。列表与 mark-paid 均不含订单项。
  billingOrders: (params: { page?: number, size?: number, userId?: number, status?: string }) =>
    api.get<unknown, R<Page<BillingOrder>>>('admin/billing/biz-orders', { params }),
  billingMarkPaid: (id: number) => api.post<unknown, R<BillingOrder>>(`admin/billing/biz-orders/${id}/mark-paid`),
  // 订单详情（含 items，订单项明细）。后端返回扁平订单字段 + items。
  billingOrderDetail: (id: number) => api.get<unknown, R<BillingOrderDetail>>(`admin/billing/biz-orders/${id}`),
}

import type {
  BillingAccount,
  BillingOverview,
  ClaimableItem,
  LedgerEntry,
  LicenseKind,
  LicenseUnit,
  Order2,
  OrderCreateReq2,
  OrderCreateResult,
  OrderDetail2,
  PurchaseConfig,
  QuoteReq2,
  QuoteResult2,
  RuntimeLogResult,
} from '@/types/billing'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

export default {
  // 账户/资金
  account: () => api.get<unknown, R<BillingAccount>>('billing/account'),
  ledger: (params: { page?: number, size?: number, subject?: string, type?: string }) =>
    api.get<unknown, R<Page<LedgerEntry>>>('billing/ledger', { params }),
  topup: (amount_cents: number) => api.post<unknown, R<BillingAccount>>('billing/topup', { amount_cents }),

  // 试用
  trials: () => api.get<unknown, R<ClaimableItem[]>>('billing/trials'),
  claimTrial: (code: string, invite_code = '') =>
    api.post<unknown, R<null>>(`billing/trials/${code}/claim`, { invite_code }),

  // ===== 购买与费用重构（2026-06-21，契约 §1）=====
  // KPI 概览（余额/席位/包月数/临时时长一次拿齐）
  overview: () => api.get<unknown, R<BillingOverview>>('billing/overview'),
  // 购买配置（支付方式/充值预设/各 kind 档位与时长/须知/临时时长包）
  purchaseConfig: () => api.get<unknown, R<PurchaseConfig>>('billing/purchase-config'),
  // 服务端权威报价（前端不自算价）
  quote2: (req: QuoteReq2) => api.post<unknown, R<QuoteResult2>>('billing/quote', req),
  // 授权单元列表（续费 tab）。后端只返回 { items }（无 total）。
  licenseUnits: (params: { kind: LicenseKind, expiring_before?: string, keyword?: string }) =>
    api.get<unknown, R<{ items: LicenseUnit[] }>>('billing/license-units', { params }),
  // 创建订单（含 recharge / 新购 / 续费 / 时长包）
  createOrder2: (req: OrderCreateReq2) => api.post<unknown, R<OrderCreateResult>>('billing/orders', req),
  // 订单列表（新形状）。后端 OKWithPage 返回 { list, total }，每单随附 items + gift。
  // 支持按状态与创建时间区间（from/to，ISO 串）过滤。
  orders2: (params: { page?: number, size?: number, status?: string, from?: string, to?: string, biz_type?: string }) =>
    api.get<unknown, R<Page<Order2>>>('billing/orders', { params }),
  // 订单详情（含 items）
  orderDetail2: (id: number) => api.get<unknown, R<OrderDetail2>>(`billing/orders/${id}`),
  // 继续支付未支付订单
  payOrder2: (id: number) => api.post<unknown, R<OrderCreateResult>>(`billing/orders/${id}/pay`),
  // 费用日志（聚合到开机会话）
  runtimeLog: (params: { page?: number, size?: number, from?: string, to?: string }) =>
    api.get<unknown, R<RuntimeLogResult>>('billing/runtime/log', { params }),
}

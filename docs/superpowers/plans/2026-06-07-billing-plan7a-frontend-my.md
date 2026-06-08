# 计费系统 Phase 1 · 计划 7a：my（C 端）计费前端接真 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 my 前台的计费原型页面（购买/用量/订单）从静态数据改为对接真实后端 `/billing/*`，并补齐账户资源概览、费用日志、充值、试用领取，打通用户「充值→下单→支付→拿额度→领试用」可视化闭环。

**Architecture:** 复用 my 既有约定（Vue3 + shadcn-vue + `@/api` axios 封装 `api.get/post<unknown,R<T>>`、`@/types/*`、`@/mock/*` `defineFakeRoute`、`DataTable`、vue-i18n 双语、`pnpm build`=vue-tsc 必过）。后端 JSON 为 **snake_case**、**金额整数分**（前端展示 ÷100）。新增 `billing` API 模块 + 类型 + mock；视图保留原型 UI 结构，仅把静态数据换成 API 调用 + 加载/错误态。无后端改动。

**Tech Stack:** Vue 3 / TS / Vite / shadcn-vue / vue-i18n / vitest（逻辑函数）/ pnpm。分支 `feature/billing-frontend-my`。

> 后端契约（已就绪）：前台 `/billing` 组（user 鉴权）：`GET account`、`GET ledger?page&size&subject&type`、`POST topup{amount_cents}`、`GET skus`、`POST quote{sku_code,cycle_months,quantity}`、`POST orders{items:[{sku_code,cycle_months,quantity}],pay_method}`、`GET orders?page&size&status`、`GET orders/:id`、`POST orders/:id/pay`、`GET entitlements`、`GET trials`、`POST trials/:code/claim{invite_code}`。响应 `{code,message,data}` code===0 成功；列表用 `{list,total}`。

---

## 计划 7a 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `my/src/types/billing.ts` | 全部 billing DTO 类型(snake_case 对齐后端) | 创建 |
| `my/src/api/modules/billing.ts` | billing API 客户端 | 创建 |
| `my/src/mock/billing.ts` | VITE_APP_MOCK 假数据(内存账户/SKU/订单/试用) | 创建 |
| `my/src/utils/money.ts` | 分→元格式化(`fmtCents`) + 折扣(`fmtDiscountBps`) | 创建 |
| `my/src/utils/money.test.ts` | money 工具单测(vitest) | 创建 |
| `my/src/views/billing/BillingPurchaseView.vue` | 购买：真 SKU/计价/下单/支付 + 账户资源概览 | 重写 |
| `my/src/views/billing/BillingOrdersView.vue` | 订单列表(真) | 重写 |
| `my/src/views/billing/BillingLedgerView.vue` | 费用日志(流水) | 创建 |
| `my/src/views/billing/BillingTrialsView.vue` | 可领试用 + 领取 | 创建 |
| `my/src/views/billing/BillingUsageView.vue` | 账户/资源概览(P1)；用量明细标注 P2 | 重写 |
| `my/src/router/routes.ts` | 增 ledger/trials 路由 | 修改 |
| `my/src/locales/{zh-CN,en}.ts` | 扩充 billing.* 文案 | 修改 |

**约定速查**：`api` 默认实例 `@/api`(`import api from '@/api'` 或 `'../index'`)；mock `ok(data)/fail(msg)`；金额分→元 `(cents/100).toFixed(2)`；折扣 bps：`8500`=8.5 折=付 85%；DataTable 列 `computed(()=>[...])`；i18n 同时改 zh-CN+en；**提交前 `pnpm build` 必过**（用独立 `pnpm`，见 my/CLAUDE.md 网络注意）。

---

## Task 1：billing API 客户端 + 类型 + mock + money 工具

**Files:** Create `types/billing.ts`, `api/modules/billing.ts`, `mock/billing.ts`, `utils/money.ts`, `utils/money.test.ts`。

- [ ] **Step 1：`utils/money.ts` + 单测（TDD）**

`utils/money.test.ts`：
```ts
import { describe, expect, it } from 'vitest'
import { fmtCents, fmtDiscountBps } from './money'

describe('money', () => {
  it('分→元两位小数', () => {
    expect(fmtCents(0)).toBe('0.00')
    expect(fmtCents(3000)).toBe('30.00')
    expect(fmtCents(12650)).toBe('126.50')
  })
  it('折扣 bps→折', () => {
    expect(fmtDiscountBps(10000)).toBe('') // 无折扣不显示
    expect(fmtDiscountBps(8500)).toBe('8.5折')
    expect(fmtDiscountBps(7000)).toBe('7折')
  })
})
```
`utils/money.ts`：
```ts
// 整数分 → 元（两位小数，千分位）
export function fmtCents(cents: number): string {
  return (cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

// 折扣基点(10000=全价) → 「8.5折」；全价返回空串
export function fmtDiscountBps(bps: number): string {
  if (bps >= 10000) return ''
  const zhe = +(bps / 1000).toFixed(1) // 8500 → 8.5
  return Number.isInteger(zhe) ? `${zhe}折` : `${zhe}折`
}
```

Run: `cd /home/root/workspace005/gloryphone-code/my && ./node_modules/.bin/vitest run src/utils/money.test.ts` → PASS。

- [ ] **Step 2：`types/billing.ts`**

```ts
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
}
export interface EntitlementsResult { capacities: CapacitySnapshot, batches: EntitlementBatch[] }

// 试用
export interface TrialPolicy {
  id: number
  code: string
  name: string
  enabled: boolean
  grant_subject: string
  grant_quantity: number
  grant_expire_days: number
  per_user_limit: number
  allow_new_user: boolean
  invite_code: string // 前台恒为空(后端抹除)
}
export interface ClaimableItem {
  policy: TrialPolicy
  claimable: boolean
  need_invite: boolean
  claimed_count: number
  reason: string
}
```

- [ ] **Step 3：`api/modules/billing.ts`**

```ts
import type {
  BillingAccount, ClaimableItem, EntitlementsResult, LedgerEntry,
  Order, OrderCreateReq, OrderDetail, QuoteRequest, QuoteResult, SkuWithTiers,
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

  // 目录/计价
  skus: () => api.get<unknown, R<SkuWithTiers[]>>('billing/skus'),
  quote: (req: QuoteRequest) => api.post<unknown, R<QuoteResult>>('billing/quote', req),

  // 订单
  createOrder: (req: OrderCreateReq) => api.post<unknown, R<OrderDetail>>('billing/orders', req),
  orders: (params: { page?: number, size?: number, status?: string }) =>
    api.get<unknown, R<Page<Order>>>('billing/orders', { params }),
  orderDetail: (id: number) => api.get<unknown, R<OrderDetail>>(`billing/orders/${id}`),
  payOrder: (id: number) => api.post<unknown, R<OrderDetail>>(`billing/orders/${id}/pay`),

  // 权益
  entitlements: () => api.get<unknown, R<EntitlementsResult>>('billing/entitlements'),

  // 试用
  trials: () => api.get<unknown, R<ClaimableItem[]>>('billing/trials'),
  claimTrial: (code: string, invite_code = '') =>
    api.post<unknown, R<null>>(`billing/trials/${code}/claim`, { invite_code }),
}
```

- [ ] **Step 4：`mock/billing.ts`（VITE_APP_MOCK 用，内存态）**

实现 `defineFakeRoute` 数组，覆盖上述所有端点，路径前缀 `/v1/billing/...`（参考 `mock/note.ts`：`ok/fail/now` 助手 + `defineFakeRoute([{ url, method, response }])`）。要点：
- 内存账户 `{ balance_cents: 1000000 }`、SKU 三类(instance_fee 3000/台月、boot_pack 2000、time_pack 20/小时)各带阶梯(月/季/年 10000/8500/7000；时长 ≥500=9000 ≥1000=8000)、容量 `{instance_seat:5,boot_seat:2,runtime_minute:6000}`、订单数组、试用两条(newbie 新用户可领 / promo 需邀请码)。
- `quote`：按后端规则算 `payable_cents=round(unit*units*bps/10000)`，返回完整 QuoteResult。
- `createOrder`：逐项 quote 求和→push pending 订单→返回 OrderDetail。`payOrder`：置 paid + 扣余额(简单)。`topup`：加余额。`claimTrial`：返回 ok（mock 不强校验）。
- 列表端点返回 `{list,total}`。
- 仅作 UI 联调；不必完全复刻后端校验。

Run: `cd /home/root/workspace005/gloryphone-code/my && pnpm build` → vue-tsc 通过（类型自洽）。

- [ ] **Step 5：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/my && ./node_modules/.bin/vitest run src/utils/money.test.ts && pnpm build
cd /home/root/workspace005/gloryphone-code
git add my/src/types/billing.ts my/src/api/modules/billing.ts my/src/mock/billing.ts my/src/utils/money.ts my/src/utils/money.test.ts
git commit -m "feat(my): billing API 客户端+类型+mock+money 工具

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> 注：`pnpm build` 在本任务可能因「视图尚引用旧静态逻辑」而仍通过（新文件独立）；真正的视图改造在 Task 2-4。若 build 因未使用的新文件无报错即可。mock 注册：检查 `vite.config.ts`/main 是否自动 glob 加载 `src/mock/*.ts`（参考既有 mock 如何被加载——通常 vite-plugin-fake-server 配置了 `include: 'src/mock'`，新增文件自动生效）。

---

## Task 2：BillingPurchaseView 接真（SKU/计价/下单/支付 + 账户资源概览）

**Files:** Modify `views/billing/BillingPurchaseView.vue`; `locales/{zh-CN,en}.ts`（补键）。

- [ ] **Step 1：读现有原型**，保留两套布局(标准/收银台)与交互骨架，替换数据源：
  - `onMounted` 调 `billingApi.skus()` 载入三类 SKU + 阶梯；`billingApi.account()` + `billingApi.entitlements()` 载入顶部概览（余额、实例席位/开机席位/时长 容量）。
  - 周期/数量/时长选择变化时（防抖）调 `billingApi.quote({sku_code,cycle_months,quantity})` 取 `payable_cents` 实时显示（替换原前端静态计算）。或本地按已载入阶梯算、下单时以后端 quote 为准。**推荐**：本地用阶梯即时显示 + 下单走后端（后端权威）。
  - 「立即购买/结算」→ `billingApi.createOrder({items, pay_method})` → 若 `pay_method==='balance'` 紧接 `billingApi.payOrder(order.id)`；微信/支付宝 → 暂提示「请在收银台扫码（开发中）」或调 createOrder 后展示待支付。成功 toast + 刷新概览。
  - 金额展示用 `fmtCents`，折扣用 `fmtDiscountBps`。加载态骨架/禁用按钮；错误 `toast.error`。
- [ ] **Step 2：补 i18n 键**（zh-CN + en，billing.* 命名空间）：账户概览、加载/错误、下单成功、待支付等新文案（复用已有 purchaseTitle/cycle_*/payMethod 等）。
- [ ] **Step 3：`cd my && pnpm build`** 通过；`VITE_APP_MOCK=true pnpm dev` 冒烟（手动确认渲染+下单走 mock 不报错——可由实现者描述，不强制截图）。
- [ ] **Step 4：提交** `git add my/src/views/billing/BillingPurchaseView.vue my/src/locales` + commit `feat(my): 购买页接真(SKU/计价/下单/支付+资源概览)`。

---

## Task 3：订单列表接真 + 费用日志页 + 充值

**Files:** Modify `views/billing/BillingOrdersView.vue`; Create `views/billing/BillingLedgerView.vue`; `router/routes.ts`; `locales/*`。充值入口可放购买页或账户概览的弹框。

- [ ] **Step 1：BillingOrdersView 接真** — `billingApi.orders({page,size,status})` 驱动 DataTable（保留筛选/趋势图可改为基于真实订单或暂隐藏趋势图标注 P2）；金额 `fmtCents`；状态/支付方式 i18n。行展开或详情可调 `orderDetail`。
- [ ] **Step 2：BillingLedgerView（费用日志）新页** — `billingApi.ledger({page,size,subject,type})` DataTable：科目/类型/时间筛选；`delta` 按科目显示单位（balance→元用 fmtCents，资源→台/分钟）；`balance_after` 同理。空态友好。
- [ ] **Step 3：充值** — 账户概览或订单页加「充值」按钮 → 弹框输入金额（元，转分）→ `billingApi.topup(cents)` → 刷新余额 + toast。
- [ ] **Step 4：router** 增 `/billing/ledger`（name `billingLedger`，菜单 `menu.billingLedger`，图标如 `ScrollText`/`ReceiptText`）。`Icon.vue` 注册新图标（若用新图标，按 my/CLAUDE.md 在 Icon.vue 同时 import+registry）。
- [ ] **Step 5：i18n + `pnpm build` + 提交** `feat(my): 订单接真+费用日志页+充值`。

---

## Task 4：账户/资源概览（用量页改造）+ 试用领取页 + 导航

**Files:** Modify `views/billing/BillingUsageView.vue`（→账户资源概览）; Create `views/billing/BillingTrialsView.vue`; `router/routes.ts`; `locales/*`。

- [ ] **Step 1：BillingUsageView 改造为「账户与资源」概览** — 调 `account()`+`entitlements()`：展示余额、三类资源容量、各资源批次明细(DataTable：科目/数量/已用/到期/来源)。原「用量图表/时长消耗明细」标注 **Phase 2（依赖中台开机时长）** 并隐藏或置灰说明（不显示伪造数据）。
- [ ] **Step 2：BillingTrialsView（试用）新页** — `billingApi.trials()` 列出可领项：卡片展示策略名/发放内容(科目+数量)/可领状态；`claimable` 显示「立即领取」，`need_invite` 显示邀请码输入框 + 领取；已达上限/不符显示 `reason` 置灰。领取 → `billingApi.claimTrial(code, inviteCode)` → toast + 刷新（成功后该项变已领/刷新资源概览）。
- [ ] **Step 3：router + 菜单** 增 `/billing/trials`（`billingTrials`，`menu.billingTrials`，图标 `Gift`）。更新分组注释（去掉「原型/暂无功能」字样）。
- [ ] **Step 4：i18n + `pnpm build` + 提交** `feat(my): 账户资源概览+试用领取页+导航`。

---

## 计划 7a 验收对照（PRD §11 my 端）

| 页面 | 覆盖 |
|---|---|
| 收银台/购买(真 SKU/计价/下单/余额支付) | Task 2 |
| 账户/资源概览(余额+三类容量+批次) | Task 2 概览 + Task 4 详情 |
| 订单(真) | Task 3 |
| 费用/账户流水日志 | Task 3 LedgerView |
| 充值 | Task 3 |
| 试用「可领/领取」 | Task 4 TrialsView |
| 用量明细 | Task 4 标注 Phase 2 隐藏(不伪造) |

> 微信/支付宝真实扫码支付为 Phase 1 桩（后端 MarkPaid 走 admin）；my 端余额支付走通，网关支付下单后提示待支付即可。admin 计费前端 = 计划 7b。

## Self-Review 记录
- **Spec 覆盖**：PRD §11 my 端全部页面；用量明细明确标注 P2 不伪造。
- **占位扫描**：mock 为联调假数据(预期)；视图改造为接真。无 TODO 残留。
- **类型一致**：types/billing.ts snake_case 对齐后端 JSON；API 客户端方法签名与后端端点一致；money 工具 fmtCents/fmtDiscountBps 全页复用；bps 语义(10000=全价)统一。
- **约定**：`pnpm build`(vue-tsc) 每任务必过；i18n 双语；金额分→元；DataTable/Icon/router 按 my 约定。
